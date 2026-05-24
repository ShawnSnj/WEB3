package model_test

import (
	"testing"
	"time"

	"github.com/shawn/jobhunttask/internal/model"
)

func TestNormalizeDate(t *testing.T) {
	t.Parallel()
	in := time.Date(2026, 5, 24, 15, 42, 7, 100, time.FixedZone("CST", 8*3600))
	got := model.NormalizeDate(in)

	wantY, wantM, wantD := in.UTC().Date()
	gotY, gotM, gotD := got.Date()
	if gotY != wantY || gotM != wantM || gotD != wantD {
		t.Errorf("date drift: %v vs %v-%v-%v", got, wantY, wantM, wantD)
	}
	if got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
		t.Errorf("not midnight: %v", got)
	}
	if got.Location() != time.UTC {
		t.Errorf("not UTC: %v", got.Location())
	}
}

func TestDailyReview_Validate(t *testing.T) {
	t.Parallel()
	base := func() *model.DailyReview {
		return &model.DailyReview{
			EnergyLevel:       5,
			ProductivityScore: 7,
			Blockers:          []string{"got rejected"},
			Wins:              []string{"sent 3 apps"},
		}
	}
	if err := base().Validate(); err != nil {
		t.Fatalf("base should be valid: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*model.DailyReview)
		want error
	}{
		{"energy too low", func(r *model.DailyReview) { r.EnergyLevel = -1 }, model.ErrInvalidEnergyLevel},
		{"energy too high", func(r *model.DailyReview) { r.EnergyLevel = 11 }, model.ErrInvalidEnergyLevel},
		{"productivity too low", func(r *model.DailyReview) { r.ProductivityScore = -1 }, model.ErrInvalidProductivity},
		{"productivity too high", func(r *model.DailyReview) { r.ProductivityScore = 11 }, model.ErrInvalidProductivity},
		{"blank blocker", func(r *model.DailyReview) { r.Blockers = []string{"x", "  "} }, model.ErrBlockerEmpty},
		{"blank win", func(r *model.DailyReview) { r.Wins = []string{""} }, model.ErrWinEmpty},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			r := base()
			c.mut(r)
			if err := r.Validate(); err != c.want {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}
