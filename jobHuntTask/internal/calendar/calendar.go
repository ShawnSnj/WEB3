// Package calendar defines calendar-day boundaries in a fixed IANA timezone.
// Task due dates, "today" filters, and dashboard metrics all use the same
// calendar so APP_TIMEZONE behaves consistently across the app.
package calendar

import (
	"fmt"
	"strings"
	"time"
)

// Calendar defines calendar-day boundaries in a fixed IANA timezone.
type Calendar struct {
	loc *time.Location
}

// Load parses an IANA timezone name (e.g. "Asia/Taipei", "UTC").
func Load(name string) (*Calendar, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "UTC"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", name, err)
	}
	return &Calendar{loc: loc}, nil
}

// UTC returns a calendar using UTC.
func UTC() *Calendar {
	return &Calendar{loc: time.UTC}
}

// Location returns the underlying IANA location.
func (c *Calendar) Location() *time.Location {
	return c.loc
}

// StartOfDay returns midnight at the start of t's calendar day in c's timezone.
func (c *Calendar) StartOfDay(t time.Time) time.Time {
	t = t.In(c.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, c.loc)
}

// NormalizeDate truncates t to midnight on its calendar day in c's timezone.
func (c *Calendar) NormalizeDate(t time.Time) time.Time {
	return c.StartOfDay(t)
}

// ParseDate parses YYYY-MM-DD as midnight on that calendar day in c's timezone.
func (c *Calendar) ParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", s, c.loc)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ParseDateSlash parses M/D/YYYY as midnight in c's timezone (CSV import).
func (c *Calendar) ParseDateSlash(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("1/2/2006", s, c.loc)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// FormatDate formats t as YYYY-MM-DD in c's timezone.
func (c *Calendar) FormatDate(t time.Time) string {
	return t.In(c.loc).Format("2006-01-02")
}

// SameDay reports whether a and b fall on the same calendar day in c's timezone.
func (c *Calendar) SameDay(a, b time.Time) bool {
	ay, am, ad := a.In(c.loc).Date()
	by, bm, bd := b.In(c.loc).Date()
	return ay == by && am == bm && ad == bd
}

// RelativeDue returns a short label for a due date relative to now using
// calendar days in c's timezone (not a rolling 24-hour window).
func (c *Calendar) RelativeDue(due, now time.Time) string {
	dueDay := c.StartOfDay(due)
	today := c.StartOfDay(now)
	switch {
	case dueDay.Before(today):
		return "overdue"
	case dueDay.Equal(today):
		return "today"
	default:
		return "due " + due.In(c.loc).Format("Jan 2")
	}
}
