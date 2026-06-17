package service

import (
	"context"
	"strings"
	"time"

	"github.com/shawn/jobhunttask/internal/crm/engine/interview"
	"github.com/shawn/jobhunttask/internal/crm/engine/jobfit"
	"github.com/shawn/jobhunttask/internal/crm/engine/jobintel"
	"github.com/shawn/jobhunttask/internal/crm/engine/market"
	"github.com/shawn/jobhunttask/internal/crm/engine/mission"
	"github.com/shawn/jobhunttask/internal/crm/engine/offer"
	"github.com/shawn/jobhunttask/internal/crm/engine/skillgap"
	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// RunIntelligencePipeline executes Layers 2–6 after job collection/scoring.
func (s *CRM) RunIntelligencePipeline(ctx context.Context) error {
	if _, err := s.AnalyzeMarket(ctx); err != nil {
		s.log.Warn("market analysis failed", "error", err.Error())
	}
	if _, err := s.AnalyzeSkillGaps(ctx); err != nil {
		s.log.Warn("skill gap analysis failed", "error", err.Error())
	}
	if _, err := s.AnalyzeInterviewReadiness(ctx); err != nil {
		s.log.Warn("interview readiness failed", "error", err.Error())
	}
	if _, err := s.AnalyzeOfferPrediction(ctx); err != nil {
		s.log.Warn("offer prediction failed", "error", err.Error())
	}
	if _, err := s.GenerateDailyBrief(ctx); err != nil {
		return err
	}
	return nil
}

// AnalyzeMarket runs Layer 2 career intelligence.
func (s *CRM) AnalyzeMarket(ctx context.Context) (*crm.MarketSnapshot, error) {
	counts, total, err := s.store.SkillCounts(ctx)
	if err != nil {
		return nil, err
	}
	// Normalize keys to lowercase for counting
	normalized := make(map[string]int)
	for sk, c := range counts {
		normalized[strings.ToLower(sk)] += c
	}
	normalized = skillgap.RelevantCounts(normalized)
	descriptions, _ := s.store.CollectJobDescriptions(ctx, 200)
	snap := market.Analyze(market.Input{
		SkillCounts: normalized,
		TotalJobs:   total,
		JobTexts:    descriptions,
	})
	snap.SnapshotDate = s.clock().Truncate(24 * time.Hour)
	if err := s.store.SaveMarketSnapshot(ctx, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// GetMarketTrends returns latest market snapshot.
func (s *CRM) GetMarketTrends(ctx context.Context) (*crm.MarketSnapshot, error) {
	snap, err := s.store.LatestMarketSnapshot(ctx)
	if err == crm.ErrNotFound {
		return s.AnalyzeMarket(ctx)
	}
	return snap, err
}

// AnalyzeInterviewReadiness runs Layer 4 for target companies.
func (s *CRM) AnalyzeInterviewReadiness(ctx context.Context) ([]crm.InterviewReadiness, error) {
	userSkills, err := s.store.ListUserSkills(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := s.store.ListTargetCompanies(ctx)
	if err != nil {
		return nil, err
	}
	var out []crm.InterviewReadiness
	for _, cp := range targets {
		r := interview.Analyze(interview.Input{
			CompanyName:     cp.CompanyName,
			InterviewTopics: cp.InterviewTopics,
			TechStack:       cp.TechStack,
			UserSkills:      userSkills,
		})
		if err := s.store.SaveInterviewReadiness(ctx, r); err != nil {
			s.log.Warn("save interview readiness failed", "company", cp.CompanyName)
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

// GetInterviewReadiness returns all company readiness scores.
func (s *CRM) GetInterviewReadiness(ctx context.Context) ([]crm.InterviewReadiness, error) {
	out, err := s.store.ListInterviewReadiness(ctx)
	if err != nil || len(out) == 0 {
		analyzed, analyzeErr := s.AnalyzeInterviewReadiness(ctx)
		if analyzeErr != nil {
			return nil, analyzeErr
		}
		return analyzed, nil
	}
	return out, err
}

// AnalyzeOfferPrediction runs Layer 5 funnel analysis.
func (s *CRM) AnalyzeOfferPrediction(ctx context.Context) (*crm.OfferPrediction, error) {
	since := s.clock().AddDate(0, 0, -30)
	applied, responses, interviews, offers, avgFit, err := s.store.FunnelStats(ctx, since)
	if err != nil {
		return nil, err
	}
	outreach, _ := s.store.CountOutreachSince(ctx, s.clock().AddDate(0, 0, -7))
	apps, _ := s.store.ListApplications(ctx)

	respRate := 0.0
	intRate := 0.0
	if applied > 0 {
		respRate = float64(responses) / float64(applied) * 100
		intRate = float64(interviews) / float64(applied) * 100
	}

	topGap := 0
	if analysis, err := s.store.LatestSkillAnalysis(ctx); err == nil && len(analysis.SkillGaps) > 0 {
		topGap = analysis.SkillGaps[0].GapScore
	}

	pred := offer.Analyze(offer.FunnelStats{
		TotalApplications: len(apps),
		Applied:           applied,
		Responses:         responses,
		Interviews:        interviews,
		Offers:            offers,
		ResponseRate:      respRate,
		InterviewRate:     intRate,
		AvgFitScore:       avgFit,
		WeeklyOutreach:    outreach,
		TopGapScore:       topGap,
	})
	if err := s.store.SaveOfferPrediction(ctx, pred); err != nil {
		return nil, err
	}
	return pred, nil
}

// GetOfferPrediction returns latest offer prediction.
func (s *CRM) GetOfferPrediction(ctx context.Context) (*crm.OfferPrediction, error) {
	p, err := s.store.LatestOfferPrediction(ctx)
	if err == crm.ErrNotFound {
		return s.AnalyzeOfferPrediction(ctx)
	}
	return p, err
}

// GetUserSkills returns the user's skill graph.
func (s *CRM) GetUserSkills(ctx context.Context) ([]crm.UserSkill, error) {
	return s.store.ListUserSkills(ctx)
}

// enhanceMatch applies Layer 1 company/growth scoring and refreshes fit tier.
func (s *CRM) enhanceMatch(m *crm.JobMatch, job *crm.JobPosting) {
	jobintel.EnhanceMatch(m, job)
	jobfit.FinalizeScores(m)
}

// buildMission assembles Layer 6 daily mission.
func (s *CRM) buildMission(ctx context.Context, profile *crm.UserProfile, topJobs []crm.JobPosting, topJob *crm.JobPosting) (*crm.DailyBrief, error) {
	today := s.clock().Truncate(24 * time.Hour)
	skillAnalysis, _ := s.store.LatestSkillAnalysis(ctx)
	readiness, _ := s.store.ListInterviewReadiness(ctx)
	autoLog, _ := s.store.GetAutomationLog(ctx, today)

	outreach := s.buildOutreachTargets(ctx, profile, topJob)

	var gaps []crm.SkillGap
	var priority []crm.SkillCount
	if skillAnalysis != nil {
		gaps = skillAnalysis.SkillGaps
		priority = skillAnalysis.LearningPriority
	}

	logMap := make(map[string]struct{ Done, Target int })
	for k, v := range autoLog {
		logMap[k] = struct{ Done, Target int }{Done: v.Done, Target: v.Target}
	}

	brief := mission.Build(mission.Input{
		Profile:          profile,
		TopJobs:          topJobs,
		SkillGaps:        gaps,
		LearningPriority: priority,
		Readiness:        readiness,
		AutomationLog:    logMap,
		OutreachTargets:  outreach,
	})
	brief.BriefDate = today
	return brief, nil
}

func (s *CRM) buildOutreachTargets(ctx context.Context, profile *crm.UserProfile, topJob *crm.JobPosting) []crm.OutreachTarget {
	if topJob == nil {
		return nil
	}
	contacts, _ := s.store.ContactsForCompany(ctx, topJob.CompanyName, 2)
	if len(contacts) < profile.DailyOutreach {
		contacts = append(contacts, s.syntheticContacts(topJob)...)
	}
	var targets []crm.OutreachTarget
	for i, c := range contacts {
		if i >= profile.DailyOutreach {
			break
		}
		target := crm.OutreachTarget{Contact: c}
		if msg, err := s.GenerateOutreach(ctx, c.ID, topJob.ID); err == nil && msg != "" {
			target.SuggestedDM = msg
		}
		targets = append(targets, target)
	}
	return targets
}
