package suggestion

// Evaluator runs every configured rule against a snapshot and returns the
// ones that fire. It is intentionally tiny — composition only, no state.
type Evaluator struct {
	rules []Rule
}

func NewEvaluator(rules ...Rule) *Evaluator {
	if len(rules) == 0 {
		rules = DefaultRules()
	}
	return &Evaluator{rules: rules}
}

// Rules returns the underlying rule list (mostly for diagnostics / tests).
func (e *Evaluator) Rules() []Rule { return e.rules }

// Evaluate runs every rule once. Order of results matches rule registration
// order so callers can rely on a stable presentation order.
func (e *Evaluator) Evaluate(s Snapshot) []Result {
	out := make([]Result, 0, len(e.rules))
	for _, r := range e.rules {
		if res, ok := r.Evaluate(s); ok {
			out = append(out, res)
		}
	}
	return out
}
