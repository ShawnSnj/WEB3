package model

import (
	"time"

	"github.com/google/uuid"
)

// ResumeLanguage identifies which master resume was uploaded.
type ResumeLanguage string

const (
	ResumeEN ResumeLanguage = "en"
	ResumeZH ResumeLanguage = "zh"
)

func (l ResumeLanguage) Valid() bool {
	return l == ResumeEN || l == ResumeZH
}

// ResumeDocument stores raw resume text from an upload.
type ResumeDocument struct {
	ID          uuid.UUID      `json:"id"`
	Language    ResumeLanguage `json:"language"`
	Filename    string         `json:"filename"`
	RawText     string         `json:"raw_text"`
	ContentType string         `json:"content_type"`
	ParsedAt    *time.Time     `json:"parsed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// CandidateProfile is the merged source of truth for all matching engines.
type CandidateProfile struct {
	ID                           uuid.UUID  `json:"id"`
	UserProfileID                *uuid.UUID `json:"user_profile_id,omitempty"`
	FullName                     string     `json:"full_name"`
	Location                     string     `json:"location"`
	Email                        string     `json:"email"`
	YearsOfExperience            int        `json:"years_of_experience"`
	SeniorityLevel               string     `json:"seniority_level"`
	TargetRoles                  []string   `json:"target_roles"`
	TargetRegions                []string   `json:"target_regions"`
	TargetCompensationMinUSD     int        `json:"target_compensation_min_usd"`
	StrongestSkills              []string   `json:"strongest_skills"`
	MediumSkills                 []string   `json:"medium_skills"`
	CoreSkills                   []string   `json:"core_skills"`
	SecondarySkills              []string   `json:"secondary_skills"`
	AvoidRoles                   []string   `json:"avoid_roles"`
	WeakSkills                   []string   `json:"weak_skills"`
	BackendSkills                []string   `json:"backend_skills"`
	Web3Skills                   []string   `json:"web3_skills"`
	InfraSkills                  []string   `json:"infra_skills"`
	DatabaseSkills               []string   `json:"database_skills"`
	MessagingSkills              []string   `json:"messaging_skills"`
	LeadershipExperience         []string   `json:"leadership_experience"`
	PaymentExperience            []string   `json:"payment_experience"`
	Web3Experience               []string   `json:"web3_experience"`
	DistributedSystemsExperience []string   `json:"distributed_systems_experience"`
	MajorAchievements            []string   `json:"major_achievements"`
	QuantifiedResults            []string   `json:"quantified_results"`
	ResumeKeywords               []string   `json:"resume_keywords"`
	CompanyDomainExperience      []string   `json:"company_domain_experience"`
	PreferredJobTypes            []string   `json:"preferred_job_types"`
	Domains                      []string   `json:"domains"`
	StructuredProfile            map[string]any `json:"structured_profile,omitempty"`
	SourceENResumeID             *uuid.UUID `json:"source_en_resume_id,omitempty"`
	SourceZHResumeID             *uuid.UUID `json:"source_zh_resume_id,omitempty"`
	LastParsedAt                 *time.Time `json:"last_parsed_at,omitempty"`
	CreatedAt                    time.Time  `json:"created_at"`
	UpdatedAt                    time.Time  `json:"updated_at"`
}

// CandidateSkill is a normalized skill row linked to the profile.
type CandidateSkill struct {
	ID         uuid.UUID `json:"id"`
	ProfileID  uuid.UUID `json:"profile_id"`
	SkillName  string    `json:"skill_name"`
	Category   string    `json:"category"`
	Level      int       `json:"level"`
	Strength   string    `json:"strength"` // strong | medium | weak
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ParsedResume is the structured output from parsing one resume.
type ParsedResume struct {
	FullName                     string   `json:"full_name"`
	Location                     string   `json:"location"`
	Email                        string   `json:"email"`
	YearsOfExperience            int      `json:"years_of_experience"`
	SeniorityLevel               string   `json:"seniority_level"`
	StrongestSkills              []string `json:"strongest_skills"`
	MediumSkills                 []string `json:"medium_skills"`
	WeakSkills                   []string `json:"weak_skills"`
	BackendSkills                []string `json:"backend_skills"`
	Web3Skills                   []string `json:"web3_skills"`
	InfraSkills                  []string `json:"infra_skills"`
	DatabaseSkills               []string `json:"database_skills"`
	MessagingSkills              []string `json:"messaging_skills"`
	LeadershipExperience         []string `json:"leadership_experience"`
	PaymentExperience            []string `json:"payment_experience"`
	Web3Experience               []string `json:"web3_experience"`
	DistributedSystemsExperience []string `json:"distributed_systems_experience"`
	MajorAchievements            []string `json:"major_achievements"`
	QuantifiedResults            []string `json:"quantified_results"`
	ResumeKeywords               []string `json:"resume_keywords"`
	CompanyDomainExperience      []string `json:"company_domain_experience"`
	Domains                      []string `json:"domains"`
}

// DefaultTargetRoles for senior backend / Web3 infra search.
func DefaultTargetRoles() []string {
	return []string{
		"Go Backend Engineer",
		"Web3 Backend Engineer",
		"Blockchain Infrastructure Engineer",
		"DeFi Backend Engineer",
		"Indexing Backend Engineer",
		"Senior Backend Engineer",
		"Platform Engineer",
		"Distributed Systems Engineer",
	}
}

// DefaultTargetRegions for remote job search.
func DefaultTargetRegions() []string {
	return []string{
		"US Remote",
		"EU Remote",
		"Singapore Remote",
		"Global Remote",
	}
}

// DefaultWeakSkills from user profile spec.
func DefaultWeakSkills() []string {
	return []string{"AWS", "Kubernetes", "Terraform", "Observability", "English interviewing"}
}
