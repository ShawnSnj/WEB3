package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shawn/jobhunttask/internal/model"
)

// PostgresTaskNoteRepository is the pgx-backed implementation.
type PostgresTaskNoteRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTaskNoteRepository(pool *pgxpool.Pool) *PostgresTaskNoteRepository {
	return &PostgresTaskNoteRepository{pool: pool}
}

var _ TaskNoteRepository = (*PostgresTaskNoteRepository)(nil)

const taskNoteColumns = `
    id, task_id, note_type, title, content,
    person_name, company, role_title, platform, profile_url,
    message_content, sent_at, reply_status, reply_at,
    job_title, job_url, application_status, applied_at,
    resume_version, fit_score, source, notes, is_marked,
    created_at, updated_at
`

func (r *PostgresTaskNoteRepository) Create(ctx context.Context, n *model.TaskNote) error {
	if err := n.Validate(); err != nil {
		return err
	}
	const q = `
        INSERT INTO task_notes (
            task_id, note_type, title, content,
            person_name, company, role_title, platform, profile_url,
            message_content, sent_at, reply_status, reply_at,
            job_title, job_url, application_status, applied_at,
            resume_version, fit_score, source, notes, is_marked
        )
        VALUES (
            $1, $2, $3, $4,
            $5, $6, $7, $8, $9,
            $10, $11, $12, $13,
            $14, $15, $16, $17,
            $18, $19, $20, $21, $22
        )
        RETURNING id, created_at, updated_at
    `
	row := r.pool.QueryRow(ctx, q,
		n.TaskID, string(n.NoteType), strings.TrimSpace(n.Title), n.Content,
		nullIfEmpty(n.PersonName), nullIfEmpty(n.Company), nullIfEmpty(n.RoleTitle),
		nullIfEmpty(n.Platform), nullIfEmpty(n.ProfileURL),
		nullIfEmpty(n.MessageContent), n.SentAt, nullIfEmpty(string(n.ReplyStatus)), n.ReplyAt,
		nullIfEmpty(n.JobTitle), nullIfEmpty(n.JobURL), nullIfEmpty(string(n.ApplicationStatus)), n.AppliedAt,
		nullIfEmpty(n.ResumeVersion), n.FitScore, nullIfEmpty(string(n.Source)), nullIfEmpty(n.Notes), n.IsMarked,
	)
	if err := row.Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return translateTaskNoteErr(err)
	}
	return nil
}

func (r *PostgresTaskNoteRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.TaskNote, error) {
	q := fmt.Sprintf(`SELECT %s FROM task_notes WHERE id = $1`, taskNoteColumns)
	row := r.pool.QueryRow(ctx, q, id)
	n, err := scanTaskNote(row)
	if err != nil {
		return nil, translateTaskNoteErr(err)
	}
	return n, nil
}

