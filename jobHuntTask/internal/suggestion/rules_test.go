package suggestion_test

import (
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/suggestion"
)

func baseSnapshot() suggestion.Snapshot {
	return suggestion.Snapshot{
		Now: time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC),
		Today: model.DailyStats{
			Breakdown: model.StatusBreakdown{Completed: 2, Pending: 1},
		},
		Weekly: model.WeeklyStats{
			Breakdown: model.StatusBreakdown{Completed: 8, Missed: 0, Pending: 1, InProgress: 1},
		},
		Streak: model.Streak{CurrentStreak: 5, TodayCompletedCount: 2},
		Categories: []model.CategoryStats{
			{Category: model.CategoryJobApply, Total: 5, Completed: 4, CompletionRate: 0.8},
		},
		AvgEstimateMinutesWeek: 30,
		HighEffortShareWeek:    0.2,
	}
}

// ---------------------------------------------------------------------------
// reduce_workload
// ---------------------------------------------------------------------------

func TestRule_ReduceWorkload_Fires(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Weekly.Breakdown = model.StatusBreakdown{Completed: 4, Missed: 4, Pending: 2}
	r := &suggestion.ReduceWorkloadRule{}
	res, ok := r.Evaluate(s)
	if !ok {
		t.Fatal("expected rule to fire")
	}
	if res.Severity != model.SeverityWarning {
		t.Errorf("severity = %v, want warning", res.Severity)
	}
	if res.Payload["missed_count"].(int) != 4 {
		t.Errorf("payload missed_count = %v", res.Payload["missed_count"])
	}
}

func TestRule_ReduceWorkload_Critical(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Weekly.Breakdown = model.StatusBreakdown{Completed: 1, Missed: 6, Pending: 3}
	r := &suggestion.ReduceWorkloadRule{}
	res, ok := r.Evaluate(s)
	if !ok {
		t.Fatal("expected critical")
	}
	if res.Severity != model.SeverityCritical {
		t.Errorf("severity = %v, want critical", res.Severity)
	}
}

func TestRule_ReduceWorkload_BelowMinCount(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Weekly.Breakdown = model.StatusBreakdown{Completed: 1, Missed: 2} // missed count below default min (3)
	r := &suggestion.ReduceWorkloadRule{}
	if _, ok := r.Evaluate(s); ok {
		t.Fatal("expected no fire when missed < min count")
	}
}

func TestRule_ReduceWorkload_NoTasks(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Weekly.Breakdown = model.StatusBreakdown{}
	r := &suggestion.ReduceWorkloadRule{}
	if _, ok := r.Evaluate(s); ok {
		t.Fatal("expected no fire on empty week")
	}
}

// ---------------------------------------------------------------------------
// smaller_tasks
// ---------------------------------------------------------------------------

func TestRule_SmallerTasks_Fires(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Weekly.Breakdown = model.StatusBreakdown{Completed: 4, Missed: 0, Pending: 2}
	s.AvgEstimateMinutesWeek = 75
	s.HighEffortShareWeek = 0.6
	r := &suggestion.SmallerTasksRule{}
	res, ok := r.Evaluate(s)
	if !ok {
		t.Fatal("expected fire")
	}
	if res.Severity != model.SeverityWarning {
		t.Errorf("severity = %v, want warning", res.Severity)
	}
}

func TestRule_SmallerTasks_CriticalWithMissedRate(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Weekly.Breakdown = model.StatusBreakdown{Completed: 2, Missed: 4, Pending: 4}
	s.AvgEstimateMinutesWeek = 75
	s.HighEffortShareWeek = 0.6
	r := &suggestion.SmallerTasksRule{}
	res, ok := r.Evaluate(s)
	if !ok {
		t.Fatal("expected fire")
	}
	if res.Severity != model.SeverityCritical {
		t.Errorf("severity = %v, want critical (high effort + high missed)", res.Severity)
	}
}

func TestRule_SmallerTasks_NoFire_LowAvg(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Weekly.Breakdown = model.StatusBreakdown{Completed: 4, Pending: 4}
	s.AvgEstimateMinutesWeek = 20
	s.HighEffortShareWeek = 0.6
	r := &suggestion.SmallerTasksRule{}
	if _, ok := r.Evaluate(s); ok {
		t.Fatal("expected no fire when avg is low")
	}
}

// ---------------------------------------------------------------------------
// easier_wins
// ---------------------------------------------------------------------------

