package jobintel

import (
	"strings"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// Known high-quality infra companies (Layer 1 company quality).
var qualityCompanies = map[string]int{
	"grafana labs": 90, "confluent": 92, "gitlab": 88, "cloudflare": 95,
	"hashicorp": 88, "datadog": 90, "stripe": 93, "coinbase": 85,
	"chainlink": 85, "alchemy": 87, "nethermind": 83, "mongodb": 86,
	"elastic": 84, "redis": 82, "vercel": 80, "supabase": 78,
	"temporal": 82, "cockroach labs": 84, "planetscale": 80,
}

// ScoreCompanyQuality returns 0-100 company quality score.
func ScoreCompanyQuality(companyName string) int {
	lower := strings.ToLower(companyName)
	for name, score := range qualityCompanies {
		if strings.Contains(lower, name) || strings.Contains(name, lower) {
			return score
		}
	}
	// Web3 companies get moderate boost
	if strings.Contains(lower, "crypto") || strings.Contains(lower, "blockchain") ||
		strings.Contains(lower, "web3") || strings.Contains(lower, "defi") {
		return 65
	}
	return 50
}

// ScoreGrowthPotential returns 0-100 based on role signals.
func ScoreGrowthPotential(job *crm.JobPosting) int {
	text := strings.ToLower(job.Title + " " + job.Description + " " + job.Seniority)
	score := 50
	if strings.Contains(text, "staff") || strings.Contains(text, "principal") {
		score += 20
	}
	if strings.Contains(text, "senior") {
		score += 10
	}
	for _, kw := range []string{"platform", "infrastructure", "distributed", "scale", "architect"} {
		if strings.Contains(text, kw) {
			score += 5
		}
	}
	if job.Web3 {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	return score
}

// EnhanceMatch adds company and growth scores to a job match.
func EnhanceMatch(m *crm.JobMatch, job *crm.JobPosting) {
	m.CompanyScore = ScoreCompanyQuality(job.CompanyName)
	m.GrowthScore = ScoreGrowthPotential(job)
	// Recalculate fit with new dimensions
	m.FitScore = (m.SkillScore*35 + m.RemoteScore*15 + m.SalaryScore*15 +
		m.SeniorityScore*10 + m.Web3Score*10 + m.CompanyScore*10 + m.GrowthScore*5) / 100
	if m.CompanyScore >= 85 {
		m.Pros = appendUnique(m.Pros, "Top-tier company")
	}
	if m.GrowthScore >= 75 {
		m.Pros = appendUnique(m.Pros, "Strong career growth signals")
	}
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
