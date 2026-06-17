package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// Client wraps OpenAI chat completions. When APIKey is empty, callers should
// use heuristic fallbacks in the matcher package.
type Client struct {
	APIKey  string
	Model   string
	BaseURL string
	client  *http.Client
}

func NewClient(apiKey, model, baseURL string) *Client {
	if model == "" {
		model = "gpt-4o-mini"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.APIKey != ""
}

type matchResponse struct {
	FitScore       int      `json:"fit_score"`
	SkillScore     int      `json:"skill_score"`
	RemoteScore    int      `json:"remote_score"`
	SalaryScore    int      `json:"salary_score"`
	SeniorityScore int      `json:"seniority_score"`
	Web3Score      int      `json:"web3_score"`
	Pros           []string `json:"pros"`
	Risks          []string `json:"risks"`
	Summary        string   `json:"summary"`
}

func (c *Client) ScoreJob(ctx context.Context, profile *crm.UserProfile, job *crm.JobPosting) (*crm.JobMatch, error) {
	prompt := fmt.Sprintf(`You are a technical recruiter. Score this job for the candidate.

Candidate skills: %s
Target titles: %s
Remote only: %v
Web3 preferred: %v
Min salary USD: %d

Job title: %s
Company: %s
Remote: %v
Web3: %v
Seniority: %s
Skills mentioned: %s
Description excerpt: %s

Respond with JSON only:
{"fit_score":0-100,"skill_score":0-100,"remote_score":0-100,"salary_score":0-100,"seniority_score":0-100,"web3_score":0-100,"pros":["..."],"risks":["..."],"summary":"one sentence"}`,
		strings.Join(profile.Skills, ", "),
		strings.Join(profile.TargetTitles, ", "),
		profile.RemoteOnly, profile.Web3Preferred, profile.MinSalaryUSD,
		job.Title, job.CompanyName, job.Remote, job.Web3, job.Seniority,
		strings.Join(job.RequiredSkills, ", "),
		truncate(job.Description, 2000),
	)
	var out matchResponse
	if err := c.chatJSON(ctx, prompt, &out); err != nil {
		return nil, err
	}
	return &crm.JobMatch{
		JobID:          job.ID,
		FitScore:       clamp(out.FitScore),
		SkillScore:     clamp(out.SkillScore),
		RemoteScore:    clamp(out.RemoteScore),
		SalaryScore:    clamp(out.SalaryScore),
		SeniorityScore: clamp(out.SeniorityScore),
		Web3Score:      clamp(out.Web3Score),
		Pros:           out.Pros,
		Risks:          out.Risks,
		Summary:        out.Summary,
		Model:          c.Model,
		ScoredAt:       time.Now().UTC(),
	}, nil
}

type resumeResponse struct {
	MatchScore      int      `json:"match_score"`
	MissingKeywords []string `json:"missing_keywords"`
	Suggestions     []string `json:"suggestions"`
}

func (c *Client) AnalyzeResume(ctx context.Context, resume string, job *crm.JobPosting) (*crm.ResumeAnalysis, error) {
	prompt := fmt.Sprintf(`Compare this resume to the job description. Respond JSON only:
{"match_score":0-100,"missing_keywords":["..."],"suggestions":["..."]}

Resume:
%s

Job: %s at %s
Description:
%s`,
		truncate(resume, 4000),
		job.Title, job.CompanyName,
		truncate(job.Description, 2000),
	)
	var out resumeResponse
	if err := c.chatJSON(ctx, prompt, &out); err != nil {
		return nil, err
	}
	return &crm.ResumeAnalysis{
		JobID:           job.ID,
		MatchScore:      clamp(out.MatchScore),
		MissingKeywords: out.MissingKeywords,
		Suggestions:     out.Suggestions,
		Model:           c.Model,
	}, nil
}

func (c *Client) GenerateOutreach(ctx context.Context, profile *crm.UserProfile, contact *crm.Contact, job *crm.JobPosting) (string, error) {
	prompt := fmt.Sprintf(`Write a concise LinkedIn DM (max 120 words) from this backend engineer to %s (%s at %s) about %s — %s.
Highlight relevant experience: %s.
Tone: professional, specific, not salesy. No subject line.`,
		contact.FullName, contact.Title, contact.CompanyName,
		job.Title, job.CompanyName,
		strings.Join(profile.Skills[:min(5, len(profile.Skills))], ", "),
	)
	var body string
	if err := c.chatText(ctx, prompt, &body); err != nil {
		return "", err
	}
	return strings.TrimSpace(body), nil
}

func (c *Client) CoachSummary(ctx context.Context, stats map[string]any) (string, error) {
	prompt := fmt.Sprintf(`You are a career coach for a senior backend engineer job search. Given weekly stats, write 3-4 actionable sentences.
Stats: %v`, stats)
	var body string
	if err := c.chatText(ctx, prompt, &body); err != nil {
		return "", err
	}
	return strings.TrimSpace(body), nil
}

func (c *Client) chatJSON(ctx context.Context, prompt string, dest any) error {
	text, err := c.complete(ctx, prompt)
	if err != nil {
		return err
	}
	text = extractJSON(text)
	return json.Unmarshal([]byte(text), dest)
}

func (c *Client) chatText(ctx context.Context, prompt string, dest *string) error {
	text, err := c.complete(ctx, prompt)
	if err != nil {
		return err
	}
	*dest = text
	return nil
}

func (c *Client) complete(ctx context.Context, prompt string) (string, error) {
	payload := map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: %s", raw)
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", err
	}
	if len(envelope.Choices) == 0 {
		return "", fmt.Errorf("openai: empty response")
	}
	return strings.TrimSpace(envelope.Choices[0].Message.Content), nil
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ParseCandidateProfile uses OpenAI to enrich structured profile from resumes.
func (c *Client) ParseCandidateProfile(ctx context.Context, en, zh *crm.ResumeDocument) (*crm.ParsedResume, error) {
	var parts []string
	if en != nil && en.RawText != "" {
		parts = append(parts, "ENGLISH RESUME:\n"+truncate(en.RawText, 6000))
	}
	if zh != nil && zh.RawText != "" {
		parts = append(parts, "CHINESE RESUME:\n"+truncate(zh.RawText, 6000))
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("no resume text")
	}
	prompt := strings.Join(parts, "\n\n") + `

Extract a merged candidate profile for a Senior Backend / Web3 Infrastructure engineer.
Respond with JSON only:
{"years_of_experience":15,"seniority_level":"Senior","strongest_skills":[],"medium_skills":[],"weak_skills":[],"backend_skills":[],"web3_skills":[],"infra_skills":[],"database_skills":[],"messaging_skills":[],"major_achievements":[],"quantified_results":[],"domains":[],"resume_keywords":[]}`

	var out crm.ParsedResume
	if err := c.chatJSON(ctx, prompt, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func clamp(n int) int {
	if n < 0 {
		return 0
	}
	if n > 100 {
		return 100
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
