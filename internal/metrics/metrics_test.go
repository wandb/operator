package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func gatherApplicationInfo(t *testing.T) []*dto.Metric {
	t.Helper()
	return gather(t, ApplicationInfo)
}

func labelMap(m *dto.Metric) map[string]string {
	out := make(map[string]string, len(m.Label))
	for _, lp := range m.Label {
		out[lp.GetName()] = lp.GetValue()
	}
	return out
}

func gather(t *testing.T, c prometheus.Collector) []*dto.Metric {
	t.Helper()
	out := make(chan prometheus.Metric, 64)
	go func() {
		c.Collect(out)
		close(out)
	}()
	var metrics []*dto.Metric
	for m := range out {
		dtoMetric := &dto.Metric{}
		assert.NoError(t, m.Write(dtoMetric))
		metrics = append(metrics, dtoMetric)
	}
	return metrics
}

func TestSetWeightsAndBiasesReady_FlipsValue(t *testing.T) {
	t.Cleanup(WeightsAndBiasesReady.Reset)
	WeightsAndBiasesReady.Reset()

	SetWeightsAndBiasesReady("wandb", "prod", true)
	got := gather(t, WeightsAndBiasesReady)
	assert.Len(t, got, 1)
	assert.Equal(t, 1.0, got[0].Gauge.GetValue())

	SetWeightsAndBiasesReady("wandb", "prod", false)
	got = gather(t, WeightsAndBiasesReady)
	assert.Len(t, got, 1)
	assert.Equal(t, 0.0, got[0].Gauge.GetValue())
}

func TestSetInfraState_ReplacesPriorState(t *testing.T) {
	t.Cleanup(InfraState.Reset)
	InfraState.Reset()

	SetInfraState("wandb", "prod", "mysql", "default", "Pending")
	SetInfraState("wandb", "prod", "mysql", "default", "Healthy")

	got := gather(t, InfraState)
	assert.Len(t, got, 1, "a state transition must not leave the previous state as an active series")
	got0 := labelMap(got[0])
	assert.Equal(t, "Healthy", got0["state"])
	assert.Equal(t, "default", got0["instance_name"])
}

func TestDeleteWeightsAndBiasesMetrics_ScopesToCR(t *testing.T) {
	t.Cleanup(func() { WeightsAndBiasesReady.Reset(); InfraState.Reset() })
	WeightsAndBiasesReady.Reset()
	InfraState.Reset()

	SetWeightsAndBiasesReady("wandb", "gone", true)
	SetInfraState("wandb", "gone", "mysql", "default", "Healthy")
	SetWeightsAndBiasesReady("wandb", "stays", true)
	SetInfraState("wandb", "stays", "redis", "default", "Healthy")

	DeleteWeightsAndBiasesMetrics("wandb", "gone")

	ready := gather(t, WeightsAndBiasesReady)
	assert.Len(t, ready, 1)
	assert.Equal(t, "stays", labelMap(ready[0])["name"])

	infra := gather(t, InfraState)
	assert.Len(t, infra, 1)
	assert.Equal(t, "stays", labelMap(infra[0])["name"])
}

func TestSetApplicationInfo_EmitsExpectedLabels(t *testing.T) {
	t.Cleanup(ApplicationInfo.Reset)
	ApplicationInfo.Reset()

	SetApplicationInfo("api", "wandb", "us-docker.pkg.dev/wandb/api", "v1.0.0", "sha256:abc")

	got := gatherApplicationInfo(t)
	assert.Len(t, got, 1)
	assert.Equal(t, 1.0, got[0].Gauge.GetValue())
	assert.Equal(t, map[string]string{
		"application_name": "api",
		"namespace":        "wandb",
		"image":            "us-docker.pkg.dev/wandb/api",
		"tag":              "v1.0.0",
		"digest":           "sha256:abc",
	}, labelMap(got[0]))
}

func TestSetApplicationInfo_TagBumpClearsStaleSeries(t *testing.T) {
	t.Cleanup(ApplicationInfo.Reset)
	ApplicationInfo.Reset()

	SetApplicationInfo("api", "wandb", "repo", "v1.0", "")
	SetApplicationInfo("api", "wandb", "repo", "v1.1", "")

	got := gatherApplicationInfo(t)
	assert.Len(t, got, 1, "stale series from previous tag must be cleared on re-set")
	assert.Equal(t, "v1.1", labelMap(got[0])["tag"])
}

func TestDeleteApplicationInfo_OnlyMatchesNameAndNamespace(t *testing.T) {
	t.Cleanup(ApplicationInfo.Reset)
	ApplicationInfo.Reset()

	SetApplicationInfo("api", "wandb-a", "repo", "v1", "")
	SetApplicationInfo("api", "wandb-b", "repo", "v1", "")
	SetApplicationInfo("executor", "wandb-a", "repo", "v1", "")

	DeleteApplicationInfo("api", "wandb-a")

	got := gatherApplicationInfo(t)
	assert.Len(t, got, 2, "delete must scope to (application_name, namespace) — leaving other apps and other namespaces intact")

	remaining := make(map[string]string)
	for _, m := range got {
		lm := labelMap(m)
		remaining[lm["application_name"]+"/"+lm["namespace"]] = lm["tag"]
	}
	assert.Contains(t, remaining, "api/wandb-b")
	assert.Contains(t, remaining, "executor/wandb-a")
	assert.NotContains(t, remaining, "api/wandb-a")
}
