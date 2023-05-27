package charts

import (
	"helm.sh/helm/v3/pkg/chart"
)

// Criterion is a single criterion for querying Chart catalog. If a Chart
// matches the criterion it must return true.
type Criterion = func(*chart.Chart) bool

/* Query Criteria */

// WithName matches the Chart name.
func WithName(name string) Criterion {
	return func(chart *chart.Chart) bool {
		return chart.Metadata.Name == name
	}
}

// WithName matches the Chart version.
func WithVersion(version string) Criterion {
	return func(chart *chart.Chart) bool {
		return chart.Metadata.Version == version
	}
}

// WithName matches the Chart appVersion.
func WithAppVersion(appVersion string) Criterion {
	return func(chart *chart.Chart) bool {
		return chart.Metadata.AppVersion == appVersion
	}
}

// All combines the provided Chart query criteria and succeeds when all of them
// return true.
func All(criteria ...Criterion) Criterion {
	return func(chart *chart.Chart) bool {
		for _, criterion := range criteria {
			if !criterion(chart) {
				return false
			}
		}

		return true
	}
}

// Any combines the provided Chart query criteria and succeeds when any of them
// returns true.
func Any(criteria ...Criterion) Criterion {
	return func(chart *chart.Chart) bool {
		for _, criterion := range criteria {
			if criterion(chart) {
				return true
			}
		}

		return false
	}
}

// None combines the provided Chart query criteria and succeeds when none of
// them return true.
func None(criteria ...Criterion) Criterion {
	return func(chart *chart.Chart) bool {
		for _, criterion := range criteria {
			if criterion(chart) {
				return false
			}
		}

		return true
	}
}
