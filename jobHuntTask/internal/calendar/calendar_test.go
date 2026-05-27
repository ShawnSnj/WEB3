package calendar_test

import (
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/calendar"
)

func TestCalendar_AsiaTaipeiTodayFilter(t *testing.T) {
	t.Parallel()

	cal, err := calendar.Load("Asia/Taipei")
	if err != nil {
		t.Fatal(err)
	}

	// Local May 26 06:00 Taipei; UTC still May 25 22:00.
	now := time.Date(2026, 5, 25, 22, 0, 0, 0, time.UTC)

	dueToday, ok := cal.ParseDate("2026-05-26")
	if !ok {
		t.Fatal("parse due today")
	}
	dueTomorrow, ok := cal.ParseDate("2026-05-27")
	if !ok {
		t.Fatal("parse due tomorrow")
	}

	todayEnd := cal.StartOfDay(now).Add(24 * time.Hour)
	upcomingFrom := cal.StartOfDay(now).Add(24 * time.Hour)

	if !dueToday.Before(todayEnd) {
		t.Errorf("due today should be included in today view (before %v), got %v", todayEnd, dueToday)
	}
	if !dueTomorrow.Before(todayEnd) {
		// tomorrow should NOT be before todayEnd
	} else {
		t.Errorf("due tomorrow should not be in today view")
	}
	if dueTomorrow.Before(upcomingFrom) {
		t.Errorf("due tomorrow should be >= upcoming lower bound %v", upcomingFrom)
	}
	if !dueToday.Before(upcomingFrom) {
		t.Errorf("due today should not appear in upcoming (due >= %v)", upcomingFrom)
	}
}

func TestCalendar_RelativeDueUsesCalendarDay(t *testing.T) {
	t.Parallel()

	cal, err := calendar.Load("Asia/Taipei")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 5, 25, 22, 0, 0, 0, time.UTC) // May 26 local
	dueToday, _ := cal.ParseDate("2026-05-26")
	dueTomorrow, _ := cal.ParseDate("2026-05-27")

	if got := cal.RelativeDue(dueToday, now); got != "today" {
		t.Errorf("RelativeDue(today) = %q, want today", got)
	}
	if got := cal.RelativeDue(dueTomorrow, now); got != "due May 27" {
		t.Errorf("RelativeDue(tomorrow) = %q, want due May 27", got)
	}
}

func TestCalendar_FormatDueDate(t *testing.T) {
	t.Parallel()

	cal, err := calendar.Load("Asia/Taipei")
	if err != nil {
		t.Fatal(err)
	}
	due, _ := cal.ParseDate("2026-05-26")
	if got := cal.FormatDueDate(due); got != "May 26, 2026" {
		t.Errorf("FormatDueDate = %q, want May 26, 2026", got)
	}
}

func TestCalendar_ParseDateMidnightInZone(t *testing.T) {
	t.Parallel()

	cal, err := calendar.Load("Asia/Taipei")
	if err != nil {
		t.Fatal(err)
	}

	got, ok := cal.ParseDate("2026-05-26")
	if !ok {
		t.Fatal("parse failed")
	}
	want := time.Date(2026, 5, 26, 0, 0, 0, 0, cal.Location())
	if !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
}
