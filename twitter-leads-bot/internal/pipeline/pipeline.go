// Package pipeline orchestrates the fetch → filter → persist loop.
//
// One Pipeline value owns the search/analyze cadence. Web handlers hold a
// reference to it for the "Run Search" button (synchronous, single keyword).
// Gemini runs only from the dashboard (“New suggestion”), not during ingest.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/shawn/twitter-leads-bot/internal/db"
	"github.com/shawn/twitter-leads-bot/internal/gemini"
	"github.com/shawn/twitter-leads-bot/internal/models"
	"github.com/shawn/twitter-leads-bot/internal/twitter"
)

// Config is the knob-set for one Pipeline instance. Zero values get sane
// defaults in New.
type Config struct {
	ManualOnly       bool          // when true, Run() does not schedule searches (use RunKeyword from UI/API)
	Interval         time.Duration // how often the background loop runs (ignored when ManualOnly)
	Workers          int           // concurrent keyword searches per tick
	KeywordStagger   time.Duration // delay before starting each keyword (index * stagger) to avoid bursting one API key
	PerKeywordMax    int           // tweets to ask for per search
	MinLikes         int           // filtering: minimum likes on a tweet
	MinTextLen       int           // filtering: minimum text length
	MaxRequestJitter time.Duration // max random delay before each worker call
}

func (c Config) withDefaults() Config {
	if !c.ManualOnly && c.Interval <= 0 {
		c.Interval = 5 * time.Minute
	}
	if c.Workers <= 0 {
		c.Workers = 1
	}
	if c.KeywordStagger <= 0 {
		c.KeywordStagger = 5 * time.Second
	}
	if c.PerKeywordMax <= 0 {
		c.PerKeywordMax = 20
	}
	if c.MinLikes <= 0 {
		c.MinLikes = 3
	}
	if c.MinTextLen <= 0 {
		c.MinTextLen = 40
	}
	if c.MaxRequestJitter <= 0 {
		c.MaxRequestJitter = 2 * time.Second
	}
	return c
}

type Pipeline struct {
	repo     db.Repository
	searcher twitter.Searcher
	analyzer gemini.Analyzer
	logger   *slog.Logger
	cfg      Config
}

func New(repo db.Repository, searcher twitter.Searcher, analyzer gemini.Analyzer, logger *slog.Logger, cfg Config) *Pipeline {
	return &Pipeline{
		repo:     repo,
		searcher: searcher,
		analyzer: analyzer,
		logger:   logger,
		cfg:      cfg.withDefaults(),
	}
}

// Run starts the background loop. Returns when ctx is cancelled.
//
// When cfg.ManualOnly is set, there is no ticker: searches run only via
// RunKeyword (e.g. the Keywords page "Run search" button).
func (p *Pipeline) Run(ctx context.Context) {
	if p.cfg.ManualOnly {
		p.logger.Info("pipeline manual-only (no background searches)")
		<-ctx.Done()
		p.logger.Info("pipeline stopped")
		return
	}

	t := time.NewTicker(p.cfg.Interval)
	defer t.Stop()

	p.logger.Info("pipeline started",
		slog.Duration("interval", p.cfg.Interval),
		slog.Int("workers", p.cfg.Workers),
		slog.Duration("keyword_stagger", p.cfg.KeywordStagger),
	)
	p.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("pipeline stopped")
			return
		case <-t.C:
			p.runOnce(ctx)
		}
	}
}

// runOnce processes every enabled keyword in parallel (bounded by Workers).
func (p *Pipeline) runOnce(ctx context.Context) {
	keywords, err := p.repo.ListKeywords(ctx, true)
	if err != nil {
		p.logger.Error("list keywords", slog.Any("err", err))
		return
	}
	if len(keywords) == 0 {
		p.logger.Debug("no enabled keywords; skipping tick")
		return
	}

	sem := make(chan struct{}, p.cfg.Workers)
	var wg sync.WaitGroup
	for i, k := range keywords {
		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
		}
		wg.Add(1)
		idx := i
		kw := k
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			// Space out keyword starts so a single RapidAPI key is not hit by
			// N concurrent searches (common cause of 429 + 1h self-inflicted cooldown).
			if d := time.Duration(idx) * p.cfg.KeywordStagger; d > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(d):
				}
			}
			p.jitter(ctx)
			if _, err := p.RunKeyword(ctx, kw.Keyword); err != nil && !errors.Is(err, context.Canceled) {
				p.logger.Warn("keyword run failed",
					slog.String("keyword", kw.Keyword),
					slog.Any("err", err),
				)
			}
		}()
	}
	wg.Wait()
}

