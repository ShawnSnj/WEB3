package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/engine/resume"
)

// CandidateProfileResponse includes profile + skills for API.
type CandidateProfileResponse struct {
	Profile      *crm.CandidateProfile `json:"profile"`
	Skills       []crm.CandidateSkill  `json:"skills"`
	ParseSummary *ResumeParseSummary   `json:"parse_summary,omitempty"`
	Resumes      struct {
		EN *crm.ResumeDocument `json:"en,omitempty"`
		ZH *crm.ResumeDocument `json:"zh,omitempty"`
	} `json:"resumes"`
}

// ResumeParseSummary shows what each language resume contributed.
type ResumeParseSummary struct {
	EN *crm.ParsedResume `json:"en,omitempty"`
	ZH *crm.ParsedResume `json:"zh,omitempty"`
}

// GetCandidateProfile returns the master profile with skills and resume refs.
func (s *CRM) GetCandidateProfile(ctx context.Context) (*CandidateProfileResponse, error) {
	p, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return nil, err
	}
	p.NormalizeMasterProfile()
	skills, err := s.store.ListCandidateSkills(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := &CandidateProfileResponse{Profile: p, Skills: skills}
	if en, err := s.store.GetLatestResume(ctx, crm.ResumeEN); err == nil {
		out.Resumes.EN = en
	}
	if zh, err := s.store.GetLatestResume(ctx, crm.ResumeZH); err == nil {
		out.Resumes.ZH = zh
	}
	out.ParseSummary = parseSummaryFromProfile(p)
	return out, nil
}

func parseSummaryFromProfile(p *crm.CandidateProfile) *ResumeParseSummary {
	if p == nil || p.StructuredProfile == nil {
		return nil
	}
	sum := &ResumeParseSummary{}
	if v, ok := p.StructuredProfile["parsed_en"]; ok {
		sum.EN = decodeParsedResume(v)
	}
	if v, ok := p.StructuredProfile["parsed_zh"]; ok {
		sum.ZH = decodeParsedResume(v)
	}
	if sum.EN == nil && sum.ZH == nil {
		return nil
	}
	return sum
}

func decodeParsedResume(v any) *crm.ParsedResume {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var p crm.ParsedResume
	if json.Unmarshal(b, &p) != nil {
		return nil
	}
	if p.FullName == "" && len(p.StrongestSkills) == 0 && len(p.MajorAchievements) == 0 {
		return nil
	}
	return &p
}

// UpdateCandidateProfile applies manual edits to the master profile.
func (s *CRM) UpdateCandidateProfile(ctx context.Context, patch *crm.CandidateProfile) (*CandidateProfileResponse, error) {
	existing, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return nil, err
	}
	mergeCandidateProfile(existing, patch)
	if err := s.store.UpsertCandidateProfile(ctx, existing); err != nil {
		return nil, err
	}
	skills := resume.SkillsFromProfile(existing.ID, existing)
	for i := range skills {
		skills[i].Source = "manual"
	}
	if err := s.store.ReplaceCandidateSkills(ctx, existing.ID, skills); err != nil {
		return nil, err
	}
	if err := s.syncUserProfileFromCandidate(ctx, existing); err != nil {
		s.log.Warn("sync user profile", "err", err)
	}
	return s.GetCandidateProfile(ctx)
}

