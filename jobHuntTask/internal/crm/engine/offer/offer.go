package offer

import (
	"time"

	crm "github.com/shawn/jobhunttask/internal/crm/model"
)

// FunnelStats holds application pipeline metrics.
type FunnelStats struct {
	TotalApplications int
	Applied           int
	Responses         int
	Interviews        int
	Offers            int
	ResponseRate      float64
	InterviewRate     float64
	OfferRate         float64
	AvgFitScore       float64
	WeeklyOutreach    int
	TopGapScore       int
}

// Analyze predicts interview/offer probability and diagnoses bottlenecks (Layer 5).
func Analyze(stats FunnelStats) *crm.OfferPrediction {
	interviewProb := stats.InterviewRate
	if interviewProb == 0 && stats.Applied > 0 {
		interviewProb = float64(stats.Interviews) / float64(stats.Applied) * 100
	}
	if interviewProb == 0 && stats.AvgFitScore > 0 {
		interviewProb = stats.AvgFitScore * 0.15
	}
	if stats.ResponseRate > 0 {
		interviewProb = max(interviewProb, stats.ResponseRate*1.2)
	}
	interviewProb = min(95, interviewProb)

	offerProb := interviewProb * 0.25
	if stats.Offers > 0 && stats.Interviews > 0 {
		offerProb = float64(stats.Offers) / float64(stats.Interviews) * 100
	}
	offerProb = min(90, offerProb)

	bottleneck, recs := diagnose(stats)

	return &crm.OfferPrediction{
		PredictionDate:  time.Now().UTC().Truncate(24 * time.Hour),
		InterviewProb:   round2(interviewProb),
		OfferProb:       round2(offerProb),
		Bottleneck:      bottleneck,
		Recommendations: recs,
		FunnelStats: map[string]any{
			"total_applications": stats.TotalApplications,
			"applied":            stats.Applied,
			"responses":          stats.Responses,
			"interviews":         stats.Interviews,
			"offers":             stats.Offers,
			"response_rate":      round2(stats.ResponseRate),
			"interview_rate":     round2(stats.InterviewRate),
			"avg_fit_score":      round2(stats.AvgFitScore),
			"weekly_outreach":    stats.WeeklyOutreach,
		},
	}
}

func diagnose(stats FunnelStats) (string, []string) {
	var recs []string

	if stats.Applied < 3 {
		return "Low application volume", []string{
			"Apply to today's top-fit job to build pipeline momentum",
			"Aim for 3+ quality applications per week",
		}
	}
	if stats.ResponseRate < 10 && stats.Applied >= 5 {
		recs = append(recs, "Tailor resume bullets to mirror job keywords")
		recs = append(recs, "Increase outreach to engineering managers before applying")
		return "Resume or targeting issue", recs
	}
	if stats.AvgFitScore < 70 && stats.AvgFitScore > 0 {
		recs = append(recs, "Focus only on jobs with fit score ≥ 75")
		recs = append(recs, "Review top 3 skill gaps and address in resume")
		return "Target company issue", recs
	}
	if stats.WeeklyOutreach < 5 {
		recs = append(recs, "Send 2 outreach messages today")
		recs = append(recs, "Target engineering managers at companies with fit ≥ 80")
		return "Low outreach volume", recs
	}
	if stats.TopGapScore > 80 {
		recs = append(recs, "Dedicate 15 min/day to your top skill gap")
		recs = append(recs, "Add learning projects to resume for gap skills")
		return "Skill gap issue", recs
	}
	if stats.InterviewRate < 5 && stats.Applied >= 10 {
		return "Conversion issue", []string{
			"Review rejected applications for patterns",
			"Request resume feedback from a senior peer",
			"Increase outreach before applying",
		}
	}
	return "On track", []string{
		"Maintain daily mission consistency",
		"Follow up on applications after 5 business days",
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
