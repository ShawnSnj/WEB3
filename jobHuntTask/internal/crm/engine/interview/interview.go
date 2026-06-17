package interview

import (
	"strings"
	"time"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// Input for interview readiness analysis.
type Input struct {
	CompanyName     string
	InterviewTopics []string
	TechStack       []string
	UserSkills      []crm.UserSkill
}

// Analyze computes readiness for a target company (Layer 4).
func Analyze(in Input) *crm.InterviewReadiness {
	userLevels := make(map[string]int)
	for _, s := range in.UserSkills {
		userLevels[strings.ToLower(s.Skill)] = s.Level
	}

	var missing, covered []string
	totalTopics := in.InterviewTopics
	if len(totalTopics) == 0 {
		totalTopics = in.TechStack
	}
	if len(totalTopics) == 0 {
		totalTopics = []string{"Go", "Distributed Systems", "System Design"}
	}

	score := 0
	for _, topic := range totalTopics {
		topicLower := strings.ToLower(topic)
		matched := false
		for skill, level := range userLevels {
			if strings.Contains(topicLower, skill) || strings.Contains(skill, topicLower) {
				if level >= 6 {
					score += 100 / len(totalTopics)
					covered = append(covered, topic)
					matched = true
					break
				}
			}
		}
		if !matched {
			// Partial credit for related infra skills
			for _, kw := range []string{"go", "kafka", "kubernetes", "aws", "sql", "distributed"} {
				if strings.Contains(topicLower, kw) {
					if lvl, ok := userLevels[kw]; ok && lvl >= 5 {
						score += (100 / len(totalTopics)) / 2
						matched = true
						break
					}
				}
			}
		}
		if !matched {
			missing = append(missing, topic)
		}
	}
	if score > 100 {
		score = 100
	}

	studyTopics := missing
	if len(studyTopics) > 3 {
		studyTopics = studyTopics[:3]
	}

	return &crm.InterviewReadiness{
		CompanyName:    in.CompanyName,
		ReadinessScore: score,
		TargetScore:    80,
		MissingTopics:  missing,
		StudyTopics:    studyTopics,
		AnalyzedAt:     time.Now().UTC(),
	}
}

// PickQuestion selects today's interview practice question.
func PickQuestion(topGap crm.SkillGap, readiness []crm.InterviewReadiness) (topic, context string) {
	// Prefer gap skill question
	if q, ok := crm.InterviewQuestionBank[topGap.Skill]; ok {
		return topGap.Skill, q
	}
	// Fall back to lowest readiness company topic
	lowest := 100
	var company string
	for _, r := range readiness {
		if r.ReadinessScore < lowest {
			lowest = r.ReadinessScore
			company = r.CompanyName
		}
	}
	if company != "" {
		for _, r := range readiness {
			if r.CompanyName == company && len(r.StudyTopics) > 0 {
				topic = r.StudyTopics[0]
				if q, ok := crm.InterviewQuestionBank[topic]; ok {
					return topic, "Prepare for " + company + ": " + q
				}
				return topic, "Study " + topic + " for your " + company + " interview track"
			}
		}
	}
	return "Distributed Systems", crm.InterviewQuestionBank["Distributed Systems"]
}