func (r *PostgresTaskNoteRepository) Update(ctx context.Context, id uuid.UUID, u TaskNoteUpdate) (*model.TaskNote, error) {
	sets := make([]string, 0, 24)
	args := make([]any, 0, 25)
	idx := 1

	add := func(col string, val any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	if u.NoteType != nil {
		add("note_type", string(*u.NoteType))
	}
	if u.Title != nil {
		add("title", strings.TrimSpace(*u.Title))
	}
	if u.Content != nil {
		add("content", *u.Content)
	}
	if u.PersonName != nil {
		add("person_name", nullIfEmpty(*u.PersonName))
	}
	if u.Company != nil {
		add("company", nullIfEmpty(*u.Company))
	}
	if u.RoleTitle != nil {
		add("role_title", nullIfEmpty(*u.RoleTitle))
	}
	if u.Platform != nil {
		add("platform", nullIfEmpty(*u.Platform))
	}
	if u.ProfileURL != nil {
		add("profile_url", nullIfEmpty(*u.ProfileURL))
	}
	if u.MessageContent != nil {
		add("message_content", nullIfEmpty(*u.MessageContent))
	}
	if u.SentAt != nil {
		add("sent_at", u.SentAt)
	}
	if u.ReplyStatus != nil {
		add("reply_status", nullIfEmpty(string(*u.ReplyStatus)))
	}
	if u.ReplyAt != nil {
		add("reply_at", u.ReplyAt)
	}
	if u.JobTitle != nil {
		add("job_title", nullIfEmpty(*u.JobTitle))
	}
	if u.JobURL != nil {
		add("job_url", nullIfEmpty(*u.JobURL))
	}
	if u.ApplicationStatus != nil {
		add("application_status", nullIfEmpty(string(*u.ApplicationStatus)))
	}
	if u.AppliedAt != nil {
		add("applied_at", u.AppliedAt)
	}
	if u.ResumeVersion != nil {
		add("resume_version", nullIfEmpty(*u.ResumeVersion))
	}
	if u.FitScore != nil {
		add("fit_score", *u.FitScore)
	}
	if u.Source != nil {
		add("source", nullIfEmpty(string(*u.Source)))
	}
	if u.Notes != nil {
		add("notes", nullIfEmpty(*u.Notes))
	}
	if u.IsMarked != nil {
		add("is_marked", *u.IsMarked)
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`
        UPDATE task_notes SET %s
        WHERE id = $%d
        RETURNING %s
    `, strings.Join(sets, ", "), idx, taskNoteColumns)

	row := r.pool.QueryRow(ctx, q, args...)
	n, err := scanTaskNote(row)
	if err != nil {
		return nil, translateTaskNoteErr(err)
	}
	if err := n.Validate(); err != nil {
		return nil, err
	}
	return n, nil
}

func (r *PostgresTaskNoteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM task_notes WHERE id = $1`, id)
	if err != nil {
		return translateTaskNoteErr(err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrTaskNoteNotFound
	}
	return nil
}

func (r *PostgresTaskNoteRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]*model.TaskNote, error) {
	q := fmt.Sprintf(`
        SELECT %s FROM task_notes
        WHERE task_id = $1
        ORDER BY updated_at DESC
    `, taskNoteColumns)
	rows, err := r.pool.Query(ctx, q, taskID)
	if err != nil {
		return nil, translateTaskNoteErr(err)
	}
	defer rows.Close()

	out := make([]*model.TaskNote, 0)
	for rows.Next() {
		n, err := scanTaskNote(rows)
		if err != nil {
			return nil, translateTaskNoteErr(err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *PostgresTaskNoteRepository) CountByNoteType(ctx context.Context, noteType model.NoteType) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM task_notes WHERE note_type = $1`, string(noteType),
	).Scan(&n)
	return n, err
}

const outreachTaskPredicate = `
    category = 'recruiter_outreach'
    OR title ILIKE '%dm%'
    OR title ILIKE '%outreach%'
`

const applicationTaskPredicate = `
    category = 'job_apply'
    OR title ILIKE '%apply%'
    OR title ILIKE '%application%'
`

func (r *PostgresTaskNoteRepository) CountOutreachTasks(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM tasks WHERE %s`, outreachTaskPredicate),
	).Scan(&n)
	return n, err
}

func (r *PostgresTaskNoteRepository) CountApplicationTasks(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM tasks WHERE %s`, applicationTaskPredicate),
	).Scan(&n)
	return n, err
}

