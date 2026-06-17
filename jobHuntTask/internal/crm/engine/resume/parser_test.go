package resume_test

import (
	"strings"
	"testing"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
	"github.com/shawn/jobhunttask/internal/crm/engine/resume"
)

const sampleEN = `
John Doe
Senior Backend Engineer | Taipei, Taiwan | john@example.com

SUMMARY
15 years building payment and distributed systems. Expert in Java, Go, Kafka.

EXPERIENCE
Led payment settlement platform — 100,000+ transactions per day, 99.99% availability
Built event-driven architecture with Kafka for 5,000 TPS peak throughput
Web3 backend: DeFi lending, NFT marketplace, blockchain event indexing
`

const sampleZH = `
张三
资深后端工程师 | 台北 | zhang@example.com

15年软件工程经验，精通 Java、Go、Kafka、支付系统、分布式系统、区块链后端。
负责结算与对账系统，日处理 10万+ 交易。
`

func TestParseEnglishResume(t *testing.T) {
	p := resume.Parse(sampleEN, crm.ResumeEN)
	if p.YearsOfExperience < 10 {
		t.Errorf("years = %d", p.YearsOfExperience)
	}
	if !contains(p.StrongestSkills, "Go") && !contains(p.MediumSkills, "Go") {
		t.Fatalf("expected Go in skills: strong=%v medium=%v", p.StrongestSkills, p.MediumSkills)
	}
	if !contains(p.Domains, "Payments") {
		t.Errorf("expected Payments domain, got %v", p.Domains)
	}
	if len(p.QuantifiedResults) == 0 {
		t.Error("expected quantified results")
	}
}

func TestParseTraditionalChineseResume(t *testing.T) {
	zh := `
鍾秀濤
資深後端工程師｜Go｜Java｜Kafka｜分散式系統｜支付系統｜Web3基礎設施
台灣桃園
15年軟體工程經驗，負責結算與對帳系統，日處理 10万+ 交易，99.99% 系統可用性
`
	p := resume.Parse(zh, crm.ResumeZH)
	if p.FullName != "鍾秀濤" {
		t.Errorf("name = %q, want 鍾秀濤", p.FullName)
	}
	if !contains(p.StrongestSkills, "Go") && !contains(p.MediumSkills, "Go") {
		t.Errorf("expected Go from ｜Go｜, skills strong=%v medium=%v", p.StrongestSkills, p.MediumSkills)
	}
	if !contains(p.StrongestSkills, "Distributed Systems") && !contains(p.MediumSkills, "Distributed Systems") {
		t.Errorf("expected Distributed Systems from 分散式, got strong=%v", p.StrongestSkills)
	}
}

func TestMergeResumes(t *testing.T) {
	en := resume.Parse(sampleEN, crm.ResumeEN)
	zh := resume.Parse(sampleZH, crm.ResumeZH)
	merged := resume.Merge(en, zh)
	if len(merged.StrongestSkills) == 0 {
		t.Fatal("merged should have skills")
	}
	if !strings.Contains(merged.FullName, "John") {
		t.Errorf("merged name = %q", merged.FullName)
	}
	profile := resume.ToCandidateProfile(merged)
	if profile.TargetCompensationMinUSD != 150000 {
		t.Errorf("comp = %d", profile.TargetCompensationMinUSD)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}
