package sizing

import (
	"sort"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type PodDemand struct {
	Component string
	CPU       resource.Quantity // per pod
	Memory    resource.Quantity // per pod
	Replicas  int32
}

// EstimateDemand mirrors reconcile (feature gating, ResolveResources/ResolveAutoscaling, ApplyInfraSizing)
// so the estimate can't drift from what gets deployed. External components aren't counted.
func EstimateDemand(wandb *apiv2.WeightsAndBiases, m manifest.Manifest) []PodDemand {
	w := wandb.DeepCopy()
	// Copy before overriding: reconcile mutates the manifest's map, we must not.
	features := make(map[string]bool, len(m.Features))
	for k, v := range m.Features {
		features[k] = v
	}
	for k, v := range w.Spec.Wandb.Features {
		features[k] = v
	}
	m.Features = features
	ApplyInfraSizing(w, m)

	var out []PodDemand
	for key, app := range m.Applications {
		if len(app.Features) > 0 && !m.FeaturesEnabled(app.Features) {
			continue
		}
		if app.Name == "" {
			app.Name = key
		}
		var containers []*corev1.ResourceRequirements
		if len(app.Containers) == 0 {
			containers = append(containers, ResolveResources(app, w, nil))
		}
		for _, c := range app.Containers {
			containers = append(containers, ResolveResources(app, w,
				c.Resources))
		}
		replicas := int32(1)
		if hpa := ResolveAutoscaling(app, w); hpa != nil && hpa.MinReplicas != nil {
			replicas = *hpa.MinReplicas
		}
		out = append(out, podDemand("app/"+app.Name, replicas, containers...))
	}

	for key, s := range w.Spec.MySQL {
		if c := s.ManagedMysql; c != nil {
			out = append(out, podDemand("mysql/"+key, atLeastOne(c.Replicas),
				&c.Config.Resources))
		}
	}
	for key, s := range w.Spec.Redis {
		if c := s.ManagedRedis; c != nil {
			out = append(out, podDemand("redis/"+key, 1, &c.Config.Resources))
		}
	}
	for key, s := range w.Spec.ClickHouse {
		if c := s.ManagedClickHouse; c != nil {
			out = append(out,
				podDemand("clickhouse/"+key, atLeastOne(c.Replicas),
					&c.Config.Resources),
				podDemand("clickhouse/"+key+"/keeper",
					atLeastOne(c.Keeper.Replicas), &c.Keeper.Config.Resources))
		}
	}
	for key, s := range w.Spec.ObjectStore {
		if c := s.ManagedObjectStore; c != nil {
			out = append(out, podDemand("objectstore/"+key,
				atLeastOne(c.Replicas), &c.Config.Resources))
		}
	}
	if c := w.Spec.Kafka.ManagedKafka; c != nil {
		out = append(out, podDemand("kafka", atLeastOne(c.Replicas),
			&c.Config.Resources))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Component < out[j].Component
	})
	return out
}

// A limit without a request counts as the request, as the apiserver defaults it.
func podDemand(component string, replicas int32, containers ...*corev1.ResourceRequirements) PodDemand {
	d := PodDemand{Component: component, Replicas: replicas}
	for _, r := range containers {
		if r == nil {
			continue
		}
		d.CPU.Add(request(r, corev1.ResourceCPU))
		d.Memory.Add(request(r, corev1.ResourceMemory))
	}
	return d
}

func request(r *corev1.ResourceRequirements, name corev1.ResourceName) resource.Quantity {
	if q, ok := r.Requests[name]; ok {
		return q
	}
	return r.Limits[name]
}

func atLeastOne(n int32) int32 {
	return max(n, 1)
}
