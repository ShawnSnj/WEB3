package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = errors.New("crm: not found")
	ErrInvalidStatus = errors.New("crm: invalid status")
)

type ApplicationStatus string

const (
	AppSaved       ApplicationStatus = "saved"
	AppApplied     ApplicationStatus = "applied"
	AppInterview   ApplicationStatus = "interview"
	AppTechnical   ApplicationStatus = "technical"
	AppFinalRound  ApplicationStatus = "final_round"
	AppOffer       ApplicationStatus = "offer"
	AppRejected    ApplicationStatus = "rejected"
)

func (s ApplicationStatus) IsValid() bool {
	switch s {
	case AppSaved, AppApplied, AppInterview, AppTechnical, AppFinalRound, AppOffer, AppRejected:
		return true
	}
	return false
}

type ContactRole string

const (
	RoleRecruiter           ContactRole = "recruiter"
	RoleEngineeringManager  ContactRole = "engineering_manager"
	RoleHiringManager       ContactRole = "hiring_manager"
	RoleFounder             ContactRole = "founder"
	RoleOther               ContactRole = "other"
)

type UserProfile struct {
	ID                 uuid.UUID `json:"id"`
	DisplayName        string    `json:"display_name"`
	Headline           string    `json:"headline"`
	Skills             []string  `json:"skills"`
	TargetTitles       []string  `json:"target_titles"`
	TargetIndustries   []string  `json:"target_industries"`
	ResumeText         string    `json:"resume_text"`
	MinSalaryUSD       int       `json:"min_salary_usd"`
	RemoteOnly         bool      `json:"remote_only"`
	Web3Preferred      bool      `json:"web3_preferred"`
	DailyApplications  int       `json:"daily_applications"`
	DailyOutreach      int       `json:"daily_outreach"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Company struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Website   string    `json:"website"`
	Industry  string    `json:"industry"`
	SizeBand  string    `json:"size_band"`
	Web3      bool      `json:"web3"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JobPosting struct {
	ID             uuid.UUID      `json:"id"`
	ExternalID     string         `json:"external_id"`
	Source         string         `json:"source"`
	Title          string         `json:"title"`
	CompanyID      *uuid.UUID     `json:"company_id,omitempty"`
	CompanyName    string         `json:"company_name"`
	SalaryMinUSD   *int           `json:"salary_min_usd,omitempty"`
	SalaryMaxUSD   *int           `json:"salary_max_usd,omitempty"`
	SalaryRaw      string         `json:"salary_raw"`
	Location       string         `json:"location"`
	Remote         bool           `json:"remote"`
	Description    string         `json:"description"`
	RequiredSkills []string       `json:"required_skills"`
	ApplicationURL string         `json:"application_url"`
	PostedAt       *time.Time     `json:"posted_at,omitempty"`
	Seniority      string         `json:"seniority"`
	Web3           bool           `json:"web3"`
	IsActive       bool           `json:"is_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Match          *JobMatch      `json:"match,omitempty"`
}

type JobMatch struct {
	ID                          uuid.UUID   `json:"id"`
	JobID                       uuid.UUID   `json:"job_id"`
	FitScore                    int         `json:"fit_score"`
	TitleScore                  int         `json:"title_score"`
	SkillScore                  int         `json:"skill_score"`
	BackendScore                int         `json:"backend_score"`
	InfraScore                  int         `json:"infra_score"`
	RemoteScore                 int         `json:"remote_score"`
	SalaryScore                 int         `json:"salary_score"`
	SeniorityScore              int         `json:"seniority_score"`
	Web3Score                   int         `json:"web3_score"`
	NegativeScore               int         `json:"negative_score"`
	FinalScore                  int         `json:"final_score"`
	FitTier                     FitTier     `json:"fit_tier"`
	CompanyScore                int         `json:"company_score"`
	GrowthScore                 int         `json:"growth_score"`
	DomainMatchScore            int         `json:"domain_match_score"`
	CareerROIScore              int         `json:"career_roi_score"`
	InterviewProbability        int         `json:"interview_probability"`
	Decision                    FitDecision `json:"decision"`
	WhyThisMatchesMe            string      `json:"why_this_matches_me"`
	MissingSkills               []string    `json:"missing_skills"`
	MissingKeywords             []string    `json:"missing_keywords"`
	ResumeKeywordsToAdd         []string    `json:"resume_keywords_to_add"`
	SuggestedResumeAngle        string      `json:"suggested_resume_angle"`
	ApplicationPriority         string      `json:"application_priority"`
	ResumeVersionRecommendation string      `json:"resume_version_recommendation"`
	UserAction                  string      `json:"user_action,omitempty"`
	Pros                        []string    `json:"pros"`
	Risks                       []string    `json:"risks"`
	Summary                     string      `json:"summary"`
	Model                       string      `json:"model"`
	ScoredAt                    time.Time   `json:"scored_at"`
}

type Application struct {
	ID          uuid.UUID         `json:"id"`
	JobID       *uuid.UUID        `json:"job_id,omitempty"`
	CompanyName string            `json:"company_name"`
	RoleTitle   string            `json:"role_title"`
	Status      ApplicationStatus `json:"status"`
	AppliedAt   *time.Time        `json:"applied_at,omitempty"`
	Notes       string            `json:"notes"`
	ResumeScore *int              `json:"resume_score,omitempty"`
	MatchScore  *int              `json:"match_score,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type Contact struct {
	ID          uuid.UUID   `json:"id"`
	CompanyID   *uuid.UUID  `json:"company_id,omitempty"`
	CompanyName string      `json:"company_name"`
	FullName    string      `json:"full_name"`
	Title       string      `json:"title"`
	RoleType    ContactRole `json:"role_type"`
	LinkedInURL string      `json:"linkedin_url"`
	Email       string      `json:"email"`
	Notes       string      `json:"notes"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type OutreachMessage struct {
	ID        uuid.UUID  `json:"id"`
	ContactID uuid.UUID  `json:"contact_id"`
	JobID     *uuid.UUID `json:"job_id,omitempty"`
	Subject   string     `json:"subject"`
	Body      string     `json:"body"`
	Status    string     `json:"status"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type SkillAnalysis struct {
	ID               uuid.UUID  `json:"id"`
	AnalysisDate     time.Time  `json:"analysis_date"`
	TopDemanded      []SkillCount `json:"top_demanded"`
	MissingSkills    []SkillCount `json:"missing_skills"`
	LearningPriority []SkillCount `json:"learning_priority"`
	SkillGaps        []SkillGap   `json:"skill_gaps"`
	JobsAnalyzed     int        `json:"jobs_analyzed"`
	CreatedAt        time.Time  `json:"created_at"`
}

type SkillCount struct {
	Skill string `json:"skill"`
	Count int    `json:"count"`
	Rank  int    `json:"rank,omitempty"`
}

type DailyBrief struct {
	ID               uuid.UUID        `json:"id"`
	BriefDate        time.Time        `json:"brief_date"`
	ApplyJob         *JobPosting      `json:"apply_job,omitempty"`
	ApplyJobs        []JobPosting     `json:"apply_jobs,omitempty"`
	ApplySummary     map[string]any   `json:"apply_summary"`
	OutreachTargets  []OutreachTarget `json:"outreach_targets"`
	LearningSkill    string           `json:"learning_skill"`
	LearningTopic    string           `json:"learning_topic"`
	LearningReason   string           `json:"learning_reason"`
	ResumeImprovement string          `json:"resume_improvement,omitempty"`
	InterviewTopic   string           `json:"interview_topic"`
	InterviewContext string           `json:"interview_context"`
	EstimatedMinutes int              `json:"estimated_minutes"`
	AutomationTasks  []AutomationTask `json:"automation_tasks"`
	MissionPayload   map[string]any   `json:"mission_payload,omitempty"`
	GeneratedAt      time.Time        `json:"generated_at"`
}

type OutreachTarget struct {
	Contact     Contact          `json:"contact"`
	Message     *OutreachMessage `json:"message,omitempty"`
	SuggestedDM string           `json:"suggested_dm"`
}

type AutomationTask struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	Target      int    `json:"target"`
	Done        int    `json:"done"`
}

type WeeklyCRMReport struct {
	ID               uuid.UUID      `json:"id"`
	WeekStart        time.Time      `json:"week_start"`
	JobsFound        int            `json:"jobs_found"`
	ApplicationsSent int            `json:"applications_sent"`
	ResponseRate     float64        `json:"response_rate"`
	InterviewRate    float64        `json:"interview_rate"`
	SkillGapChanges  []SkillCount   `json:"skill_gap_changes"`
	CoachSummary     string         `json:"coach_summary"`
	Payload          map[string]any `json:"payload"`
	CreatedAt        time.Time      `json:"created_at"`
}

type ResumeAnalysis struct {
	ID              uuid.UUID `json:"id"`
	JobID           uuid.UUID `json:"job_id"`
	MatchScore      int       `json:"match_score"`
	MissingKeywords []string  `json:"missing_keywords"`
	Suggestions     []string  `json:"suggestions"`
	Model           string    `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
}

type RawJob struct {
	ExternalID     string
	Source         string
	Title          string
	CompanyName    string
	SalaryRaw      string
	SalaryMinUSD   *int
	SalaryMaxUSD   *int
	Location       string
	Remote         bool
	Description    string
	RequiredSkills []string
	ApplicationURL string
	PostedAt       *time.Time
	Seniority      string
	Web3           bool
	RawPayload     map[string]any
}
