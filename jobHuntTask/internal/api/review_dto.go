package api

import (
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// upsertReviewRequest is the JSON body for PUT /api/v1/reviews/:date and
// PUT /api/v1/reviews/today. Every field is optional so callers may PATCH
// individual values; absent fields preserve existing values on update.
type upsertReviewRequest struct {
	Reflection        *string   `json:"reflection"         binding:"omitempty,max=8000"`
	Blockers          *[]string `json:"blockers"           binding:"omitempty,dive,max=500"`
	Wins              *[]string `json:"wins"               binding:"omitempty,dive,max=500"`
	Distractions      *[]string `json:"distractions"       binding:"omitempty,dive,max=500"`
	Notes             *string   `json:"notes"              binding:"omitempty,max=8000"`
	EnergyLevel       *int      `json:"energy_level"       binding:"omitempty,gte=0,lte=10"`
	ProductivityScore *int      `json:"productivity_score" binding:"omitempty,gte=0,lte=10"`
}

func (r upsertReviewRequest) toInput(date time.Time) service.UpsertReviewInput {
	return service.UpsertReviewInput{
		Date:              date,
		Reflection:        r.Reflection,
		Blockers:          r.Blockers,
		Wins:              r.Wins,
		Distractions:      r.Distractions,
		Notes:             r.Notes,
		EnergyLevel:       r.EnergyLevel,
		ProductivityScore: r.ProductivityScore,
	}
}

// reviewDTO is the canonical wire representation.
type reviewDTO struct {
	ID                uuid.UUID `json:"id"`
	ReviewDate        string    `json:"review_date"` // YYYY-MM-DD
	Reflection        string    `json:"reflection"`
	Blockers          []string  `json:"blockers"`
	Wins              []string  `json:"wins"`
	Distractions      []string  `json:"distractions"`
	Notes             string    `json:"notes"`
	EnergyLevel       int       `json:"energy_level"`
	ProductivityScore int       `json:"productivity_score"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func reviewResponse(r *model.DailyReview) reviewDTO {
	blk := r.Blockers
	if blk == nil {
		blk = []string{}
	}
	wins := r.Wins
	if wins == nil {
		wins = []string{}
	}
	dist := r.Distractions
	if dist == nil {
		dist = []string{}
	}
	return reviewDTO{
		ID:                r.ID,
		ReviewDate:        r.ReviewDate.Format("2006-01-02"),
		Reflection:        r.Reflection,
		Blockers:          blk,
		Wins:              wins,
		Distractions:      dist,
		Notes:             r.Notes,
		EnergyLevel:       r.EnergyLevel,
		ProductivityScore: r.ProductivityScore,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

func reviewListResponse(items []*model.DailyReview) []reviewDTO {
	out := make([]reviewDTO, 0, len(items))
	for _, r := range items {
		out = append(out, reviewResponse(r))
	}
	return out
}
