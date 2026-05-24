package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/repository"
	"github.com/shawn/jobhunttask/internal/service"
)

type importTaskRepo struct {
	items []*model.Task
}

func (r *importTaskRepo) Create(_ context.Context, t *model.Task) error {
	t.ID = uuid.New()
	now := time.Now()
	t.CreatedAt, t.UpdatedAt = now, now
	cp := *t
	r.items = append(r.items, &cp)
	return nil
}
func (r *importTaskRepo) GetByID(context.Context, uuid.UUID) (*model.Task, error) {
	return nil, model.ErrTaskNotFound
}
func (r *importTaskRepo) Update(context.Context, uuid.UUID, repository.TaskUpdate) (*model.Task, error) {
	return nil, model.ErrTaskNotFound
}
func (r *importTaskRepo) Delete(context.Context, uuid.UUID) error { return nil }
func (r *importTaskRepo) List(context.Context, repository.TaskFilter) ([]*model.Task, error) {
	return nil, nil
}
func (r *importTaskRepo) ListOverdue(context.Context, time.Time) ([]*model.Task, error) {
	return nil, nil
}

func TestImportFromCSV_WithHeader(t *testing.T) {
	t.Parallel()
	repo := &importTaskRepo{}
	svc := service.NewTaskService(repo, service.SystemClock)
	today := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

	csv := `title,description,category,priority,estimated_minutes,due_date
Task A,Desc a,job_apply,high,30,2026-05-25
Task B,,misc,medium,15,
,empty title,,,,
Task C,,misc,not_a_priority,10,
`
	res, err := svc.ImportFromCSV(context.Background(), strings.NewReader(csv), today)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 2 {
		t.Fatalf("created = %d, want 2", res.Created)
	}
	if res.Skipped != 2 {
		t.Fatalf("skipped = %d, want 2", res.Skipped)
	}
	if len(repo.items) != 2 {
		t.Fatalf("repo items = %d", len(repo.items))
	}
	if repo.items[0].Title != "Task A" {
		t.Fatalf("first title = %q", repo.items[0].Title)
	}
	wantDue := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	if repo.items[1].DueDate == nil || !repo.items[1].DueDate.Equal(wantDue) {
		t.Fatalf("blank due_date should default to today, got %v", repo.items[1].DueDate)
	}
}

func TestImportFromCSV_TaskIDHeaderAndFloatMinutes(t *testing.T) {
	t.Parallel()
	repo := &importTaskRepo{}
	svc := service.NewTaskService(repo, service.SystemClock)
	today := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

	csv := `task_id,title,description,category,priority,estimated_minutes,due_date
W1D1-01,Apply to Acme,Portal,job_apply,high,45.0,
W1D1-02,LinkedIn post,,twitter,medium,15,
W1D1-03,Study caching,,learning,low,60 min,
`
	res, err := svc.ImportFromCSV(context.Background(), strings.NewReader(csv), today)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 3 {
		t.Fatalf("created = %d, want 3 (errors: %+v)", res.Created, res.Errors)
	}
	if repo.items[0].Title != "Apply to Acme" {
		t.Fatalf("title = %q", repo.items[0].Title)
	}
	if repo.items[0].EstimatedMinutes != 45 {
		t.Fatalf("minutes = %d", repo.items[0].EstimatedMinutes)
	}
	if repo.items[2].EstimatedMinutes != 60 {
		t.Fatalf("minutes with suffix = %d", repo.items[2].EstimatedMinutes)
	}
}

func TestImportFromCSV_CategoryAliases(t *testing.T) {
	t.Parallel()
	repo := &importTaskRepo{}
	svc := service.NewTaskService(repo, service.SystemClock)
	today := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

	csv := `task_id,title,category,priority,status,due_date,estimated_minutes,notes
W1D1-01,Rewrite resume,career,HIGH,TODO,2026-05-25,120,notes
W1D5-01,Apply to roles,application,HIGH,TODO,2026-05-29,120,
W1D5-02,Send outreach,outreach,HIGH,TODO,2026-05-29,45,
W1D2-02,Project showcase,portfolio,HIGH,TODO,2026-05-26,90,
W2D2-02,Twitter post,visibility,MEDIUM,TODO,2026-06-02,60,
W1D7-02,Weekly review,review,MEDIUM,TODO,2026-05-31,45,
`
	res, err := svc.ImportFromCSV(context.Background(), strings.NewReader(csv), today)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 6 {
		t.Fatalf("created = %d, want 6 (errors: %+v)", res.Created, res.Errors)
	}
	want := []model.Category{
		model.CategoryMisc, model.CategoryJobApply, model.CategoryRecruiterOutreach,
		model.CategoryGithub, model.CategoryTwitter, model.CategoryMisc,
	}
	for i, cat := range want {
		if repo.items[i].Category != cat {
			t.Fatalf("row %d category = %q, want %q", i, repo.items[i].Category, cat)
		}
	}
	if repo.items[0].Description != "notes" {
		t.Fatalf("notes column should map to description, got %q", repo.items[0].Description)
	}
}
