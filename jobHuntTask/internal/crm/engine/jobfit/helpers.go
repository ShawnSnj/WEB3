package jobfit

import (
	"strings"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// UserProfileFromCandidate overlays candidate master profile onto legacy user_profile.
func UserProfileFromCandidate(c *crm.CandidateProfile, base *crm.UserProfile) *crm.UserProfile {
	if base == nil {
		base = &crm.UserProfile{}
	}
	p := *base
	if c == nil {
		return &p
	}
	c.NormalizeMasterProfile()
	skills := append(append([]string{}, c.CoreSkills...), c.SecondarySkills...)
	if len(skills) == 0 {
		skills = append(c.StrongestSkills, c.MediumSkills...)
	}
	if len(skills) > 0 {
		p.Skills = dedupe(skills)
	}
	if c.FullName != "" {
		p.DisplayName = c.FullName
	}
	if len(c.TargetRoles) > 0 {
		p.TargetTitles = c.TargetRoles
	}
	if len(c.Domains) > 0 {
		p.TargetIndustries = c.Domains
	}
	if c.TargetCompensationMinUSD > 0 {
		p.MinSalaryUSD = c.TargetCompensationMinUSD
	}
	p.RemoteOnly = true
	p.Web3Preferred = true
	return &p
}

func jobText(job *crm.JobPosting) string {
	return strings.ToLower(job.Title + " " + job.Description + " " + job.Seniority + " " + strings.Join(job.RequiredSkills, " "))
}

func scoreDomain(c *crm.CandidateProfile, text string) int {
	if c == nil || len(c.Domains) == 0 {
		return 50
	}
	hits := 0
	for _, d := range c.Domains {
		switch strings.ToLower(d) {
		case "payments", "payment":
			if strings.Contains(text, "payment") || strings.Contains(text, "fintech") || strings.Contains(text, "billing") {
				hits++
			}
		case "web3":
			if strings.Contains(text, "web3") || strings.Contains(text, "blockchain") || strings.Contains(text, "crypto") {
				hits++
			}
		case "backend infrastructure", "infrastructure":
			if strings.Contains(text, "infrastructure") || strings.Contains(text, "platform") || strings.Contains(text, "backend") {
				hits++
			}
		}
	}
	if hits == 0 {
		return 40
	}
	return minInt(100, 50+hits*25)
}

func recommendResume(c *crm.CandidateProfile, text string) string {
	web3 := strings.Contains(text, "web3") || strings.Contains(text, "blockchain") || strings.Contains(text, "defi") || strings.Contains(text, "index")
	infra := strings.Contains(text, "infrastructure") || strings.Contains(text, "platform") || strings.Contains(text, "observability")
	switch {
	case web3 && strings.Contains(text, "index"):
		return "DeFi / Indexing Backend Resume"
	case web3:
		return "Web3 Backend Engineer Resume"
	case infra:
		return "Blockchain Infrastructure / Platform Resume"
	default:
		return "Go Backend Engineer Resume"
	}
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
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
