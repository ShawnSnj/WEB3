package skillgap

import (
	"math"
	"sort"
	"strings"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// Input for skill gap analysis.
type Input struct {
	UserSkills  []crm.UserSkill
	SkillCounts map[string]int // from job postings
	TotalJobs   int
}

// RelevantCounts keeps demand analysis focused on the user's backend/Web3 track.
func RelevantCounts(counts map[string]int) map[string]int {
	out := make(map[string]int)
	for skill, count := range counts {
		if isCareerSkill(skill) {
			out[skill] += count
		}
	}
	return out
}

// Analyze computes gap scores and ROI-ranked priorities (Layer 3).
func Analyze(in Input) ([]crm.SkillGap, []crm.SkillCount) {
	if in.TotalJobs == 0 {
		in.TotalJobs = 1
	}

	levelMap := make(map[string]int)
	for _, us := range in.UserSkills {
		levelMap[strings.ToLower(us.Skill)] = us.Level
	}
	// Fallback to defaults for skills in demand but not in user_skills
	for skill, count := range in.SkillCounts {
		key := strings.ToLower(skill)
		if _, ok := levelMap[key]; !ok {
			if lvl, ok := crm.DefaultSkillLevels[titleCase(skill)]; ok {
				levelMap[key] = lvl
			} else if containsSkill(in.UserSkills, skill) {
				levelMap[key] = 5
			} else {
				levelMap[key] = 0
			}
		}
		_ = count
	}

	var gaps []crm.SkillGap
	for skill, count := range in.SkillCounts {
		if !isCareerSkill(skill) {
			continue
		}
		key := strings.ToLower(skill)
		level := levelMap[key]
		demandPct := float64(count) / float64(in.TotalJobs) * 100
		gapScore := int(math.Round(demandPct * float64(10-level) / 10))
		if gapScore < 0 {
			gapScore = 0
		}
		roi := float64(gapScore) * math.Sqrt(demandPct/100) * focusMultiplier(skill)
		gaps = append(gaps, crm.SkillGap{
			Skill:     titleCase(skill),
			UserLevel: level,
			DemandPct: math.Round(demandPct*10) / 10,
			GapScore:  gapScore,
			ROI:       math.Round(roi*10) / 10,
		})
	}

	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].ROI == gaps[j].ROI {
			return gaps[i].GapScore > gaps[j].GapScore
		}
		return gaps[i].ROI > gaps[j].ROI
	})
	for i := range gaps {
		gaps[i].Rank = i + 1
	}

	top3 := gaps
	if len(top3) > 3 {
		top3 = top3[:3]
	}
	priority := make([]crm.SkillCount, len(top3))
	for i, g := range top3 {
		priority[i] = crm.SkillCount{
			Skill: g.Skill,
			Count: g.GapScore,
			Rank:  i + 1,
		}
	}
	return gaps, priority
}

func isCareerSkill(skill string) bool {
	key := strings.ToLower(strings.TrimSpace(skill))
	if key == "" {
		return false
	}
	if _, ok := excludedSkills[key]; ok {
		return false
	}
	if _, ok := roleSkillWeights[key]; ok {
		return true
	}
	return false
}

func focusMultiplier(skill string) float64 {
	if w, ok := roleSkillWeights[strings.ToLower(strings.TrimSpace(skill))]; ok {
		return w
	}
	return 1
}

var excludedSkills = map[string]struct{}{
	"customer support": {},
	"sales":            {},
	"marketing":        {},
	"recruiting":       {},
	"recruitment":      {},
	"hr":               {},
	"design":           {},
	"figma":            {},
}

var roleSkillWeights = map[string]float64{
	"go":                  1.35,
	"golang":              1.35,
	"java":                1.2,
	"kafka":               1.35,
	"sql":                 1.15,
	"postgresql":          1.2,
	"redis":               1.15,
	"kubernetes":          1.35,
	"k8s":                 1.35,
	"docker":              1.1,
	"aws":                 1.25,
	"gcp":                 1.15,
	"azure":               1.1,
	"terraform":           1.3,
	"grpc":                1.25,
	"distributed systems": 1.45,
	"microservices":       1.2,
	"ci/cd":               1.1,
	"rust":                1.25,
	"python":              1.1,
	"blockchain":          1.35,
	"web3":                1.35,
	"solidity":            1.35,
	"ethereum":            1.35,
	"defi":                1.25,
}

func containsSkill(skills []crm.UserSkill, skill string) bool {
	for _, s := range skills {
		if strings.EqualFold(s.Skill, skill) {
			return true
		}
	}
	return false
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, " ")
}

// LearningTopic returns a specific study topic for a skill.
func LearningTopic(skill string) string {
	topics := map[string]string{
		"Terraform":  "Terraform state management and remote backends",
		"Kubernetes": "Pod scheduling, resource limits, and HPA configuration",
		"AWS":        "IAM least-privilege patterns and VPC networking",
		"Kafka":      "Consumer group tuning and exactly-once semantics",
		"Web3":       "Smart contract interaction from backend services",
		"Go":         "Context propagation and graceful shutdown patterns",
		"Blockchain": "Node architecture and mempool mechanics",
	}
	if t, ok := topics[skill]; ok {
		return t
	}
	return skill + " fundamentals for senior backend interviews"
}
