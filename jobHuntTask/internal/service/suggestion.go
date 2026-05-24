package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/suggestion"
)

// SuggestionService orchestrates the rule engine: it builds a snapshot
// from the metrics layer, runs the evaluator, and reconciles persisted
// suggestions with what the rules currently say.
//
// Reconciliation contract:
//   - For every rule that fires, Upsert() inserts a fresh row OR returns
//     the existing active row with the same (kind, ISO-week) dedup key.
//   - Active suggestions whose kind is NOT in the firing set are expired,
//     so the active list always reflects the live state.
//   - User-dismissed rows are NOT resurrected within the same ISO week.
type SuggestionService struct {
	repo        repository.SuggestionRepository
	metricsRepo repository.MetricsRepository
	metrics     *MetricsService
	evaluator   *suggestion.Evaluator
	clock       Clock
	highEffortMin int
}

// SuggestionServiceConfig is purely tuning; defaults via WithDefaults().
type SuggestionServiceConfig struct {
	// HighEffortMinutesPerTask must mirror the value in the
	// SmallerTasksRule so the SQL slicing matches the rule's threshold.
	HighEffortMinutesPerTask int
}

func (c SuggestionServiceConfig) withDefaults() SuggestionServiceConfig {
	if c.HighEffortMinutesPerTask == 0 {
		c.HighEffortMinutesPerTask = 45
	}
	return c
}

// NewSuggestionService constructs the service. A nil evaluator falls back
// to suggestion.NewEvaluator() with the default rule set.
func NewSuggestionService(
	repo repository.SuggestionRepository,
	metricsRepo repository.MetricsRepository,
	metrics *MetricsService,
	evaluator *suggestion.Evaluator,
	clock Clock,
	cfg SuggestionServiceConfig,
) *SuggestionService {
	if evaluator == nil {
		evaluator = suggestion.NewEvaluator()
	}
	if clock == nil {
		clock = SystemClock
	}
	cfg = cfg.withDefaults()
	return &SuggestionService{
		repo:          repo,
		metricsRepo:   metricsRepo,
		metrics:       metrics,
		evaluator:     evaluator,
		clock:         clock,
		highEffortMin: cfg.HighEffortMinutesPerTask,
	}
}

// ---------------------------------------------------------------------------
// Refresh — the main entry point
// ---------------------------------------------------------------------------

// RefreshResult summarises what changed during a Refresh call.
type RefreshResult struct {
	Created     []*model.Suggestion // newly inserted active rows
	Kept        []*model.Suggestion // already-active rows whose rule still fires
	ExpiredCount int                // rows transitioned to expired
}

// Refresh re-evaluates every rule and reconciles the database. Safe to
// call repeatedly; the dedup index makes it idempotent within an ISO week.
func (s *SuggestionService) Refresh(ctx context.Context) (*RefreshResult, error) {
	snap, err := s.buildSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("build snapshot: %w", err)
	}

	results := s.evaluator.Evaluate(snap)
	now := s.clock.Now()

	out := &RefreshResult{
		Created: make([]*model.Suggestion, 0, len(results)),
		Kept:    make([]*model.Suggestion, 0, len(results)),
	}
	firedKinds := make([]model.SuggestionKind, 0, len(results))

	for _, r := range results {
		sg := &model.Suggestion{
			Kind:        r.Kind,
			Severity:    r.Severity,
			Status:      model.SuggestionStatusActive,
			Title:       r.Title,
			Message:     r.Message,
			Payload:     r.Payload,
			DedupKey:    model.DedupKeyForWeek(r.Kind, now),
			GeneratedAt: now,
		}
		created, err := s.repo.Upsert(ctx, sg)
		if err != nil {
			return nil, fmt.Errorf("upsert %s: %w", r.Kind, err)
		}
		if created {
			out.Created = append(out.Created, sg)
		} else {
			out.Kept = append(out.Kept, sg)
		}
		firedKinds = append(firedKinds, r.Kind)
	}

	expired, err := s.repo.ExpireActiveExcept(ctx, firedKinds, now)
	if err != nil {
		return nil, fmt.Errorf("expire stale: %w", err)
	}
	out.ExpiredCount = expired
	return out, nil
}

// ---------------------------------------------------------------------------
// Snapshot assembly
// ---------------------------------------------------------------------------

func (s *SuggestionService) buildSnapshot(ctx context.Context) (suggestion.Snapshot, error) {
	now := s.clock.Now()
	today, err := s.metrics.Today(ctx)
	if err != nil {
		return suggestion.Snapshot{}, err
	}
	weekly, err := s.metrics.Weekly(ctx)
	if err != nil {
		return suggestion.Snapshot{}, err
	}
	streak, err := s.metrics.Streak(ctx)
	if err != nil {
		return suggestion.Snapshot{}, err
	}
	cats, err := s.metrics.Categories(ctx, time.Time{}, time.Time{})
	if err != nil {
		return suggestion.Snapshot{}, err
	}
	mm, err := s.metrics.MostMissed(ctx)
	if err != nil {
		return suggestion.Snapshot{}, err
	}
	from, to := weekly.From, weekly.To
	avgEst, large, total, err := s.metricsRepo.EffortDistribution(ctx, from, to, s.highEffortMin)
	if err != nil {
		return suggestion.Snapshot{}, err
	}
	var share float64
	if total > 0 {
		share = float64(large) / float64(total)
	}
	return suggestion.Snapshot{
		Now:                    now,
		Today:                  today,
		Weekly:                 weekly,
		Streak:                 streak,
		Categories:             cats,
		MostMissed:             mm,
		AvgEstimateMinutesWeek: avgEst,
		HighEffortShareWeek:    share,
	}, nil
}

// ---------------------------------------------------------------------------
// Reads / state transitions
// ---------------------------------------------------------------------------

func (s *SuggestionService) Get(ctx context.Context, id uuid.UUID) (*model.Suggestion, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SuggestionService) List(ctx context.Context, f repository.SuggestionFilter) ([]*model.Suggestion, error) {
	return s.repo.List(ctx, f)
}

// ListActive is the default UI feed: every active suggestion, most recent
// first.
func (s *SuggestionService) ListActive(ctx context.Context) ([]*model.Suggestion, error) {
	return s.repo.List(ctx, repository.SuggestionFilter{
		Statuses: []model.SuggestionStatus{model.SuggestionStatusActive},
	})
}

// Dismiss transitions an active suggestion to dismissed.
func (s *SuggestionService) Dismiss(ctx context.Context, id uuid.UUID) (*model.Suggestion, error) {
	sg, err := s.repo.Dismiss(ctx, id, s.clock.Now())
	if err != nil {
		if errors.Is(err, model.ErrSuggestionNotFound) {
			// Could be: row doesn't exist, OR row exists but isn't active.
			if existing, gerr := s.repo.GetByID(ctx, id); gerr == nil {
				if existing.Status != model.SuggestionStatusActive {
					return nil, model.ErrSuggestionInvalidTransition
				}
			}
		}
		return nil, err
	}
	return sg, nil
}

func (s *SuggestionService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
