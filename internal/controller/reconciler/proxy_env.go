package reconciler

import (
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"

	apiv2 "github.com/wandb/operator/api/v2"
)

// proxyNoProxyStatic is the always-present in-cluster NO_PROXY baseline. Both
// .svc and the full suffix are emitted because suffix-match rules differ across
// stacks (Go matches name+subdomains; Python urllib does plain string-suffix).
// Datastore and app service hosts are NOT enumerated here: the operator emits
// FQDNs (<name>.<ns>.svc.cluster.local) for every wired service, so the
// .svc.cluster.local suffix already covers the whole in-cluster HTTP mesh.
var proxyNoProxyStatic = []string{
	"localhost", "127.0.0.1", "::1",
	".svc", ".svc.cluster.local", ".cluster.local",
	"kubernetes.default.svc",
}

// computeNoProxy builds the NO_PROXY value the app workloads receive: the static
// in-cluster baseline plus the API-server ClusterIP (an IP literal no dot-suffix
// rule covers) plus the user's extra entries, deduplicated with order preserved.
// The API-server IP comes from the operator pod's own $KUBERNETES_SERVICE_HOST,
// which is the same in-cluster API endpoint every app pod sees.
func computeNoProxy(extras []string) string {
	entries := append([]string{}, proxyNoProxyStatic...)
	if apiHost := os.Getenv("KUBERNETES_SERVICE_HOST"); apiHost != "" {
		entries = append(entries, apiHost)
	}
	for _, e := range extras {
		if e != "" {
			entries = append(entries, e)
		}
	}
	return joinNoProxy(entries)
}

// joinNoProxy deduplicates entries (order preserved) and comma-joins them.
func joinNoProxy(entries []string) string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return strings.Join(out, ",")
}

// proxyValueEnvVars turns one ProxyValue into the upper/lower env-var pair for
// the given base name. A literal value becomes a literal env var; a valueFrom
// becomes a SecretKeyRef env source (both casings reference the same key) so
// credential-bearing URLs stay in the Secret and never land in the workload
// spec. Returns nil when the value is unset.
func proxyValueEnvVars(upper, lower string, pv *apiv2.ProxyValue) []corev1.EnvVar {
	if pv == nil {
		return nil
	}
	switch {
	case pv.Value != "":
		return []corev1.EnvVar{
			{Name: upper, Value: pv.Value},
			{Name: lower, Value: pv.Value},
		}
	case pv.ValueFrom != nil && pv.ValueFrom.SecretKeyRef != nil:
		return []corev1.EnvVar{
			{Name: upper, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: pv.ValueFrom.SecretKeyRef.DeepCopy()}},
			{Name: lower, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: pv.ValueFrom.SecretKeyRef.DeepCopy()}},
		}
	default:
		return nil
	}
}

// proxyEnvVars builds the full six-variable proxy env set for spec.global.proxy:
// HTTP_PROXY/HTTPS_PROXY plus the operator-computed NO_PROXY, and each one's
// lowercase twin (Go honors both casings; many libraries read only lowercase).
// NO_PROXY is emitted whenever any proxy URL is set — the computed baseline is
// never empty.
func proxyEnvVars(proxy *apiv2.ProxySpec) []corev1.EnvVar {
	if proxy == nil {
		return nil
	}
	var envVars []corev1.EnvVar
	envVars = append(envVars, proxyValueEnvVars("HTTP_PROXY", "http_proxy", proxy.HTTPProxy)...)
	envVars = append(envVars, proxyValueEnvVars("HTTPS_PROXY", "https_proxy", proxy.HTTPSProxy)...)
	if proxy.HTTPProxy != nil || proxy.HTTPSProxy != nil {
		noProxy := computeNoProxy(proxy.NoProxy)
		envVars = append(envVars,
			corev1.EnvVar{Name: "NO_PROXY", Value: noProxy},
			corev1.EnvVar{Name: "no_proxy", Value: noProxy},
		)
	}
	return envVars
}

// applyProxyToWorkload appends the spec.global.proxy env vars to a workload's
// env, skipping any name already present. Injected AFTER customCACerts env and
// BEFORE applyLegacyOverrideEnv, so a legacyOverrides entry can still override
// or blank any proxy var per-app (the deliberate escape hatch). No-op when
// spec.global.proxy is unset.
func applyProxyToWorkload(wandb *apiv2.WeightsAndBiases, envVars []corev1.EnvVar) []corev1.EnvVar {
	if wandb.Spec.Global.Proxy == nil {
		return envVars
	}
	return appendMissingEnvVars(envVars, proxyEnvVars(wandb.Spec.Global.Proxy))
}
