package matcher

import (
	"context"
	"strings"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/ai"
)

// Engine scores jobs against the user profile.
type Engine struct {
	AI *ai.Client
}

func New(aiClient *ai.Client) *Engine {
	return &Engine{AI: aiClient}
}

func (e *Engine) Score(ctx context.Context, profile *crm.UserProfile, job *crm.JobPosting) (*crm.JobMatch, error) {
	if e.AI != nil && e.AI.Enabled() {
		if m, err := e.AI.ScoreJob(ctx, profile, job); err == nil {
			return m, nil
		}
	}
	return heuristicScore(profile, job), nil
}

func heuristicScore(profile *crm.UserProfile, job *crm.JobPosting) *crm.JobMatch {
	text := strings.ToLower(job.Title + " " + job.Description + " " + strings.Join(job.RequiredSkills, " "))
	skillScore := scoreSkills(profile.Skills, text, job.RequiredSkills)
	remoteScore := scoreRemote(profile.RemoteOnly, job.Remote, job.Location)
	salaryScore := scoreSalary(profile.MinSalaryUSD, job.SalaryMinUSD, job.SalaryMaxUSD)
	seniorityScore := scoreSeniority(profile.TargetTitles, job.Title, job.Seniority)
	web3Score := scoreWeb3(profile.Web3Preferred, job.Web3, text)

	fit := (skillScore*40 + remoteScore*20 + salaryScore*15 + seniorityScore*15 + web3Score*10) / 100

	pros := []string{}
	risks := []string{}

	if skillScore >= 70 {
		pros = append(pros, "Strong skill overlap")
	}
	for _, s := range profile.Skills {
		if strings.Contains(text, strings.ToLower(s)) {
			pros = append(pros, capitalize(s)+" experience relevant")
			if len(pros) >= 3 {
				break
			}
		}
	}
	if job.Remote || remoteScore >= 80 {
		pros = append(pros, "Remote friendly")
	}
	if job.Web3 && profile.Web3Preferred {
		pros = append(pros, "Web3 aligned")
	}

	missing := findMissing(profile.Skills, text)
	for _, m := range missing {
		if m == "Terraform" || m == "Kubernetes" || m == "Aws" {
			risks = append(risks, m+" required")
		}
	}
	if salaryScore < 50 && profile.MinSalaryUSD > 0 {
		risks = append(risks, "Salary may be below target")
	}
	if !job.Remote && profile.RemoteOnly {
		risks = append(risks, "Not fully remote")
	}

	return &crm.JobMatch{
		JobID:          job.ID,
		FitScore:       fit,
		SkillScore:     skillScore,
		RemoteScore:    remoteScore,
		SalaryScore:    salaryScore,
		SeniorityScore: seniorityScore,
		Web3Score:      web3Score,
		Pros:           dedupe(pros),
		Risks:          dedupe(risks),
		Summary:        "Heuristic match based on skills, remote policy, and seniority.",
		Model:          "heuristic",
	}
}

func scoreSkills(profileSkills []string, text string, required []string) int {
	if len(profileSkills) == 0 {
		return 50
	}
	hits := 0
	for _, s := range profileSkills {
		if strings.Contains(text, strings.ToLower(s)) {
			hits++
		}
	}
	base := hits * 100 / len(profileSkills)
	if len(required) > 0 {
		reqHits := 0
		for _, r := range required {
			if strings.Contains(text, strings.ToLower(r)) {
				reqHits++
			}
		}
		base = (base + reqHits*100/len(required)) / 2
	}
	if base > 100 {
		return 100
	}
	return base
}

func scoreRemote(remoteOnly, jobRemote bool, location string) int {
	loc := strings.ToLower(location)
	if jobRemote || strings.Contains(loc, "remote") {
		return 100
	}
	if remoteOnly {
		return 20
	}
	return 60
}

func scoreSalary(min int, jobMin, jobMax *int) int {
	if min == 0 {
		return 70
	}
	if jobMax != nil && *jobMax >= min {
		return 100
	}
	if jobMin != nil && *jobMin >= min {
		return 90
	}
	if jobMin == nil && jobMax == nil {
		return 60
	}
	return 40
}

func scoreSeniority(targets []string, title, seniority string) int {
	t := strings.ToLower(title + " " + seniority)
	for _, target := range targets {
		if strings.Contains(t, strings.ToLower(target)) {
			return 95
		}
	}
	if strings.Contains(t, "backend") || strings.Contains(t, "platform") || strings.Contains(t, "infrastructure") {
		return 80
	}
	return 50
}

func scoreWeb3(preferred, jobWeb3 bool, text string) int {
	if !preferred {
		return 70
	}
	if jobWeb3 || strings.Contains(text, "web3") || strings.Contains(text, "blockchain") {
		return 100
	}
	return 40
}

func findMissing(profileSkills []string, text string) []string {
	demand := []string{"terraform", "kubernetes", "aws", "ci/cd", "rust", "solidity"}
	var out []string
	profileSet := make(map[string]struct{})
	for _, s := range profileSkills {
		profileSet[strings.ToLower(s)] = struct{}{}
	}
	for _, d := range demand {
		if strings.Contains(text, d) {
			if _, ok := profileSet[d]; !ok {
				out = append(out, capitalize(d))
			}
		}
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
