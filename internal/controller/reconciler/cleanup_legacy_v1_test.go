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

// readyDeployment builds a fully rolled-out Deployment for gate tests.
func readyDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Generation: 2},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2,
			Replicas:           1,
			ReadyReplicas:      1,
		},
	}
}

func TestDeploymentsHealthy(t *testing.T) {
	const namespace = "default"
	desired := map[string]bool{"api": true, "console": true}

	tests := []struct {
		name        string
		desired     map[string]bool
		deployments []*appsv1.Deployment
		wantHealthy bool
		wantBlocked []string
	}{
		{
			name:        "empty desired set is never healthy",
			desired:     map[string]bool{},
			wantHealthy: false,
		},
		{
			name:        "missing deployment blocks gate",
			desired:     desired,
			deployments: []*appsv1.Deployment{readyDeployment("api", namespace)},
			wantHealthy: false,
			wantBlocked: []string{"console"},
		},
		{
			name:    "mid-rollout deployment blocks gate",
			desired: desired,
			deployments: func() []*appsv1.Deployment {
				rolling := readyDeployment("console", namespace)
				rolling.Status.ReadyReplicas = 0
				return []*appsv1.Deployment{readyDeployment("api", namespace), rolling}
			}(),
			wantHealthy: false,
			wantBlocked: []string{"console"},
		},
		{
			name:    "zero replicas blocks gate",
			desired: desired,
			deployments: func() []*appsv1.Deployment {
				scaled := readyDeployment("console", namespace)
				scaled.Status.Replicas = 0
				scaled.Status.ReadyReplicas = 0
				return []*appsv1.Deployment{readyDeployment("api", namespace), scaled}
			}(),
			wantHealthy: false,
			wantBlocked: []string{"console"},
		},
		{
			name:    "stale observedGeneration blocks gate",
			desired: desired,
			deployments: func() []*appsv1.Deployment {
				stale := readyDeployment("console", namespace)
				stale.Status.ObservedGeneration = 1
				return []*appsv1.Deployment{readyDeployment("api", namespace), stale}
			}(),
			wantHealthy: false,
			wantBlocked: []string{"console"},
		},
		{
			name:        "all deployments rolled out opens gate",
			desired:     desired,
			deployments: []*appsv1.Deployment{readyDeployment("api", namespace), readyDeployment("console", namespace)},
			wantHealthy: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(newCleanupFixtureScheme(t))
			for _, dep := range tc.deployments {
				builder = builder.WithObjects(dep)
			}
			healthy, blocked := deploymentsHealthy(context.Background(), builder.Build(), namespace, tc.desired)
			require.Equal(t, tc.wantHealthy, healthy)
			require.Equal(t, tc.wantBlocked, blocked)
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

func TestDeploymentsHealthy_BlockedListSorted(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(newCleanupFixtureScheme(t)).Build()
	healthy, blocked := deploymentsHealthy(context.Background(), cl, "default",
		map[string]bool{"weave": true, "api": true, "glue": true})
	require.False(t, healthy)
	require.Equal(t, []string{"api", "glue", "weave"}, blocked)
}
