package jobfit

import (
	"strings"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

var preferSkills = []string{
	"go", "golang", "kafka", "postgresql", "postgres", "solidity",
	"defi", "indexing", "blockchain", "distributed", "infrastructure",
}

var backendSignals = []string{
	"backend", "platform", "infrastructure", "distributed", "microservice",
	"api", "service", "go", "golang", "java", "kafka", "postgresql",
}

var web3Signals = []string{
	"web3", "blockchain", "crypto", "defi", "solidity", "ethereum",
	"indexing", "indexer", "protocol", "wallet", "on-chain",
}

var infraSignals = []string{
	"infrastructure", "platform", "observability", "kubernetes", "terraform",
	"distributed", "scalability", "reliability", "sre", "devops",
}

// Score computes personalized fit from the master resume profile.
func Score(candidate *crm.CandidateProfile, job *crm.JobPosting) *crm.JobMatch {
	if job == nil {
		return &crm.JobMatch{Model: "job-fit-engine"}
	}
	c := cloneCandidate(candidate)
	c.NormalizeMasterProfile()
	text := jobText(job)

	m := &crm.JobMatch{
		JobID: job.ID,
		Model: "job-fit-engine",
	}

	m.TitleScore = scoreTitle(c, job.Title, text)
	m.SkillScore = scoreSkillOverlap(c, text, job.RequiredSkills)
	m.Web3Score = scoreSignalList(c.Web3Skills, web3Signals, text)
	m.BackendScore = scoreSignalList(c.BackendSkills, backendSignals, text)
	m.InfraScore = scoreSignalList(c.InfraSkills, infraSignals, text)
	m.SeniorityScore = scoreSeniority(c, job.Title, job.Seniority, text)
	m.RemoteScore = scoreRemote(c, job)
	m.SalaryScore = scoreSalary(c.TargetCompensationMinUSD, job.SalaryMinUSD, job.SalaryMaxUSD)
	m.NegativeScore = scoreNegative(c, text, job)
	m.DomainMatchScore = scoreDomain(c, text)

	preferBoost := scorePrefer(text)
	m.FinalScore = calcFinalScore(m, preferBoost)
	m.FitScore = m.FinalScore
	m.FitTier = crm.TierFromScore(m.FinalScore)
	m.CareerROIScore = m.FinalScore
	m.InterviewProbability = clamp((m.FinalScore*70 + m.SkillScore*30) / 100)

	buildExplanations(m, c, job, text)
	m.Decision = decisionFromTier(m)
	m.ResumeVersionRecommendation = m.SuggestedResumeAngle
	m.Summary = m.FitTier.Label() + " · score " + itoa(m.FinalScore)
	return m
}

// EnrichMatch scores a job using the master resume (replaces legacy enrich path).
func EnrichMatch(m *crm.JobMatch, candidate *crm.CandidateProfile, job *crm.JobPosting) {
	scored := Score(candidate, job)
	if m == nil {
		return
	}
	scored.ID = m.ID
	scored.CompanyScore = m.CompanyScore
	scored.GrowthScore = m.GrowthScore
	scored.UserAction = m.UserAction
	if scored.CompanyScore > 0 || scored.GrowthScore > 0 {
		scored.FinalScore = clamp(scored.FinalScore + (scored.CompanyScore+scored.GrowthScore)/20)
		scored.FitScore = scored.FinalScore
		scored.FitTier = crm.TierFromScore(scored.FinalScore)
		scored.CareerROIScore = scored.FinalScore
		scored.Decision = decisionFromTier(scored)
		scored.Summary = scored.FitTier.Label() + " · score " + itoa(scored.FinalScore)
	}
	*m = *scored
}

func cloneCandidate(c *crm.CandidateProfile) *crm.CandidateProfile {
	if c == nil {
		return &crm.CandidateProfile{}
	}
	cp := *c
	return &cp
}

func scoreTitle(c *crm.CandidateProfile, title, text string) int {
	t := strings.ToLower(title)
	score := 40
	for _, role := range c.TargetRoles {
		r := strings.ToLower(role)
		if strings.Contains(t, r) || strings.Contains(text, r) {
			score += 25
		}
	}
	for _, avoid := range c.AvoidRoles {
		if strings.Contains(t, strings.ToLower(avoid)) {
			score -= 30
		}
	}
	if strings.Contains(t, "backend") || strings.Contains(t, "platform") || strings.Contains(t, "infrastructure") {
		score += 15
	}
	return clamp(score)
}

func scoreSkillOverlap(c *crm.CandidateProfile, text string, required []string) int {
	all := append(append(append([]string{}, c.CoreSkills...), c.SecondarySkills...), c.WeakSkills...)
	if len(all) == 0 {
		all = append(c.StrongestSkills, c.MediumSkills...)
	}
	if len(all) == 0 {
		return 40
	}
	hits := 0
	for _, s := range all {
		if strings.Contains(text, strings.ToLower(s)) {
			hits++
		}
	}
	base := hits * 100 / len(all)
	if len(required) > 0 {
		reqHits := 0
		for _, r := range required {
			if strings.Contains(text, strings.ToLower(r)) {
				reqHits++
			}
		}
		base = (base + reqHits*100/len(required)) / 2
	}
	return clamp(base)
}

func scoreSignalList(profileSkills, signals []string, text string) int {
	hits := 0
	for _, sig := range signals {
		if strings.Contains(text, sig) {
			hits++
		}
	}
	for _, s := range profileSkills {
		if strings.Contains(text, strings.ToLower(s)) {
			hits++
		}
	}
	if hits == 0 {
		return 35
	}
	return clamp(40 + hits*8)
}

func scoreSeniority(c *crm.CandidateProfile, title, seniority, text string) int {
	combined := strings.ToLower(title + " " + seniority + " " + text)
	if strings.Contains(combined, "junior") || strings.Contains(combined, "intern") || strings.Contains(combined, "entry") {
		return 10
	}
	want := strings.ToLower(c.SeniorityLevel)
	if want == "" {
		want = "senior"
	}
	switch {
	case strings.Contains(combined, "staff") || strings.Contains(combined, "principal"):
		return 95
	case strings.Contains(combined, "senior") || strings.Contains(combined, "sr."):
		return 90
	case strings.Contains(combined, "lead"):
		return 85
	default:
		return 70
	}
}

func scoreRemote(c *crm.CandidateProfile, job *crm.JobPosting) int {
	loc := strings.ToLower(job.Location)
	if job.Remote || strings.Contains(loc, "remote") || strings.Contains(loc, "async") {
		return 100
	}
	for _, r := range c.TargetRegions {
		if strings.Contains(loc, strings.ToLower(r)) {
			return 80
		}
	}
	return 25
}

func scoreSalary(minTarget int, jobMin, jobMax *int) int {
	if minTarget <= 0 {
		return 70
	}
	if jobMin != nil && *jobMin >= minTarget {
		return 100
	}
	if jobMax != nil && *jobMax >= minTarget {
		return 85
	}
	return 45
}

func scoreNegative(c *crm.CandidateProfile, text string, job *crm.JobPosting) int {
	penalty := 0
	frontendHeavy := (strings.Contains(text, "frontend") || strings.Contains(text, "react") || strings.Contains(text, "vue")) &&
		!strings.Contains(text, "backend")
	if frontendHeavy {
		penalty += 35
	}
	if strings.Contains(text, "marketing") || strings.Contains(text, " content ") || strings.Contains(text, "growth ") {
		penalty += 30
	}
	if strings.Contains(text, "junior") || strings.Contains(text, "intern") || strings.Contains(text, "entry level") {
		penalty += 40
	}
	if !job.Remote && !strings.Contains(strings.ToLower(job.Location), "remote") {
		penalty += 25
	}
	for _, avoid := range c.AvoidRoles {
		if strings.Contains(text, strings.ToLower(avoid)) {
			penalty += 15
		}
	}
	return clamp(penalty)
}

func scorePrefer(text string) int {
	hits := 0
	for _, p := range preferSkills {
		if strings.Contains(text, p) {
			hits++
		}
	}
	return minInt(hits*3, 15)
}

func calcFinalScore(m *crm.JobMatch, preferBoost int) int {
	raw := (m.TitleScore*15 + m.SkillScore*20 + m.Web3Score*10 + m.BackendScore*15 +
		m.InfraScore*10 + m.SeniorityScore*10 + m.RemoteScore*10 + m.SalaryScore*10) / 100
	raw += preferBoost
	raw -= m.NegativeScore / 2
	return clamp(raw)
}

func buildExplanations(m *crm.JobMatch, c *crm.CandidateProfile, job *crm.JobPosting, text string) {
	var reasons []string
	for _, s := range append(c.CoreSkills, c.BackendSkills...) {
		if strings.Contains(text, strings.ToLower(s)) {
			reasons = append(reasons, "Your "+s+" experience aligns with this role")
			if len(reasons) >= 3 {
				break
			}
		}
	}
	if m.RemoteScore >= 80 {
		reasons = append(reasons, "Remote/async-friendly for your US/EU target")
	}
	if job.Web3 || m.Web3Score >= 70 {
		reasons = append(reasons, "Web3/blockchain backend focus matches your profile")
	}
	if m.BackendScore >= 70 {
		reasons = append(reasons, "Backend/platform engineering emphasis")
	}
	m.WhyThisMatchesMe = strings.Join(reasons, ". ")
	if m.WhyThisMatchesMe == "" {
		m.WhyThisMatchesMe = "Partial overlap with your Go/Kafka/PostgreSQL backend profile"
	}

	m.MissingSkills = findMissingSkills(c, text)
	m.MissingKeywords = m.MissingSkills
	m.ResumeKeywordsToAdd = append([]string{}, m.MissingSkills...)
	m.SuggestedResumeAngle = recommendResume(c, text)
	m.ApplicationPriority = priorityFromTier(m.FitTier)

	m.Pros = reasons
	if len(m.Pros) > 3 {
		m.Pros = m.Pros[:3]
	}
	m.Risks = negativeRisks(m.NegativeScore, text)
}

func findMissingSkills(c *crm.CandidateProfile, text string) []string {
	known := map[string]struct{}{}
	for _, s := range append(append(c.CoreSkills, c.SecondarySkills...), c.WeakSkills...) {
		known[strings.ToLower(s)] = struct{}{}
	}
	demand := []string{"go", "kafka", "postgresql", "solidity", "terraform", "kubernetes", "rust", "grpc", "defi", "indexing"}
	var out []string
	for _, d := range demand {
		if strings.Contains(text, d) {
			if _, ok := known[d]; !ok {
				out = append(out, strings.ToUpper(d[:1])+d[1:])
			}
		}
	}
	return out
}

func negativeRisks(neg int, text string) []string {
	var risks []string
	if strings.Contains(text, "frontend") && !strings.Contains(text, "backend") {
		risks = append(risks, "Frontend-heavy role")
	}
	if strings.Contains(text, "junior") || strings.Contains(text, "intern") {
		risks = append(risks, "Junior-level role")
	}
	if strings.Contains(text, "marketing") || strings.Contains(text, "content") {
		risks = append(risks, "Marketing/content focus")
	}
	if neg >= 30 {
		risks = append(risks, "Multiple mismatch signals")
	}
	return dedupe(risks)
}

func priorityFromTier(t crm.FitTier) string {
	switch t {
	case crm.TierA:
		return "high"
	case crm.TierB:
		return "medium"
	default:
		return "low"
	}
}

func decisionFromTier(m *crm.JobMatch) crm.FitDecision {
	if m.NegativeScore >= 40 || m.FinalScore < 65 {
		return crm.FitSkip
	}
	switch m.FitTier {
	case crm.TierA:
		return crm.FitApply
	case crm.TierB:
		return crm.FitMaybe
	default:
		return crm.FitSkip
	}
}

// FinalizeScores applies company/growth boost and refreshes tier/decision.
func FinalizeScores(m *crm.JobMatch) {
	if m == nil {
		return
	}
	if m.CompanyScore > 0 || m.GrowthScore > 0 {
		m.FinalScore = clamp(m.FinalScore + (m.CompanyScore+m.GrowthScore)/20)
	}
	m.FitScore = m.FinalScore
	m.FitTier = crm.TierFromScore(m.FinalScore)
	m.CareerROIScore = m.FinalScore
	m.Decision = decisionFromTier(m)
	m.ApplicationPriority = priorityFromTier(m.FitTier)
	m.Summary = m.FitTier.Label() + " · score " + itoa(m.FinalScore)
}
