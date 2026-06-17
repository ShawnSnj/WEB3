package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/crm/aggregator"
	"github.com/shawn/jobhunttask/internal/crm/ai"
	"github.com/shawn/jobhunttask/internal/crm/engine/jobfit"
	"github.com/shawn/jobhunttask/internal/crm/engine/skillgap"
	"github.com/shawn/jobhunttask/internal/crm/matcher"
	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/repository"
)

type Clock func() time.Time

func SystemClock() time.Time { return time.Now().UTC() }

// CRM orchestrates collection, matching, daily briefs, and analytics.
type CRM struct {
	store  *repository.Store
	agg    *aggregator.Registry
	match  *matcher.Engine
	ai     *ai.Client
	clock  Clock
	log    *slog.Logger
	events EventPublisher
}

type EventPublisher interface {
	PublishJobIngested(ctx context.Context, jobID uuid.UUID) error
	PublishJobScored(ctx context.Context, jobID uuid.UUID, fitScore int) error
	PublishMissionGenerated(ctx context.Context, brief *crm.DailyBrief) error
}

type noopPublisher struct{}

func (noopPublisher) PublishJobIngested(context.Context, uuid.UUID) error    { return nil }
func (noopPublisher) PublishJobScored(context.Context, uuid.UUID, int) error { return nil }
func (noopPublisher) PublishMissionGenerated(context.Context, *crm.DailyBrief) error {
	return nil
}

func New(store *repository.Store, log *slog.Logger, aiClient *ai.Client, events EventPublisher) *CRM {
	if events == nil {
		events = noopPublisher{}
	}
	return &CRM{
		store:  store,
		agg:    aggregator.New(log.With(slog.String("component", "aggregator"))),
		match:  matcher.New(aiClient),
		ai:     aiClient,
		clock:  SystemClock,
		log:    log,
		events: events,
	}
}

// CollectJobs fetches from all sources, upserts, and publishes ingest events.
func (s *CRM) CollectJobs(ctx context.Context) (int, error) {
	raw, err := s.agg.CollectAll(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range raw {
		job, err := s.store.UpsertJob(ctx, &r)
		if err != nil {
			s.log.Warn("upsert job failed", slog.String("title", r.Title), slog.String("error", err.Error()))
			continue
		}
		count++
		if err := s.events.PublishJobIngested(ctx, job.ID); err != nil {
			s.log.Warn("kafka publish failed", slog.String("error", err.Error()))
		}
	}
	return count, nil
}

// ScorePending scores unscored jobs against the master resume profile.
func (s *CRM) ScorePending(ctx context.Context, limit int) (int, error) {
	candidate, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return 0, err
	}
	candidate.NormalizeMasterProfile()
	ids, err := s.store.ListUnscoredJobIDs(ctx, limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, id := range ids {
		job, err := s.store.GetJob(ctx, id)
		if err != nil {
			continue
		}
		m := jobfit.Score(candidate, job)
		s.enhanceMatch(m, job)
		if err := s.store.UpsertMatch(ctx, m); err != nil {
			continue
		}
		done++
		if s.events != nil {
			_ = s.events.PublishJobScored(ctx, id, m.FinalScore)
		}
	}
	return done, nil
}

// ScoreJob scores a single job against the candidate master profile.
func (s *CRM) ScoreJob(ctx context.Context, jobID uuid.UUID) (*crm.JobMatch, error) {
	candidate, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return nil, err
	}
	candidate.NormalizeMasterProfile()
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	m := jobfit.Score(candidate, job)
	s.enhanceMatch(m, job)
	if err := s.store.UpsertMatch(ctx, m); err != nil {
		return nil, err
	}
	if s.events != nil {
		_ = s.events.PublishJobScored(ctx, jobID, m.FinalScore)
	}
	return m, nil
}

// RunDailyPipeline collects, scores, runs intelligence layers, and generates today's mission.
func (s *CRM) RunDailyPipeline(ctx context.Context) error {
	if _, err := s.CollectJobs(ctx); err != nil {
		s.log.Warn("collect failed", slog.String("error", err.Error()))
	}
	if _, err := s.ScorePending(ctx, 200); err != nil {
		s.log.Warn("score failed", slog.String("error", err.Error()))
	}
	return s.RunIntelligencePipeline(ctx)
}

func (s *CRM) ListJobs(ctx context.Context, f repository.JobFilter) ([]crm.JobPosting, error) {
	return s.store.ListJobs(ctx, f)
}

func (s *CRM) GetJob(ctx context.Context, id uuid.UUID) (*crm.JobPosting, error) {
	return s.store.GetJob(ctx, id)
}

func (s *CRM) GetProfile(ctx context.Context) (*crm.UserProfile, error) {
	return s.store.GetProfile(ctx)
}

