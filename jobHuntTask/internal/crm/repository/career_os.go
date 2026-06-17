package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// ---------------------------------------------------------------------------
// User Skills (Layer 3)
// ---------------------------------------------------------------------------

func (s *Store) ListUserSkills(ctx context.Context) ([]crm.UserSkill, error) {
	const q = `SELECT id, skill, level, category FROM user_skills ORDER BY skill`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []crm.UserSkill
	for rows.Next() {
		var us crm.UserSkill
		if err := rows.Scan(&us.ID, &us.Skill, &us.Level, &us.Category); err != nil {
			return nil, err
		}
		out = append(out, us)
	}
	return out, rows.Err()
}

func (s *Store) UpsertUserSkill(ctx context.Context, skill, category string, level int) error {
	const q = `
		INSERT INTO user_skills (skill, level, category)
		VALUES ($1, $2, $3)
		ON CONFLICT (skill) DO UPDATE SET
			level = EXCLUDED.level,
			category = EXCLUDED.category`
	_, err := s.pool.Exec(ctx, q, skill, level, category)
	return err
}

// ---------------------------------------------------------------------------
// Market Snapshots (Layer 2)
// ---------------------------------------------------------------------------

func (s *Store) SaveMarketSnapshot(ctx context.Context, snap *crm.MarketSnapshot) error {
	demand, _ := json.Marshal(snap.SkillDemand)
	trending, _ := json.Marshal(snap.TrendingTech)
	topics, _ := json.Marshal(snap.InterviewTopics)
	combos, _ := json.Marshal(snap.SalaryCombos)
	const q = `
		INSERT INTO market_snapshots (snapshot_date, skill_demand, trending_tech, interview_topics, salary_combos, jobs_analyzed)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (snapshot_date) DO UPDATE SET
			skill_demand = EXCLUDED.skill_demand,
			trending_tech = EXCLUDED.trending_tech,
			interview_topics = EXCLUDED.interview_topics,
			salary_combos = EXCLUDED.salary_combos,
			jobs_analyzed = EXCLUDED.jobs_analyzed
		RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q,
		snap.SnapshotDate, demand, trending, topics, combos, snap.JobsAnalyzed,
	).Scan(&snap.ID, &snap.CreatedAt)
}

func (s *Store) LatestMarketSnapshot(ctx context.Context) (*crm.MarketSnapshot, error) {
	const q = `
		SELECT id, snapshot_date, skill_demand, trending_tech, interview_topics, salary_combos, jobs_analyzed, created_at
		FROM market_snapshots ORDER BY snapshot_date DESC LIMIT 1`
	var snap crm.MarketSnapshot
	var demand, trending, topics, combos []byte
	err := s.pool.QueryRow(ctx, q).Scan(
		&snap.ID, &snap.SnapshotDate, &demand, &trending, &topics, &combos, &snap.JobsAnalyzed, &snap.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(demand, &snap.SkillDemand)
	_ = json.Unmarshal(trending, &snap.TrendingTech)
	_ = json.Unmarshal(topics, &snap.InterviewTopics)
	_ = json.Unmarshal(combos, &snap.SalaryCombos)
	return &snap, nil
}

// ---------------------------------------------------------------------------
// Company Profiles
// ---------------------------------------------------------------------------

func (s *Store) ListTargetCompanies(ctx context.Context) ([]crm.CompanyProfile, error) {
	const q = `
		SELECT id, company_id, company_name, quality_score, growth_score, interview_topics, tech_stack, is_target
		FROM company_profiles WHERE is_target = TRUE ORDER BY quality_score DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []crm.CompanyProfile
	for rows.Next() {
		var cp crm.CompanyProfile
		if err := rows.Scan(&cp.ID, &cp.CompanyID, &cp.CompanyName, &cp.QualityScore, &cp.GrowthScore,
			&cp.InterviewTopics, &cp.TechStack, &cp.IsTarget); err != nil {
			return nil, err
		}
		out = append(out, cp)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Interview Readiness (Layer 4)
// ---------------------------------------------------------------------------

func (s *Store) SaveInterviewReadiness(ctx context.Context, r *crm.InterviewReadiness) error {
	const q = `
		INSERT INTO interview_readiness (company_name, readiness_score, target_score, missing_topics, study_topics, analyzed_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (company_name) DO UPDATE SET
			readiness_score = EXCLUDED.readiness_score,
			target_score = EXCLUDED.target_score,
			missing_topics = EXCLUDED.missing_topics,
			study_topics = EXCLUDED.study_topics,
			analyzed_at = EXCLUDED.analyzed_at
		RETURNING id`
	return s.pool.QueryRow(ctx, q,
		r.CompanyName, r.ReadinessScore, r.TargetScore, r.MissingTopics, r.StudyTopics, r.AnalyzedAt,
	).Scan(&r.ID)
}

func (s *Store) ListInterviewReadiness(ctx context.Context) ([]crm.InterviewReadiness, error) {
	const q = `
		SELECT id, company_name, readiness_score, target_score, missing_topics, study_topics, analyzed_at
		FROM interview_readiness ORDER BY readiness_score ASC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []crm.InterviewReadiness
	for rows.Next() {
		var r crm.InterviewReadiness
		if err := rows.Scan(&r.ID, &r.CompanyName, &r.ReadinessScore, &r.TargetScore,
			&r.MissingTopics, &r.StudyTopics, &r.AnalyzedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Offer Predictions (Layer 5)
// ---------------------------------------------------------------------------

func (s *Store) SaveOfferPrediction(ctx context.Context, p *crm.OfferPrediction) error {
	stats, _ := json.Marshal(p.FunnelStats)
	const q = `
		INSERT INTO offer_predictions (prediction_date, interview_prob, offer_prob, bottleneck, recommendations, funnel_stats)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (prediction_date) DO UPDATE SET
			interview_prob = EXCLUDED.interview_prob,
			offer_prob = EXCLUDED.offer_prob,
			bottleneck = EXCLUDED.bottleneck,
			recommendations = EXCLUDED.recommendations,
			funnel_stats = EXCLUDED.funnel_stats
		RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q,
		p.PredictionDate, p.InterviewProb, p.OfferProb, p.Bottleneck, p.Recommendations, stats,
	).Scan(&p.ID, &p.CreatedAt)
}

func (s *Store) LatestOfferPrediction(ctx context.Context) (*crm.OfferPrediction, error) {
	const q = `
		SELECT id, prediction_date, interview_prob, offer_prob, bottleneck, recommendations, funnel_stats, created_at
		FROM offer_predictions ORDER BY prediction_date DESC LIMIT 1`
	var p crm.OfferPrediction
	var stats []byte
	err := s.pool.QueryRow(ctx, q).Scan(
		&p.ID, &p.PredictionDate, &p.InterviewProb, &p.OfferProb,
		&p.Bottleneck, &p.Recommendations, &stats, &p.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, crm.ErrNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(stats, &p.FunnelStats)
	return &p, nil
}

// ---------------------------------------------------------------------------
// Funnel stats helper
// ---------------------------------------------------------------------------

func (s *Store) FunnelStats(ctx context.Context, since time.Time) (applied, responses, interviews, offers int, avgFit float64, err error) {
	const q = `
		SELECT
			COUNT(*) FILTER (WHERE status IN ('applied','interview','technical','final_round','offer','rejected')),
			COUNT(*) FILTER (WHERE status IN ('interview','technical','final_round','offer')),
			COUNT(*) FILTER (WHERE status IN ('interview','technical','final_round')),
			COUNT(*) FILTER (WHERE status = 'offer'),
			COALESCE(AVG(match_score) FILTER (WHERE match_score IS NOT NULL), 0)
		FROM applications WHERE created_at >= $1`
	err = s.pool.QueryRow(ctx, q, since).Scan(&applied, &responses, &interviews, &offers, &avgFit)
	return
}

func (s *Store) CountOutreachSince(ctx context.Context, since time.Time) (int, error) {
	const q = `SELECT COALESCE(SUM(completed_count), 0) FROM daily_automation_log WHERE kind = 'outreach' AND log_date >= $1::date`
	var n int
	err := s.pool.QueryRow(ctx, q, since).Scan(&n)
	return n, err
}

func (s *Store) CollectJobDescriptions(ctx context.Context, limit int) ([]string, error) {
	const q = `SELECT description FROM job_postings WHERE is_active = TRUE ORDER BY updated_at DESC LIMIT $1`
	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) SkillCounts(ctx context.Context) (map[string]int, int, error) {
	allSkills, err := s.CollectAllSkills(ctx)
	if err != nil {
		return nil, 0, err
	}
	counts := make(map[string]int)
	for _, skills := range allSkills {
		for _, sk := range skills {
			counts[strings.ToLower(sk)]++
		}
	}
	return counts, len(allSkills), nil
}
