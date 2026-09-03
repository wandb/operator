/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	apiv1 "github.com/wandb/operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ApplyPendingSinceAnnotation = "operator.wandb.com/apply-pending-since"
	applyPendingAgeMetricName   = "wandb_operator_apply_pending_age_seconds"
)

type applyPendingCollector struct {
	reader client.Reader
	now    func() time.Time
	desc   *prometheus.Desc
}

// NewApplyPendingCollector returns a collector that derives pending age from WeightsAndBiases annotations.
func NewApplyPendingCollector(reader client.Reader) prometheus.Collector {
	return newApplyPendingCollector(reader, time.Now)
}

func newApplyPendingCollector(reader client.Reader, now func() time.Time) *applyPendingCollector {
	return &applyPendingCollector{
		reader: reader,
		now:    now,
		desc: prometheus.NewDesc(
			applyPendingAgeMetricName,
			"Seconds since the operator first observed unapplied desired state; 0 when fully applied.",
			[]string{"namespace", "name"},
			nil,
		),
	}
}

func (c *applyPendingCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *applyPendingCollector) Collect(ch chan<- prometheus.Metric) {
	instances := &apiv1.WeightsAndBiasesList{}
	if err := c.reader.List(context.Background(), instances); err != nil {
		ctrllog.Log.Error(err, "Failed to list WeightsAndBiases instances for apply pending metric")
		return
	}

	now := c.now()
	for i := range instances.Items {
		instance := &instances.Items[i]
		age := applyPendingAge(instance, now)
		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			age.Seconds(),
			instance.Namespace,
			instance.Name,
		)
	}
}

func applyPendingAge(instance *apiv1.WeightsAndBiases, now time.Time) time.Duration {
	if !instance.DeletionTimestamp.IsZero() {
		return 0
	}

	pendingSince, err := time.Parse(time.RFC3339Nano, instance.Annotations[ApplyPendingSinceAnnotation])
	if err != nil || pendingSince.After(now) {
		return 0
	}
	return now.Sub(pendingSince)
}
