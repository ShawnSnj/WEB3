package scheduler_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/scheduler"
)

// fixedClock is a tiny test clock; scheduler tests need it only for the
// Stop assertion below.
type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time { return c.t }

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestScheduler_RegisterUnknownSpecFails(t *testing.T) {
	t.Parallel()
	s, err := scheduler.New(scheduler.Config{
		Enabled: true, TimeZone: "UTC", JobTimeout: time.Second,
	}, silentLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = s.Register("bad", "this is not cron", func(ctx context.Context) error { return nil })
	if err == nil {
		t.Error("expected parse error")
	}
}

func TestScheduler_EmptySpecIsNoop(t *testing.T) {
	t.Parallel()
	s, err := scheduler.New(scheduler.Config{
		Enabled: true, TimeZone: "UTC", JobTimeout: time.Second,
	}, silentLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := s.Register("disabled", "", func(ctx context.Context) error { return nil }); err != nil {
		t.Errorf("empty spec should be ok, got: %v", err)
	}
}

func TestScheduler_StartStopGraceful(t *testing.T) {
	t.Parallel()
	s, err := scheduler.New(scheduler.Config{
		Enabled: true, TimeZone: "UTC", JobTimeout: time.Second,
	}, silentLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Register a job that runs every second; we won't wait long enough for
	// it to actually fire, but ensures the cron is happy.
	if err := s.Register("tick", "* * * * *", func(ctx context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	s.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.Stop(ctx); err != nil {
		t.Errorf("Stop should be graceful, got %v", err)
	}
}

func TestScheduler_DisabledDoesNotStart(t *testing.T) {
	t.Parallel()
	s, err := scheduler.New(scheduler.Config{
		Enabled: false, TimeZone: "UTC", JobTimeout: time.Second,
	}, silentLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.Start() // must not panic; no-op
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = s.Stop(ctx) // safe to call even though never started
}

func TestScheduler_InvalidTimezoneFails(t *testing.T) {
	t.Parallel()
	_, err := scheduler.New(scheduler.Config{
		Enabled: true, TimeZone: "Atlantis/Lost",
	}, silentLogger())
	if err == nil {
		t.Error("expected error for invalid TZ")
	}
}
