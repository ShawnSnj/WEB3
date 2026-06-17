package repository

import "strings"

// Shared SQL column list for job_matches fit engine fields.
const matchSelectCols = `
	m.id, m.fit_score, m.title_score, m.skill_score, m.backend_score, m.infra_score,
	m.remote_score, m.salary_score, m.seniority_score, m.web3_score, m.negative_score,
	m.final_score, m.fit_tier, m.company_score, m.growth_score,
	m.domain_match_score, m.career_roi_score, m.interview_probability,
	m.decision, m.why_this_matches_me, m.missing_skills, m.missing_keywords,
	m.resume_keywords_to_add, m.suggested_resume_angle, m.application_priority,
	m.resume_version_recommendation, m.user_action,
	m.pros, m.risks, m.summary, m.model, m.scored_at`

func matchSelectSQL() string {
	return strings.TrimSpace(matchSelectCols)
}
