package service

import (
	"context"
	"log/slog"

	"github.com/shawn/jobhunttask/internal/model"
)

// Notifier delivers a Reminder via some channel. Implementations are
// expected to be idempotent — the reminder service guards duplicate
// dispatch, but transient retry is normal.
type Notifier interface {
	Notify(ctx context.Context, r *model.Reminder) error
}

// SlogNotifier writes a structured "reminder dispatched" log entry. It is
// the default implementation and the right tool for development and tests.
// Production callers should swap in a real channel (email, push, webhook).
type SlogNotifier struct {
	log *slog.Logger
}

// NewSlogNotifier returns a notifier that writes to the given logger.
// A nil logger falls back to slog.Default().
func NewSlogNotifier(log *slog.Logger) *SlogNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &SlogNotifier{log: log}
}

func (n *SlogNotifier) Notify(_ context.Context, r *model.Reminder) error {
	n.log.Info("reminder dispatched",
		slog.String("id", r.ID.String()),
		slog.String("kind", string(r.Kind)),
		slog.Time("scheduled_for", r.ScheduledFor),
		slog.Any("payload", r.Payload),
		slog.Int("attempts", r.Attempts+1),
	)
	return nil
}
