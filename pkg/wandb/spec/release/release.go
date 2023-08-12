package release

import (
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/release/cdk8s"
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
	spec.Release
	spec.Validatable
}

func Get(maybeRelease interface{}) spec.Release {
	releases := []ValidatableRelease{
		&cdk8s.Cdk8sContainer{},
		&cdk8s.Cdk8sLocal{},
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
