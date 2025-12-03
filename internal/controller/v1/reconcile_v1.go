/*
Copyright 2023.

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

package v1

import (
	"context"
	"reflect"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/record"

	"github.com/wandb/operator/pkg/wandb/spec/state"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer"
	"github.com/wandb/operator/pkg/wandb/spec/operator"
	"github.com/wandb/operator/pkg/wandb/spec/state/secrets"
	"github.com/wandb/operator/pkg/wandb/spec/utils"
	"github.com/wandb/operator/pkg/wandb/status"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const resFinalizer = "finalizer.app.wandb.com"

type WandbV1ReconcileConfig struct {
	DeployerClient deployer.DeployerInterface
	Recorder       record.EventRecorder
	DryRun         bool
	IsAirgapped    bool
	Debug          bool
}

func Reconcile(
	ctx context.Context,
	req ctrl.Request,
	client client.Client,
	config *WandbV1ReconcileConfig,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info(
		"=== Reconciling Weights & Biases instance...",
		"NamespacedName", req.NamespacedName,
		"Name", req.Name,
		"start", true,
	)

	wandb := &apiv1.WeightsAndBiases{}
	if err := client.Get(ctx, req.NamespacedName, wandb); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrlqueue.DoNotRequeue()
		}
		return ctrlqueue.RequeueWithError(err)
	}

	log.Info(
		"Found Weights & Biases instance, processing the spec...",
		"Spec", wandb.Spec,
	)

	config.Recorder.Event(wandb, corev1.EventTypeNormal, "Reconciling", "Reconciling")

	statusManager := status.NewManager(ctx, client, wandb)
	configMapState := secrets.New(ctx, client, wandb, client.Scheme())
	specManager := state.New(ctx, client, wandb, client.Scheme(), configMapState)

	config.Recorder.Event(wandb, corev1.EventTypeNormal, "LoadingConfig", "Loading desired configuration")

	userInputSpec, err := specManager.GetUserInput()
	if apierrors.IsNotFound(err) {
		log.Info("No user input spec found, creating a new one")
		userInputSpec = &spec.Spec{Values: map[string]interface{}{}}
		err = specManager.SetUserInput(userInputSpec)
		if err != nil {
			return ctrlqueue.RequeueWithError(err)
		}
	} else if err != nil {
		log.Error(err, "error retrieving user input spec")
		return ctrlqueue.RequeueWithError(err)
	}

	var releaseID string
	if releaseIDValue, ok := userInputSpec.Values["_releaseId"].(string); ok {
		releaseID = releaseIDValue
		log.Info("Version Pinning is enabled", "releaseId:", releaseID)
	}

	crdSpec := operator.Spec(wandb)

	currentActiveSpec, err := specManager.GetActive()
	if err != nil {
		// This scenario can happen if we have not successfully deployed in the
		// past.
		log.Info("No active spec found.")
	}

	license := utils.GetLicense(ctx, client, wandb, crdSpec, userInputSpec)

	var deployerSpec *spec.Spec
	if !config.IsAirgapped {
		deployerSpec, err = config.DeployerClient.GetSpec(deployer.GetSpecOptions{
			License:     license,
			ActiveState: currentActiveSpec,
			ReleaseId:   releaseID,
			Debug:       config.Debug,
		})
		if err != nil {
			log.Info("Failed to get spec from deployer", "error", err)
			// This scenario may occur if the user disables networking, or if the deployer
			// is not operational, and a version has been deployed successfully. Rather than
			// reverting to the container defaults, we've stored the most recent successful
			// deployer release in the cache
			// Attempt to retrieve the cached release
			if deployerSpec, err = specManager.Get("latest-cached-release"); err != nil {
				log.Info("No cached release found", "error", err.Error())
			}
			if config.Debug && deployerSpec != nil {
				log.Info("Using cached deployer spec", "spec", deployerSpec.SensitiveValuesMasked())
			}
		}

		if deployerSpec != nil {
			if config.Debug {
				log.Info("Writing deployer spec to cache", "spec", deployerSpec.SensitiveValuesMasked())
			}
			if err := specManager.Set("latest-cached-release", deployerSpec); err != nil {
				config.Recorder.Event(wandb, corev1.EventTypeNormal, "SecretWriteFailed", "Unable to write secret to kubernetes")
				log.Error(err, "Unable to save latest release.")
				return ctrlqueue.DoNotRequeue()
			}
		}
	}

	desiredSpec := new(spec.Spec)

	if config.Debug {
		log.Info("Initial desired spec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	// First takes precedence
	if err := desiredSpec.Merge(crdSpec); err != nil {
		log.Error(err, "Failed to merge CRD spec into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}
	if config.Debug {
		log.Info("Desired spec after merging crdSpec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	if err := desiredSpec.Merge(userInputSpec); err != nil {
		log.Error(err, "Failed to merge user input spec into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}
	if config.Debug {
		log.Info("Desired spec after merging userInputSpec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	if err := desiredSpec.Merge(deployerSpec); err != nil {
		log.Error(err, "Failed to merge deployer spec into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}

	if config.Debug {
		log.Info("Desired spec after merging deployerSpec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	if err := desiredSpec.Merge(operator.Defaults(wandb, client.Scheme())); err != nil {
		log.Error(err, "Failed to merge operator defaults into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}
	if config.Debug {
		log.Info("Desired spec after merging operator defaults", "spec", desiredSpec.SensitiveValuesMasked())
	}

	log.Info("Desired spec", "spec", desiredSpec.SensitiveValuesMasked())

	hasNotBeenFlaggedForDeletion := wandb.ObjectMeta.DeletionTimestamp.IsZero()
	if hasNotBeenFlaggedForDeletion {
		if currentActiveSpec != nil {
			log.Info("Active spec found", "spec", currentActiveSpec.SensitiveValuesMasked())
			if currentActiveSpec.IsEqual(desiredSpec) {
				log.Info("No changes found")
				statusManager.Set(status.Completed)
				return ctrlqueue.Requeue(desiredSpec)
			} else {
				diff := currentActiveSpec.DiffValues(desiredSpec)
				log.Info("Changes found", "diff", diff)
			}
		}

		if desiredSpec.Chart == nil {
			statusManager.Set(status.InvalidConfig)
			log.Error(err, "No release type was found in the spec")
			return ctrlqueue.Requeue(desiredSpec)
		}
		t := reflect.TypeOf(desiredSpec.Chart)
		typ := t.Name()
		if t.Kind() == reflect.Ptr {
			typ = "*" + t.Elem().Name()
		}
		log.Info("Found release type "+typ, "release", reflect.TypeOf(desiredSpec.Chart))

		statusManager.Set(status.Loading)

		if !ctrlqueue.ContainsString(wandb.GetFinalizers(), resFinalizer) {
			wandb.ObjectMeta.Finalizers = append(wandb.ObjectMeta.Finalizers, resFinalizer)
			if err := client.Update(ctx, wandb); err != nil {
				return ctrlqueue.Requeue(desiredSpec)
			}
		}

		log.Info("Applying spec...", "spec", desiredSpec.SensitiveValuesMasked())
		if !config.DryRun {
			if err := desiredSpec.Apply(ctx, client, wandb, client.Scheme()); err != nil {
				statusManager.Set(status.InvalidConfig)
				config.Recorder.Event(wandb, corev1.EventTypeNormal, "ApplyFailed", "Invalid config for apply")
				log.Error(err, "Failed to apply config changes.")
				return ctrlqueue.Requeue(desiredSpec)
			}
		}
		log.Info("Successfully applied spec", "spec", desiredSpec.SensitiveValuesMasked())

		if err := specManager.SetActive(desiredSpec); err != nil {
			config.Recorder.Event(wandb, corev1.EventTypeNormal, "SetActiveFailed", "Failed to save active state")
			log.Error(err, "Failed to save active successful spec.")
			statusManager.Set(status.InvalidConfig)
			return ctrlqueue.Requeue(desiredSpec)
		}
		if config.Debug {
			log.Info("Successfully saved active spec", "spec", desiredSpec.SensitiveValuesMasked())
		}

		config.Recorder.Event(wandb, corev1.EventTypeNormal, "Completed", "Completed reconcile successfully")
		if err := discoverAndPatchResources(ctx, client, wandb); err != nil {
			log.Error(err, "Failed to discover and patch resources")
			return ctrlqueue.Requeue(desiredSpec)
		}
		statusManager.Set(status.Completed)

		return ctrlqueue.Requeue(desiredSpec)
	}

	if ctrlqueue.ContainsString(wandb.ObjectMeta.Finalizers, resFinalizer) {
		if desiredSpec.Chart != nil {
			log.Info("Deprovisioning", "release", reflect.TypeOf(desiredSpec.Chart))
			if !config.DryRun {
				if err := desiredSpec.Prune(ctx, client, wandb, client.Scheme()); err != nil {
					log.Error(err, "Failed to cleanup deployment.")
					return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
				} else {
					log.Info("Successfully cleaned up resources")
				}
			}
		}

		controllerutil.RemoveFinalizer(wandb, resFinalizer)
		client.Update(ctx, wandb)
	}

	return ctrlqueue.DoNotRequeue()
}

func discoverAndPatchResources(ctx context.Context, c client.Client, wandb *apiv1.WeightsAndBiases) error {
	log := ctrllog.FromContext(ctx)
	var managedResources []client.Object
	resourceKinds := []struct {
		name string
		list client.ObjectList
	}{
		{"Deployment", &appsv1.DeploymentList{}},
		{"StatefulSet", &appsv1.StatefulSetList{}},
		{"Ingress", &networkingv1.IngressList{}},
		{"DaemonSet", &appsv1.DaemonSetList{}},
		{"Service", &corev1.ServiceList{}},
		{"ConfigMap", &corev1.ConfigMapList{}},
		{"Secret", &corev1.SecretList{}},
		{"Role", &rbacv1.RoleList{}},
		{"RoleBinding", &rbacv1.RoleBindingList{}},
	}

	// Discover resources
	for _, resourceKind := range resourceKinds {
		log.Info("Fetching resources managed by Helm chart 'wandb'", "kind", resourceKind.name)

		labels := client.MatchingLabels{
			"app.kubernetes.io/managed-by": "Helm",
			"app.kubernetes.io/instance":   wandb.ObjectMeta.Name,
		}
		if err := c.List(ctx, resourceKind.list, client.InNamespace(wandb.Namespace), labels); err != nil {
			log.Error(err, "Failed to list resources", "kind", resourceKind.name)
			continue
		}

		items := reflect.ValueOf(resourceKind.list).Elem().FieldByName("Items")
		for i := 0; i < items.Len(); i++ {
			resource := items.Index(i).Addr().Interface().(client.Object)
			managedResources = append(managedResources, resource)
			log.Info("Found resource", "name", resource.GetName(), "kind", resourceKind.name)
		}
	}

	// Add owner references to discovered resources
	for _, resource := range managedResources {
		if err := controllerutil.SetOwnerReference(wandb, resource, c.Scheme()); err != nil {
			log.Error(err, "Failed to set owner reference", "resource", resource.GetName(), "kind", resource.GetObjectKind().GroupVersionKind().Kind)
			continue
		}
		if err := c.Update(ctx, resource); err != nil {
			log.Error(err, "Failed to update resource with owner reference", "resource", resource.GetName(), "kind", resource.GetObjectKind().GroupVersionKind().Kind)
		} else {
			log.Info("Owner reference added successfully", "resource", resource.GetName(), "kind", resource.GetObjectKind().GroupVersionKind().Kind)
		}
	}
	return nil
}
