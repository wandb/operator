package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// ApplicationInfo follows the Prometheus "info" pattern: one series per
// managed W&B Application, value always 1, identity carried in the labels.
// Dashboards use this to render per-service image columns without inferring
// from cAdvisor's `image` label (which varies across runtimes and exposes
// the raw digest string).
var ApplicationInfo = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "wandb_application_info",
		Help: "Currently-running image for each managed W&B Application, decomposed into repository, tag, and digest.",
	},
	[]string{"application_name", "namespace", "image", "tag", "digest"},
)

// WeightsAndBiasesReady is 1 when a WeightsAndBiases CR reports Ready, else 0.
var WeightsAndBiasesReady = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "wandb_weightsandbiases_ready",
		Help: "Whether a WeightsAndBiases custom resource is Ready (1) or not (0).",
	},
	[]string{"namespace", "name"},
)

// InfraState carries the current state of each backing dependency in the `state`
// label; value is always 1 for the active state.
var InfraState = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "wandb_infra_state",
		Help: "Current state of each backing dependency of a WeightsAndBiases custom resource.",
	},
	[]string{"namespace", "name", "component", "instance_name", "state"},
)

func init() {
	metrics.Registry.MustRegister(ApplicationInfo, WeightsAndBiasesReady, InfraState)
}

func SetWeightsAndBiasesReady(namespace, name string, ready bool) {
	value := 0.0
	if ready {
		value = 1
	}
	WeightsAndBiasesReady.With(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	}).Set(value)
}

// SetInfraState records a dependency's current state, clearing its prior state
// series first so a transition doesn't leave the old state active.
func SetInfraState(namespace, name, component, instanceName, state string) {
	InfraState.DeletePartialMatch(prometheus.Labels{
		"namespace":     namespace,
		"name":          name,
		"component":     component,
		"instance_name": instanceName,
	})
	InfraState.With(prometheus.Labels{
		"namespace":     namespace,
		"name":          name,
		"component":     component,
		"instance_name": instanceName,
		"state":         state,
	}).Set(1)
}

// DeleteWeightsAndBiasesMetrics clears the readiness and infra-state series for
// a CR being torn down, so a deleted resource doesn't linger as Ready forever.
func DeleteWeightsAndBiasesMetrics(namespace, name string) {
	WeightsAndBiasesReady.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
	InfraState.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
}

// SetApplicationInfo records the running image for a single Application.
// Existing series for the same (applicationName, namespace) pair are cleared
// first so that an image bump doesn't leave a stale row behind in dashboards.
func SetApplicationInfo(applicationName, namespace, repository, tag, digest string) {
	DeleteApplicationInfo(applicationName, namespace)
	ApplicationInfo.With(prometheus.Labels{
		"application_name": applicationName,
		"namespace":        namespace,
		"image":            repository,
		"tag":              tag,
		"digest":           digest,
	}).Set(1)
}

// DeleteApplicationInfo clears all series for an Application that is no
// longer in the desired set (removed from manifest or disabled by feature
// flag). Without this, dashboards would show stale apps as "currently
// running" indefinitely.
func DeleteApplicationInfo(applicationName, namespace string) {
	ApplicationInfo.DeletePartialMatch(prometheus.Labels{
		"application_name": applicationName,
		"namespace":        namespace,
	})
}
