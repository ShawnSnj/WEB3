package repository

import (
	"time"

	"github.com/google/uuid"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

func scanJobMatch(
	mid *uuid.UUID,
	fit, title, skill, backend, infra, remote, salary, seniority, web3, negative, final *int,
	company, growth, domain, roi, interview *int,
	tier, decision, why, resumeAngle, appPriority, resumeVer, userAction, summary, model *string,
	missingSkills, missingKw, resumeKw, pros, risks []string,
	scoredAt *time.Time,
	jobID uuid.UUID,
) *crm.JobMatch {
	if mid == nil {
		return nil
	}
	return &crm.JobMatch{
		ID:                          *mid,
		JobID:                       jobID,
		FitScore:                    derefInt(fit),
		TitleScore:                  derefInt(title),
		SkillScore:                  derefInt(skill),
		BackendScore:                derefInt(backend),
		InfraScore:                  derefInt(infra),
		RemoteScore:                 derefInt(remote),
		SalaryScore:                 derefInt(salary),
		SeniorityScore:              derefInt(seniority),
		Web3Score:                   derefInt(web3),
		NegativeScore:               derefInt(negative),
		FinalScore:                  derefInt(final),
		FitTier:                     crm.FitTier(derefStr(tier)),
		CompanyScore:                derefInt(company),
		GrowthScore:                 derefInt(growth),
		DomainMatchScore:            derefInt(domain),
		CareerROIScore:              derefInt(roi),
		InterviewProbability:        derefInt(interview),
		Decision:                    crm.FitDecision(derefStr(decision)),
		WhyThisMatchesMe:            derefStr(why),
		MissingSkills:               missingSkills,
		MissingKeywords:             missingKw,
		ResumeKeywordsToAdd:         resumeKw,
		SuggestedResumeAngle:        derefStr(resumeAngle),
		ApplicationPriority:         derefStr(appPriority),
		ResumeVersionRecommendation: derefStr(resumeVer),
		UserAction:                  derefStr(userAction),
		Pros:                        pros,
		Risks:                       risks,
		Summary:                     derefStr(summary),
		Model:                       derefStr(model),
		ScoredAt:                    derefTime(scoredAt),
	}
}
