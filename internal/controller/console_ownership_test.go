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
	"errors"
	"testing"
	"time"

	wandbcomv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer/deployerfakes"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type recordingChart struct {
	applied bool
}

func (c *recordingChart) Apply(
	context.Context,
	client.Client,
	*wandbcomv1.WeightsAndBiases,
	*runtime.Scheme,
	spec.Values,
) error {
	c.applied = true
	return nil
}

func (c *recordingChart) Prune(
	context.Context,
	client.Client,
	*wandbcomv1.WeightsAndBiases,
	*runtime.Scheme,
	spec.Values,
) error {
	return nil
}

func (c *recordingChart) Chart() (*chart.Chart, error) {
	return &chart.Chart{}, nil
}

func TestValidateConsoleServiceOwnership(t *testing.T) {
	t.Parallel()

	ownedByStandaloneConsole := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-console",
			Namespace: "customer",
			Annotations: map[string]string{
				"meta.helm.sh/release-name": "console",
			},
		},
	}
	ownedByAnotherRelease := ownedByStandaloneConsole.DeepCopy()
	ownedByAnotherRelease.Annotations["meta.helm.sh/release-name"] = "another-release"

	tests := []struct {
		name            string
		consoleEnabled  bool
		objects         []runtime.Object
		wantConflictErr bool
	}{
		{
			name:           "allows standalone service while bundled console is disabled",
			consoleEnabled: false,
			objects:        []runtime.Object{ownedByStandaloneConsole},
		},
		{
			name:           "allows bundled console when target service does not exist",
			consoleEnabled: true,
		},
		{
			name:           "allows bundled console when another Helm release owns the target service",
			consoleEnabled: true,
			objects:        []runtime.Object{ownedByAnotherRelease},
		},
		{
			name:            "rejects bundled console when standalone release owns the target service",
			consoleEnabled:  true,
			objects:         []runtime.Object{ownedByStandaloneConsole},
			wantConflictErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("add core scheme: %v", err)
			}
			objects := make([]runtime.Object, len(tt.objects))
			for i, object := range tt.objects {
				objects[i] = object.DeepCopyObject()
			}
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
			wandb := &wandbcomv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "customer"}}
			desired := &spec.Spec{Values: spec.Values{
				"console": map[string]interface{}{"install": tt.consoleEnabled},
			}}

			err := validateConsoleServiceOwnership(context.Background(), client, wandb, desired)
			if tt.wantConflictErr {
				if !errors.Is(err, errStandaloneConsoleOwnsService) {
					t.Fatalf("expected ownership conflict, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestReconcileStopsBeforeApplyOnConsoleOwnershipConflict(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := wandbcomv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add W&B scheme: %v", err)
	}

	wandb := &wandbcomv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "customer"}}
	standaloneService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      "wandb-console",
		Namespace: "customer",
		Annotations: map[string]string{
			helmReleaseNameAnnotation: standaloneConsoleRelease,
		},
	}}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(wandb, standaloneService).Build()

	recorder := record.NewFakeRecorder(10)
	chart := &recordingChart{}
	deployerClient := &deployerfakes.FakeDeployerInterface{}
	deployerClient.GetSpecReturns(&spec.Spec{
		Chart: chart,
		Values: spec.Values{
			"console": map[string]interface{}{"install": true},
		},
	}, nil)
	reconciler := &WeightsAndBiasesReconciler{
		Client:         k8sClient,
		DeployerClient: deployerClient,
		Scheme:         scheme,
		Recorder:       recorder,
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{
		Name:      wandb.Name,
		Namespace: wandb.Namespace,
	}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.RequeueAfter != time.Hour {
		t.Fatalf("expected one-hour requeue, got %v", result.RequeueAfter)
	}
	if chart.applied {
		t.Fatal("chart Apply was called despite the ownership conflict")
	}

	activeState := &corev1.Secret{}
	err = k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      "wandb-spec-active",
		Namespace: wandb.Namespace,
	}, activeState)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected no active spec to be saved, got %v", err)
	}

	foundConflictEvent := false
	for len(recorder.Events) > 0 {
		if event := <-recorder.Events; event == "Warning ConsoleOwnershipConflict Bundled Console cannot be enabled while the standalone Console release owns its Service" {
			foundConflictEvent = true
		}
	}
	if !foundConflictEvent {
		t.Fatal("expected a controlled ConsoleOwnershipConflict warning event")
	}
}

func TestReconcileDetectsConsoleOwnershipConflictWhenSpecIsUnchanged(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := wandbcomv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add W&B scheme: %v", err)
	}

	wandb := &wandbcomv1.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "customer"}}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(wandb).Build()

	recorder := record.NewFakeRecorder(20)
	chart := &charts.RepoRelease{
		URL:     "https://charts.wandb.ai",
		Name:    "operator-wandb",
		Version: "0.44.6",
	}
	deployerClient := &deployerfakes.FakeDeployerInterface{}
	deployerClient.GetSpecCalls(func(options deployer.GetSpecOptions) (*spec.Spec, error) {
		if options.ActiveState != nil {
			return options.ActiveState, nil
		}
		return &spec.Spec{
			Chart: chart,
			Values: spec.Values{
				"console": map[string]interface{}{"install": true},
			},
		}, nil
	})
	reconciler := &WeightsAndBiasesReconciler{
		Client:         k8sClient,
		DeployerClient: deployerClient,
		Scheme:         scheme,
		Recorder:       recorder,
		DryRun:         true,
	}
	request := ctrl.Request{NamespacedName: types.NamespacedName{
		Name:      wandb.Name,
		Namespace: wandb.Namespace,
	}}

	if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}
	if deployerClient.GetSpecCallCount() != 1 {
		t.Fatalf("expected one deployer call after initial reconcile, got %d", deployerClient.GetSpecCallCount())
	}
	for len(recorder.Events) > 0 {
		<-recorder.Events
	}

	standaloneService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      "wandb-console",
		Namespace: wandb.Namespace,
		Annotations: map[string]string{
			helmReleaseNameAnnotation: standaloneConsoleRelease,
		},
	}}
	if err := k8sClient.Create(context.Background(), standaloneService); err != nil {
		t.Fatalf("create standalone Console Service: %v", err)
	}

	if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("unchanged reconcile: %v", err)
	}
	if deployerClient.GetSpecCallCount() != 2 {
		t.Fatalf("expected two deployer calls after unchanged reconcile, got %d", deployerClient.GetSpecCallCount())
	}
	if deployerClient.GetSpecArgsForCall(1).ActiveState == nil {
		t.Fatal("expected unchanged reconcile to load the active spec")
	}

	foundConflictEvent := false
	for len(recorder.Events) > 0 {
		if event := <-recorder.Events; event == "Warning ConsoleOwnershipConflict Bundled Console cannot be enabled while the standalone Console release owns its Service" {
			foundConflictEvent = true
		}
	}
	if !foundConflictEvent {
		t.Fatal("expected the unchanged reconciliation to report a ConsoleOwnershipConflict warning event")
	}
}