func (s *CRM) UpdateProfile(ctx context.Context, p *crm.UserProfile) error {
	return s.store.UpdateProfile(ctx, p)
}

// GenerateDailyBrief builds today's mission via Layer 6.
func (s *CRM) GenerateDailyBrief(ctx context.Context) (*crm.DailyBrief, error) {
	profile, err := s.store.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	topJobs, _ := s.RecommendedJobs(ctx, 3)
	var topJob *crm.JobPosting
	if len(topJobs) > 0 {
		topJob = &topJobs[0]
	}
	brief, err := s.buildMission(ctx, profile, topJobs, topJob)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveDailyBrief(ctx, brief); err != nil {
		return nil, err
	}
	if s.events != nil {
		_ = s.events.PublishMissionGenerated(ctx, brief)
	}
	return brief, nil
}

func (s *CRM) GetDailyBrief(ctx context.Context) (*crm.DailyBrief, error) {
	today := s.clock().Truncate(24 * time.Hour)
	brief, err := s.store.GetDailyBrief(ctx, today)
	if err == crm.ErrNotFound {
		return s.GenerateDailyBrief(ctx)
	}
	if err != nil {
		return nil, err
	}
	s.enrichApplyJobs(ctx, brief)
	return brief, nil
}

func (s *CRM) enrichApplyJobs(ctx context.Context, brief *crm.DailyBrief) {
	if len(brief.ApplyJobs) >= 3 {
		return
	}
	if len(brief.ApplyJobs) == 0 && brief.ApplyJob != nil {
		brief.ApplyJobs = []crm.JobPosting{*brief.ApplyJob}
	}
	if len(brief.ApplyJobs) < 3 {
		jobs, err := s.RecommendedJobs(ctx, 3)
		if err == nil && len(jobs) > 0 {
			brief.ApplyJobs = jobs
			if brief.ApplyJob == nil {
				brief.ApplyJob = &jobs[0]
			}
		}
	}
}

func (s *CRM) syntheticContacts(job *crm.JobPosting) []crm.Contact {
	return []crm.Contact{
		{FullName: "Engineering Manager", CompanyName: job.CompanyName, Title: "Engineering Manager", RoleType: crm.RoleEngineeringManager},
		{FullName: "Technical Recruiter", CompanyName: job.CompanyName, Title: "Recruiter", RoleType: crm.RoleRecruiter},
	}
}

// Applications
func (s *CRM) ListApplications(ctx context.Context) ([]crm.Application, error) {
	return s.store.ListApplications(ctx)
}

