package jobfit_test

import (
	"testing"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/engine/jobfit"
)

func TestScoreBackendApply(t *testing.T) {
	candidate := &crm.CandidateProfile{
		CoreSkills:     []string{"Go", "Kafka", "PostgreSQL"},
		BackendSkills:  []string{"Go", "Java", "Distributed Systems"},
		TargetRoles:    []string{"Senior Backend Engineer"},
		SeniorityLevel: "Senior",
		TargetRegions:  []string{"US Remote"},
	}
	job := &crm.JobPosting{
		Title:       "Senior Backend Engineer",
		CompanyName: "Grafana Labs",
		Remote:      true,
		Description: "Go, Kafka, PostgreSQL, distributed systems, observability platform backend",
	}
	m := jobfit.Score(candidate, job)
	if m.FitTier != crm.TierA && m.FitTier != crm.TierB {
		t.Fatalf("tier = %q score=%d want A or B", m.FitTier, m.FinalScore)
	}
	if m.Decision == crm.FitSkip {
		t.Fatalf("decision = skip for backend role, score=%d", m.FinalScore)
	}
	if m.WhyThisMatchesMe == "" {
		t.Fatal("expected why_this_matches_me")
	}
}

func TestScoreSkipFrontend(t *testing.T) {
	candidate := &crm.CandidateProfile{
		CoreSkills:  []string{"Go", "Kafka"},
		AvoidRoles:  []string{"Frontend Engineer"},
		TargetRoles: []string{"Backend Engineer"},
	}
	job := &crm.JobPosting{
		Title: "Frontend Engineer", Remote: true,
		Description: "React, TypeScript, CSS, frontend architecture",
	}
	m := jobfit.Score(candidate, job)
	if m.FitTier != crm.TierC {
		t.Fatalf("tier = %q want C", m.FitTier)
	}
	if m.Decision != crm.FitSkip {
		t.Fatalf("decision = %q want skip", m.Decision)
	}
	if m.NegativeScore < 30 {
		t.Fatalf("negative = %d want high penalty", m.NegativeScore)
	}
}

func TestTierFromScore(t *testing.T) {
	if crm.TierFromScore(85) != crm.TierA {
		t.Fatal("85 should be A")
	}
	if crm.TierFromScore(70) != crm.TierB {
		t.Fatal("70 should be B")
	}
	if crm.TierFromScore(50) != crm.TierC {
		t.Fatal("50 should be C")
	}
}