func (r *PostgresTaskNoteRepository) ListByNoteType(ctx context.Context, noteType model.NoteType, limit int) ([]*model.TaskNoteWithTask, error) {
	if limit <= 0 {
		limit = 200
	}
	const q = `
        SELECT
            n.id, n.task_id, n.note_type, n.title, n.content,
            n.person_name, n.company, n.role_title, n.platform, n.profile_url,
            n.message_content, n.sent_at, n.reply_status, n.reply_at,
            n.job_title, n.job_url, n.application_status, n.applied_at,
            n.resume_version, n.fit_score, n.source, n.notes, n.is_marked,
            n.created_at, n.updated_at,
            t.title AS task_title
        FROM task_notes n
        JOIN tasks t ON t.id = n.task_id
        WHERE n.note_type = $1
        ORDER BY COALESCE(n.sent_at, n.applied_at, n.updated_at) DESC
        LIMIT $2
    `
	rows, err := r.pool.Query(ctx, q, string(noteType), limit)
	if err != nil {
		return nil, translateTaskNoteErr(err)
	}
	defer rows.Close()

	out := make([]*model.TaskNoteWithTask, 0)
	for rows.Next() {
		item, err := scanTaskNoteWithTask(rows)
		if err != nil {
			return nil, translateTaskNoteErr(err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PostgresTaskNoteRepository) ListOutreachTasks(ctx context.Context, limit int) ([]*model.Task, error) {
	return r.listTasksByPredicate(ctx, outreachTaskPredicate, limit)
}

func (r *PostgresTaskNoteRepository) ListApplicationTasks(ctx context.Context, limit int) ([]*model.Task, error) {
	return r.listTasksByPredicate(ctx, applicationTaskPredicate, limit)
}

func (r *PostgresTaskNoteRepository) listTasksByPredicate(ctx context.Context, predicate string, limit int) ([]*model.Task, error) {
	if limit <= 0 {
		limit = 200
	}
	q := fmt.Sprintf(`
        SELECT
            id, title, description, priority, category, status,
            estimated_minutes, actual_minutes, due_date, carry_over_count,
            completed_at, created_at, updated_at
        FROM tasks
        WHERE %s
        ORDER BY due_date NULLS LAST, created_at DESC
        LIMIT $1
    `, predicate)
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*model.Task, 0)
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type taskNoteScanner interface {
	Scan(dest ...any) error
}

func scanTaskNote(row taskNoteScanner) (*model.TaskNote, error) {
	var n model.TaskNote
	var noteType, replyStatus, appStatus, source *string
	var personName, company, roleTitle, platform, profileURL *string
	var messageContent, jobTitle, jobURL, resumeVersion, notes *string
	var fitScore *int
	if err := row.Scan(
		&n.ID, &n.TaskID, &noteType, &n.Title, &n.Content,
		&personName, &company, &roleTitle, &platform, &profileURL,
		&messageContent, &n.SentAt, &replyStatus, &n.ReplyAt,
		&jobTitle, &jobURL, &appStatus, &n.AppliedAt,
		&resumeVersion, &fitScore, &source, &notes, &n.IsMarked,
		&n.CreatedAt, &n.UpdatedAt,
	); err != nil {
		return nil, err
	}
	n.PersonName = derefStr(personName)
	n.Company = derefStr(company)
	n.RoleTitle = derefStr(roleTitle)
	n.Platform = derefStr(platform)
	n.ProfileURL = derefStr(profileURL)
	n.MessageContent = derefStr(messageContent)
	n.JobTitle = derefStr(jobTitle)
	n.JobURL = derefStr(jobURL)
	n.ResumeVersion = derefStr(resumeVersion)
	n.Notes = derefStr(notes)
	if noteType != nil {
		n.NoteType = model.NoteType(*noteType)
	} else {
		n.NoteType = model.NoteTypeGeneral
	}
	if replyStatus != nil {
		n.ReplyStatus = model.ReplyStatus(*replyStatus)
	}
	if appStatus != nil {
		n.ApplicationStatus = model.ApplicationStatus(*appStatus)
	}
	if source != nil {
		n.Source = model.ApplicationSource(*source)
	}
	n.FitScore = fitScore
	return &n, nil
}

func scanTaskNoteWithTask(row taskNoteScanner) (*model.TaskNoteWithTask, error) {
	var n model.TaskNoteWithTask
	var noteType, replyStatus, appStatus, source *string
	var personName, company, roleTitle, platform, profileURL *string
	var messageContent, jobTitle, jobURL, resumeVersion, notes *string
	var fitScore *int
	if err := row.Scan(
		&n.ID, &n.TaskID, &noteType, &n.Title, &n.Content,
		&personName, &company, &roleTitle, &platform, &profileURL,
		&messageContent, &n.SentAt, &replyStatus, &n.ReplyAt,
		&jobTitle, &jobURL, &appStatus, &n.AppliedAt,
		&resumeVersion, &fitScore, &source, &notes, &n.IsMarked,
		&n.CreatedAt, &n.UpdatedAt,
		&n.TaskTitle,
	); err != nil {
		return nil, err
	}
	n.PersonName = derefStr(personName)
	n.Company = derefStr(company)
	n.RoleTitle = derefStr(roleTitle)
	n.Platform = derefStr(platform)
	n.ProfileURL = derefStr(profileURL)
	n.MessageContent = derefStr(messageContent)
	n.JobTitle = derefStr(jobTitle)
	n.JobURL = derefStr(jobURL)
	n.ResumeVersion = derefStr(resumeVersion)
	n.Notes = derefStr(notes)
	if noteType != nil {
		n.NoteType = model.NoteType(*noteType)
	} else {
		n.NoteType = model.NoteTypeGeneral
	}
	if replyStatus != nil {
		n.ReplyStatus = model.ReplyStatus(*replyStatus)
	}
	if appStatus != nil {
		n.ApplicationStatus = model.ApplicationStatus(*appStatus)
	}
	if source != nil {
		n.Source = model.ApplicationSource(*source)
	}
	n.FitScore = fitScore
	return &n, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (r *PostgresTaskNoteRepository) GetMarkedWithTask(ctx context.Context) (*model.TaskNoteWithTask, error) {
	const q = `
        SELECT
            n.id, n.task_id, n.note_type, n.title, n.content,
            n.person_name, n.company, n.role_title, n.platform, n.profile_url,
            n.message_content, n.sent_at, n.reply_status, n.reply_at,
            n.job_title, n.job_url, n.application_status, n.applied_at,
            n.resume_version, n.fit_score, n.source, n.notes, n.is_marked,
            n.created_at, n.updated_at,
            t.title AS task_title
        FROM task_notes n
        JOIN tasks t ON t.id = n.task_id
        WHERE n.is_marked = TRUE
        ORDER BY n.updated_at DESC
        LIMIT 1
    `
	row := r.pool.QueryRow(ctx, q)
	n, err := scanTaskNoteWithTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, translateTaskNoteErr(err)
	}
	return n, nil
}

func (r *PostgresTaskNoteRepository) SetMarked(ctx context.Context, id uuid.UUID, marked bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if marked {
		if _, err := tx.Exec(ctx, `UPDATE task_notes SET is_marked = FALSE WHERE is_marked = TRUE`); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE task_notes SET is_marked = TRUE WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return model.ErrTaskNoteNotFound
		}
	} else {
		tag, err := tx.Exec(ctx, `UPDATE task_notes SET is_marked = FALSE WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return model.ErrTaskNoteNotFound
		}
	}
	return tx.Commit(ctx)
}

func translateTaskNoteErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ErrTaskNoteNotFound
	}
	return err
}

// scanTaskRow scans a task row for dashboard task lists (minimal duplicate of task repo).
func scanTaskRow(row pgx.Row) (*model.Task, error) {
	var t model.Task
	var priority, category, status string
	var dueDate *time.Time
	if err := row.Scan(
		&t.ID, &t.Title, &t.Description, &priority, &category, &status,
		&t.EstimatedMinutes, &t.ActualMinutes, &dueDate, &t.CarryOverCount,
		&t.CompletedAt, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, err
	}
	t.Priority = model.Priority(priority)
	t.Category = model.Category(category)
	t.Status = model.Status(status)
	t.DueDate = dueDate
	return &t, nil
}
