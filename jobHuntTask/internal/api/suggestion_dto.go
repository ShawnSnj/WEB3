package api

import (
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
)

// suggestionDTO is the canonical wire representation. The domain model
// has no json tags by design; this keeps the wire shape stable when the
// internal fields evolve.
type suggestionDTO struct {
	ID           uuid.UUID      `json:"id"`
	Kind         string         `json:"kind"`
	Severity     string         `json:"severity"`
	Status       string         `json:"status"`
	Title        string         `json:"title"`
	Message      string         `json:"message"`
	Payload      map[string]any `json:"payload"`
	DedupKey     string         `json:"dedup_key"`
	GeneratedAt  time.Time      `json:"generated_at"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
	DismissedAt  *time.Time     `json:"dismissed_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func suggestionResponse(s *model.Suggestion) suggestionDTO {
	payload := s.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return suggestionDTO{
		ID:          s.ID,
		Kind:        string(s.Kind),
		Severity:    string(s.Severity),
		Status:      string(s.Status),
		Title:       s.Title,
		Message:     s.Message,
		Payload:     payload,
		DedupKey:    s.DedupKey,
		GeneratedAt: s.GeneratedAt,
		ExpiresAt:   s.ExpiresAt,
		DismissedAt: s.DismissedAt,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func suggestionListResponse(items []*model.Suggestion) []suggestionDTO {
	out := make([]suggestionDTO, 0, len(items))
	for _, s := range items {
		out = append(out, suggestionResponse(s))
	}
	return out
}
