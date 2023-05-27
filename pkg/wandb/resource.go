package wandb

import "github.com/wandb/operator/pkg/helm/values"

type Resource interface {
	Features
}

// Features represents the features that are specified in the underlying
// GitLab resource. Note that these features are the desired status of an
// instance and does not necessarily mean that the instance exists or is in the
// desired state.
type Features interface {
	// WantsFeature queries this GitLab resource for the specified feature.
	// Returns true if the instance has the feature in its specification.
	//
	// Note that this function only checks the specification of the GitLab
	// resource and does not verify the state of the GitLab instance.
	WantsFeature(check FeatureCheck) bool

	// WantsComponent is a shorthand for checking if a specific GitLab component
	// is enabled in the specification of this GitLab resource.
	//
	// Note that this function only checks the specification of the GitLab
	// resource and does not verify the state of the GitLab instance.
	WantsComponent(component Component) bool
}

// Component is an alias type for representing an individual GitLab component.
type Component string

// Components is a type for grouping and addressing a collection of GitLab
// components.
type Components []Component

// FeatureCheck is a callback for assessing the availability of a GitLab feature
// based on the values of specification of a GitLab resource.
type FeatureCheck func(values values.Values) bool

// Name returns the name of the component.
func (c Component) Name() string {
	return string(c)
}

// Names returns the name of the components in the collection in the same order.
func (c Components) Names() []string {
	result := make([]string, len(c))
	for i := 0; i < len(c); i++ {
		result[i] = c[i].Name()
	}

	return result
}

// Contains checks wheather the collection contains the specified component.
func (c Components) Contains(component Component) bool {
	for _, i := range c {
		if i == component {
			return true
		}
	}

	return false
}
