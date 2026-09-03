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
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	apiv1 "github.com/wandb/operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyPendingCollector(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	scheme := runtime.NewScheme()
	if err := apiv1.AddToScheme(scheme); err != nil {
		t.Fatalf("could not register WeightsAndBiases types: %v", err)
	}

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&apiv1.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "applied",
				Namespace: "default",
			},
		},
		&apiv1.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pending",
				Namespace: "wandb-test",
				Annotations: map[string]string{
					ApplyPendingSinceAnnotation: now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
				},
			},
		},
		&apiv1.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-annotation",
				Namespace: "default",
				Annotations: map[string]string{
					ApplyPendingSinceAnnotation: "not-a-time",
				},
			},
		},
	).Build()

	collector := newApplyPendingCollector(reader, func() time.Time { return now })
	expected := `
# HELP wandb_operator_apply_pending_age_seconds Seconds since the operator first observed unapplied desired state; 0 when fully applied.
# TYPE wandb_operator_apply_pending_age_seconds gauge
wandb_operator_apply_pending_age_seconds{name="applied",namespace="default"} 0
wandb_operator_apply_pending_age_seconds{name="invalid-annotation",namespace="default"} 0
wandb_operator_apply_pending_age_seconds{name="pending",namespace="wandb-test"} 600
`
	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected), applyPendingAgeMetricName); err != nil {
		t.Fatal(err)
	}
}

func TestApplyPendingAgeClampsFutureTimestamps(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	instance := &apiv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{
			ApplyPendingSinceAnnotation: now.Add(time.Minute).Format(time.RFC3339Nano),
		},
	}}
	if age := applyPendingAge(instance, now); age != 0 {
		t.Fatalf("applyPendingAge() = %s, want 0", age)
	}
}
