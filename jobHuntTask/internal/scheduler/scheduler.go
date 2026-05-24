// Package scheduler wraps robfig/cron with structured logging, panic
// recovery, per-job timeouts, and graceful shutdown. It owns the
// reminder/automation cron jobs of the job-hunt service.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// Config bundles the configurable knobs for the scheduler. Every cron
// spec is a standard 5-field cron expression interpreted in TimeZone.
type Config struct {
	TimeZone string // IANA name; empty -> UTC

	// Cron specs. Empty string disables that job.
	MorningReminderSpec       string
	EveningReviewSpec         string
	WeeklyReviewSpec          string
	OverdueScannerSpec        string
	AutoCarryOverSpec         string
	ReminderDispatcherSpec    string

	// JobTimeout caps how long any single invocation may run.
	JobTimeout time.Duration

	// Enabled is a master switch (false = scheduler never starts).
	Enabled bool
}

// DefaultConfig returns sensible defaults suitable for a personal job-hunt
// service running in UTC.
func DefaultConfig() Config {
	return Config{
		TimeZone:               "UTC",
		Enabled:                true,
		MorningReminderSpec:    "0 9 * * *",
		EveningReviewSpec:      "0 21 * * *",
		WeeklyReviewSpec:       "0 20 * * 0",
		OverdueScannerSpec:     "*/15 * * * *",
		AutoCarryOverSpec:      "5 0 * * *",
		ReminderDispatcherSpec: "* * * * *",
		JobTimeout:             2 * time.Minute,
	}
}

// JobFunc is the canonical scheduler-job signature. The provided context is
// already bounded by Config.JobTimeout.
type JobFunc func(ctx context.Context) error

// Scheduler is a thin lifecycle owner around *cron.Cron.
type Scheduler struct {
	cfg     Config
	cron    *cron.Cron
	log     *slog.Logger
	loc     *time.Location
}

// New constructs a Scheduler with the given config and logger. A nil logger
// falls back to slog.Default().
func New(cfg Config, log *slog.Logger) (*Scheduler, error) {
	if log == nil {
		log = slog.Default()
	}
	loc, err := loadLocation(cfg.TimeZone)
	if err != nil {
		return nil, err
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 2 * time.Minute
	}

	c := cron.New(
		cron.WithLocation(loc),
		cron.WithChain(
			cron.Recover(cronLogger{log: log}), // panic -> log instead of dying
		),
		cron.WithLogger(cronLogger{log: log}),
	)
	return &Scheduler{cfg: cfg, cron: c, log: log, loc: loc}, nil
}

// Register adds one cron job. `spec` is a 5-field cron expression; an
// empty string disables the job (returns nil without error). The job's
// context is bounded by Config.JobTimeout and is cancelled when the
// scheduler shuts down.
func (s *Scheduler) Register(name, spec string, fn JobFunc) error {
	if spec == "" {
		s.log.Info("scheduler: job disabled", slog.String("name", name))
		return nil
	}
	wrapped := func() {
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.JobTimeout)
		defer cancel()

		start := time.Now()
		s.log.Info("scheduler: job started", slog.String("name", name))
		err := fn(ctx)
		took := time.Since(start)
		if err != nil {
			s.log.Error("scheduler: job failed",
				slog.String("name", name),
				slog.Duration("took", took),
				slog.String("error", err.Error()),
			)
			return
		}
		s.log.Info("scheduler: job ok",
			slog.String("name", name),
			slog.Duration("took", took),
		)
	}
	if _, err := s.cron.AddFunc(spec, wrapped); err != nil {
		return fmt.Errorf("register %s (%q): %w", name, spec, err)
	}
	s.log.Info("scheduler: job registered",
		slog.String("name", name),
		slog.String("spec", spec),
		slog.String("tz", s.loc.String()),
	)
	return nil
}

// Start runs the cron scheduler in a background goroutine. Safe to call
// even when Config.Enabled is false (returns immediately).
func (s *Scheduler) Start() {
	if !s.cfg.Enabled {
		s.log.Info("scheduler: disabled by config, not starting")
		return
	}
	s.cron.Start()
	s.log.Info("scheduler: started", slog.String("tz", s.loc.String()))
}

// Stop signals the scheduler to stop accepting new ticks and waits for
// any in-flight job to finish — or until ctx is cancelled. This is the
// graceful shutdown path.
func (s *Scheduler) Stop(ctx context.Context) error {
	stopCtx := s.cron.Stop() // returns a context that closes when jobs finish
	s.log.Info("scheduler: stopping")

	select {
	case <-stopCtx.Done():
		s.log.Info("scheduler: stopped cleanly")
		return nil
	case <-ctx.Done():
		s.log.Warn("scheduler: shutdown timeout — jobs may still be running")
		return ctx.Err()
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func loadLocation(tz string) (*time.Location, error) {
	if tz == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", tz, err)
	}
	return loc, nil
}

// cronLogger adapts our slog logger to the robfig/cron v3 logger
// interface (Info / Error). Used by both WithLogger and the Recover chain.
type cronLogger struct{ log *slog.Logger }

func (c cronLogger) Info(msg string, keysAndValues ...any) {
	c.log.Info("cron: "+msg, keysAndValues...)
}
func (c cronLogger) Error(err error, msg string, keysAndValues ...any) {
	kv := append([]any{slog.String("error", err.Error())}, keysAndValues...)
	c.log.Error("cron: "+msg, kv...)
}
