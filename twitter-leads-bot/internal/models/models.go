// Package models holds plain-data types shared across the db, pipeline, and
// web layers. Keeping them in their own package prevents an import cycle
// between db and pipeline (both produce/consume these shapes).
package models

import "time"

// Lead status values written into lead_analysis.status.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusSkipped  = "skipped"
	StatusSent     = "sent"
)

type Keyword struct {
	ID             int64
	Keyword        string
	Enabled        bool
	LastSearchedAt *time.Time
	CreatedAt      time.Time
	TweetCount     int // populated by repo for display only
}

type ProcessedTweet struct {
	TweetID   string
	Keyword   string
	Username  string
	Text      string
	Likes     int
	CreatedAt time.Time // tweet's own timestamp from the source
	SeenAt    time.Time // when the pipeline first stored it
}

type Analysis struct {
	TweetID         string
	Score           int    // 1-10
	Reason          string
	ReplySuggestion string
	Status          string // pending | approved | skipped | sent
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// LeadView is the joined row returned to the dashboard.
type LeadView struct {
	Tweet    ProcessedTweet
	Analysis Analysis
}

// URL returns the canonical twitter.com link for the lead's tweet.
func (l LeadView) URL() string {
	return "https://twitter.com/" + l.Tweet.Username + "/status/" + l.Tweet.TweetID
}
