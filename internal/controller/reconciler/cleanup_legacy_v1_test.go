/*
Copyright 2025.

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

package reconciler

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

func newCleanupFixtureScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))
	return scheme
}

func legacyDeployment(wandbName, suffix, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", wandbName, suffix),
			Namespace: namespace,
		},
	}
}

func TestLegacyV1AppsHealthy(t *testing.T) {
	tests := []struct {
		name        string
		statuses    map[string]apiv2.ApplicationStatus
		desired     map[string]bool
		wantHealthy bool
	}{
		{
			name:        "empty desired set is never healthy",
			statuses:    map[string]apiv2.ApplicationStatus{"api": {Ready: true}},
			desired:     map[string]bool{},
			wantHealthy: false,
		},
		{
			name:        "missing status entry blocks gate",
			statuses:    map[string]apiv2.ApplicationStatus{"api": {Ready: true}},
			desired:     map[string]bool{"api": true, "console": true},
			wantHealthy: false,
		},
		{
			name:        "one not-ready app blocks gate",
			statuses:    map[string]apiv2.ApplicationStatus{"api": {Ready: true}, "console": {Ready: false}},
			desired:     map[string]bool{"api": true, "console": true},
			wantHealthy: false,
		},
		{
			name:        "all desired apps ready opens gate",
			statuses:    map[string]apiv2.ApplicationStatus{"api": {Ready: true}, "console": {Ready: true}},
			desired:     map[string]bool{"api": true, "console": true},
			wantHealthy: true,
		},
		{
			name:        "extra status entries do not block gate",
			statuses:    map[string]apiv2.ApplicationStatus{"api": {Ready: true}, "console": {Ready: true}, "stale": {Ready: false}},
			desired:     map[string]bool{"api": true, "console": true},
			wantHealthy: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.wantHealthy, appsHealthy(tc.statuses, tc.desired))
		})
	}
}

func TestBuildDesiredAppNames(t *testing.T) {
	manifest := serverManifest.Manifest{
		Features: map[string]bool{
			"weaveTrace": true,
			"disabled":   false,
		},
		Applications: map[string]serverManifest.Application{
			"api":               {Name: "api"},
			"console":           {Name: "console"},
			"weave-trace":       {Name: "weave-trace", Features: []string{"weaveTrace"}},
			"behind-disabled":   {Name: "behind-disabled", Features: []string{"disabled"}},
			"behind-unset-flag": {Name: "behind-unset-flag", Features: []string{"missing"}},
		},
	}

	got := buildDesiredAppNames(manifest)
	require.Equal(t, map[string]bool{
		"api":         true,
		"console":     true,
		"weave-trace": true,
	}, got)
}

func TestCleanupLegacyV1Deployments(t *testing.T) {
	const (
		wandbName = "wandb"
		namespace = "default"
	)

	allSuffixes := legacyV1DeploymentSuffixes

	t.Run("no deployments seeded is a no-op", func(t *testing.T) {
		scheme := newCleanupFixtureScheme(t)
		c := fake.NewClientBuilder().WithScheme(scheme).Build()
		wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: wandbName, Namespace: namespace}}

		require.NoError(t, cleanupLegacyV1Deployments(context.Background(), c, wandb))
	})

	t.Run("all five deployments are deleted when present", func(t *testing.T) {
		scheme := newCleanupFixtureScheme(t)
		var seeds []ctrlClient.Object
		for _, suffix := range allSuffixes {
			seeds = append(seeds, legacyDeployment(wandbName, suffix, namespace))
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(seeds...).Build()
		wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: wandbName, Namespace: namespace}}

		require.NoError(t, cleanupLegacyV1Deployments(context.Background(), c, wandb))

		for _, suffix := range allSuffixes {
			err := c.Get(context.Background(), types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", wandbName, suffix),
				Namespace: namespace,
			}, &appsv1.Deployment{})
			require.True(t, apiErrors.IsNotFound(err), "expected %s-%s deleted, got %v", wandbName, suffix, err)
		}
	})

	t.Run("only present deployments are touched", func(t *testing.T) {
		scheme := newCleanupFixtureScheme(t)
		present := []string{"app-bc", "weave-bc"}
		var seeds []ctrlClient.Object
		for _, suffix := range present {
			seeds = append(seeds, legacyDeployment(wandbName, suffix, namespace))
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(seeds...).Build()
		wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: wandbName, Namespace: namespace}}

		require.NoError(t, cleanupLegacyV1Deployments(context.Background(), c, wandb))

		for _, suffix := range present {
			err := c.Get(context.Background(), types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", wandbName, suffix),
				Namespace: namespace,
			}, &appsv1.Deployment{})
			require.True(t, apiErrors.IsNotFound(err), "expected %s-%s deleted, got %v", wandbName, suffix, err)
		}
	})

	t.Run("non-suffixed deployments in same namespace are untouched", func(t *testing.T) {
		scheme := newCleanupFixtureScheme(t)
		other := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: wandbName + "-app", Namespace: namespace},
		}
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(other).Build()
		wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: wandbName, Namespace: namespace}}

		require.NoError(t, cleanupLegacyV1Deployments(context.Background(), c, wandb))

		require.NoError(t, c.Get(context.Background(), types.NamespacedName{
			Name: wandbName + "-app", Namespace: namespace,
		}, &appsv1.Deployment{}))
	})

	t.Run("delete failure on one suffix does not strand the rest", func(t *testing.T) {
		scheme := newCleanupFixtureScheme(t)
		var seeds []ctrlClient.Object
		for _, suffix := range allSuffixes {
			seeds = append(seeds, legacyDeployment(wandbName, suffix, namespace))
		}
		failOn := fmt.Sprintf("%s-%s", wandbName, allSuffixes[0])

		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(seeds...).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cl ctrlClient.WithWatch, obj ctrlClient.Object, opts ...ctrlClient.DeleteOption) error {
					if obj.GetName() == failOn {
						return fmt.Errorf("synthetic delete error")
					}
					return cl.Delete(ctx, obj, opts...)
				},
			}).
			Build()
		wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: wandbName, Namespace: namespace}}

		err := cleanupLegacyV1Deployments(context.Background(), c, wandb)
		require.Error(t, err)
		require.Contains(t, err.Error(), "synthetic delete error")

		err = c.Get(context.Background(), types.NamespacedName{Name: failOn, Namespace: namespace}, &appsv1.Deployment{})
		require.NoError(t, err, "deployment that failed to delete should still exist")

		for _, suffix := range allSuffixes[1:] {
			err := c.Get(context.Background(), types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", wandbName, suffix),
				Namespace: namespace,
			}, &appsv1.Deployment{})
			require.True(t, apiErrors.IsNotFound(err), "expected %s-%s deleted, got %v", wandbName, suffix, err)
		}
	})
}

func TestNotReadyApps(t *testing.T) {
	statuses := map[string]apiv2.ApplicationStatus{
		"api":      {Ready: false},
		"frontend": {Ready: true},
		"weave":    {Ready: false},
	}
	desired := map[string]bool{"api": true, "frontend": true, "weave": true, "glue": true}
	require.Equal(t, []string{"api", "glue", "weave"}, notReadyApps(statuses, desired),
		"unready and missing apps are listed sorted")

	require.Empty(t, notReadyApps(map[string]apiv2.ApplicationStatus{"api": {Ready: true}}, map[string]bool{"api": true}))
}
