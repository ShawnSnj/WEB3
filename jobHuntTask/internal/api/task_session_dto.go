package api

import (
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// finishSessionRequest is the body for POST /sessions/:id/stop and
// /sessions/:id/complete. All fields are optional.
type finishSessionRequest struct {
	Interruptions     *int    `json:"interruptions"      binding:"omitempty,gte=0,lte=1000"`
	CompletionQuality *int    `json:"completion_quality" binding:"omitempty,gte=0,lte=5"`
	Notes             *string `json:"notes"              binding:"omitempty,max=4000"`
}

func (r finishSessionRequest) toInput() service.FinishSessionInput {
	return service.FinishSessionInput{
		Interruptions:     r.Interruptions,
		CompletionQuality: r.CompletionQuality,
		Notes:             r.Notes,
	}
}

// sessionDTO is the canonical wire representation.
type sessionDTO struct {
	ID                 uuid.UUID  `json:"id"`
	TaskID             uuid.UUID  `json:"task_id"`
	Status             string     `json:"status"`
	StartedAt          time.Time  `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	PausedAt           *time.Time `json:"paused_at,omitempty"`
	TotalPausedSeconds int        `json:"total_paused_seconds"`
	EffectiveSeconds   int        `json:"effective_seconds"`
	EffectiveMinutes   int        `json:"effective_minutes"`
	Interruptions      int        `json:"interruptions"`
	CompletionQuality  int        `json:"completion_quality"`
	Notes              string     `json:"notes"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func sessionResponse(s *model.TaskSession, now time.Time) sessionDTO {
	return sessionDTO{
		ID:                 s.ID,
		TaskID:             s.TaskID,
		Status:             string(s.Status),
		StartedAt:          s.StartedAt,
		EndedAt:            s.EndedAt,
		PausedAt:           s.PausedAt,
		TotalPausedSeconds: s.TotalPausedSeconds,
		EffectiveSeconds:   s.EffectiveSeconds(now),
		EffectiveMinutes:   s.EffectiveMinutes(now),
		Interruptions:      s.Interruptions,
		CompletionQuality:  s.CompletionQuality,
		Notes:              s.Notes,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
}

func sessionListResponse(items []*model.TaskSession, now time.Time) []sessionDTO {
	out := make([]sessionDTO, 0, len(items))
	for _, s := range items {
		out = append(out, sessionResponse(s, now))
	}
	return out
}
