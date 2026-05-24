// Package suggestion houses the rule engine that turns metric snapshots
// into actionable recommendations.
//
// Design:
//   - Rules are pure functions of a Snapshot. They never touch the DB,
//     the clock, or the network. That makes them trivially unit-testable
//     and side-effect free.
//   - Every rule declares its own thresholds via a config struct. Defaults
//     are sensible for a typical solo job-search workload but every value
//     is overridable.
//   - The Evaluator composes a list of rules and returns whichever ones
//     fire for the given snapshot.
package suggestion

import (
	"fmt"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

// ---------------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------------

// Snapshot is the read-only view of the user's recent behavior that rules
// reason about. The service builds it from the MetricsService outputs.
type Snapshot struct {
	Now            time.Time
	Today          model.DailyStats
	Weekly         model.WeeklyStats
	Streak         model.Streak
	Categories     []model.CategoryStats
	MostMissed     *model.CategoryMissed
	// AvgEstimateMinutesWeek is the mean estimated_minutes across all tasks
	// CREATED in the weekly window (regardless of completion status).
	AvgEstimateMinutesWeek float64
	// HighEffortShareWeek is the fraction of weekly tasks whose estimated
	// minutes exceeded HighEffortMinutesPerTask (defined in RuleConfig).
	HighEffortShareWeek float64
}

// ---------------------------------------------------------------------------
// Result
// ---------------------------------------------------------------------------

// Result is what a Rule emits when it fires. The service materialises it
// into a model.Suggestion before persisting.
type Result struct {
	Kind     model.SuggestionKind
	Severity model.SuggestionSeverity
	Title    string
	Message  string
	Payload  map[string]any
}

// Rule is the abstraction every detector implements.
type Rule interface {
	Code() model.SuggestionKind
	Evaluate(s Snapshot) (Result, bool)
}

// ---------------------------------------------------------------------------
// Configurable thresholds
// ---------------------------------------------------------------------------

// RuleConfig collects every knob the rule set exposes. Zero values fall back
// to sensible defaults via WithDefaults.
type RuleConfig struct {
	// reduce_workload
	MissedRateWarn     float64 // default 0.30
	MissedRateCritical float64 // default 0.50
	MissedMinCount     int     // default 3 — avoid firing on tiny samples

	// smaller_tasks
	HighEffortAvgMinutes    float64 // default 60
	HighEffortMinutesPerTask int    // default 45  (a task >= this is "large")
	HighEffortShare         float64 // default 0.50 — fraction of large tasks
	HighEffortMinCount      int     // default 4

	// easier_wins
	LowStreakThreshold int // default 2 — streak < this fires

	// focus_shift
	LowROIRate      float64 // default 0.30
	LowROIMinTasks  int     // default 3 — per-category minimum
	LowROIMinCount  int     // default 2 — at least N categories must qualify
}

func (c RuleConfig) WithDefaults() RuleConfig {
	if c.MissedRateWarn == 0 {
		c.MissedRateWarn = 0.30
	}
	if c.MissedRateCritical == 0 {
		c.MissedRateCritical = 0.50
	}
	if c.MissedMinCount == 0 {
		c.MissedMinCount = 3
	}
	if c.HighEffortAvgMinutes == 0 {
		c.HighEffortAvgMinutes = 60
	}
	if c.HighEffortMinutesPerTask == 0 {
		c.HighEffortMinutesPerTask = 45
	}
	if c.HighEffortShare == 0 {
		c.HighEffortShare = 0.50
	}
	if c.HighEffortMinCount == 0 {
		c.HighEffortMinCount = 4
	}
	if c.LowStreakThreshold == 0 {
		c.LowStreakThreshold = 2
	}
	if c.LowROIRate == 0 {
		c.LowROIRate = 0.30
	}
	if c.LowROIMinTasks == 0 {
		c.LowROIMinTasks = 3
	}
	if c.LowROIMinCount == 0 {
		c.LowROIMinCount = 2
	}
	return c
}

// DefaultRules returns the canonical rule set with default thresholds.
func DefaultRules() []Rule {
	cfg := RuleConfig{}.WithDefaults()
	return []Rule{
		&ReduceWorkloadRule{Cfg: cfg},
		&SmallerTasksRule{Cfg: cfg},
		&EasierWinsRule{Cfg: cfg},
		&FocusShiftRule{Cfg: cfg},
	}
}

// ---------------------------------------------------------------------------
// reduce_workload — too many missed tasks
// ---------------------------------------------------------------------------

type ReduceWorkloadRule struct{ Cfg RuleConfig }

func (r *ReduceWorkloadRule) Code() model.SuggestionKind { return model.SuggestionReduceWorkload }

func (r *ReduceWorkloadRule) Evaluate(s Snapshot) (Result, bool) {
	cfg := r.Cfg.WithDefaults()
	total := s.Weekly.Breakdown.Total()
	missed := s.Weekly.Breakdown.Missed
	if total == 0 || missed < cfg.MissedMinCount {
		return Result{}, false
	}
	rate := float64(missed) / float64(total)
	if rate < cfg.MissedRateWarn {
		return Result{}, false
	}
	sev := model.SeverityWarning
	if rate >= cfg.MissedRateCritical {
		sev = model.SeverityCritical
	}
	return Result{
		Kind:     model.SuggestionReduceWorkload,
		Severity: sev,
		Title:    "Reduce your weekly workload",
		Message: fmt.Sprintf(
			"%d of %d tasks were missed this week (%.0f%%). Consider planning fewer tasks per day or shortening estimates.",
			missed, total, rate*100,
		),
		Payload: map[string]any{
			"missed_count": missed,
			"total":        total,
			"missed_rate":  rate,
		},
	}, true
}

// ---------------------------------------------------------------------------
// smaller_tasks — average effort is too high
// ---------------------------------------------------------------------------

type SmallerTasksRule struct{ Cfg RuleConfig }

func (r *SmallerTasksRule) Code() model.SuggestionKind { return model.SuggestionSmallerTasks }

func (r *SmallerTasksRule) Evaluate(s Snapshot) (Result, bool) {
	cfg := r.Cfg.WithDefaults()
	total := s.Weekly.Breakdown.Total()
	if total < cfg.HighEffortMinCount {
		return Result{}, false
	}
	if s.AvgEstimateMinutesWeek < cfg.HighEffortAvgMinutes {
		return Result{}, false
	}
	if s.HighEffortShareWeek < cfg.HighEffortShare {
		return Result{}, false
	}
	// Escalate when a high-effort week ALSO had a high miss rate.
	sev := model.SeverityWarning
	if total > 0 {
		mr := float64(s.Weekly.Breakdown.Missed) / float64(total)
		if mr >= cfg.MissedRateWarn {
			sev = model.SeverityCritical
		}
	}
	return Result{
		Kind:     model.SuggestionSmallerTasks,
		Severity: sev,
		Title:    "Break your tasks into smaller chunks",
		Message: fmt.Sprintf(
			"Your average estimate this week is %.0f minutes, and %.0f%% of tasks are %d+ minutes. Smaller chunks ship more reliably.",
			s.AvgEstimateMinutesWeek,
			s.HighEffortShareWeek*100,
			cfg.HighEffortMinutesPerTask,
		),
		Payload: map[string]any{
			"avg_estimate_minutes": s.AvgEstimateMinutesWeek,
			"high_effort_share":    s.HighEffortShareWeek,
			"large_threshold":      cfg.HighEffortMinutesPerTask,
		},
	}, true
}

// ---------------------------------------------------------------------------
// easier_wins — low streak
// ---------------------------------------------------------------------------

type EasierWinsRule struct{ Cfg RuleConfig }

func (r *EasierWinsRule) Code() model.SuggestionKind { return model.SuggestionEasierWins }

func (r *EasierWinsRule) Evaluate(s Snapshot) (Result, bool) {
	cfg := r.Cfg.WithDefaults()
	if s.Streak.CurrentStreak >= cfg.LowStreakThreshold {
		return Result{}, false
	}
	// If today already has completions, downgrade to info — a streak is
	// forming, the user just hasn't built it yet.
	sev := model.SeverityWarning
	if s.Streak.TodayCompletedCount > 0 {
		sev = model.SeverityInfo
	}
	return Result{
		Kind:     model.SuggestionEasierWins,
		Severity: sev,
		Title:    "Schedule an easy win",
		Message: fmt.Sprintf(
			"Your current streak is %d day(s). Add one short, low-effort task today to start rebuilding momentum.",
			s.Streak.CurrentStreak,
		),
		Payload: map[string]any{
			"current_streak":  s.Streak.CurrentStreak,
			"today_completed": s.Streak.TodayCompletedCount,
		},
	}, true
}

// ---------------------------------------------------------------------------
// focus_shift — multiple low-ROI categories
// ---------------------------------------------------------------------------

type FocusShiftRule struct{ Cfg RuleConfig }

func (r *FocusShiftRule) Code() model.SuggestionKind { return model.SuggestionFocusShift }

func (r *FocusShiftRule) Evaluate(s Snapshot) (Result, bool) {
	cfg := r.Cfg.WithDefaults()
	type laggard struct {
		Category       model.Category
		Total          int
		CompletionRate float64
	}
	laggards := make([]laggard, 0, 4)
	totalLowTasks := 0
	for _, c := range s.Categories {
		if c.Total < cfg.LowROIMinTasks {
			continue
		}
		if c.CompletionRate >= cfg.LowROIRate {
			continue
		}
		laggards = append(laggards, laggard{c.Category, c.Total, c.CompletionRate})
		totalLowTasks += c.Total
	}
	if len(laggards) < cfg.LowROIMinCount {
		return Result{}, false
	}
	sev := model.SeverityWarning
	if totalLowTasks >= 5 {
		sev = model.SeverityCritical
	}

	// Build a stable, human-readable category list for the payload.
	names := make([]string, 0, len(laggards))
	for _, l := range laggards {
		names = append(names, string(l.Category))
	}

	return Result{
		Kind:     model.SuggestionFocusShift,
		Severity: sev,
		Title:    "Shift focus away from low-ROI categories",
		Message: fmt.Sprintf(
			"%d categories had completion rates below %.0f%% this week (%d tasks total). Pause low-yield work and double down on what's converting.",
			len(laggards), cfg.LowROIRate*100, totalLowTasks,
		),
		Payload: map[string]any{
			"low_categories":   names,
			"total_low_tasks":  totalLowTasks,
			"threshold_rate":   cfg.LowROIRate,
		},
	}, true
}
