package market

import (
	"math"
	"sort"
	"strings"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// Input for career intelligence analysis.
type Input struct {
	SkillCounts map[string]int // skill -> job count
	TotalJobs   int
	JobTexts    []string // descriptions for topic mining
}

// Analyze produces a market snapshot from active job data (Layer 2).
func Analyze(in Input) *crm.MarketSnapshot {
	if in.TotalJobs == 0 {
		in.TotalJobs = 1
	}

	demand := skillDemand(in.SkillCounts, in.TotalJobs)
	trending := topSkills(in.SkillCounts, 8)
	topics := mineInterviewTopics(in.JobTexts)
	combos := salarySkillCombos(in.SkillCounts)

	return &crm.MarketSnapshot{
		SkillDemand:     demand,
		TrendingTech:    trending,
		InterviewTopics: topics,
		SalaryCombos:    combos,
		JobsAnalyzed:    in.TotalJobs,
	}
}

func skillDemand(counts map[string]int, total int) []crm.SkillDemand {
	type pair struct {
		skill string
		count int
	}
	var pairs []pair
	for s, c := range counts {
		pairs = append(pairs, pair{skill: titleCase(s), count: c})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	if len(pairs) > 15 {
		pairs = pairs[:15]
	}
	out := make([]crm.SkillDemand, len(pairs))
	for i, p := range pairs {
		out[i] = crm.SkillDemand{
			Skill:     p.skill,
			DemandPct: math.Round(float64(p.count)/float64(total)*1000) / 10,
			JobCount:  p.count,
		}
	}
	return out
}

func topSkills(counts map[string]int, limit int) []crm.SkillCount {
	type pair struct {
		skill string
		count int
	}
	var pairs []pair
	for s, c := range counts {
		pairs = append(pairs, pair{skill: titleCase(s), count: c})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]crm.SkillCount, len(pairs))
	for i, p := range pairs {
		out[i] = crm.SkillCount{Skill: p.skill, Count: p.count, Rank: i + 1}
	}
	return out
}

var topicKeywords = []string{
	"system design", "distributed", "kafka", "kubernetes", "terraform",
	"concurrency", "database", "sql", "microservices", "api design",
	"observability", "monitoring", "scaling", "performance", "security",
}

func mineInterviewTopics(texts []string) []crm.SkillCount {
	counts := make(map[string]int)
	combined := strings.ToLower(strings.Join(texts, " "))
	for _, kw := range topicKeywords {
		if strings.Contains(combined, kw) {
			counts[kw] = strings.Count(combined, kw)
		}
	}
	return topSkills(counts, 8)
}

func salarySkillCombos(counts map[string]int) []crm.SkillCount {
	// High-value infra combos for senior backend roles
	combos := map[string]int{
		"Go + Kafka":           min2(counts, "go", "kafka"),
		"Go + Kubernetes":      min2(counts, "go", "kubernetes"),
		"Java + Kafka":         min2(counts, "java", "kafka"),
		"AWS + Terraform":      min2(counts, "aws", "terraform"),
		"Go + Distributed":     counts["distributed systems"],
		"PostgreSQL + Go":      min2(counts, "postgresql", "go"),
	}
	return topSkills(combos, 6)
}

func min2(counts map[string]int, a, b string) int {
	ca, cb := counts[a], counts[b]
	if ca < cb {
		return ca
	}
	return cb
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
