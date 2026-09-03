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

package controller

import (
	"context"
	"testing"
	"time"

	apiv1 "github.com/wandb/operator/api/v1"
	operatormetrics "github.com/wandb/operator/internal/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyPendingAnnotationLifecycle(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	scheme := runtime.NewScheme()
	if err := apiv1.AddToScheme(scheme); err != nil {
		t.Fatalf("could not register WeightsAndBiases types: %v", err)
	}

	instance := &apiv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
		Annotations: map[string]string{
			"example.com/unrelated": "preserved",
		},
	}}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	reconciler := &WeightsAndBiasesReconciler{Client: k8sClient}

	if err := reconciler.ensureApplyPendingSince(ctx, instance, now); err != nil {
		t.Fatalf("ensureApplyPendingSince() error = %v", err)
	}
	assertAnnotation(t, ctx, k8sClient, operatormetrics.ApplyPendingSinceAnnotation, now.Format(time.RFC3339Nano))

	if err := reconciler.ensureApplyPendingSince(ctx, instance, now.Add(time.Hour)); err != nil {
		t.Fatalf("second ensureApplyPendingSince() error = %v", err)
	}
	assertAnnotation(t, ctx, k8sClient, operatormetrics.ApplyPendingSinceAnnotation, now.Format(time.RFC3339Nano))

	if err := reconciler.clearApplyPendingSince(ctx, instance); err != nil {
		t.Fatalf("clearApplyPendingSince() error = %v", err)
	}
	assertAnnotation(t, ctx, k8sClient, operatormetrics.ApplyPendingSinceAnnotation, "")
	assertAnnotation(t, ctx, k8sClient, "example.com/unrelated", "preserved")
}

func TestEnsureApplyPendingSinceReplacesInvalidTimestamp(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	scheme := runtime.NewScheme()
	if err := apiv1.AddToScheme(scheme); err != nil {
		t.Fatalf("could not register WeightsAndBiases types: %v", err)
	}

	instance := &apiv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
		Annotations: map[string]string{
			operatormetrics.ApplyPendingSinceAnnotation: "not-a-time",
		},
	}}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	reconciler := &WeightsAndBiasesReconciler{Client: k8sClient}

	if err := reconciler.ensureApplyPendingSince(ctx, instance, now); err != nil {
		t.Fatalf("ensureApplyPendingSince() error = %v", err)
	}
	assertAnnotation(t, ctx, k8sClient, operatormetrics.ApplyPendingSinceAnnotation, now.Format(time.RFC3339Nano))
}

func assertAnnotation(
	t *testing.T,
	ctx context.Context,
	k8sClient client.Client,
	key string,
	want string,
) {
	t.Helper()
	instance := &apiv1.WeightsAndBiases{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test", Namespace: "default"}, instance); err != nil {
		t.Fatalf("could not get WeightsAndBiases instance: %v", err)
	}
	if got := instance.Annotations[key]; got != want {
		t.Fatalf("annotation %q = %q, want %q", key, got, want)
	}
}
