package model_test

import (
	"testing"

	"github.com/shawn/jobhunttask/internal/model"
)

func TestCountsRate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		c    model.Counts
		want float64
	}{
		{"empty", model.Counts{N: 0, Total: 0}, 0},
		{"none", model.Counts{N: 0, Total: 10}, 0},
		{"half", model.Counts{N: 5, Total: 10}, 0.5},
		{"all", model.Counts{N: 10, Total: 10}, 1.0},
		{"clamp_negative", model.Counts{N: -1, Total: 4}, 0},
		{"clamp_over", model.Counts{N: 5, Total: 4}, 1.0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.c.Rate(); got != tc.want {
				t.Fatalf("Rate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatusBreakdownTotal(t *testing.T) {
	t.Parallel()
	b := model.StatusBreakdown{Pending: 1, InProgress: 2, Completed: 3, Missed: 4}
	if got, want := b.Total(), 10; got != want {
		t.Fatalf("Total = %d, want %d", got, want)
	}
}
