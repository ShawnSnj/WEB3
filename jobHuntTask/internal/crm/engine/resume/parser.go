package resume

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

var (
	emailRE      = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	yearsRE      = regexp.MustCompile(`(?i)(\d{1,2})\+?\s*(years?|yrs?|年)`)
	quantifiedRE = regexp.MustCompile(`(?i)(\d[\d,.\s]*(?:%|\+)?(?:\s*(?:TPS|tps|tx|transactions?|users?|USD|\$|B|M|K|days?|hours?|ms|availability|可用性|交易))?[^.\n]{0,40})`)
	cjkNameRE    = regexp.MustCompile(`^[\p{Han}·]{2,8}$`)
)

type skillRule struct {
	Name      string
	Category  string
	Strength  string // strong if mentioned in achievements section
	Patterns  []string
}

// skillCatalog maps resume text patterns to normalized skills.
var skillCatalog = []skillRule{
	{Name: "Java", Category: "backend", Patterns: []string{"java", "spring", "spring boot", "jvm"}},
	{Name: "Go", Category: "backend", Patterns: []string{" golang", " go,", ", go", " go ", "|go", "go|", "go语言", "golang", "in go", "｜go", "go｜"}},
	{Name: "Kafka", Category: "messaging", Patterns: []string{"kafka", "kafka streams", "schema registry"}},
	{Name: "SQL", Category: "database", Patterns: []string{" sql", "sql ", "mysql", "postgresql", "postgres"}},
	{Name: "Redis", Category: "database", Patterns: []string{"redis", "缓存", "快取"}},
	{Name: "Payment Systems", Category: "payment", Patterns: []string{"payment", "payments", "billing", "支付", "结算", "結算", "支付系統", "支付系统"}},
	{Name: "Settlement Systems", Category: "payment", Patterns: []string{"settlement", "清算", "结算系统", "結算系統"}},
	{Name: "Reconciliation Systems", Category: "payment", Patterns: []string{"reconciliation", "对账", "對帳"}},
	{Name: "Distributed Systems", Category: "distributed", Patterns: []string{"distributed system", "distributed systems", "microservice", "分布式", "分散式"}},
	{Name: "Event-Driven Architecture", Category: "distributed", Patterns: []string{"event-driven", "event driven", "event sourcing", "事件驱动"}},
	{Name: "Web3 Backend", Category: "web3", Patterns: []string{"web3", "blockchain backend", "链上", "区块链后端", "web3基礎設施", "web3基础设施"}},
	{Name: "DeFi Lending", Category: "web3", Patterns: []string{"defi", "lending", "借贷"}},
	{Name: "NFT Marketplace", Category: "web3", Patterns: []string{"nft", "marketplace"}},
	{Name: "Blockchain Event Indexing", Category: "web3", Patterns: []string{"indexer", "indexing", "event indexing", "链上索引"}},
	{Name: "AWS", Category: "infra", Patterns: []string{" aws", "amazon web services", "ec2", "s3", "lambda"}},
	{Name: "Kubernetes", Category: "infra", Patterns: []string{"kubernetes", " k8s", "k8s "}},
	{Name: "Terraform", Category: "infra", Patterns: []string{"terraform", "iac", "infrastructure as code"}},
	{Name: "Observability", Category: "infra", Patterns: []string{"observability", "prometheus", "grafana", "datadog", "tracing", "可观测"}},
	{Name: "Docker", Category: "infra", Patterns: []string{"docker", "container"}},
	{Name: "gRPC", Category: "backend", Patterns: []string{"grpc", "protobuf"}},
}

var leadershipPatterns = []string{
	"led ", "lead ", "managed", "mentor", "tech lead", "architect", "principal",
	"带领", "负责", "架构", "团队",
}

var domainPatterns = map[string][]string{
	"Payments":              {"payment", "fintech", "billing", "支付", "金融"},
	"Web3":                  {"web3", "blockchain", "crypto", "defi", "区块链"},
	"Backend Infrastructure": {"infrastructure", "platform", "backend", "infra", "基础设施"},
}

