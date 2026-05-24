package model_test

import (
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

func TestSuggestionKindIsValid(t *testing.T) {
	t.Parallel()
	for _, k := range []model.SuggestionKind{
		model.SuggestionReduceWorkload,
		model.SuggestionSmallerTasks,
		model.SuggestionEasierWins,
		model.SuggestionFocusShift,
	} {
		if !k.IsValid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if model.SuggestionKind("bogus").IsValid() {
		t.Error("bogus should be invalid")
	}
}

func TestSuggestionStatusTransitions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to model.SuggestionStatus
		ok       bool
	}{
		{model.SuggestionStatusActive, model.SuggestionStatusDismissed, true},
		{model.SuggestionStatusActive, model.SuggestionStatusExpired, true},
		{model.SuggestionStatusActive, model.SuggestionStatusActive, false},
		{model.SuggestionStatusDismissed, model.SuggestionStatusActive, false},
		{model.SuggestionStatusExpired, model.SuggestionStatusDismissed, false},
	}
	for _, tc := range cases {
		if got := tc.from.CanTransitionTo(tc.to); got != tc.ok {
			t.Errorf("%s -> %s: got %v, want %v", tc.from, tc.to, got, tc.ok)
		}
	}
}

func TestSuggestionValidate(t *testing.T) {
	t.Parallel()
	valid := &model.Suggestion{
		Kind:     model.SuggestionEasierWins,
		Severity: model.SeverityInfo,
		Status:   model.SuggestionStatusActive,
		Title:    "t",
		Message:  "m",
		DedupKey: "easier_wins:2026-W21",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid: %v", err)
	}

	cases := []struct {
		name string
		mut  func(s *model.Suggestion)
		want error
	}{
		{"bad_kind", func(s *model.Suggestion) { s.Kind = "x" }, model.ErrInvalidSuggestionKind},
		{"bad_status", func(s *model.Suggestion) { s.Status = "x" }, model.ErrInvalidSuggestionStatus},
		{"bad_severity", func(s *model.Suggestion) { s.Severity = "x" }, model.ErrInvalidSuggestionSeverity},
		{"empty_title", func(s *model.Suggestion) { s.Title = "" }, model.ErrSuggestionTitleEmpty},
		{"empty_msg", func(s *model.Suggestion) { s.Message = "" }, model.ErrSuggestionMessageEmpty},
		{"empty_key", func(s *model.Suggestion) { s.DedupKey = "" }, model.ErrSuggestionDedupKeyEmpty},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := *valid
			tc.mut(&s)
			if err := s.Validate(); err != tc.want {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDedupKeyForWeek(t *testing.T) {
	t.Parallel()
	// 2026-05-24 is Sunday; ISOWeek("2026-05-24") = 2026-W21
	got := model.DedupKeyForWeek(model.SuggestionEasierWins, time.Date(2026, 5, 24, 9, 0, 0, 0, time.UTC))
	want := "easier_wins:2026-W21"
	if got != want {
		t.Errorf("dedup = %q, want %q", got, want)
	}
}