// jitter sleeps for a random duration up to MaxRequestJitter, so multiple
// workers don't burst the same upstream simultaneously.
func (p *Pipeline) jitter(ctx context.Context) {
	if p.cfg.MaxRequestJitter <= 0 {
		return
	}
	d := time.Duration(rand.Int63n(int64(p.cfg.MaxRequestJitter)))
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// RunKeyword searches one keyword, filters, persists new tweets, and adds a
// placeholder analysis row for each (no LLM). Returns the number of *new*
// leads produced (existing tweets are skipped).
//
// Exposed so the dashboard "Run Search" button can trigger an immediate fetch
// for a single keyword without waiting for the next tick.
func (p *Pipeline) RunKeyword(ctx context.Context, keyword string) (int, error) {
	logger := p.logger.With(slog.String("keyword", keyword))
	logger.Info("searching")

	tweets, err := p.searcher.Search(ctx, keyword, p.cfg.PerKeywordMax)
	if err != nil {
		return 0, fmt.Errorf("search: %w", err)
	}
	if err := p.repo.TouchKeywordSearched(ctx, keyword); err != nil {
		logger.Warn("touch last_searched_at", slog.Any("err", err))
	}

	newLeads := 0
	for _, t := range tweets {
		if !p.acceptable(t) {
			continue
		}
		exists, err := p.repo.TweetExists(ctx, t.ID)
		if err != nil {
			logger.Warn("dedup check", slog.String("tweet_id", t.ID), slog.Any("err", err))
			continue
		}
		if exists {
			continue
		}

		created := t.CreatedAt
		if created.IsZero() {
			created = time.Now()
		}
		if err := p.repo.InsertTweet(ctx, &models.ProcessedTweet{
			TweetID:   t.ID,
			Keyword:   keyword,
			Username:  t.Author,
			Text:      t.Text,
			Likes:     t.Likes,
			CreatedAt: created,
		}); err != nil {
			logger.Warn("insert tweet", slog.String("tweet_id", t.ID), slog.Any("err", err))
			continue
		}

		// Placeholder only — Gemini runs when the user clicks “New suggestion”.
		if err := p.repo.UpsertAnalysis(ctx, &models.Analysis{
			TweetID:         t.ID,
			Score:           0,
			Reason:          `AI not run yet — click “New suggestion” to score and draft a reply.`,
			ReplySuggestion: "",
			Status:          models.StatusPending,
		}); err != nil {
			logger.Warn("upsert analysis", slog.String("tweet_id", t.ID), slog.Any("err", err))
			continue
		}
		newLeads++
	}

	logger.Info("done", slog.Int("new_leads", newLeads), slog.Int("scanned", len(tweets)))
	return newLeads, nil
}

// acceptable applies the spec's filtering rules:
//   - skip replies / retweets
//   - skip tweets under MinLikes
//   - skip tweets shorter than MinTextLen characters
func (p *Pipeline) acceptable(t twitter.Tweet) bool {
	if t.IsReply || t.IsRetweet {
		return false
	}
	if t.Likes < p.cfg.MinLikes {
		return false
	}
	if len(t.Text) < p.cfg.MinTextLen {
		return false
	}
	return true
}

// RunLeadAnalysis calls Gemini for score, reason, and reply and persists them.
// Triggered from the dashboard (“New suggestion”), not during ingest.
func (p *Pipeline) RunLeadAnalysis(ctx context.Context, tweetID, tweetText string) error {
	analysis, err := p.analyzer.Analyze(ctx, tweetText)
	if err != nil {
		return err
	}
	return p.repo.UpsertAnalysis(ctx, &models.Analysis{
		TweetID:         tweetID,
		Score:           analysis.Score,
		Reason:          analysis.Reason,
		ReplySuggestion: analysis.ReplySuggestion,
		Status:          models.StatusPending,
	})
}