// RecommendedJobs returns A/B tier matches not yet acted on.
func (s *CRM) RecommendedJobs(ctx context.Context, limit int) ([]crm.JobPosting, error) {
	if limit <= 0 {
		limit = 5
	}
	applied, err := s.store.ListAppliedJobIDs(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := s.store.ListJobs(ctx, s.defaultFitFilter(50))
	if err != nil {
		return nil, err
	}
	var out []crm.JobPosting
	for _, j := range jobs {
		if j.Match != nil && j.Match.UserAction == "skipped" {
			continue
		}
		if _, done := applied[j.ID]; done {
			continue
		}
		out = append(out, j)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *CRM) defaultFitFilter(limit int) repository.JobFilter {
	return repository.JobFilter{
		Tier:           "AB",
		MinFinalScore:  65,
		ScoredOnly:     true,
		ExcludeSkipped: true,
		HideFrontend:   true,
		HideJunior:     true,
		HideNonRemote:  true,
		HideMarketing:  true,
		Limit:          limit,
		Active:         true,
	}
}

func (s *CRM) SaveApplication(ctx context.Context, jobID uuid.UUID, status crm.ApplicationStatus) (*crm.Application, error) {
	if existing, err := s.store.GetApplicationByJobID(ctx, jobID); err == nil {
		return existing, nil
	}
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	now := s.clock()
	a := &crm.Application{
		JobID:       &jobID,
		CompanyName: job.CompanyName,
		RoleTitle:   job.Title,
		Status:      status,
	}
	if status == crm.AppApplied {
		a.AppliedAt = &now
	}
	if job.Match != nil {
		score := job.Match.FitScore
		a.MatchScore = &score
	}
	if err := s.store.CreateApplication(ctx, a); err != nil {
		return nil, err
	}
	_ = s.store.BumpAutomation(ctx, now.Truncate(24*time.Hour), "application", 1)
	return a, nil
}

func (s *CRM) UpdateApplicationStatus(ctx context.Context, id uuid.UUID, status crm.ApplicationStatus) error {
	var appliedAt *time.Time
	if status == crm.AppApplied {
		t := s.clock()
		appliedAt = &t
	}
	return s.store.UpdateApplicationStatus(ctx, id, status, appliedAt)
}

func (s *CRM) LatestSkillGaps(ctx context.Context) (*crm.SkillAnalysis, error) {
	a, err := s.store.LatestSkillAnalysis(ctx)
	if err == crm.ErrNotFound {
		return s.AnalyzeSkillGaps(ctx)
	}
	return a, err
}

// AnalyzeSkillGaps runs Layer 3 skill gap engine.
func (s *CRM) AnalyzeSkillGaps(ctx context.Context) (*crm.SkillAnalysis, error) {
	userSkills, err := s.store.ListUserSkills(ctx)
	if err != nil {
		return nil, err
	}
	counts, total, err := s.store.SkillCounts(ctx)
	if err != nil {
		return nil, err
	}
	normalized := make(map[string]int)
	for sk, c := range counts {
		normalized[strings.ToLower(sk)] += c
	}
	normalized = skillgap.RelevantCounts(normalized)

	gaps, priority := skillgap.Analyze(skillgap.Input{
		UserSkills:  userSkills,
		SkillCounts: normalized,
		TotalJobs:   total,
	})

	// Build legacy top_demanded and missing_skills for backwards compat
	top := sortSkills(normalized, 20)
	profile, _ := s.store.GetProfile(ctx)
	var missing []crm.SkillCount
	if profile != nil {
		profileSet := make(map[string]struct{})
		for _, sk := range profile.Skills {
			profileSet[strings.ToLower(sk)] = struct{}{}
		}
		for _, sc := range top {
			if _, ok := profileSet[strings.ToLower(sc.Skill)]; !ok {
				missing = append(missing, sc)
			}
		}
	}

	a := &crm.SkillAnalysis{
		AnalysisDate:     s.clock().Truncate(24 * time.Hour),
		TopDemanded:      top,
		MissingSkills:    missing,
		LearningPriority: priority,
		SkillGaps:        gaps,
		JobsAnalyzed:     total,
	}
	if err := s.store.SaveSkillAnalysis(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func sortSkills(counts map[string]int, limit int) []crm.SkillCount {
	type pair struct {
		skill string
		count int
	}
	var pairs []pair
	for s, c := range counts {
		pairs = append(pairs, pair{skill: strings.Title(s), count: c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].skill < pairs[j].skill
		}
		return pairs[i].count > pairs[j].count
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]crm.SkillCount, len(pairs))
	for i, p := range pairs {
		out[i] = crm.SkillCount{Skill: p.skill, Count: p.count, Rank: i + 1}
	}
	return out
}

// AnalyzeResume compares profile resume to a job.
func (s *CRM) AnalyzeResume(ctx context.Context, jobID uuid.UUID) (*crm.ResumeAnalysis, error) {
	profile, err := s.store.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	var analysis *crm.ResumeAnalysis
	if s.ai != nil && s.ai.Enabled() && profile.ResumeText != "" {
		analysis, err = s.ai.AnalyzeResume(ctx, profile.ResumeText, job)
		if err != nil {
			s.log.Warn("ai resume analysis failed", slog.String("error", err.Error()))
		}
	}
	if analysis == nil {
		analysis = heuristicResume(profile, job)
	}
	if err := s.store.SaveResumeAnalysis(ctx, analysis); err != nil {
		return nil, err
	}
	return analysis, nil
}

func heuristicResume(profile *crm.UserProfile, job *crm.JobPosting) *crm.ResumeAnalysis {
	text := strings.ToLower(job.Description + " " + strings.Join(job.RequiredSkills, " "))
	var missing []string
	hits := 0
	for _, sk := range job.RequiredSkills {
		if strings.Contains(strings.ToLower(profile.ResumeText+strings.Join(profile.Skills, " ")), strings.ToLower(sk)) {
			hits++
		} else {
			missing = append(missing, sk)
		}
	}
	if len(job.RequiredSkills) == 0 {
		for _, sk := range profile.Skills {
			if strings.Contains(text, strings.ToLower(sk)) {
				hits++
			}
		}
	}
	score := 65
	if len(job.RequiredSkills) > 0 {
		score = hits * 100 / len(job.RequiredSkills)
	}
	suggestions := []string{
		"Lead with distributed systems impact metrics",
		"Mirror job keywords in your top 3 bullets",
	}
	if len(missing) > 0 {
		suggestions = append(suggestions, "Address gap: "+missing[0])
	}
	return &crm.ResumeAnalysis{
		JobID:           job.ID,
		MatchScore:      score,
		MissingKeywords: missing,
		Suggestions:     suggestions,
		Model:           "heuristic",
	}
}

// GenerateOutreach creates a personalized DM.
func (s *CRM) GenerateOutreach(ctx context.Context, contactID, jobID uuid.UUID) (string, error) {
	profile, err := s.store.GetProfile(ctx)
	if err != nil {
		return "", err
	}
	job, err := s.store.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}
	contacts, err := s.store.ListContacts(ctx, 100)
	if err != nil {
		return "", err
	}
	var contact *crm.Contact
	for i := range contacts {
		if contacts[i].ID == contactID {
			contact = &contacts[i]
			break
		}
	}
	if contact == nil {
		// Synthetic contact from brief
		synth := s.syntheticContacts(job)
		if len(synth) > 0 {
			c := synth[0]
			contact = &c
		}
	}
	if contact == nil {
		return "", crm.ErrNotFound
	}
	if s.ai != nil && s.ai.Enabled() {
		return s.ai.GenerateOutreach(ctx, profile, contact, job)
	}
	return fmt.Sprintf(
		"Hi %s — I'm a senior backend engineer (Go/Kafka/distributed systems) exploring the %s role at %s. I've shipped high-throughput services and would love to learn more about the team's infrastructure challenges. Open to a brief chat this week?",
		contact.FullName, job.Title, job.CompanyName,
	), nil
}

func (s *CRM) ListContacts(ctx context.Context) ([]crm.Contact, error) {
	return s.store.ListContacts(ctx, 100)
}

func (s *CRM) CompleteOutreach(ctx context.Context) error {
	return s.store.BumpAutomation(ctx, s.clock().Truncate(24*time.Hour), "outreach", 1)
}

// GenerateWeeklyReport builds the weekly CRM review.
func (s *CRM) GenerateWeeklyReport(ctx context.Context) (*crm.WeeklyCRMReport, error) {
	now := s.clock()
	weekStart := now.AddDate(0, 0, -int(now.Weekday())).Truncate(24 * time.Hour)
	jobsFound, _ := s.store.CountActiveJobs(ctx)
	sent, responses, interviews, _ := s.store.ApplicationStats(ctx, weekStart)

	respRate := 0.0
	if sent > 0 {
		respRate = float64(responses) / float64(sent) * 100
	}
	intRate := 0.0
	if sent > 0 {
		intRate = float64(interviews) / float64(sent) * 100
	}

	prevAnalysis, _ := s.store.LatestSkillAnalysis(ctx)
	currAnalysis, _ := s.AnalyzeSkillGaps(ctx)
	var gapChanges []crm.SkillCount
	if prevAnalysis != nil && currAnalysis != nil {
		gapChanges = currAnalysis.LearningPriority
		if len(gapChanges) > 5 {
			gapChanges = gapChanges[:5]
		}
	}

	stats := map[string]any{
		"jobs_found": jobsFound, "applications_sent": sent,
		"response_rate": respRate, "interview_rate": intRate,
	}
	coach := "Focus on high-fit remote backend roles. Send follow-ups 5 days after applying."
	if s.ai != nil && s.ai.Enabled() {
		if summary, err := s.ai.CoachSummary(ctx, stats); err == nil {
			coach = summary
		}
	}

	report := &crm.WeeklyCRMReport{
		WeekStart:        weekStart,
		JobsFound:        jobsFound,
		ApplicationsSent: sent,
		ResponseRate:     respRate,
		InterviewRate:    intRate,
		SkillGapChanges:  gapChanges,
		CoachSummary:     coach,
		Payload:          stats,
	}
	if err := s.store.SaveWeeklyReport(ctx, report); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *CRM) GetWeeklyReport(ctx context.Context) (*crm.WeeklyCRMReport, error) {
	now := s.clock()
	weekStart := now.AddDate(0, 0, -int(now.Weekday())).Truncate(24 * time.Hour)
	report, err := s.store.GetWeeklyReport(ctx, weekStart)
	if err == crm.ErrNotFound {
		return s.GenerateWeeklyReport(ctx)
	}
	return report, err
}

func (s *CRM) ApplicationAnalytics(ctx context.Context) (map[string]any, error) {
	apps, err := s.store.ListApplications(ctx)
	if err != nil {
		return nil, err
	}
	byStatus := make(map[string]int)
	for _, a := range apps {
		byStatus[string(a.Status)]++
	}
	return map[string]any{
		"total":     len(apps),
		"by_status": byStatus,
	}, nil
}

func (s *CRM) Coach(ctx context.Context) (map[string]any, error) {
	report, err := s.GetWeeklyReport(ctx)
	if err != nil {
		return nil, err
	}
	skills, _ := s.store.LatestSkillAnalysis(ctx)
	return map[string]any{
		"weekly_report": report,
		"skill_gaps":    skills,
		"recommendations": []string{
			report.CoachSummary,
			"Prioritize jobs with fit score ≥ 80",
			"Batch outreach to engineering managers at top-fit companies",
		},
	}, nil
}
