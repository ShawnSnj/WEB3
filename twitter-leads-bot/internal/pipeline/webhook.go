package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// WebhookPoster POSTs JSON to TWITTER_REPLY_WEBHOOK_URL so you can bridge to
// the real Twitter/X API (n8n, Cloudflare Worker, custom service) without
// embedding OAuth in this repo.
//
// Body: {"tweet_id":"...","text":"..."}
type WebhookPoster struct {
	URL     string
	Secret  string // optional Authorization: Bearer <Secret>
	Log     *slog.Logger
	Client  *http.Client
	Fallback Poster // optional; called after successful HTTP 2xx to keep logs
}

func (p *WebhookPoster) Reply(ctx context.Context, tweetID, text string) error {
	if p.URL == "" {
		return fmt.Errorf("webhook poster: empty URL")
	}
	body, err := json.Marshal(map[string]string{"tweet_id": tweetID, "text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+p.Secret)
	}
	c := p.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook http %d: %s", resp.StatusCode, string(raw))
	}
	if p.Log != nil {
		p.Log.Info("reply webhook ok",
			slog.String("tweet_id", tweetID),
			slog.Int("status", resp.StatusCode),
		)
	}
	if p.Fallback != nil {
		_ = p.Fallback.Reply(ctx, tweetID, text)
	}
	return nil
}

// NewPoster picks WebhookPoster when url is non-empty, otherwise LogPoster.
func NewPoster(webhookURL, webhookSecret string, log *slog.Logger) Poster {
	base := &LogPoster{Logger: log.With(slog.String("channel", "reply"))}
	if webhookURL == "" {
		return base
	}
	return &WebhookPoster{
		URL:      webhookURL,
		Secret:   webhookSecret,
		Log:      log.With(slog.String("channel", "reply-webhook")),
		Fallback: base,
	}
}
