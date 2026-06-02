package api

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

// ---------------------------------------------------------------------------
// Requests
// ---------------------------------------------------------------------------

// createTaskRequest is the JSON body for POST /api/v1/tasks.
type createTaskRequest struct {
	Title            string     `json:"title"             binding:"required,min=1,max=300"`
	Description      string     `json:"description"       binding:"max=4000"`
	Priority         string     `json:"priority"          binding:"omitempty,oneof=low medium high urgent"`
	Category         string     `json:"category"          binding:"omitempty,oneof=job_apply recruiter_outreach github twitter networking learning interview misc"`
	EstimatedMinutes int        `json:"estimated_minutes" binding:"gte=0,lte=1440"`
	DueDate          *time.Time `json:"due_date"          binding:"omitempty"`
}

func (r createTaskRequest) toInput() service.CreateTaskInput {
	return service.CreateTaskInput{
		Title:            strings.TrimSpace(r.Title),
		Description:      r.Description,
		Priority:         model.Priority(r.Priority),
		Category:         model.Category(r.Category),
		EstimatedMinutes: r.EstimatedMinutes,
		DueDate:          r.DueDate,
	}
}

// updateTaskRequest is the JSON body for PATCH /api/v1/tasks/:id. Every
// field is optional; absent fields preserve the existing value.
type updateTaskRequest struct {
	Title            *string    `json:"title"             binding:"omitempty,min=1,max=300"`
	Description      *string    `json:"description"       binding:"omitempty,max=4000"`
	Priority         *string    `json:"priority"          binding:"omitempty,oneof=low medium high urgent"`
	Category         *string    `json:"category"          binding:"omitempty,oneof=job_apply recruiter_outreach github twitter networking learning interview misc"`
	EstimatedMinutes *int       `json:"estimated_minutes" binding:"omitempty,gte=0,lte=1440"`
	DueDate          *time.Time `json:"due_date"          binding:"omitempty"`
	ClearDueDate     bool       `json:"clear_due_date"`
}

func (r updateTaskRequest) toInput() service.UpdateTaskInput {
	in := service.UpdateTaskInput{
		Title:            r.Title,
		Description:      r.Description,
		EstimatedMinutes: r.EstimatedMinutes,
		DueDate:          r.DueDate,
		ClearDueDate:     r.ClearDueDate,
	}
	if r.Priority != nil {
		p := model.Priority(*r.Priority)
		in.Priority = &p
	}
	if r.Category != nil {
		c := model.Category(*r.Category)
		in.Category = &c
	}
	return in
}

// completeTaskRequest is the body for POST /api/v1/tasks/:id/complete.
type completeTaskRequest struct {
	ActualMinutes int `json:"actual_minutes" binding:"gte=0,lte=1440"`
}

// ---------------------------------------------------------------------------
// Responses
// ---------------------------------------------------------------------------

// taskDTO is the canonical wire representation of a task.
type taskDTO struct {
	ID               uuid.UUID  `json:"id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Priority         string     `json:"priority"`
	Category         string     `json:"category"`
	Status           string     `json:"status"`
	EstimatedMinutes int        `json:"estimated_minutes"`
	ActualMinutes    int        `json:"actual_minutes"`
	DueDate          *time.Time `json:"due_date,omitempty"`
	CarryOverCount   int        `json:"carry_over_count"`
	IsOverdue        bool       `json:"is_overdue"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func taskResponse(t *model.Task, now time.Time) taskDTO {
	// Calendar-day overdue: due before start of today's UTC date.
	// Web UI uses APP_TIMEZONE via calendar.Calendar; API uses UTC midnight.
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return taskDTO{
		ID:               t.ID,
		Title:            t.Title,
		Description:      t.Description,
		Priority:         string(t.Priority),
		Category:         string(t.Category),
		Status:           string(t.Status),
		EstimatedMinutes: t.EstimatedMinutes,
		ActualMinutes:    t.ActualMinutes,
		DueDate:          t.DueDate,
		CarryOverCount:   t.CarryOverCount,
		IsOverdue:        t.IsOverdue(startOfToday),
		CompletedAt:      t.CompletedAt,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}
}

func taskListResponse(tasks []*model.Task, now time.Time) []taskDTO {
	out := make([]taskDTO, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, taskResponse(t, now))
	}
	return out
}
