// Package gemini wraps the Google Gemini API for scoring tweets and drafting
// reply suggestions using the official google.golang.org/genai client.
package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	defaultModel    = "gemini-2.5-flash"
	defaultAttempts = 3
	defaultBaseDelay = 500 * time.Millisecond
	defaultAttemptTO = 20 * time.Second
)

// Analysis is the structured JSON shape returned by AnalyzeTweet.
type Analysis struct {
	Score           int    `json:"score"`
	Reason          string `json:"reason"`
	ReplySuggestion string `json:"replySuggestion"`
}

// Analyzer is the interface the pipeline and web layer depend on.
type Analyzer interface {
	Analyze(ctx context.Context, tweet string) (Analysis, error)
	GenerateReply(ctx context.Context, tweet string) (string, error)
}

// Client implements Analyzer using Gemini GenerateContent.
type Client struct {
	g           *genai.Client
	model       string
	maxAttempts int
	baseDelay   time.Duration
	attemptTO   time.Duration
}

// New builds a Client. model defaults to gemini-2.5-flash when empty.
func New(apiKey, model string) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: empty API key")
	}
	if model == "" {
		model = defaultModel
	}
	g, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: new client: %w", err)
	}
	return &Client{
		g:           g,
		model:       model,
		maxAttempts: defaultAttempts,
		baseDelay:   defaultBaseDelay,
		attemptTO:   defaultAttemptTO,
	}, nil
}

// Analyze implements Analyzer by delegating to AnalyzeTweet.
func (c *Client) Analyze(ctx context.Context, tweet string) (Analysis, error) {
	return c.AnalyzeTweet(ctx, tweet)
}

// GenerateReply implements Analyzer by delegating to GenerateReplySuggestion.
func (c *Client) GenerateReply(ctx context.Context, tweet string) (string, error) {
	return c.GenerateReplySuggestion(ctx, tweet)
}

// AnalyzeTweet scores a tweet 1–10 and returns score, reason, and replySuggestion as structured JSON.
func (c *Client) AnalyzeTweet(ctx context.Context, tweet string) (Analysis, error) {
	minS, maxS := 1.0, 10.0
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: analyzeSystem}}},
		Temperature:       genai.Ptr[float32](0.4),
		ResponseMIMEType:  "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"score":           {Type: genai.TypeInteger, Minimum: &minS, Maximum: &maxS},
				"reason":          {Type: genai.TypeString},
				"replySuggestion": {Type: genai.TypeString},
			},
			Required: []string{"score", "reason", "replySuggestion"},
		},
	}

	raw, err := c.withRetry(ctx, func(actx context.Context) (string, error) {
		resp, err := c.g.Models.GenerateContent(actx, c.model, genai.Text("Tweet:\n"+tweet), cfg)
		if err != nil {
			return "", err
		}
		out := strings.TrimSpace(resp.Text())
		if out == "" {
			return "", fmt.Errorf("gemini: empty model response")
		}
		return out, nil
	})
	if err != nil {
		return Analysis{}, err
	}
	return validateAnalysisJSON(raw)
}

// GenerateReplySuggestion drafts a fresh reply (plain text) for the regenerate flow.
func (c *Client) GenerateReplySuggestion(ctx context.Context, tweet string) (string, error) {
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: replySystem}}},
		Temperature:       genai.Ptr[float32](0.9),
	}
	prompt := "Tweet to reply to:\n" + tweet + "\n\nWrite the reply now."
	raw, err := c.withRetry(ctx, func(actx context.Context) (string, error) {
		resp, err := c.g.Models.GenerateContent(actx, c.model, genai.Text(prompt), cfg)
		if err != nil {
			return "", err
		}
		out := strings.TrimSpace(resp.Text())
		if out == "" {
			return "", fmt.Errorf("gemini: empty model response")
		}
		return out, nil
	})
	if err != nil {
		return "", err
	}
	return trimReply(raw), nil
}

func (c *Client) withRetry(parent context.Context, call func(context.Context) (string, error)) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(parent, c.attemptTO)
		s, err := call(attemptCtx)
		cancel()
		if err == nil {
			return s, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == c.maxAttempts {
			return "", err
		}
		delay := c.backoff(attempt)
		select {
		case <-parent.Done():
			return "", fmt.Errorf("gemini: %w (last error: %v)", parent.Err(), lastErr)
		case <-time.After(delay):
		}
	}
	return "", lastErr
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 429 || apiErr.Code >= 500
	}
	return true
}

func (c *Client) backoff(attempt int) time.Duration {
	d := c.baseDelay << (attempt - 1)
	jitter := time.Duration(rand.Int63n(int64(c.baseDelay)))
	return d + jitter
}

func validateAnalysisJSON(raw string) (Analysis, error) {
	var a Analysis
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return Analysis{}, fmt.Errorf("gemini: invalid json: %w", err)
	}
	a.Score = clampScore(a.Score)
	a.Reason = strings.TrimSpace(a.Reason)
	a.ReplySuggestion = trimReply(strings.TrimSpace(a.ReplySuggestion))
	if a.Reason == "" {
		return Analysis{}, fmt.Errorf("gemini: invalid analysis: empty reason")
	}
	if a.ReplySuggestion == "" {
		return Analysis{}, fmt.Errorf("gemini: invalid analysis: empty replySuggestion")
	}
	return a, nil
}

const analyzeSystem = `You evaluate tweets as potential leads for a crypto analytics product.

Score 1-10 how likely the author would be interested in any of:
  - smart money tracking
  - profitable wallet alerts
  - copy trading
  - prediction markets

Then draft a single reply tailored to the tweet. Reply rules (strict):
  - Sound human and intelligent.
  - Encourage curiosity and engagement, never close a sale.
  - No emojis. No hashtags. No URLs.
  - No hype, no spam, no direct selling.
  - Maximum 240 characters.

Respond with ONLY a JSON object matching the schema: score (integer 1-10), reason (one short sentence), replySuggestion (the reply text).`

const replySystem = `You draft a single Twitter/X reply tailored to a given tweet.

Rules (strict):
- Sound human and intelligent.
- Encourage curiosity and engagement, never close a sale.
- No emojis. No hashtags. No URLs.
- No hype, no spam, no direct selling.
- Maximum 240 characters.
- Output the reply text only — no quotes, no preamble, no markdown.`

func clampScore(s int) int {
	if s < 1 {
		return 1
	}
	if s > 10 {
		return 10
	}
	return s
}

func trimReply(s string) string {
	s = strings.TrimSpace(s)
	const max = 240
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, " "); i > max-40 {
		cut = cut[:i]
	}
	return cut + "…"
}
