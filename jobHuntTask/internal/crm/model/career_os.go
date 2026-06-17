package model

import (
	"time"

	"github.com/google/uuid"
)

// UserSkill is a node in the user's skill graph (Layer 3).
type UserSkill struct {
	ID       uuid.UUID `json:"id"`
	Skill    string    `json:"skill"`
	Level    int       `json:"level"`
	Category string    `json:"category"`
}

// SkillGap compares user level vs market demand.
type SkillGap struct {
	Skill     string  `json:"skill"`
	UserLevel int     `json:"user_level"`
	DemandPct float64 `json:"demand_pct"`
	GapScore  int     `json:"gap_score"`
	ROI       float64 `json:"roi"`
	Rank      int     `json:"rank,omitempty"`
}

// MarketSnapshot is daily career intelligence (Layer 2).
type MarketSnapshot struct {
	ID              uuid.UUID     `json:"id"`
	SnapshotDate    time.Time     `json:"snapshot_date"`
	SkillDemand     []SkillDemand `json:"skill_demand"`
	TrendingTech    []SkillCount  `json:"trending_tech"`
	InterviewTopics []SkillCount  `json:"interview_topics"`
	SalaryCombos    []SkillCount  `json:"salary_combos"`
	JobsAnalyzed    int           `json:"jobs_analyzed"`
	CreatedAt       time.Time     `json:"created_at"`
}

// SkillDemand is a skill with market percentage.
type SkillDemand struct {
	Skill     string  `json:"skill"`
	DemandPct float64 `json:"demand_pct"`
	JobCount  int     `json:"job_count"`
}

// CompanyProfile enriches company intelligence.
type CompanyProfile struct {
	ID              uuid.UUID  `json:"id"`
	CompanyID       *uuid.UUID `json:"company_id,omitempty"`
	CompanyName     string     `json:"company_name"`
	QualityScore    int        `json:"quality_score"`
	GrowthScore     int        `json:"growth_score"`
	InterviewTopics []string   `json:"interview_topics"`
	TechStack       []string   `json:"tech_stack"`
	IsTarget        bool       `json:"is_target"`
}

// InterviewReadiness per target company (Layer 4).
type InterviewReadiness struct {
	ID             uuid.UUID `json:"id"`
	CompanyName    string    `json:"company_name"`
	ReadinessScore int       `json:"readiness_score"`
	TargetScore    int       `json:"target_score"`
	MissingTopics  []string  `json:"missing_topics"`
	StudyTopics    []string  `json:"study_topics"`
	AnalyzedAt     time.Time `json:"analyzed_at"`
}

// OfferPrediction funnel analysis (Layer 5).
type OfferPrediction struct {
	ID              uuid.UUID      `json:"id"`
	PredictionDate  time.Time      `json:"prediction_date"`
	InterviewProb   float64        `json:"interview_prob"`
	OfferProb       float64        `json:"offer_prob"`
	Bottleneck      string         `json:"bottleneck"`
	Recommendations []string       `json:"recommendations"`
	FunnelStats     map[string]any `json:"funnel_stats"`
	CreatedAt       time.Time      `json:"created_at"`
}

// TargetCompanies for interview readiness engine.
var TargetCompanies = []string{
	"Grafana Labs", "Confluent", "GitLab", "Cloudflare",
	"Chainlink", "Alchemy", "Nethermind",
}

// DefaultSkillLevels seeds the user skill graph.
var DefaultSkillLevels = map[string]int{
	"Go": 8, "Java": 7, "Kafka": 7, "SQL": 8, "PostgreSQL": 8,
	"Distributed Systems": 8, "Cloud": 6, "AWS": 4, "Docker": 7,
	"Kubernetes": 3, "Terraform": 1, "gRPC": 7, "Redis": 6,
	"Web3": 2, "Blockchain": 2,
}

// InterviewQuestionBank maps skills to practice questions.
var InterviewQuestionBank = map[string]string{
	"Kafka":               "Explain Kafka consumer group rebalancing and how to minimize downtime during a rebalance.",
	"Terraform":           "How does Terraform state management work, and what happens when two engineers apply concurrently?",
	"Kubernetes":          "Walk through designing a zero-downtime deployment strategy for a stateful service on Kubernetes.",
	"Go":                  "Explain Go's memory model: how do goroutines communicate safely, and when would you use channels vs mutexes?",
	"AWS":                 "Design a highly available API on AWS using at least three services. How do you handle failover?",
	"Distributed Systems": "How would you implement idempotency for a payment processing service at scale?",
	"Web3":                "Explain the difference between L1 and L2 scaling solutions and their trade-offs for a backend engineer.",
	"PostgreSQL":          "How would you diagnose and fix a slow query on a 100M-row table with complex joins?",
}
