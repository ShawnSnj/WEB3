// rapidapi.go — Searcher implementation backed by twitter-api45.p.rapidapi.com.
//
// We pick this provider because it has a free tier (~100 requests/day at the
// time of writing), uses a single GET endpoint (`/search.php`), and returns
// stable JSON. Any other RapidAPI scraper with a similar contract can be
// swapped in by changing the host + URL — the rest of the app only depends on
// the Searcher interface defined in client.go.
package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// QuotaError is returned when a RapidAPI provider says we're out of quota
// (or otherwise being throttled). Pool uses this to park the affected member
// for a cooldown period and try the next one.
type QuotaError struct {
	Status int
	Body   string
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("rapidapi quota/throttle (http %d): %s", e.Status, e.Body)
}

// IsQuotaError reports whether err originated from a quota / throttling
// response, so callers (especially Pool) can treat it as transient.
func IsQuotaError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*QuotaError)
	return ok
}

const (
	rapidAPIDefaultHost = "twitter-api45.p.rapidapi.com"
	rapidAPISearchPath  = "/search.php"
)

// RapidAPIClient is a Searcher backed by a RapidAPI Twitter scraper provider.
type RapidAPIClient struct {
	apiKey string
	host   string
	http   *http.Client
}

// NewRapidAPI constructs a RapidAPI-backed Searcher. Pass an empty host to use
// the default (twitter-api45.p.rapidapi.com).
func NewRapidAPI(apiKey, host string) *RapidAPIClient {
	if host == "" {
		host = rapidAPIDefaultHost
	}
	return &RapidAPIClient{
		apiKey: apiKey,
		host:   host,
		http:   &http.Client{Timeout: 20 * time.Second},
	}
}

// rapidAPISearchResp models twitter-api45's /search.php response. We only
// pull the fields we need; extras are ignored. Unknown shape variants for the
// author block are normalized in normalizeAuthor.
type rapidAPISearchResp struct {
	Timeline []rapidAPITweet `json:"timeline"`
}

type rapidAPITweet struct {
	TweetID            string          `json:"tweet_id"`
	ScreenName         string          `json:"screen_name"`
	Text               string          `json:"text"`
	Favorites          int             `json:"favorites"`
	CreatedAt          string          `json:"created_at"`
	InReplyToScreen    string          `json:"in_reply_to_screen_name,omitempty"`
	ConversationID     string          `json:"conversation_id_str,omitempty"`
	RetweetedStatus    json.RawMessage `json:"retweeted_status,omitempty"`
	UserInfo           json.RawMessage `json:"user_info"`
}

func (c *RapidAPIClient) Search(ctx context.Context, query string, max int) ([]Tweet, error) {
	if max <= 0 {
		max = 20
	}

	u := url.URL{
		Scheme: "https",
		Host:   c.host,
		Path:   rapidAPISearchPath,
	}
	q := u.Query()
	q.Set("query", query)
	// twitter-api45 supports "Top" or "Latest"; Latest is what we want for lead-gen.
	q.Set("search_type", "Latest")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-rapidapi-key", c.apiKey)
	req.Header.Set("x-rapidapi-host", c.host)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rapidapi request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if isQuotaResponse(resp.StatusCode, body) {
			return nil, &QuotaError{Status: resp.StatusCode, Body: snippet(body)}
		}
		return nil, fmt.Errorf("rapidapi http %d: %s", resp.StatusCode, snippet(body))
	}

	var parsed rapidAPISearchResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode rapidapi response: %w (body: %s)", err, snippet(body))
	}

	out := make([]Tweet, 0, len(parsed.Timeline))
	for _, t := range parsed.Timeline {
		if len(out) >= max {
			break
		}
		// Skip non-tweet entries (some providers mix in promoted content / cursors).
		if t.TweetID == "" || t.Text == "" {
			continue
		}
		author, authorID := normalizeAuthor(t.UserInfo, t.ScreenName)

		// Reply detection: prefer the structured field; fall back to text prefix.
		isReply := t.InReplyToScreen != "" || strings.HasPrefix(t.Text, "@")
		// Some providers expose conversation_id; if it differs from tweet id, the
		// tweet is part of a thread/reply.
		if !isReply && t.ConversationID != "" && t.ConversationID != t.TweetID {
			isReply = true
		}

		out = append(out, Tweet{
			ID:        t.TweetID,
			Author:    author,
			AuthorID:  authorID,
			Text:      t.Text,
			URL:       fmt.Sprintf("https://twitter.com/%s/status/%s", author, t.TweetID),
			Likes:     t.Favorites,
			CreatedAt: parseTwitterTime(t.CreatedAt),
			IsReply:   isReply,
			IsRetweet: len(t.RetweetedStatus) > 0 || strings.HasPrefix(t.Text, "RT @"),
		})
	}
	return out, nil
}

// parseTwitterTime accepts the two formats observed across RapidAPI providers:
// the legacy Twitter "Mon Jan 02 15:04:05 -0700 2006" string and ISO 8601.
// Returns zero time if neither parses; the pipeline tolerates that and falls
// back to NOW() when persisting.
func parseTwitterTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RubyDate, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// normalizeAuthor pulls a username + numeric id out of the user_info block.
// Different RapidAPI providers shape this differently (object vs nested), so
// we parse defensively into a map and fall back to the top-level screen_name.
func normalizeAuthor(raw json.RawMessage, fallback string) (username, id string) {
	username = fallback
	if len(raw) == 0 {
		return username, ""
	}
	var info map[string]any
	if err := json.Unmarshal(raw, &info); err != nil {
		return username, ""
	}
	if v, ok := info["screen_name"].(string); ok && v != "" {
		username = v
	}
	if v, ok := info["rest_id"].(string); ok && v != "" {
		id = v
	} else if v, ok := info["rest_id"].(float64); ok {
		id = strconv.FormatFloat(v, 'f', 0, 64)
	}
	return username, id
}

// isQuotaResponse classifies an HTTP error as a quota/throttle situation.
// RapidAPI's gateway uses 429 for "monthly quota exceeded" and "rate limit",
// and providers themselves sometimes return 403 with a message body.
func isQuotaResponse(status int, body []byte) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	if status == http.StatusForbidden {
		s := strings.ToLower(string(body))
		return strings.Contains(s, "quota") ||
			strings.Contains(s, "rate limit") ||
			strings.Contains(s, "exceeded")
	}
	return false
}

func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}
