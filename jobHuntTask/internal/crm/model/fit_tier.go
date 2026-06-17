package model

// FitTier ranks job match quality against the master resume.
type FitTier string

const (
	TierA FitTier = "A"
	TierB FitTier = "B"
	TierC FitTier = "C"
)

func TierFromScore(score int) FitTier {
	switch {
	case score >= 80:
		return TierA
	case score >= 65:
		return TierB
	default:
		return TierC
	}
}

func (t FitTier) Label() string {
	switch t {
	case TierA:
		return "A — Strong match"
	case TierB:
		return "B — Good match"
	default:
		return "C — Weak match"
	}
}

// DefaultAvoidRoles are titles/patterns to deprioritize.
func DefaultAvoidRoles() []string {
	return []string{
		"Frontend Engineer", "Junior Engineer", "Intern", "Marketing",
		"Content", "Growth", "Recruiter", "Sales", "Designer", "DevRel",
	}
}

// NormalizeMasterProfile fills canonical skill/role fields from legacy columns.
func (p *CandidateProfile) NormalizeMasterProfile() {
	if p == nil {
		return
	}
	if len(p.CoreSkills) == 0 && len(p.StrongestSkills) > 0 {
		p.CoreSkills = append([]string{}, p.StrongestSkills...)
	}
	if len(p.SecondarySkills) == 0 && len(p.MediumSkills) > 0 {
		p.SecondarySkills = append([]string{}, p.MediumSkills...)
	}
	if len(p.AvoidRoles) == 0 {
		p.AvoidRoles = DefaultAvoidRoles()
	}
	if len(p.TargetRoles) == 0 {
		p.TargetRoles = DefaultTargetRoles()
	}
	if len(p.BackendSkills) == 0 {
		p.BackendSkills = mergeUnique(p.CoreSkills, []string{"Go", "Java", "Kafka", "PostgreSQL"})
	}
	if len(p.Web3Skills) == 0 {
		p.Web3Skills = []string{"Solidity", "Blockchain", "DeFi", "Indexing", "Web3"}
	}
	if len(p.InfraSkills) == 0 {
		p.InfraSkills = []string{"Distributed Systems", "Infrastructure", "Platform", "Observability"}
	}
}

func mergeUnique(a, b []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range append(a, b...) {
		k := lower(s)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	return out
}

func lower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}
