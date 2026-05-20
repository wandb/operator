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

func init() {
	metrics.Registry.MustRegister(ApplicationInfo)
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