// Parse extracts structured fields from raw resume text.
func Parse(raw string, lang crm.ResumeLanguage) crm.ParsedResume {
	text := normalizeText(raw)
	matchText := normalizeForMatch(text)
	lower := strings.ToLower(matchText)

	out := crm.ParsedResume{
		SeniorityLevel: inferSeniority(lower),
		ResumeKeywords: uniqueStrings(extractKeywords(lower)),
	}

	if email := emailRE.FindString(text); email != "" {
		out.Email = email
	}
	out.YearsOfExperience = inferYears(lower)
	out.FullName = inferName(text, lang)
	out.Location = inferLocation(text)

	matched := matchSkills(lower)
	out.StrongestSkills = matched.strong
	out.MediumSkills = matched.medium
	out.WeakSkills = matched.weak
	out.BackendSkills = matched.byCategory["backend"]
	out.Web3Skills = matched.byCategory["web3"]
	out.InfraSkills = matched.byCategory["infra"]
	out.DatabaseSkills = matched.byCategory["database"]
	out.MessagingSkills = matched.byCategory["messaging"]
	out.PaymentExperience = uniqueStrings(matched.byCategory["payment"])
	out.Web3Experience = uniqueStrings(out.Web3Skills)
	out.DistributedSystemsExperience = uniqueStrings(matched.byCategory["distributed"])
	out.LeadershipExperience = extractLeadership(text)
	out.MajorAchievements = extractAchievements(text)
	out.QuantifiedResults = extractQuantified(text)
	out.CompanyDomainExperience = extractDomains(lower)
	out.Domains = out.CompanyDomainExperience

	// Default weak skills if not detected but user target profile expects them
	for _, w := range crm.DefaultWeakSkills() {
		if !containsSkill(out.StrongestSkills, w) && !containsSkill(out.MediumSkills, w) {
			if !containsSkill(out.WeakSkills, w) {
				out.WeakSkills = append(out.WeakSkills, w)
			}
		}
	}
	out.WeakSkills = uniqueStrings(out.WeakSkills)

	return out
}

// Merge combines English and Chinese parsed resumes into one profile.
func Merge(en, zh crm.ParsedResume) crm.ParsedResume {
	out := crm.ParsedResume{
		FullName:                     mergeName(en.FullName, zh.FullName),
		Location:                     mergeLocation(en.Location, zh.Location),
		Email:                        coalesce(en.Email, zh.Email),
		YearsOfExperience:            maxInt(en.YearsOfExperience, zh.YearsOfExperience),
		SeniorityLevel:               coalesce(en.SeniorityLevel, zh.SeniorityLevel),
		StrongestSkills:              union(en.StrongestSkills, zh.StrongestSkills),
		MediumSkills:                 union(en.MediumSkills, zh.MediumSkills),
		WeakSkills:                   union(en.WeakSkills, zh.WeakSkills),
		BackendSkills:                union(en.BackendSkills, zh.BackendSkills),
		Web3Skills:                   union(en.Web3Skills, zh.Web3Skills),
		InfraSkills:                  union(en.InfraSkills, zh.InfraSkills),
		DatabaseSkills:               union(en.DatabaseSkills, zh.DatabaseSkills),
		MessagingSkills:              union(en.MessagingSkills, zh.MessagingSkills),
		LeadershipExperience:         union(en.LeadershipExperience, zh.LeadershipExperience),
		PaymentExperience:            union(en.PaymentExperience, zh.PaymentExperience),
		Web3Experience:               union(en.Web3Experience, zh.Web3Experience),
		DistributedSystemsExperience: union(en.DistributedSystemsExperience, zh.DistributedSystemsExperience),
		MajorAchievements:            union(en.MajorAchievements, zh.MajorAchievements),
		QuantifiedResults:            union(en.QuantifiedResults, zh.QuantifiedResults),
		ResumeKeywords:               union(en.ResumeKeywords, zh.ResumeKeywords),
		CompanyDomainExperience:      union(en.CompanyDomainExperience, zh.CompanyDomainExperience),
		Domains:                      union(en.Domains, zh.Domains),
	}
	// Reconcile skill tiers: strongest wins over medium/weak
	out.MediumSkills = subtractSkills(out.MediumSkills, out.StrongestSkills)
	out.WeakSkills = subtractSkills(out.WeakSkills, append(out.StrongestSkills, out.MediumSkills...))
	return out
}

