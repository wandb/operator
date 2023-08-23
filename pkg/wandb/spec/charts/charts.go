package charts

import (
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
)

func Is(v spec.Validatable, data interface{}) error {
	specBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(specBytes, v); err != nil {
		return err
	}

	if err := v.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

type ValidatableRelease interface {
	spec.Chart
	spec.Validatable
}

// Get returns tries to match the spec of a given type to a release.
func Get(maybeRelease interface{}) spec.Chart {
	releases := []ValidatableRelease{
		new(LocalRelease),
		new(RepoRelease),
	}

	for _, release := range releases {
		if err := Is(release, maybeRelease); err != nil {
			continue
		}
		return release
	}

	return nil
}
