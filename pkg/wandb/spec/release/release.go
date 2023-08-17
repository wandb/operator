package release

import (
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/release/helm"
)

func Is(v spec.Validatable, data interface{}) error {
	specBytes, _ := json.Marshal(data)

	if err := json.Unmarshal(specBytes, v); err != nil {
		return err
	}

	if err := v.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

type ValidatableRelease interface {
	spec.HelmRelease
	spec.Validatable
}

// Get returns tries to match the spec of a given type to a release.
func Get(maybeRelease interface{}) spec.HelmRelease {
	releases := []ValidatableRelease{
		&helm.LocalRelease{},
	}

	var errs []error
	for _, release := range releases {
		if err := Is(release, maybeRelease); err != nil {
			errs = append(errs, err)
			continue
		}
		return release
	}

	for _, err := range errs {
		fmt.Println(err)
	}

	return nil
}