func TestRule_EasierWins_FiresWhenZero(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Streak = model.Streak{CurrentStreak: 0, TodayCompletedCount: 0}
	r := &suggestion.EasierWinsRule{}
	res, ok := r.Evaluate(s)
	if !ok {
		t.Fatal("expected fire")
	}
	if res.Severity != model.SeverityWarning {
		t.Errorf("severity = %v, want warning", res.Severity)
	}
}

func TestRule_EasierWins_InfoIfTodayProgress(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Streak = model.Streak{CurrentStreak: 1, TodayCompletedCount: 2}
	r := &suggestion.EasierWinsRule{}
	res, ok := r.Evaluate(s)
	if !ok {
		t.Fatal("expected fire (streak < 2)")
	}
	if res.Severity != model.SeverityInfo {
		t.Errorf("severity = %v, want info", res.Severity)
	}
}

func TestRule_EasierWins_NoFireWhenHealthy(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	r := &suggestion.EasierWinsRule{}
	if _, ok := r.Evaluate(s); ok {
		t.Fatal("expected no fire when streak is healthy")
	}
}

// ---------------------------------------------------------------------------
// focus_shift
// ---------------------------------------------------------------------------

func TestRule_FocusShift_Fires(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Categories = []model.CategoryStats{
		{Category: model.CategoryTwitter, Total: 4, Completed: 1, CompletionRate: 0.25},
		{Category: model.CategoryGithub, Total: 5, Completed: 1, CompletionRate: 0.20},
		{Category: model.CategoryJobApply, Total: 6, Completed: 5, CompletionRate: 0.83},
	}
	r := &suggestion.FocusShiftRule{}
	res, ok := r.Evaluate(s)
	if !ok {
		t.Fatal("expected fire")
	}
	if res.Severity != model.SeverityCritical {
		t.Errorf("severity = %v, want critical (9 low-yield tasks)", res.Severity)
	}
	cats := res.Payload["low_categories"].([]string)
	if len(cats) != 2 {
		t.Errorf("low_categories count = %d, want 2", len(cats))
	}
}

func TestRule_FocusShift_NoFire_FewLaggards(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Categories = []model.CategoryStats{
		{Category: model.CategoryTwitter, Total: 4, Completed: 1, CompletionRate: 0.25},
		{Category: model.CategoryJobApply, Total: 6, Completed: 5, CompletionRate: 0.83},
	}
	r := &suggestion.FocusShiftRule{}
	if _, ok := r.Evaluate(s); ok {
		t.Fatal("expected no fire with only 1 laggard (default min is 2)")
	}
}

func TestRule_FocusShift_IgnoresSmallSamples(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	s.Categories = []model.CategoryStats{
		{Category: model.CategoryTwitter, Total: 2, Completed: 0, CompletionRate: 0.0}, // total < 3
		{Category: model.CategoryGithub, Total: 2, Completed: 0, CompletionRate: 0.0},
	}
	r := &suggestion.FocusShiftRule{}
	if _, ok := r.Evaluate(s); ok {
		t.Fatal("expected no fire when laggards have too few tasks")
	}
}

// ---------------------------------------------------------------------------
// Evaluator composition
// ---------------------------------------------------------------------------

func TestEvaluator_AllFire(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	// crank everything into the alert zone
	s.Weekly.Breakdown = model.StatusBreakdown{Completed: 1, Missed: 6, Pending: 3}
	s.AvgEstimateMinutesWeek = 80
	s.HighEffortShareWeek = 0.7
	s.Streak = model.Streak{CurrentStreak: 0}
	s.Categories = []model.CategoryStats{
		{Category: model.CategoryTwitter, Total: 4, Completed: 0, CompletionRate: 0},
		{Category: model.CategoryGithub, Total: 4, Completed: 1, CompletionRate: 0.25},
	}

	ev := suggestion.NewEvaluator() // defaults
	results := ev.Evaluate(s)
	if len(results) != 4 {
		t.Fatalf("expected 4 rules to fire, got %d", len(results))
	}
}

func TestEvaluator_NoneFire(t *testing.T) {
	t.Parallel()
	s := baseSnapshot()
	ev := suggestion.NewEvaluator()
	results := ev.Evaluate(s)
	if len(results) != 0 {
		t.Fatalf("expected zero rules to fire, got %d", len(results))
	}
}