// ToCandidateProfile maps parsed resume data onto a CandidateProfile record.
func ToCandidateProfile(merged crm.ParsedResume) *crm.CandidateProfile {
	return &crm.CandidateProfile{
		FullName:                     merged.FullName,
		Location:                     merged.Location,
		Email:                        merged.Email,
		YearsOfExperience:            merged.YearsOfExperience,
		SeniorityLevel:               merged.SeniorityLevel,
		TargetRoles:                  crm.DefaultTargetRoles(),
		TargetRegions:                crm.DefaultTargetRegions(),
		TargetCompensationMinUSD:     150000,
		StrongestSkills:              merged.StrongestSkills,
		MediumSkills:                 merged.MediumSkills,
		WeakSkills:                   merged.WeakSkills,
		BackendSkills:                merged.BackendSkills,
		Web3Skills:                   merged.Web3Skills,
		InfraSkills:                  merged.InfraSkills,
		DatabaseSkills:               merged.DatabaseSkills,
		MessagingSkills:              merged.MessagingSkills,
		LeadershipExperience:         merged.LeadershipExperience,
		PaymentExperience:            merged.PaymentExperience,
		Web3Experience:               merged.Web3Experience,
		DistributedSystemsExperience: merged.DistributedSystemsExperience,
		MajorAchievements:            merged.MajorAchievements,
		QuantifiedResults:            merged.QuantifiedResults,
		ResumeKeywords:               merged.ResumeKeywords,
		CompanyDomainExperience:      merged.CompanyDomainExperience,
		PreferredJobTypes:            []string{"Remote", "Backend", "Infrastructure", "Web3"},
		Domains:                      merged.Domains,
	}
}

type matchedSkills struct {
	strong      []string
	medium      []string
	weak        []string
	byCategory  map[string][]string
}

func matchSkills(lower string) matchedSkills {
	out := matchedSkills{byCategory: map[string][]string{}}
	for _, rule := range skillCatalog {
		hits := 0
		for _, p := range rule.Patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				hits++
			}
		}
		if hits == 0 {
			continue
		}
		strength := "medium"
		if hits >= 2 || isStrongMention(lower, rule.Name) {
			strength = "strong"
			out.strong = append(out.strong, rule.Name)
		} else {
			out.medium = append(out.medium, rule.Name)
		}
		out.byCategory[rule.Category] = append(out.byCategory[rule.Category], rule.Name)
		_ = strength
	}
	// Known weak areas from target profile if mentioned lightly
	for _, w := range crm.DefaultWeakSkills() {
		if strings.Contains(lower, strings.ToLower(w)) && !containsSkill(out.strong, w) {
			out.weak = append(out.weak, w)
		}
	}
	out.strong = uniqueStrings(out.strong)
	out.medium = uniqueStrings(subtractSkills(out.medium, out.strong))
	out.weak = uniqueStrings(subtractSkills(out.weak, append(out.strong, out.medium...)))
	for cat, skills := range out.byCategory {
		out.byCategory[cat] = uniqueStrings(skills)
	}
	return out
}

func isStrongMention(lower, skill string) bool {
	strongContext := []string{"expert", "deep", "extensive", "architect", "built", "designed", "led", "精通", "资深"}
	for _, c := range strongContext {
		idx := strings.Index(lower, strings.ToLower(skill))
		if idx < 0 {
			continue
		}
		start := idx - 40
		if start < 0 {
			start = 0
		}
		end := idx + 40
		if end > len(lower) {
			end = len(lower)
		}
		window := lower[start:end]
		if strings.Contains(window, c) {
			return true
		}
	}
	return false
}

func inferSeniority(lower string) string {
	switch {
	case strings.Contains(lower, "principal") || strings.Contains(lower, "staff"):
		return "Staff"
	case strings.Contains(lower, "senior") || strings.Contains(lower, "资深") || strings.Contains(lower, "資深"):
		return "Senior"
	default:
		return "Senior"
	}
}

func inferYears(lower string) int {
	m := yearsRE.FindStringSubmatch(lower)
	if len(m) >= 2 {
		n := 0
		for _, c := range m[1] {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		if n > 0 && n <= 40 {
			return n
		}
	}
	return 15 // default for target user
}

func inferName(text string, lang crm.ResumeLanguage) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines[:min(8, len(lines))] {
		line = strings.TrimSpace(line)
		if len(line) < 2 || len(line) > 60 {
			continue
		}
		if emailRE.MatchString(line) {
			continue
		}
		if strings.Contains(strings.ToLower(line), "resume") || strings.Contains(line, "简历") || strings.Contains(line, "履歷") {
			continue
		}
		if strings.Contains(line, "工程師") || strings.Contains(line, "工程师") {
			continue
		}
		// Chinese name — usually first line, 2–4 Han characters
		if lang == crm.ResumeZH || containsHan(line) {
			name := strings.TrimSpace(strings.Split(line, "|")[0])
			name = strings.TrimSpace(strings.Split(name, "｜")[0])
			if cjkNameRE.MatchString(name) {
				return name
			}
			if containsHan(name) && len([]rune(name)) <= 6 && !strings.Contains(name, "@") {
				return name
			}
		}
		words := strings.Fields(line)
		if len(words) >= 2 && len(words) <= 5 {
			return line
		}
	}
	return ""
}

