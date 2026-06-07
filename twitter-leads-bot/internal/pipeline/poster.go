// poster.go — abstraction over "send a reply tweet".
//
// Real send is intentionally not implemented in this MVP. Twitter's official
// v2 write endpoint requires user-level OAuth (not just the bearer token used
// for search), and the cookie-based scrapers are gated behind X's anti-bot
// stack (see cmd/scraper-test for the saga). Both are out of scope for the
// dashboard demo.
//
// LogPoster lets the rest of the app run end-to-end — the approve flow marks
// the lead as "sent" in the DB and writes the reply to structured logs, which
// is the right shape for hooking up a real Poster later.
package pipeline

import (
	"context"
	"log/slog"
)

type Poster interface {
	Reply(ctx context.Context, tweetID, text string) error
}

// LogPoster prints replies to slog instead of actually sending them. Replace
// it in main.go when you wire up a real client.
type LogPoster struct {
	Logger *slog.Logger
}

func (p *LogPoster) Reply(_ context.Context, tweetID, text string) error {
	p.Logger.Info("reply (stub send)",
		slog.String("tweet_id", tweetID),
		slog.String("text", text),
	)
	return nil
}
