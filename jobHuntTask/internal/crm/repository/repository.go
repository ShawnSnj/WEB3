package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// ---------------------------------------------------------------------------
// Profile
// ---------------------------------------------------------------------------

func (s *Store) GetProfile(ctx context.Context) (*crm.UserProfile, error) {
	const q = `
		SELECT id, display_name, headline, skills, target_titles, target_industries,
		       resume_text, min_salary_usd, remote_only, web3_preferred,
		       daily_applications, daily_outreach, created_at, updated_at
		FROM user_profile
		ORDER BY created_at
		LIMIT 1`
	var p crm.UserProfile
	err := s.pool.QueryRow(ctx, q).Scan(
		&p.ID, &p.DisplayName, &p.Headline, &p.Skills, &p.TargetTitles, &p.TargetIndustries,
		&p.ResumeText, &p.MinSalaryUSD, &p.RemoteOnly, &p.Web3Preferred,
		&p.DailyApplications, &p.DailyOutreach, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (s *Store) UpdateProfile(ctx context.Context, p *crm.UserProfile) error {
	const q = `
		UPDATE user_profile SET
			display_name = $2, headline = $3, skills = $4, target_titles = $5,
			target_industries = $6, resume_text = $7, min_salary_usd = $8,
			remote_only = $9, web3_preferred = $10, daily_applications = $11,
			daily_outreach = $12, updated_at = NOW()
		WHERE id = $1`
	_, err := s.pool.Exec(ctx, q,
		p.ID, p.DisplayName, p.Headline, p.Skills, p.TargetTitles, p.TargetIndustries,
		p.ResumeText, p.MinSalaryUSD, p.RemoteOnly, p.Web3Preferred,
		p.DailyApplications, p.DailyOutreach,
	)
	return err
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

func (s *Store) UpsertJob(ctx context.Context, j *crm.RawJob) (*crm.JobPosting, error) {
	companyID, err := s.ensureCompany(ctx, j.CompanyName, j.Web3)
	if err != nil {
		return nil, err
	}
	const q = `
		INSERT INTO job_postings (
			external_id, source, title, company_id, company_name,
			salary_min_usd, salary_max_usd, salary_raw, location, remote,
			description, required_skills, application_url, posted_at,
			seniority, web3, raw_payload, is_active
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,TRUE)
		ON CONFLICT (source, external_id) DO UPDATE SET
			title = EXCLUDED.title,
			company_name = EXCLUDED.company_name,
			salary_min_usd = EXCLUDED.salary_min_usd,
			salary_max_usd = EXCLUDED.salary_max_usd,
			salary_raw = EXCLUDED.salary_raw,
			location = EXCLUDED.location,
			remote = EXCLUDED.remote,
			description = EXCLUDED.description,
			required_skills = EXCLUDED.required_skills,
			application_url = EXCLUDED.application_url,
			posted_at = EXCLUDED.posted_at,
			seniority = EXCLUDED.seniority,
			web3 = EXCLUDED.web3,
			raw_payload = EXCLUDED.raw_payload,
			is_active = TRUE,
			updated_at = NOW()
		RETURNING id, external_id, source, title, company_id, company_name,
		          salary_min_usd, salary_max_usd, salary_raw, location, remote,
		          description, required_skills, application_url, posted_at,
		          seniority, web3, is_active, created_at, updated_at`
	raw, _ := json.Marshal(j.RawPayload)
	var out crm.JobPosting
	var cid *uuid.UUID
	err = s.pool.QueryRow(ctx, q,
		j.ExternalID, j.Source, j.Title, companyID, j.CompanyName,
		j.SalaryMinUSD, j.SalaryMaxUSD, j.SalaryRaw, j.Location, j.Remote,
		j.Description, j.RequiredSkills, j.ApplicationURL, j.PostedAt,
		j.Seniority, j.Web3, raw,
	).Scan(
		&out.ID, &out.ExternalID, &out.Source, &out.Title, &cid, &out.CompanyName,
		&out.SalaryMinUSD, &out.SalaryMaxUSD, &out.SalaryRaw, &out.Location, &out.Remote,
		&out.Description, &out.RequiredSkills, &out.ApplicationURL, &out.PostedAt,
		&out.Seniority, &out.Web3, &out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	out.CompanyID = cid
	return &out, nil
}

func (s *Store) ensureCompany(ctx context.Context, name string, web3 bool) (*uuid.UUID, error) {
	if name == "" {
		return nil, nil
	}
	const q = `
		INSERT INTO companies (name, web3) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET web3 = companies.web3 OR EXCLUDED.web3
		RETURNING id`
	var id uuid.UUID
	if err := s.pool.QueryRow(ctx, q, name, web3).Scan(&id); err != nil {
		return nil, err
	}
	return &id, nil
}

type JobFilter struct {
	MinFit         int
	MinROI         int
	MinFinalScore  int
	Tier           string
	Decision       string
	ScoredOnly     bool
	ExcludeSkipped bool
	HideFrontend   bool
	HideJunior     bool
	HideNonRemote  bool
	HideMarketing  bool
	Remote         *bool
	Web3           *bool
	Search         string
	Limit          int
	Offset         int
	Active         bool
}

func (s *Store) ListJobs(ctx context.Context, f JobFilter) ([]crm.JobPosting, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	active := true
	if f.Active {
		active = f.Active
	}
	q := `
		SELECT j.id, j.external_id, j.source, j.title, j.company_id, j.company_name,
		       j.salary_min_usd, j.salary_max_usd, j.salary_raw, j.location, j.remote,
		       j.description, j.required_skills, j.application_url, j.posted_at,
		       j.seniority, j.web3, j.is_active, j.created_at, j.updated_at,` + matchSelectCols + `
		FROM job_postings j
		LEFT JOIN job_matches m ON m.job_id = j.id
		WHERE j.is_active = $1`
	args := []any{active}
	idx := 2
	if f.Remote != nil {
		q += fmt.Sprintf(" AND j.remote = $%d", idx)
		args = append(args, *f.Remote)
		idx++
	}
	if f.Web3 != nil {
		q += fmt.Sprintf(" AND j.web3 = $%d", idx)
		args = append(args, *f.Web3)
		idx++
	}
	if f.MinFit > 0 {
		q += fmt.Sprintf(" AND COALESCE(m.fit_score, 0) >= $%d", idx)
		args = append(args, f.MinFit)
		idx++
	}
	if f.MinROI > 0 {
		q += fmt.Sprintf(" AND COALESCE(m.career_roi_score, 0) >= $%d", idx)
		args = append(args, f.MinROI)
		idx++
	}
	if f.MinFinalScore > 0 {
		q += fmt.Sprintf(" AND COALESCE(m.final_score, 0) >= $%d", idx)
		args = append(args, f.MinFinalScore)
		idx++
	}
	if f.Tier == "A" {
		q += " AND m.fit_tier = 'A'"
	} else if f.Tier == "B" {
		q += " AND m.fit_tier = 'B'"
	} else if f.Tier == "AB" {
		q += " AND m.fit_tier IN ('A', 'B')"
	}
	if f.ExcludeSkipped {
		q += " AND COALESCE(m.user_action, '') <> 'skipped'"
	}
	if f.HideFrontend {
		q += ` AND NOT (
			(j.title ILIKE '%frontend%' OR j.description ILIKE '%frontend%' OR j.description ILIKE '%react%')
			AND j.title NOT ILIKE '%backend%' AND j.description NOT ILIKE '%backend%'
		)`
	}
	if f.HideJunior {
		q += ` AND j.title NOT ILIKE '%junior%' AND j.title NOT ILIKE '%intern%'
			AND j.description NOT ILIKE '%junior%' AND j.description NOT ILIKE '%entry level%'`
	}
	if f.HideNonRemote {
		q += ` AND (j.remote OR j.location ILIKE '%remote%' OR j.location ILIKE '%async%')`
	}
	if f.HideMarketing {
		q += ` AND j.title NOT ILIKE '%marketing%' AND j.title NOT ILIKE '%content%'
			AND j.title NOT ILIKE '%growth%' AND j.description NOT ILIKE '%marketing%'`
	}
	if f.Decision != "" {
		q += fmt.Sprintf(" AND m.decision = $%d", idx)
		args = append(args, f.Decision)
		idx++
	}
	if f.ScoredOnly {
		q += " AND m.id IS NOT NULL AND COALESCE(m.decision, '') <> ''"
	}
	if f.Search != "" {
		q += fmt.Sprintf(` AND (
			j.title ILIKE $%d OR
			j.company_name ILIKE $%d OR
			j.description ILIKE $%d OR
			array_to_string(j.required_skills, ' ') ILIKE $%d
		)`, idx, idx, idx, idx)
		args = append(args, "%"+f.Search+"%")
		idx++
	}
	q += fmt.Sprintf(" ORDER BY COALESCE(m.final_score, 0) DESC, COALESCE(m.fit_tier, 'C') ASC, j.posted_at DESC NULLS LAST LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []crm.JobPosting
	for rows.Next() {
		var j crm.JobPosting
		var cid *uuid.UUID
		var mid *uuid.UUID
		var fit, titleSc, skill, backend, infra, remote, salary, seniority, web3, negative, final *int
		var company, growth, domain, roi, interview *int
		var tier, decision, why, resumeAngle, appPriority, resumeVer, userAction, summary, model *string
		var missingSkills, missingKw, resumeKw, pros, risks []string
		var scoredAt *time.Time
		if err := rows.Scan(
			&j.ID, &j.ExternalID, &j.Source, &j.Title, &cid, &j.CompanyName,
			&j.SalaryMinUSD, &j.SalaryMaxUSD, &j.SalaryRaw, &j.Location, &j.Remote,
			&j.Description, &j.RequiredSkills, &j.ApplicationURL, &j.PostedAt,
			&j.Seniority, &j.Web3, &j.IsActive, &j.CreatedAt, &j.UpdatedAt,
			&mid, &fit, &titleSc, &skill, &backend, &infra, &remote, &salary, &seniority, &web3, &negative, &final,
			&tier, &company, &growth, &domain, &roi, &interview,
			&decision, &why, &missingSkills, &missingKw, &resumeKw, &resumeAngle, &appPriority, &resumeVer, &userAction,
			&pros, &risks, &summary, &model, &scoredAt,
		); err != nil {
			return nil, err
		}
		j.CompanyID = cid
		j.Match = scanJobMatch(mid, fit, titleSc, skill, backend, infra, remote, salary, seniority, web3, negative, final,
			company, growth, domain, roi, interview,
			tier, decision, why, resumeAngle, appPriority, resumeVer, userAction, summary, model,
			missingSkills, missingKw, resumeKw, pros, risks, scoredAt, j.ID)
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) GetJob(ctx context.Context, id uuid.UUID) (*crm.JobPosting, error) {
	const q = `
		SELECT j.id, j.external_id, j.source, j.title, j.company_id, j.company_name,
		       j.salary_min_usd, j.salary_max_usd, j.salary_raw, j.location, j.remote,
		       j.description, j.required_skills, j.application_url, j.posted_at,
		       j.seniority, j.web3, j.is_active, j.created_at, j.updated_at
		FROM job_postings j WHERE j.id = $1`
	var j crm.JobPosting
	var cid *uuid.UUID
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&j.ID, &j.ExternalID, &j.Source, &j.Title, &cid, &j.CompanyName,
		&j.SalaryMinUSD, &j.SalaryMaxUSD, &j.SalaryRaw, &j.Location, &j.Remote,
		&j.Description, &j.RequiredSkills, &j.ApplicationURL, &j.PostedAt,
		&j.Seniority, &j.Web3, &j.IsActive, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	j.CompanyID = cid
	m, err := s.GetMatch(ctx, id)
	if err == nil {
		j.Match = m
	}
	return &j, nil
}

func (s *Store) CountActiveJobs(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM job_postings WHERE is_active`).Scan(&n)
	return n, err
}

func (s *Store) ListUnscoredJobIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT j.id FROM job_postings j
		LEFT JOIN job_matches m ON m.job_id = j.id
		WHERE j.is_active AND (m.id IS NULL OR j.updated_at > m.scored_at)
		ORDER BY COALESCE(m.scored_at, j.created_at) ASC, j.updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---------------------------------------------------------------------------
// Matches
// ---------------------------------------------------------------------------

func (s *Store) UpsertMatch(ctx context.Context, m *crm.JobMatch) error {
	const q = `
		INSERT INTO job_matches (
			job_id, fit_score, title_score, skill_score, backend_score, infra_score,
			remote_score, salary_score, seniority_score, web3_score, negative_score,
			final_score, fit_tier, company_score, growth_score,
			domain_match_score, career_roi_score, interview_probability,
			decision, why_this_matches_me, missing_skills, missing_keywords,
			resume_keywords_to_add, suggested_resume_angle, application_priority,
			resume_version_recommendation, user_action,
			pros, risks, summary, model, scored_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,NOW())
		ON CONFLICT (job_id) DO UPDATE SET
			fit_score = EXCLUDED.fit_score,
			title_score = EXCLUDED.title_score,
			skill_score = EXCLUDED.skill_score,
			backend_score = EXCLUDED.backend_score,
			infra_score = EXCLUDED.infra_score,
			remote_score = EXCLUDED.remote_score,
			salary_score = EXCLUDED.salary_score,
			seniority_score = EXCLUDED.seniority_score,
			web3_score = EXCLUDED.web3_score,
			negative_score = EXCLUDED.negative_score,
			final_score = EXCLUDED.final_score,
			fit_tier = EXCLUDED.fit_tier,
			company_score = EXCLUDED.company_score,
			growth_score = EXCLUDED.growth_score,
			domain_match_score = EXCLUDED.domain_match_score,
			career_roi_score = EXCLUDED.career_roi_score,
			interview_probability = EXCLUDED.interview_probability,
			decision = EXCLUDED.decision,
			why_this_matches_me = EXCLUDED.why_this_matches_me,
			missing_skills = EXCLUDED.missing_skills,
			missing_keywords = EXCLUDED.missing_keywords,
			resume_keywords_to_add = EXCLUDED.resume_keywords_to_add,
			suggested_resume_angle = EXCLUDED.suggested_resume_angle,
			application_priority = EXCLUDED.application_priority,
			resume_version_recommendation = EXCLUDED.resume_version_recommendation,
			pros = EXCLUDED.pros,
			risks = EXCLUDED.risks,
			summary = EXCLUDED.summary,
			model = EXCLUDED.model,
			scored_at = NOW(),
			updated_at = NOW()
		RETURNING id`
	decision := string(m.Decision)
	if decision == "" {
		decision = string(crm.FitSkip)
	}
	tier := string(m.FitTier)
	if tier == "" {
		tier = string(crm.TierFromScore(m.FinalScore))
	}
	if m.CareerROIScore == 0 {
		m.CareerROIScore = m.FinalScore
	}
	if m.FitScore == 0 {
		m.FitScore = m.FinalScore
	}
	return s.pool.QueryRow(ctx, q,
		m.JobID, m.FitScore, m.TitleScore, m.SkillScore, m.BackendScore, m.InfraScore,
		m.RemoteScore, m.SalaryScore, m.SeniorityScore, m.Web3Score, m.NegativeScore,
		m.FinalScore, tier, m.CompanyScore, m.GrowthScore,
		m.DomainMatchScore, m.CareerROIScore, m.InterviewProbability,
		decision, m.WhyThisMatchesMe, m.MissingSkills, m.MissingKeywords,
		m.ResumeKeywordsToAdd, m.SuggestedResumeAngle, m.ApplicationPriority,
		m.ResumeVersionRecommendation, m.UserAction,
		m.Pros, m.Risks, m.Summary, m.Model,
	).Scan(&m.ID)
}

func (s *Store) SetJobUserAction(ctx context.Context, jobID uuid.UUID, action string) error {
	const q = `UPDATE job_matches SET user_action = $2, updated_at = NOW() WHERE job_id = $1`
	tag, err := s.pool.Exec(ctx, q, jobID, action)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return crm.ErrNotFound
	}
	return nil
}

func (s *Store) GetMatch(ctx context.Context, jobID uuid.UUID) (*crm.JobMatch, error) {
	q := `SELECT ` + matchSelectSQL() + ` FROM job_matches m WHERE m.job_id = $1`
	var mid uuid.UUID
	var fit, titleSc, skill, backend, infra, remote, salary, seniority, web3, negative, final *int
	var company, growth, domain, roi, interview *int
	var tier, decision, why, resumeAngle, appPriority, resumeVer, userAction, summary, model *string
	var missingSkills, missingKw, resumeKw, pros, risks []string
	var scoredAt *time.Time
	err := s.pool.QueryRow(ctx, q, jobID).Scan(
		&mid, &fit, &titleSc, &skill, &backend, &infra, &remote, &salary, &seniority, &web3, &negative, &final,
		&tier, &company, &growth, &domain, &roi, &interview,
		&decision, &why, &missingSkills, &missingKw, &resumeKw, &resumeAngle, &appPriority, &resumeVer, &userAction,
		&pros, &risks, &summary, &model, &scoredAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	return scanJobMatch(&mid, fit, titleSc, skill, backend, infra, remote, salary, seniority, web3, negative, final,
		company, growth, domain, roi, interview,
		tier, decision, why, resumeAngle, appPriority, resumeVer, userAction, summary, model,
		missingSkills, missingKw, resumeKw, pros, risks, scoredAt, jobID), nil
}

func (s *Store) TopMatchedJob(ctx context.Context, minFit int) (*crm.JobPosting, error) {
	jobs, err := s.ListJobs(ctx, JobFilter{MinFit: minFit, Limit: 1, Active: true})
	if err != nil || len(jobs) == 0 {
		return nil, err
	}
	return &jobs[0], nil
}

// ---------------------------------------------------------------------------
// Applications
// ---------------------------------------------------------------------------

func (s *Store) CreateApplication(ctx context.Context, a *crm.Application) error {
	const q = `
		INSERT INTO applications (job_id, company_name, role_title, status, applied_at, notes, resume_score, match_score)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at, updated_at`
	return s.pool.QueryRow(ctx, q,
		a.JobID, a.CompanyName, a.RoleTitle, string(a.Status), a.AppliedAt, a.Notes, a.ResumeScore, a.MatchScore,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func (s *Store) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status crm.ApplicationStatus, appliedAt *time.Time) error {
	if !status.IsValid() {
		return crm.ErrInvalidStatus
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE applications SET status = $2, applied_at = COALESCE($3, applied_at), updated_at = NOW()
		WHERE id = $1`, id, string(status), appliedAt)
	return err
}

func (s *Store) GetApplicationByJobID(ctx context.Context, jobID uuid.UUID) (*crm.Application, error) {
	const q = `
		SELECT id, job_id, company_name, role_title, status, applied_at, notes,
		       resume_score, match_score, created_at, updated_at
		FROM applications WHERE job_id = $1 ORDER BY created_at DESC LIMIT 1`
	var a crm.Application
	var st string
	err := s.pool.QueryRow(ctx, q, jobID).Scan(
		&a.ID, &a.JobID, &a.CompanyName, &a.RoleTitle, &st, &a.AppliedAt, &a.Notes,
		&a.ResumeScore, &a.MatchScore, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	a.Status = crm.ApplicationStatus(st)
	return &a, nil
}

func (s *Store) ListAppliedJobIDs(ctx context.Context) (map[uuid.UUID]struct{}, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT job_id FROM applications WHERE job_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]struct{})
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) ListApplications(ctx context.Context) ([]crm.Application, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, job_id, company_name, role_title, status, applied_at, notes,
		       resume_score, match_score, created_at, updated_at
		FROM applications ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []crm.Application
	for rows.Next() {
		var a crm.Application
		var st string
		if err := rows.Scan(
			&a.ID, &a.JobID, &a.CompanyName, &a.RoleTitle, &st, &a.AppliedAt, &a.Notes,
			&a.ResumeScore, &a.MatchScore, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		a.Status = crm.ApplicationStatus(st)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ApplicationStats(ctx context.Context, since time.Time) (sent, responses, interviews int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status IN ('applied','interview','technical','final_round','offer','rejected')),
			COUNT(*) FILTER (WHERE status IN ('interview','technical','final_round','offer')),
			COUNT(*) FILTER (WHERE status IN ('interview','technical','final_round'))
		FROM applications WHERE created_at >= $1`, since).Scan(&sent, &responses, &interviews)
	return
}

// ---------------------------------------------------------------------------
// Contacts
// ---------------------------------------------------------------------------

func (s *Store) UpsertContact(ctx context.Context, c *crm.Contact) error {
	if c.ID == uuid.Nil {
		const q = `
			INSERT INTO contacts (company_id, company_name, full_name, title, role_type, linkedin_url, email, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			RETURNING id, created_at, updated_at`
		return s.pool.QueryRow(ctx, q,
			c.CompanyID, c.CompanyName, c.FullName, c.Title, string(c.RoleType),
			c.LinkedInURL, c.Email, c.Notes,
		).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE contacts SET company_name=$2, full_name=$3, title=$4, role_type=$5,
			linkedin_url=$6, email=$7, notes=$8, updated_at=NOW() WHERE id=$1`,
		c.ID, c.CompanyName, c.FullName, c.Title, string(c.RoleType),
		c.LinkedInURL, c.Email, c.Notes)
	return err
}

func (s *Store) ListContacts(ctx context.Context, limit int) ([]crm.Contact, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, company_id, company_name, full_name, title, role_type,
		       linkedin_url, email, notes, created_at, updated_at
		FROM contacts ORDER BY updated_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContacts(rows)
}

func (s *Store) ContactsForCompany(ctx context.Context, companyName string, limit int) ([]crm.Contact, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, company_id, company_name, full_name, title, role_type,
		       linkedin_url, email, notes, created_at, updated_at
		FROM contacts WHERE company_name ILIKE $1
		ORDER BY CASE role_type
			WHEN 'engineering_manager' THEN 1
			WHEN 'hiring_manager' THEN 2
			WHEN 'recruiter' THEN 3
			ELSE 4 END
		LIMIT $2`, companyName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContacts(rows)
}

func scanContacts(rows pgx.Rows) ([]crm.Contact, error) {
	var out []crm.Contact
	for rows.Next() {
		var c crm.Contact
		var rt string
		if err := rows.Scan(
			&c.ID, &c.CompanyID, &c.CompanyName, &c.FullName, &c.Title, &rt,
			&c.LinkedInURL, &c.Email, &c.Notes, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c.RoleType = crm.ContactRole(rt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Outreach
// ---------------------------------------------------------------------------

func (s *Store) SaveOutreach(ctx context.Context, m *crm.OutreachMessage) error {
	const q = `
		INSERT INTO outreach_messages (contact_id, job_id, subject, body, status)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q, m.ContactID, m.JobID, m.Subject, m.Body, m.Status).Scan(&m.ID, &m.CreatedAt)
}

// ---------------------------------------------------------------------------
// Skill analysis
// ---------------------------------------------------------------------------

func (s *Store) SaveSkillAnalysis(ctx context.Context, a *crm.SkillAnalysis) error {
	top, _ := json.Marshal(a.TopDemanded)
	missing, _ := json.Marshal(a.MissingSkills)
	priority, _ := json.Marshal(a.LearningPriority)
	gaps, _ := json.Marshal(a.SkillGaps)
	const q = `
		INSERT INTO skill_analyses (analysis_date, top_demanded, missing_skills, learning_priority, skill_gaps, jobs_analyzed)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (analysis_date) DO UPDATE SET
			top_demanded = EXCLUDED.top_demanded,
			missing_skills = EXCLUDED.missing_skills,
			learning_priority = EXCLUDED.learning_priority,
			skill_gaps = EXCLUDED.skill_gaps,
			jobs_analyzed = EXCLUDED.jobs_analyzed
		RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q, a.AnalysisDate, top, missing, priority, gaps, a.JobsAnalyzed).Scan(&a.ID, &a.CreatedAt)
}

func (s *Store) LatestSkillAnalysis(ctx context.Context) (*crm.SkillAnalysis, error) {
	const q = `
		SELECT id, analysis_date, top_demanded, missing_skills, learning_priority, skill_gaps, jobs_analyzed, created_at
		FROM skill_analyses ORDER BY analysis_date DESC LIMIT 1`
	var a crm.SkillAnalysis
	var top, missing, priority, gaps []byte
	err := s.pool.QueryRow(ctx, q).Scan(&a.ID, &a.AnalysisDate, &top, &missing, &priority, &gaps, &a.JobsAnalyzed, &a.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(top, &a.TopDemanded)
	_ = json.Unmarshal(missing, &a.MissingSkills)
	_ = json.Unmarshal(priority, &a.LearningPriority)
	_ = json.Unmarshal(gaps, &a.SkillGaps)
	return &a, nil
}

// ---------------------------------------------------------------------------
// Daily brief
// ---------------------------------------------------------------------------

func (s *Store) SaveDailyBrief(ctx context.Context, b *crm.DailyBrief) error {
	applySummary, _ := json.Marshal(b.ApplySummary)
	outreach, _ := json.Marshal(b.OutreachTargets)
	tasks, _ := json.Marshal(b.AutomationTasks)
	if b.MissionPayload == nil {
		b.MissionPayload = map[string]any{}
	}
	if len(b.ApplyJobs) > 0 {
		jobsBytes, _ := json.Marshal(b.ApplyJobs)
		var jobsAny []map[string]any
		_ = json.Unmarshal(jobsBytes, &jobsAny)
		b.MissionPayload["apply_jobs"] = jobsAny
	}
	mission, _ := json.Marshal(b.MissionPayload)
	var applyJobID *uuid.UUID
	if b.ApplyJob != nil {
		applyJobID = &b.ApplyJob.ID
	}
	const q = `
		INSERT INTO daily_briefs (brief_date, apply_job_id, apply_summary, outreach_targets,
		                          learning_skill, learning_reason, learning_topic,
		                          interview_topic, interview_context, estimated_minutes,
		                          automation_tasks, mission_payload, generated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW())
		ON CONFLICT (brief_date) DO UPDATE SET
			apply_job_id = EXCLUDED.apply_job_id,
			apply_summary = EXCLUDED.apply_summary,
			outreach_targets = EXCLUDED.outreach_targets,
			learning_skill = EXCLUDED.learning_skill,
			learning_reason = EXCLUDED.learning_reason,
			learning_topic = EXCLUDED.learning_topic,
			interview_topic = EXCLUDED.interview_topic,
			interview_context = EXCLUDED.interview_context,
			estimated_minutes = EXCLUDED.estimated_minutes,
			automation_tasks = EXCLUDED.automation_tasks,
			mission_payload = EXCLUDED.mission_payload,
			generated_at = NOW()
		RETURNING id, generated_at`
	return s.pool.QueryRow(ctx, q,
		b.BriefDate, applyJobID, applySummary, outreach,
		b.LearningSkill, b.LearningReason, b.LearningTopic,
		b.InterviewTopic, b.InterviewContext, b.EstimatedMinutes,
		tasks, mission,
	).Scan(&b.ID, &b.GeneratedAt)
}

func (s *Store) GetDailyBrief(ctx context.Context, date time.Time) (*crm.DailyBrief, error) {
	const q = `
		SELECT id, brief_date, apply_job_id, apply_summary, outreach_targets,
		       learning_skill, learning_reason, learning_topic,
		       interview_topic, interview_context, estimated_minutes,
		       automation_tasks, mission_payload, generated_at
		FROM daily_briefs WHERE brief_date = $1`
	var b crm.DailyBrief
	var applyJobID *uuid.UUID
	var applySummary, outreach, tasks, mission []byte
	err := s.pool.QueryRow(ctx, q, date).Scan(
		&b.ID, &b.BriefDate, &applyJobID, &applySummary, &outreach,
		&b.LearningSkill, &b.LearningReason, &b.LearningTopic,
		&b.InterviewTopic, &b.InterviewContext, &b.EstimatedMinutes,
		&tasks, &mission, &b.GeneratedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(applySummary, &b.ApplySummary)
	_ = json.Unmarshal(outreach, &b.OutreachTargets)
	_ = json.Unmarshal(tasks, &b.AutomationTasks)
	_ = json.Unmarshal(mission, &b.MissionPayload)
	if jobsRaw, ok := b.MissionPayload["apply_jobs"]; ok {
		jobsBytes, _ := json.Marshal(jobsRaw)
		_ = json.Unmarshal(jobsBytes, &b.ApplyJobs)
	}
	if applyJobID != nil {
		job, err := s.GetJob(ctx, *applyJobID)
		if err == nil {
			b.ApplyJob = job
		}
	}
	return &b, nil
}

// ---------------------------------------------------------------------------
// Weekly report
// ---------------------------------------------------------------------------

func (s *Store) SaveWeeklyReport(ctx context.Context, r *crm.WeeklyCRMReport) error {
	changes, _ := json.Marshal(r.SkillGapChanges)
	payload, _ := json.Marshal(r.Payload)
	const q = `
		INSERT INTO weekly_crm_reports (
			week_start, jobs_found, applications_sent, response_rate, interview_rate,
			skill_gap_changes, coach_summary, payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (week_start) DO UPDATE SET
			jobs_found = EXCLUDED.jobs_found,
			applications_sent = EXCLUDED.applications_sent,
			response_rate = EXCLUDED.response_rate,
			interview_rate = EXCLUDED.interview_rate,
			skill_gap_changes = EXCLUDED.skill_gap_changes,
			coach_summary = EXCLUDED.coach_summary,
			payload = EXCLUDED.payload
		RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q,
		r.WeekStart, r.JobsFound, r.ApplicationsSent, r.ResponseRate, r.InterviewRate,
		changes, r.CoachSummary, payload,
	).Scan(&r.ID, &r.CreatedAt)
}

func (s *Store) GetWeeklyReport(ctx context.Context, weekStart time.Time) (*crm.WeeklyCRMReport, error) {
	const q = `
		SELECT id, week_start, jobs_found, applications_sent, response_rate, interview_rate,
		       skill_gap_changes, coach_summary, payload, created_at
		FROM weekly_crm_reports WHERE week_start = $1`
	var r crm.WeeklyCRMReport
	var changes, payload []byte
	err := s.pool.QueryRow(ctx, q, weekStart).Scan(
		&r.ID, &r.WeekStart, &r.JobsFound, &r.ApplicationsSent, &r.ResponseRate, &r.InterviewRate,
		&changes, &r.CoachSummary, &payload, &r.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(changes, &r.SkillGapChanges)
	_ = json.Unmarshal(payload, &r.Payload)
	return &r, nil
}

// ---------------------------------------------------------------------------
// Automation log
// ---------------------------------------------------------------------------

func (s *Store) BumpAutomation(ctx context.Context, date time.Time, kind string, delta int) error {
	const q = `
		INSERT INTO daily_automation_log (log_date, kind, target_count, completed_count)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (log_date, kind) DO UPDATE SET
			completed_count = daily_automation_log.completed_count + $3,
			updated_at = NOW()`
	_, err := s.pool.Exec(ctx, q, date, kind, delta)
	return err
}

func (s *Store) GetAutomationLog(ctx context.Context, date time.Time) (map[string]crm.AutomationTask, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT kind, target_count, completed_count FROM daily_automation_log WHERE log_date = $1`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]crm.AutomationTask)
	for rows.Next() {
		var kind string
		var t crm.AutomationTask
		if err := rows.Scan(&kind, &t.Target, &t.Done); err != nil {
			return nil, err
		}
		t.Kind = kind
		t.Completed = t.Done >= t.Target
		out[kind] = t
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Resume analysis
// ---------------------------------------------------------------------------

func (s *Store) SaveResumeAnalysis(ctx context.Context, a *crm.ResumeAnalysis) error {
	const q = `
		INSERT INTO resume_analyses (job_id, match_score, missing_keywords, suggestions, model)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q,
		a.JobID, a.MatchScore, a.MissingKeywords, a.Suggestions, a.Model,
	).Scan(&a.ID, &a.CreatedAt)
}

func (s *Store) CollectAllSkills(ctx context.Context) ([][]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT required_skills FROM job_postings WHERE is_active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var skills []string
		if err := rows.Scan(&skills); err != nil {
			return nil, err
		}
		out = append(out, skills)
	}
	return out, rows.Err()
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefTime(p *time.Time) time.Time {
	if p == nil {
		return time.Time{}
	}
	return *p
}
