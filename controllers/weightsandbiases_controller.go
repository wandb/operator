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

package controllers

import (
	"context"
	rbacv1 "k8s.io/api/rbac/v1"
	"reflect"

	"github.com/wandb/operator/pkg/wandb/spec/state"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"

	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/controllers/internal/ctrlqueue"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer"
	"github.com/wandb/operator/pkg/wandb/spec/operator"
	"github.com/wandb/operator/pkg/wandb/spec/state/secrets"
	"github.com/wandb/operator/pkg/wandb/spec/utils"
	"github.com/wandb/operator/pkg/wandb/status"

	"k8s.io/apimachinery/pkg/api/errors"
)

const resFinalizer = "finalizer.app.wandb.com"

// WeightsAndBiasesReconciler reconciles a WeightsAndBiases object
type WeightsAndBiasesReconciler struct {
	client.Client
	IsAirgapped    bool
	DeployerClient deployer.DeployerInterface
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	DryRun         bool
	Debug          bool
}

//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps;events;persistentvolumeclaims;secrets;serviceaccounts;services,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups="",resources=endpoints;ingresses;nodes;nodes/spec;nodes/stats;nodes/metrics;nodes/proxy;namespaces;namespaces/status;replicationcontrollers;replicationcontrollers/status;resourcequotas;pods;pods/log;pods/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments;controllerrevisions;daemonsets;replicasets;statefulsets,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=apps,resources=deployments/status;daemonsets/status;replicasets/status;statefulsets/status,verbs=get
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=batch,resources=cronjobs;jobs,verbs=get;list;watch;create;delete;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=list;watch
//+kubebuilder:rbac:groups=cloud.google.com,resources=backendconfigs,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;ingresses/status;networkpolicies,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:urls=/metrics,verbs=get

