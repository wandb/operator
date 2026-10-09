package preflight

import (
	"context"
	"strings"
)

// SkipPreflightsAnnotation holds a comma-separated list of check names to skip; empty runs all checks.
const SkipPreflightsAnnotation = "skipPreflights"

type Outcome string

const (
	OutcomePass    Outcome = "pass"
	OutcomeFail    Outcome = "fail"
	OutcomeUnknown Outcome = "unknown"
	OutcomeSkipped Outcome = "skipped"
)

type Result struct {
	Check   string
	Outcome Outcome
	Message string
}

type CheckFunc func(ctx context.Context, params map[string]string) Result

type Check struct {
	Name string
	Run  CheckFunc
}

func ParseSkipList(annotation string) map[string]bool {
	skip := map[string]bool{}
	for _, name := range strings.Split(annotation, ",") {
		if name = strings.TrimSpace(name); name != "" {
			skip[name] = true
		}
	}
	return skip
}
