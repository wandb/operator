package preflight

import (
	"context"
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
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

// ValueResolver turns a ValueOrSecret into its value; callers supply Secret access so checks stay client-agnostic.
type ValueResolver func(ctx context.Context, v apiv2.ValueOrSecret) (string, error)

// CheckFunc receives the CR struct found at the check's CRField, as returned by Check.NewSpec.
type CheckFunc func(ctx context.Context, spec any, resolve ValueResolver) Result

type Check struct {
	Name string
	// CRField is the CR field path that requires this check; "*" matches any instance key.
	CRField string
	// NewSpec returns an empty value of the CRField's type, so callers can decode the matched field into it.
	NewSpec func() any
	Run     CheckFunc
}

// typedCheck adapts a check written against its concrete spec type to the untyped registry signature.
func typedCheck[T any](name, crField string, run func(context.Context, *T, ValueResolver) Result) Check {
	return Check{
		Name:    name,
		CRField: crField,
		NewSpec: func() any { return new(T) },
		Run: func(ctx context.Context, spec any, resolve ValueResolver) Result {
			typed, ok := spec.(*T)
			if !ok || typed == nil {
				return Result{Check: name, Outcome: OutcomeUnknown, Message: fmt.Sprintf("expected %T, got %T", typed, spec)}
			}
			return run(ctx, typed, resolve)
		},
	}
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

func resolveFields(ctx context.Context, resolve ValueResolver, fields map[string]apiv2.ValueOrSecret) (map[string]string, error) {
	out := make(map[string]string, len(fields))
	for name, v := range fields {
		val, err := resolve(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[name] = val
	}
	return out, nil
}

func notImplemented(check, variant string) Result {
	return Result{Check: check, Outcome: OutcomeUnknown, Message: variant + " check not implemented"}
}
