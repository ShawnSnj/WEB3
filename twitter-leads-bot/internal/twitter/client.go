// Package twitter is a thin client around the Twitter/X v2 recent-search endpoint.
//
// We avoid pulling in a heavy SDK because we only need a single endpoint and
// a stable, mockable interface for the rest of the app.
package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const recentSearchURL = "https://api.twitter.com/2/tweets/search/recent"

type Tweet struct {
	ID        string
	Author    string
	AuthorID  string
	Text      string
	URL       string
	Likes     int
	IsReply   bool
	IsRetweet bool
	CreatedAt time.Time
}

type Searcher interface {
	Search(ctx context.Context, query string, max int) ([]Tweet, error)
}

type Client struct {
	bearer string
	http   *http.Client
}

func New(bearerToken string) *Client {
	return &Client{
		bearer: bearerToken,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

type apiResponse struct {
	Data     []apiTweet `json:"data"`
	Includes struct {
		Users []apiUser `json:"users"`
	} `json:"includes"`
	Errors []struct {
		Message string `json:"message"`
		Title   string `json:"title"`
	} `json:"errors"`
}

type apiTweet struct {
	ID            string         `json:"id"`
	Text          string         `json:"text"`
	AuthorID      string         `json:"author_id"`
	CreatedAt     string         `json:"created_at"`
	PublicMetrics *publicMetrics `json:"public_metrics,omitempty"`
}

type publicMetrics struct {
	LikeCount int `json:"like_count"`
}

type apiUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func (c *Client) Search(ctx context.Context, query string, max int) ([]Tweet, error) {
	if max < 10 {
		max = 10
	}
	if max > 100 {
		max = 100
	}

	q := url.Values{}
	q.Set("query", query+" -is:retweet lang:en")
	q.Set("max_results", strconv.Itoa(max))
	q.Set("tweet.fields", "author_id,created_at,public_metrics")
	q.Set("expansions", "author_id")
	q.Set("user.fields", "username")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, recentSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.bearer)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitter request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter http %d", resp.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode twitter response: %w", err)
	}
	if len(body.Errors) > 0 {
		return nil, fmt.Errorf("twitter api error: %s", body.Errors[0].Message)
	}

	users := make(map[string]string, len(body.Includes.Users))
	for _, u := range body.Includes.Users {
		users[u.ID] = u.Username
	}

	out := make([]Tweet, 0, len(body.Data))
	for _, t := range body.Data {
		username := users[t.AuthorID]
		if username == "" {
			username = "unknown"
		}
		likes := 0
		if t.PublicMetrics != nil {
			likes = t.PublicMetrics.LikeCount
		}
		createdAt, _ := time.Parse(time.RFC3339, t.CreatedAt)
		out = append(out, Tweet{
			ID:        t.ID,
			Author:    username,
			Text:      t.Text,
			URL:       fmt.Sprintf("https://twitter.com/%s/status/%s", username, t.ID),
			Likes:     likes,
			CreatedAt: createdAt,
			IsReply:   strings.HasPrefix(t.Text, "@"),
			IsRetweet: false, // v2 query already filters out retweets via -is:retweet
			AuthorID: t.AuthorID,
		})
	}
	return out, nil
}