func mergeName(en, zh string) string {
	en = strings.TrimSpace(en)
	zh = strings.TrimSpace(zh)
	if zh == "Candidate" {
		zh = ""
	}
	switch {
	case en != "" && zh != "" && en != zh:
		return en + " (" + zh + ")"
	case en != "":
		return en
	default:
		return zh
	}
}

func mergeLocation(en, zh string) string {
	if strings.TrimSpace(zh) != "" && containsHan(zh) {
		return strings.TrimSpace(zh)
	}
	return coalesce(en, zh)
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func inferLocation(text string) string {
	locPatterns := []string{"Taipei", "Taiwan", "Singapore", "Remote", "台北", "台湾", "台灣", "桃園", "新加坡", "Taoyuan"}
	lower := strings.ToLower(text)
	for _, p := range locPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return p
		}
	}
	return ""
}

func extractLeadership(text string) []string {
	lower := strings.ToLower(text)
	var out []string
	for _, p := range leadershipPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return uniqueStrings(out)
}

func extractAchievements(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 20 {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "achiev") || strings.Contains(line, "成果") ||
			strings.Contains(line, "负责") || strings.Contains(line, "負責") ||
			strings.Contains(line, "建構") || strings.Contains(line, "设计") ||
			quantifiedRE.MatchString(line) {
			out = append(out, line)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func extractQuantified(text string) []string {
	matches := quantifiedRE.FindAllString(text, -1)
	return uniqueStrings(matches)
}

func extractDomains(lower string) []string {
	var out []string
	for domain, patterns := range domainPatterns {
		for _, p := range patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				out = append(out, domain)
				break
			}
		}
	}
	return uniqueStrings(out)
}

func extractKeywords(lower string) []string {
	var out []string
	for _, rule := range skillCatalog {
		for _, p := range rule.Patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				out = append(out, rule.Name)
				break
			}
		}
	}
	return out
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// normalizeForMatch converts fullwidth punctuation so EN/ZH skill tokens align.
func normalizeForMatch(s string) string {
	repl := strings.NewReplacer(
		"｜", "|", "：", ":", "，", ",", "。", ".", "（", "(", "）", ")",
		"　", " ", "／", "/", "－", "-",
	)
	return repl.Replace(s)
}

func coalesce(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func union(a, b []string) []string {
	return uniqueStrings(append(append([]string{}, a...), b...))
}

func subtractSkills(from, remove []string) []string {
	var out []string
	for _, s := range from {
		if !containsSkill(remove, s) {
			out = append(out, s)
		}
	}
	return out
}

func containsSkill(list []string, skill string) bool {
	sl := strings.ToLower(skill)
	for _, s := range list {
		if strings.ToLower(s) == sl {
			return true
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SkillsFromProfile builds CandidateSkill rows from a profile.
func SkillsFromProfile(profileID uuid.UUID, p *crm.CandidateProfile) []crm.CandidateSkill {
	add := func(name, cat, strength string, level int) crm.CandidateSkill {
		return crm.CandidateSkill{
			ProfileID: profileID,
			SkillName: name,
			Category:  cat,
			Level:     level,
			Strength:  strength,
			Source:    "resume",
		}
	}
	var out []crm.CandidateSkill
	for _, s := range p.StrongestSkills {
		out = append(out, add(s, skillCategory(s), "strong", 8))
	}
	for _, s := range p.MediumSkills {
		out = append(out, add(s, skillCategory(s), "medium", 5))
	}
	for _, s := range p.WeakSkills {
		out = append(out, add(s, skillCategory(s), "weak", 2))
	}
	return out
}

func skillCategory(name string) string {
	for _, rule := range skillCatalog {
		if strings.EqualFold(rule.Name, name) {
			return rule.Category
		}
	}
	return "general"
}

// IsMostlyCJK reports whether text is primarily Chinese (for language hint).
func IsMostlyCJK(text string) bool {
	var cjk, total int
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.Is(unicode.Han, r) {
			total++
			if unicode.Is(unicode.Han, r) {
				cjk++
			}
		}
	}
	return total > 0 && cjk*100/total > 30
}
