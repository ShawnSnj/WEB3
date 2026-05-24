package model

import "time"

// Counts is the standard (n / total) pair used across many metrics.
// Rate() returns the ratio safely (0.0 when total == 0).
type Counts struct {
	N     int `json:"n"`
	Total int `json:"total"`
}

// Rate returns N / Total clamped to [0, 1]. Total = 0 yields 0.
func (c Counts) Rate() float64 {
	if c.Total <= 0 {
		return 0
	}
	r := float64(c.N) / float64(c.Total)
	switch {
	case r < 0:
		return 0
	case r > 1:
		return 1
	}
	return r
}

// StatusBreakdown is the count-per-status histogram for a date range.
type StatusBreakdown struct {
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
	Missed     int `json:"missed"`
}

// Total returns the sum across all statuses.
func (s StatusBreakdown) Total() int {
	return s.Pending + s.InProgress + s.Completed + s.Missed
}

// DailyStats covers the "today" view of the dashboard.
type DailyStats struct {
	Date              time.Time       `json:"date"`
	Breakdown         StatusBreakdown `json:"breakdown"`
	CompletionRate    float64         `json:"completion_rate"`
	OverdueCount      int             `json:"overdue_count"`
	AvgActualMinutes  float64         `json:"avg_actual_minutes"`
	CompletedTotal    int             `json:"completed_total"`
}

// WeeklyStats covers a 7-day rolling window. Per-day breakdown lets the UI
// render a sparkline.
type WeeklyStats struct {
	From              time.Time         `json:"from"`
	To                time.Time         `json:"to"`
	Breakdown         StatusBreakdown   `json:"breakdown"`
	CompletionRate    float64           `json:"completion_rate"`
	CarryOverRate     float64           `json:"carry_over_rate"`
	OverdueRate       float64           `json:"overdue_rate"`
	AvgActualMinutes  float64           `json:"avg_actual_minutes"`
	DailyCompletions  []DailyCompletion `json:"daily_completions"`
}

// DailyCompletion is one bar in the weekly sparkline.
type DailyCompletion struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}

// CategoryStats is one row in the per-category ROI table.
type CategoryStats struct {
	Category         Category `json:"category"`
	Total            int      `json:"total"`
	Completed        int      `json:"completed"`
	Missed           int      `json:"missed"`
	CompletionRate   float64  `json:"completion_rate"`
	AvgActualMinutes float64  `json:"avg_actual_minutes"`
	AvgEstimateMin   float64  `json:"avg_estimated_minutes"`
	TimeEfficiency   float64  `json:"time_efficiency"`  // est / actual
	TasksPerHour     float64  `json:"tasks_per_hour"`   // completed / total actual hours
}

// Trend compares one 7-day window to the previous one.
type Trend struct {
	CurrentFrom         time.Time `json:"current_from"`
	CurrentTo           time.Time `json:"current_to"`
	PreviousFrom        time.Time `json:"previous_from"`
	PreviousTo          time.Time `json:"previous_to"`
	CompletionRateNow   float64   `json:"completion_rate_now"`
	CompletionRatePrev  float64   `json:"completion_rate_prev"`
	CompletionRateDelta float64   `json:"completion_rate_delta"` // now - prev
	CompletedNow        int       `json:"completed_now"`
	CompletedPrev       int       `json:"completed_prev"`
	CompletedDelta      int       `json:"completed_delta"`
}

// Streak captures the current consecutive-day completion run plus
// historical context (longest run in the lookback window, count of
// missed days in the last week).
type Streak struct {
	CurrentStreak       int       `json:"current_streak"`
	LongestStreak       int       `json:"longest_streak"`
	MissedDayCount      int       `json:"missed_day_count"` // last 7 days, today excluded
	TodayCompletedCount int       `json:"today_completed_count"`
	LastCompletionDate  time.Time `json:"last_completion_date"`
}

// DashboardSummary is the top-of-app aggregate used by the UI.
type DashboardSummary struct {
	Today              DailyStats      `json:"today"`
	Weekly             WeeklyStats     `json:"weekly"`
	Trend              Trend           `json:"trend"`
	Streak             Streak          `json:"streak"`
	MostMissedCategory *CategoryMissed `json:"most_missed_category,omitempty"`
}

// CategoryMissed is the "worst offender" surfaced by the dashboard.
type CategoryMissed struct {
	Category Category `json:"category"`
	Count    int      `json:"count"`
}
