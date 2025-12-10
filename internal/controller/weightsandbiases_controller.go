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

package controller

import (
	"context"
	"time"

	apiv1 "github.com/wandb/operator/api/v1"
	apiv2 "github.com/wandb/operator/api/v2"
	v1 "github.com/wandb/operator/internal/controller/v1"
	v2 "github.com/wandb/operator/internal/controller/v2"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// WeightsAndBiasesReconciler reconciles a WeightsAndBiases object
type WeightsAndBiasesReconciler struct {
	client.Client
	IsAirgapped    bool
	DeployerClient deployer.DeployerInterface
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	DryRun         bool
	Debug          bool
	EnableV2       bool
}

//+kubebuilder:rbac:groups="",resources=configmaps;events;persistentvolumeclaims;secrets;serviceaccounts;services,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups="",resources=endpoints;ingresses;nodes;nodes/spec;nodes/stats;nodes/metrics;nodes/proxy;namespaces;namespaces/status;replicationcontrollers;replicationcontrollers/status;resourcequotas;pods;pods/log;pods/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments/status;daemonsets/status;replicasets/status;statefulsets/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments;controllerrevisions;daemonsets;replicasets;statefulsets,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=batch,resources=cronjobs;jobs,verbs=get;list;watch;create;delete;update;patch
//+kubebuilder:rbac:groups=clickhouse.altinity.com,resources=clickhouseinstallations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=clickhouse.altinity.com,resources=clickhouseinstallations/status,verbs=get
//+kubebuilder:rbac:groups=cloud.google.com,resources=backendconfigs,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=list;watch
//+kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkanodepools,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkanodepools/status,verbs=get
//+kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkas/status,verbs=get
//+kubebuilder:rbac:groups=minio.min.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minio.min.io,resources=tenants/status,verbs=get
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;ingresses/status;networkpolicies,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusterbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusterbackups/status,verbs=get
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusters/status,verbs=get
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis;redissentinels;redisreplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis/status,verbs=get
//+kubebuilder:rbac:urls=/metrics,verbs=get

// Deprecated/Erroneously required RBAC rules
//+kubebuilder:rbac:groups=extensions,resources=daemonsets;deployments;replicasets;ingresses;ingresses/status,verbs=get;list;watch

var defaultRequeueMinutes = 1
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WeightsAndBiasesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info(
		"=== Reconciling Weights & Biases V2 instance...",
		"NamespacedName", req.NamespacedName,
		"Name", req.Name,
		"start", true,
	)

	if r.EnableV2 {
		wandbv1 := &apiv1.WeightsAndBiases{}
		wandbv2 := &apiv2.WeightsAndBiases{}

		if err := r.Get(ctx, req.NamespacedName, wandbv2); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
			}
			return ctrl.Result{}, err
		}

		// TODO: Once proper conversion is done, remove this logic
		if version, ok := wandbv2.Annotations["legacy.operator.wandb.com/version"]; ok && version == "v1" {
			log.Info("Detected legacy Weights & Biases instance, reconciling as V1...")

			err := wandbv1.ConvertFrom(wandbv2)
			if err != nil {
				return ctrl.Result{}, err
			}
			v1Cfg := v1.WandbV1ReconcileConfig{
				DeployerClient: r.DeployerClient,
				Recorder:       r.Recorder,
				DryRun:         r.DryRun,
				Debug:          r.Debug,
				IsAirgapped:    r.IsAirgapped,
			}
			return v1.Reconcile(ctx, wandbv1, r.Client, &v1Cfg)
		} else {
			return v2.Reconcile(ctx, r.Client, wandbv2)
		}
	} else {
		wandb := &apiv1.WeightsAndBiases{}
		if err := r.Get(ctx, req.NamespacedName, wandb); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
			}
			return ctrl.Result{}, err
		}
		v1Cfg := v1.WandbV1ReconcileConfig{
			DeployerClient: r.DeployerClient,
			Recorder:       r.Recorder,
			DryRun:         r.DryRun,
			Debug:          r.Debug,
			IsAirgapped:    r.IsAirgapped,
		}
		return v1.Reconcile(ctx, wandb, r.Client, &v1Cfg)
	}
}

// Delete was taken from original v1 reconciler
func (r *WeightsAndBiasesReconciler) Delete(e event.DeleteEvent) bool {
	return !e.DeleteStateUnknown
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var b *ctrl.Builder
	if r.EnableV2 {
		b = ctrl.NewControllerManagedBy(mgr).
			For(&apiv2.WeightsAndBiases{}).
			Owns(&corev1.Secret{}).
			Owns(&corev1.ConfigMap{})
	} else {
		b = ctrl.NewControllerManagedBy(mgr).
			For(&apiv1.WeightsAndBiases{}, builder.WithPredicates(filterWBEventsForV1{})).
			Owns(&corev1.Secret{}, builder.WithPredicates(filterSecretEventsForV1{})).
			Owns(&corev1.ConfigMap{})
	}
	return b.Complete(r)
}

type filterWBEventsForV1 struct {
	predicate.Funcs
}

func (filterWBEventsForV1) Update(e event.UpdateEvent) bool {
	// Checking whether the Object's Generation has changed. If it has not
	// (indicating a non-spec change), it returns false - thus ignoring the
	// event.
	return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
}

func (filterWBEventsForV1) Create(e event.CreateEvent) bool {
	return true
}

func (filterWBEventsForV1) Delete(e event.DeleteEvent) bool {
	return true
}

func (filterWBEventsForV1) Generic(e event.GenericEvent) bool {
	return false
}

type filterSecretEventsForV1 struct {
	predicate.Funcs
}

func (filterSecretEventsForV1) Update(e event.UpdateEvent) bool {
	return true
}

func (filterSecretEventsForV1) Create(e event.CreateEvent) bool {
	return false
}

func (filterSecretEventsForV1) Delete(e event.DeleteEvent) bool {
	return true
}

func (filterSecretEventsForV1) Generic(e event.GenericEvent) bool {
	return false
}