// Deprecated/Erroneously required RBAC rules
//+kubebuilder:rbac:groups=extensions,resources=daemonsets;deployments;replicasets;ingresses;ingresses/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WeightsAndBiasesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info(
		"=== Reconciling Weights & Biases instance...",
		"NamespacedName", req.NamespacedName,
		"Name", req.Name,
		"start", true,
	)

	wandb := &apiv1.WeightsAndBiases{}
	if err := r.Client.Get(ctx, req.NamespacedName, wandb); err != nil {
		if errors.IsNotFound(err) {
			return ctrlqueue.DoNotRequeue()
		}
		return ctrlqueue.RequeueWithError(err)
	}

	log.Info(
		"Found Weights & Biases instance, processing the spec...",
		"Spec", wandb.Spec,
	)

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "Reconciling", "Reconciling")

	statusManager := status.NewManager(ctx, r.Client, wandb)
	configMapState := secrets.New(ctx, r.Client, wandb, r.Scheme)
	specManager := state.New(ctx, r.Client, wandb, r.Scheme, configMapState)

	r.Recorder.Event(wandb, corev1.EventTypeNormal, "LoadingConfig", "Loading desired configuration")

	userInputSpec, err := specManager.GetUserInput()
	if errors.IsNotFound(err) {
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

	license := utils.GetLicense(ctx, r.Client, wandb, crdSpec, userInputSpec)

	var deployerSpec *spec.Spec
	if !r.IsAirgapped {
		deployerSpec, err = r.DeployerClient.GetSpec(deployer.GetSpecOptions{
			License:     license,
			ActiveState: currentActiveSpec,
			ReleaseId:   releaseID,
			Debug:       r.Debug,
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
			if r.Debug {
				log.Info("Using cached deployer spec", "spec", deployerSpec.SensitiveValuesMasked())
			}
		}

		if deployerSpec != nil {
			if r.Debug {
				log.Info("Writing deployer spec to cache", "spec", deployerSpec.SensitiveValuesMasked())
			}
			if err := specManager.Set("latest-cached-release", deployerSpec); err != nil {
				r.Recorder.Event(wandb, corev1.EventTypeNormal, "SecretWriteFailed", "Unable to write secret to kubernetes")
				log.Error(err, "Unable to save latest release.")
				return ctrlqueue.DoNotRequeue()
			}
		}
	}

	desiredSpec := new(spec.Spec)

	if r.Debug {
		log.Info("Initial desired spec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	// First takes precedence
	if err := desiredSpec.Merge(crdSpec); err != nil {
		log.Error(err, "Failed to merge CRD spec into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}
	if r.Debug {
		log.Info("Desired spec after merging crdSpec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	if err := desiredSpec.Merge(userInputSpec); err != nil {
		log.Error(err, "Failed to merge user input spec into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}
	if r.Debug {
		log.Info("Desired spec after merging userInputSpec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	if err := desiredSpec.Merge(deployerSpec); err != nil {
		log.Error(err, "Failed to merge deployer spec into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}

	if r.Debug {
		log.Info("Desired spec after merging deployerSpec", "spec", desiredSpec.SensitiveValuesMasked())
	}

	if err := desiredSpec.Merge(operator.Defaults(wandb, r.Scheme)); err != nil {
		log.Error(err, "Failed to merge operator defaults into desired spec")
		return ctrlqueue.RequeueWithError(err)
	}
	if r.Debug {
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
			if err := r.Client.Update(ctx, wandb); err != nil {
				return ctrlqueue.Requeue(desiredSpec)
			}
		}

		log.Info("Applying spec...", "spec", desiredSpec.SensitiveValuesMasked())
		if !r.DryRun {
			if err := desiredSpec.Apply(ctx, r.Client, wandb, r.Scheme); err != nil {
				statusManager.Set(status.InvalidConfig)
				r.Recorder.Event(wandb, corev1.EventTypeNormal, "ApplyFailed", "Invalid config for apply")
				log.Error(err, "Failed to apply config changes.")
				return ctrlqueue.Requeue(desiredSpec)
			}
		}
		log.Info("Successfully applied spec", "spec", desiredSpec.SensitiveValuesMasked())

		if err := specManager.SetActive(desiredSpec); err != nil {
			r.Recorder.Event(wandb, corev1.EventTypeNormal, "SetActiveFailed", "Failed to save active state")
			log.Error(err, "Failed to save active successful spec.")
			statusManager.Set(status.InvalidConfig)
			return ctrlqueue.Requeue(desiredSpec)
		}
		if r.Debug {
			log.Info("Successfully saved active spec", "spec", desiredSpec.SensitiveValuesMasked())
		}

		r.Recorder.Event(wandb, corev1.EventTypeNormal, "Completed", "Completed reconcile successfully")
		if err := r.discoverAndPatchResources(ctx, wandb); err != nil {
			log.Error(err, "Failed to discover and patch resources")
			return ctrlqueue.Requeue(desiredSpec)
		}
		statusManager.Set(status.Completed)

		return ctrlqueue.Requeue(desiredSpec)
	}

	if ctrlqueue.ContainsString(wandb.ObjectMeta.Finalizers, resFinalizer) {
		if desiredSpec.Chart != nil {
			log.Info("Deprovisioning", "release", reflect.TypeOf(desiredSpec.Chart))
			if !r.DryRun {
				if err := desiredSpec.Prune(ctx, r.Client, wandb, r.Scheme); err != nil {
					log.Error(err, "Failed to cleanup deployment.")
				} else {
					log.Info("Successfully cleaned up resources")
				}
			}
		}

		controllerutil.RemoveFinalizer(wandb, resFinalizer)
		r.Client.Update(ctx, wandb)
	}

	return ctrlqueue.DoNotRequeue()
}

func (r *WeightsAndBiasesReconciler) discoverAndPatchResources(ctx context.Context, wandb *apiv1.WeightsAndBiases) error {
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
		if err := r.Client.List(ctx, resourceKind.list, client.InNamespace(wandb.Namespace), labels); err != nil {
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
		if err := controllerutil.SetOwnerReference(wandb, resource, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference", "resource", resource.GetName(), "kind", resource.GetObjectKind().GroupVersionKind().Kind)
			continue
		}
		if err := r.Client.Update(ctx, resource); err != nil {
			log.Error(err, "Failed to update resource with owner reference", "resource", resource.GetName(), "kind", resource.GetObjectKind().GroupVersionKind().Kind)
		} else {
			log.Info("Owner reference added successfully", "resource", resource.GetName(), "kind", resource.GetObjectKind().GroupVersionKind().Kind)
		}
	}
	return nil
}

func (r *WeightsAndBiasesReconciler) Delete(e event.DeleteEvent) bool {
	return !e.DeleteStateUnknown
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&apiv1.WeightsAndBiases{}, builder.WithPredicates(filterWBEvents{})).
		Owns(&corev1.Secret{}, builder.WithPredicates(filterSecretEvents{})).
		Owns(&corev1.ConfigMap{})
	return builder.Complete(r)
}

type filterWBEvents struct {
	predicate.Funcs
}

func (filterWBEvents) Update(e event.UpdateEvent) bool {
	// Checking whether the Object's Generation has changed. If it has not
	// (indicating a non-spec change), it returns false - thus ignoring the
	// event.
	return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
}

func (filterWBEvents) Create(e event.CreateEvent) bool {
	return true
}

func (filterWBEvents) Delete(e event.DeleteEvent) bool {
	return true
}

func (filterWBEvents) Generic(e event.GenericEvent) bool {
	return false
}

type filterSecretEvents struct {
	predicate.Funcs
}

func (filterSecretEvents) Update(e event.UpdateEvent) bool {
	return true
}

func (filterSecretEvents) Create(e event.CreateEvent) bool {
	return false
}

func (filterSecretEvents) Delete(e event.DeleteEvent) bool {
	return true
}

func (filterSecretEvents) Generic(e event.GenericEvent) bool {
	return false
}
