package model

// FitDecision is the apply recommendation for a scored job.
type FitDecision string

const (
	FitApply FitDecision = "apply"
	FitMaybe FitDecision = "maybe"
	FitSkip  FitDecision = "skip"
)

func (d FitDecision) Valid() bool {
	switch d {
	case FitApply, FitMaybe, FitSkip:
		return true
	}
	return false
}

func (d FitDecision) Label() string {
	switch d {
	case FitApply:
		return "Apply"
	case FitMaybe:
		return "Maybe"
	case FitSkip:
		return "Skip"
	default:
		return string(d)
	}
}