// UploadResume stores raw resume text for EN or ZH.
func (s *CRM) UploadResume(ctx context.Context, lang crm.ResumeLanguage, filename, rawText, contentType string) (*crm.ResumeDocument, error) {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return nil, fmt.Errorf("resume text is empty")
	}
	if !lang.Valid() {
		return nil, fmt.Errorf("invalid language: %s", lang)
	}
	if contentType == "" {
		contentType = "text/plain"
	}
	doc := &crm.ResumeDocument{
		Language:    lang,
		Filename:    filename,
		RawText:     rawText,
		ContentType: contentType,
	}
	if err := s.store.SaveResumeDocument(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// UploadResumeFile extracts plain text from a document upload then stores it.
func (s *CRM) UploadResumeFile(ctx context.Context, lang crm.ResumeLanguage, filename string, data []byte, contentType string) (*crm.ResumeDocument, error) {
	text, err := resume.ExtractText(filename, contentType, data)
	if err != nil {
		return nil, err
	}
	return s.UploadResume(ctx, lang, filename, text, contentType)
}

// ParseResumes merges EN + ZH resumes into the Candidate Master Profile.
func (s *CRM) ParseResumes(ctx context.Context) (*CandidateProfileResponse, error) {
	existing, err := s.store.EnsureCandidateProfile(ctx)
	if err != nil {
		return nil, err
	}

	var enDoc, zhDoc *crm.ResumeDocument
	var enParsed, zhParsed crm.ParsedResume

	if enDoc, err = s.store.GetLatestResume(ctx, crm.ResumeEN); err == nil {
		enParsed = resume.Parse(enDoc.RawText, crm.ResumeEN)
	}
	if zhDoc, err = s.store.GetLatestResume(ctx, crm.ResumeZH); err == nil {
		zhParsed = resume.Parse(zhDoc.RawText, crm.ResumeZH)
	}
	if enDoc == nil && zhDoc == nil {
		return nil, fmt.Errorf("upload at least one resume (en or zh) before parsing")
	}

	merged := resume.Merge(enParsed, zhParsed)
	profile := resume.ToCandidateProfile(merged)
	profile.ID = existing.ID
	profile.UserProfileID = existing.UserProfileID
	if enDoc != nil {
		profile.SourceENResumeID = &enDoc.ID
	}
	if zhDoc != nil {
		profile.SourceZHResumeID = &zhDoc.ID
	}
	now := s.clock()
	profile.LastParsedAt = &now
	profile.StructuredProfile = map[string]any{
		"parsed_en": enParsed,
		"parsed_zh": zhParsed,
		"merged":    merged,
	}

	// Optional OpenAI enrichment when configured
	if s.ai != nil && s.ai.Enabled() {
		if enriched, err := s.ai.ParseCandidateProfile(ctx, enDoc, zhDoc); err == nil && enriched != nil {
			applyAIEnrichment(profile, enriched)
		}
	}

	if err := s.store.UpsertCandidateProfile(ctx, profile); err != nil {
		return nil, err
	}
	skills := resume.SkillsFromProfile(profile.ID, profile)
	if err := s.store.ReplaceCandidateSkills(ctx, profile.ID, skills); err != nil {
		return nil, err
	}

	// Mark resumes parsed
	for _, doc := range []*crm.ResumeDocument{enDoc, zhDoc} {
		if doc == nil {
			continue
		}
		_ = s.store.MarkResumeParsed(ctx, doc.ID, now)
	}

	if err := s.syncUserProfileFromCandidate(ctx, profile); err != nil {
		s.log.Warn("sync user profile", "err", err)
	}
	// Sync user_skills for existing engines
	if err := s.syncUserSkillsFromCandidate(ctx, skills); err != nil {
		s.log.Warn("sync user skills", "err", err)
	}

	return s.GetCandidateProfile(ctx)
}

func mergeCandidateProfile(dst, patch *crm.CandidateProfile) {
	if patch.FullName != "" {
		dst.FullName = patch.FullName
	}
	if patch.Location != "" {
		dst.Location = patch.Location
	}
	if patch.Email != "" {
		dst.Email = patch.Email
	}
	if patch.YearsOfExperience > 0 {
		dst.YearsOfExperience = patch.YearsOfExperience
	}
	if patch.SeniorityLevel != "" {
		dst.SeniorityLevel = patch.SeniorityLevel
	}
	if len(patch.TargetRoles) > 0 {
		dst.TargetRoles = patch.TargetRoles
	}
	if len(patch.TargetRegions) > 0 {
		dst.TargetRegions = patch.TargetRegions
	}
	if patch.TargetCompensationMinUSD > 0 {
		dst.TargetCompensationMinUSD = patch.TargetCompensationMinUSD
	}
	if len(patch.StrongestSkills) > 0 {
		dst.StrongestSkills = patch.StrongestSkills
	}
	if len(patch.MediumSkills) > 0 {
		dst.MediumSkills = patch.MediumSkills
	}
	if len(patch.WeakSkills) > 0 {
		dst.WeakSkills = patch.WeakSkills
	}
	if len(patch.BackendSkills) > 0 {
		dst.BackendSkills = patch.BackendSkills
	}
	if len(patch.Web3Skills) > 0 {
		dst.Web3Skills = patch.Web3Skills
	}
	if len(patch.InfraSkills) > 0 {
		dst.InfraSkills = patch.InfraSkills
	}
	if len(patch.MajorAchievements) > 0 {
		dst.MajorAchievements = patch.MajorAchievements
	}
	if len(patch.Domains) > 0 {
		dst.Domains = patch.Domains
	}
}

func (s *CRM) syncUserProfileFromCandidate(ctx context.Context, p *crm.CandidateProfile) error {
	up, err := s.store.GetProfile(ctx)
	if err != nil {
		return err
	}
	if p.FullName != "" {
		up.DisplayName = p.FullName
	}
	allSkills := append(append([]string{}, p.StrongestSkills...), p.MediumSkills...)
	if len(allSkills) > 0 {
		up.Skills = allSkills
	}
	if len(p.TargetRoles) > 0 {
		up.TargetTitles = p.TargetRoles
	}
	if len(p.Domains) > 0 {
		up.TargetIndustries = p.Domains
	}
	if p.TargetCompensationMinUSD > 0 {
		up.MinSalaryUSD = p.TargetCompensationMinUSD
	}
	// Merge resume texts for legacy resume analysis
	var parts []string
	if en, err := s.store.GetLatestResume(ctx, crm.ResumeEN); err == nil {
		parts = append(parts, en.RawText)
	}
	if zh, err := s.store.GetLatestResume(ctx, crm.ResumeZH); err == nil {
		parts = append(parts, zh.RawText)
	}
	if len(parts) > 0 {
		up.ResumeText = strings.Join(parts, "\n\n---\n\n")
	}
	return s.store.UpdateProfile(ctx, up)
}

func (s *CRM) syncUserSkillsFromCandidate(ctx context.Context, skills []crm.CandidateSkill) error {
	for _, sk := range skills {
		if err := s.store.UpsertUserSkill(ctx, sk.SkillName, sk.Category, sk.Level); err != nil {
			return err
		}
	}
	return nil
}

func applyAIEnrichment(dst *crm.CandidateProfile, enriched *crm.ParsedResume) {
	if len(enriched.StrongestSkills) > 0 {
		dst.StrongestSkills = enriched.StrongestSkills
	}
	if len(enriched.MediumSkills) > 0 {
		dst.MediumSkills = enriched.MediumSkills
	}
	if len(enriched.WeakSkills) > 0 {
		dst.WeakSkills = enriched.WeakSkills
	}
	if len(enriched.MajorAchievements) > 0 {
		dst.MajorAchievements = enriched.MajorAchievements
	}
	if enriched.YearsOfExperience > 0 {
		dst.YearsOfExperience = enriched.YearsOfExperience
	}
}
