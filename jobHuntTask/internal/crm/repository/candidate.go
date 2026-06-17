package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// ---------------------------------------------------------------------------
// Resume documents
// ---------------------------------------------------------------------------

func (s *Store) SaveResumeDocument(ctx context.Context, doc *crm.ResumeDocument) error {
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	const q = `
		INSERT INTO resume_documents (id, language, filename, raw_text, content_type, parsed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())`
	_, err := s.pool.Exec(ctx, q,
		doc.ID, string(doc.Language), doc.Filename, doc.RawText, doc.ContentType, doc.ParsedAt,
	)
	return err
}

func (s *Store) MarkResumeParsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	const q = `UPDATE resume_documents SET parsed_at = $2, updated_at = NOW() WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, at)
	return err
}

func (s *Store) GetLatestResume(ctx context.Context, lang crm.ResumeLanguage) (*crm.ResumeDocument, error) {
	const q = `
		SELECT id, language, filename, raw_text, content_type, parsed_at, created_at, updated_at
		FROM resume_documents
		WHERE language = $1
		ORDER BY created_at DESC
		LIMIT 1`
	var doc crm.ResumeDocument
	var langStr string
	err := s.pool.QueryRow(ctx, q, string(lang)).Scan(
		&doc.ID, &langStr, &doc.Filename, &doc.RawText, &doc.ContentType,
		&doc.ParsedAt, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	doc.Language = crm.ResumeLanguage(langStr)
	return &doc, nil
}

// ---------------------------------------------------------------------------
// Candidate profile
// ---------------------------------------------------------------------------

func (s *Store) GetCandidateProfile(ctx context.Context) (*crm.CandidateProfile, error) {
	const q = `
		SELECT id, user_profile_id, full_name, location, email, years_of_experience,
		       seniority_level, target_roles, target_regions, target_compensation_min_usd,
		       strongest_skills, medium_skills, weak_skills, backend_skills, web3_skills,
		       infra_skills, database_skills, messaging_skills, leadership_experience,
		       payment_experience, web3_experience, distributed_systems_experience,
		       major_achievements, quantified_results, resume_keywords,
		       company_domain_experience, preferred_job_types, domains,
		       structured_profile, source_en_resume_id, source_zh_resume_id,
		       last_parsed_at, created_at, updated_at
		FROM candidate_profiles
		ORDER BY created_at
		LIMIT 1`
	return scanCandidateProfile(s.pool.QueryRow(ctx, q))
}

func (s *Store) UpsertCandidateProfile(ctx context.Context, p *crm.CandidateProfile) error {
	if p.ID == uuid.Nil {
		existing, err := s.GetCandidateProfile(ctx)
		if err == nil {
			p.ID = existing.ID
		} else {
			p.ID = uuid.New()
		}
	}
	structured, err := json.Marshal(p.StructuredProfile)
	if err != nil {
		structured = []byte("{}")
	}
	const q = `
		INSERT INTO candidate_profiles (
			id, user_profile_id, full_name, location, email, years_of_experience,
			seniority_level, target_roles, target_regions, target_compensation_min_usd,
			strongest_skills, medium_skills, weak_skills, backend_skills, web3_skills,
			infra_skills, database_skills, messaging_skills, leadership_experience,
			payment_experience, web3_experience, distributed_systems_experience,
			major_achievements, quantified_results, resume_keywords,
			company_domain_experience, preferred_job_types, domains,
			structured_profile, source_en_resume_id, source_zh_resume_id,
			last_parsed_at, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,
			$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,NOW(),NOW()
		)
		ON CONFLICT (id) DO UPDATE SET
			user_profile_id = EXCLUDED.user_profile_id,
			full_name = EXCLUDED.full_name,
			location = EXCLUDED.location,
			email = EXCLUDED.email,
			years_of_experience = EXCLUDED.years_of_experience,
			seniority_level = EXCLUDED.seniority_level,
			target_roles = EXCLUDED.target_roles,
			target_regions = EXCLUDED.target_regions,
			target_compensation_min_usd = EXCLUDED.target_compensation_min_usd,
			strongest_skills = EXCLUDED.strongest_skills,
			medium_skills = EXCLUDED.medium_skills,
			weak_skills = EXCLUDED.weak_skills,
			backend_skills = EXCLUDED.backend_skills,
			web3_skills = EXCLUDED.web3_skills,
			infra_skills = EXCLUDED.infra_skills,
			database_skills = EXCLUDED.database_skills,
			messaging_skills = EXCLUDED.messaging_skills,
			leadership_experience = EXCLUDED.leadership_experience,
			payment_experience = EXCLUDED.payment_experience,
			web3_experience = EXCLUDED.web3_experience,
			distributed_systems_experience = EXCLUDED.distributed_systems_experience,
			major_achievements = EXCLUDED.major_achievements,
			quantified_results = EXCLUDED.quantified_results,
			resume_keywords = EXCLUDED.resume_keywords,
			company_domain_experience = EXCLUDED.company_domain_experience,
			preferred_job_types = EXCLUDED.preferred_job_types,
			domains = EXCLUDED.domains,
			structured_profile = EXCLUDED.structured_profile,
			source_en_resume_id = EXCLUDED.source_en_resume_id,
			source_zh_resume_id = EXCLUDED.source_zh_resume_id,
			last_parsed_at = EXCLUDED.last_parsed_at,
			updated_at = NOW()`
	_, err = s.pool.Exec(ctx, q,
		p.ID, p.UserProfileID, p.FullName, p.Location, p.Email, p.YearsOfExperience,
		p.SeniorityLevel, p.TargetRoles, p.TargetRegions, p.TargetCompensationMinUSD,
		p.StrongestSkills, p.MediumSkills, p.WeakSkills, p.BackendSkills, p.Web3Skills,
		p.InfraSkills, p.DatabaseSkills, p.MessagingSkills, p.LeadershipExperience,
		p.PaymentExperience, p.Web3Experience, p.DistributedSystemsExperience,
		p.MajorAchievements, p.QuantifiedResults, p.ResumeKeywords,
		p.CompanyDomainExperience, p.PreferredJobTypes, p.Domains,
		structured, p.SourceENResumeID, p.SourceZHResumeID, p.LastParsedAt,
	)
	return err
}

func (s *Store) ReplaceCandidateSkills(ctx context.Context, profileID uuid.UUID, skills []crm.CandidateSkill) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM candidate_skills WHERE profile_id = $1`, profileID); err != nil {
		return err
	}
	const q = `
		INSERT INTO candidate_skills (id, profile_id, skill_name, category, level, strength, source, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`
	for _, sk := range skills {
		id := sk.ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		if _, err := tx.Exec(ctx, q, id, profileID, sk.SkillName, sk.Category, sk.Level, sk.Strength, sk.Source); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListCandidateSkills(ctx context.Context, profileID uuid.UUID) ([]crm.CandidateSkill, error) {
	const q = `
		SELECT id, profile_id, skill_name, category, level, strength, source, created_at, updated_at
		FROM candidate_skills
		WHERE profile_id = $1
		ORDER BY level DESC, skill_name`
	rows, err := s.pool.Query(ctx, q, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []crm.CandidateSkill
	for rows.Next() {
		var sk crm.CandidateSkill
		if err := rows.Scan(
			&sk.ID, &sk.ProfileID, &sk.SkillName, &sk.Category, &sk.Level,
			&sk.Strength, &sk.Source, &sk.CreatedAt, &sk.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, sk)
	}
	return out, rows.Err()
}

func scanCandidateProfile(row pgx.Row) (*crm.CandidateProfile, error) {
	var p crm.CandidateProfile
	var structured []byte
	err := row.Scan(
		&p.ID, &p.UserProfileID, &p.FullName, &p.Location, &p.Email, &p.YearsOfExperience,
		&p.SeniorityLevel, &p.TargetRoles, &p.TargetRegions, &p.TargetCompensationMinUSD,
		&p.StrongestSkills, &p.MediumSkills, &p.WeakSkills, &p.BackendSkills, &p.Web3Skills,
		&p.InfraSkills, &p.DatabaseSkills, &p.MessagingSkills, &p.LeadershipExperience,
		&p.PaymentExperience, &p.Web3Experience, &p.DistributedSystemsExperience,
		&p.MajorAchievements, &p.QuantifiedResults, &p.ResumeKeywords,
		&p.CompanyDomainExperience, &p.PreferredJobTypes, &p.Domains,
		&structured, &p.SourceENResumeID, &p.SourceZHResumeID,
		&p.LastParsedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	if len(structured) > 0 {
		_ = json.Unmarshal(structured, &p.StructuredProfile)
	}
	return &p, nil
}

// EnsureCandidateProfile creates a default profile if none exists.
func (s *Store) EnsureCandidateProfile(ctx context.Context) (*crm.CandidateProfile, error) {
	p, err := s.GetCandidateProfile(ctx)
	if err == nil {
		return p, nil
	}
	if err != crm.ErrNotFound {
		return nil, err
	}
	up, err := s.GetProfile(ctx)
	if err != nil {
		return nil, fmt.Errorf("ensure candidate profile: %w", err)
	}
	now := time.Now().UTC()
	p = &crm.CandidateProfile{
		ID:                       uuid.New(),
		UserProfileID:            &up.ID,
		FullName:                 up.DisplayName,
		YearsOfExperience:        15,
		SeniorityLevel:           "Senior",
		TargetRoles:              crm.DefaultTargetRoles(),
		TargetRegions:            crm.DefaultTargetRegions(),
		TargetCompensationMinUSD: max(up.MinSalaryUSD, 150000),
		CoreSkills:               []string{"Go", "Java", "Kafka", "PostgreSQL", "Distributed Systems"},
		StrongestSkills:          []string{"Go", "Java", "Kafka", "PostgreSQL", "Distributed Systems"},
		BackendSkills:            []string{"Go", "Java", "Kafka", "PostgreSQL"},
		Web3Skills:               []string{"Solidity", "Blockchain", "DeFi", "Indexing"},
		InfraSkills:              []string{"Distributed Systems", "Platform", "Infrastructure"},
		AvoidRoles:               crm.DefaultAvoidRoles(),
		WeakSkills:               crm.DefaultWeakSkills(),
		Domains:                  []string{"Payments", "Web3", "Backend Infrastructure"},
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := s.UpsertCandidateProfile(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
