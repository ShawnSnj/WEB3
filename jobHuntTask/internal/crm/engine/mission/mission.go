package mission

import (
	"fmt"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/engine/interview"
	"github.com/shawn/jobhunttask/internal/crm/engine/skillgap"
)

const DefaultMinutes = 30

// Input aggregates all intelligence layer outputs.
type Input struct {
	Profile          *crm.UserProfile
	TopJobs          []crm.JobPosting
	SkillGaps        []crm.SkillGap
	LearningPriority []crm.SkillCount
	Readiness        []crm.InterviewReadiness
	AutomationLog    map[string]struct{ Done, Target int }
	OutreachTargets  []crm.OutreachTarget
}

// Build assembles the daily mission (Layer 6).
func Build(in Input) *crm.DailyBrief {
	var topJob *crm.JobPosting
	if len(in.TopJobs) > 0 {
		topJob = &in.TopJobs[0]
	}
	brief := &crm.DailyBrief{
		ApplyJob:         topJob,
		ApplyJobs:        in.TopJobs,
		OutreachTargets:  in.OutreachTargets,
		AutomationTasks:  []crm.AutomationTask{},
		EstimatedMinutes: DefaultMinutes,
	}

	if topJob != nil && topJob.Match != nil {
		brief.ApplySummary = map[string]any{
			"final_score":   topJob.Match.FinalScore,
			"fit_tier":      topJob.Match.FitTier,
			"why_match":     topJob.Match.WhyThisMatchesMe,
			"missing_skills": topJob.Match.MissingSkills,
			"resume_angle":  topJob.Match.SuggestedResumeAngle,
		}
		if len(topJob.Match.ResumeKeywordsToAdd) > 0 {
			kw := topJob.Match.ResumeKeywordsToAdd
			if len(kw) > 3 {
				kw = kw[:3]
			}
			brief.ResumeImprovement = "Add keywords to resume: " + joinStrings(kw, ", ")
		} else if topJob.Match.SuggestedResumeAngle != "" {
			brief.ResumeImprovement = "Tailor resume: " + topJob.Match.SuggestedResumeAngle
		}
	}

	// Learning priority from skill gap engine
	if len(in.LearningPriority) > 0 {
		top := in.LearningPriority[0]
		brief.LearningSkill = top.Skill
		brief.LearningTopic = skillgap.LearningTopic(top.Skill)
		if len(in.SkillGaps) > 0 {
			for _, g := range in.SkillGaps {
				if g.Skill == top.Skill {
					brief.LearningReason = fmt.Sprintf(
						"Gap score %d — demanded in %.0f%% of target jobs, your level: %d/10",
						g.GapScore, g.DemandPct, g.UserLevel,
					)
					break
				}
			}
		}
		if brief.LearningReason == "" {
			brief.LearningReason = fmt.Sprintf("Highest ROI learning priority (gap score %d)", top.Count)
		}
	}

	// Interview question
	var topGap crm.SkillGap
	if len(in.SkillGaps) > 0 {
		topGap = in.SkillGaps[0]
	}
	topic, ctx := interview.PickQuestion(topGap, in.Readiness)
	brief.InterviewTopic = topic
	brief.InterviewContext = ctx

	// Automation goals
	profile := in.Profile
	if profile == nil {
		profile = &crm.UserProfile{DailyApplications: 1, DailyOutreach: 2}
	}
	appDone := 0
	outDone := 0
	if log, ok := in.AutomationLog["application"]; ok {
		appDone = log.Done
	}
	if log, ok := in.AutomationLog["outreach"]; ok {
		outDone = log.Done
	}
	brief.AutomationTasks = []crm.AutomationTask{
		{
			Kind:        "application",
			Description: "Apply to your top 3 job picks",
			Target:      profile.DailyApplications,
			Done:        appDone,
		},
		{
			Kind:        "outreach",
			Description: fmt.Sprintf("Send %d outreach messages", profile.DailyOutreach),
			Target:      profile.DailyOutreach,
			Done:        outDone,
		},
		{
			Kind:        "learning",
			Description: "Study: " + brief.LearningTopic,
			Target:      1,
			Done:        0,
		},
		{
			Kind:        "interview",
			Description: "Practice: " + brief.InterviewTopic,
			Target:      1,
			Done:        0,
		},
	}
	for i := range brief.AutomationTasks {
		brief.AutomationTasks[i].Completed = brief.AutomationTasks[i].Done >= brief.AutomationTasks[i].Target
	}

	brief.MissionPayload = map[string]any{
		"estimated_minutes":    brief.EstimatedMinutes,
		"interview_topic":      brief.InterviewTopic,
		"learning_topic":       brief.LearningTopic,
		"resume_improvement":   brief.ResumeImprovement,
		"apply_job_count":      len(brief.ApplyJobs),
		"skill_gap_task":       brief.LearningSkill,
	}
	return brief
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}
