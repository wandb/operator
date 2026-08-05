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
	"fmt"

	mocov1beta2 "github.com/cybozu-go/moco/api/v1beta2"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/reconciler"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/spec/channel/deployer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// WeightsAndBiasesReconciler reconciles a WeightsAndBiases object
type WeightsAndBiasesReconciler struct {
	client.Client
	IsAirgapped        bool
	DeployerClient     deployer.DeployerInterface
	Scheme             *runtime.Scheme
	Recorder           record.EventRecorder
	DryRun             bool
	Debug              bool
	EnableV2           bool
	TelemetryConfigRef types.NamespacedName
}

//+kubebuilder:rbac:groups="",resources=configmaps;events;persistentvolumeclaims;secrets;serviceaccounts;services,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups="",resources=endpoints;nodes;nodes/spec;nodes/stats;nodes/metrics;nodes/proxy;namespaces;namespaces/status;replicationcontrollers;replicationcontrollers/status;resourcequotas;pods;pods/log;pods/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments/status;daemonsets/status;replicasets/status;statefulsets/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments;controllerrevisions;daemonsets;replicasets;statefulsets,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=batch,resources=cronjobs;jobs,verbs=get;list;watch;create;delete;update;patch
//+kubebuilder:rbac:groups=clickhouse.altinity.com,resources=clickhouseinstallations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=clickhouse.altinity.com,resources=clickhouseinstallations/status,verbs=get
//+kubebuilder:rbac:groups=clickhouse-keeper.altinity.com,resources=clickhousekeeperinstallations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=clickhouse-keeper.altinity.com,resources=clickhousekeeperinstallations/status,verbs=get
//+kubebuilder:rbac:groups=cloud.google.com,resources=backendconfigs,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=list;watch
//+kubebuilder:rbac:groups=grafana.integreatly.org,resources=grafanas;grafanadashboards;grafanadatasources,verbs=get;list;watch
//+kubebuilder:rbac:groups=grafana.integreatly.org,resources=grafanas/status;grafanadashboards/status;grafanadatasources/status,verbs=get
//+kubebuilder:rbac:groups=seaweed.seaweedfs.com,resources=seaweeds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=seaweed.seaweedfs.com,resources=seaweeds/status,verbs=get
//+kubebuilder:rbac:groups=moco.cybozu.com,resources=mysqlclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=moco.cybozu.com,resources=mysqlclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.victoriametrics.com,resources=vmagents;vmalerts;vmnodescrapes;vmpodscrapes;vmrules;vmservicescrapes;vmsingles;vlsingles;vtsingles,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.victoriametrics.com,resources=vmagents/status;vmalerts/status;vmsingles/status;vlsingles/status;vtsingles/status,verbs=get
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways;httproutes;backendtlspolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status;backendtlspolicies/status,verbs=get
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=gateway.nginx.org,resources=clientsettingspolicies,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=networking.gke.io,resources=healthcheckpolicies,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis;redissentinels;redisreplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis/status,verbs=get
//+kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,resourceNames=nonroot-v2,verbs=use
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
		"=== Reconciling Weights & Biases V2 instance...",
		"NamespacedName", req.NamespacedName,
		"Name", req.Name,
		"start", true,
	)

	wandb := &apiv2.WeightsAndBiases{}

	if err := r.Get(ctx, req.NamespacedName, wandb); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	telemetryConfig, err := r.loadTelemetryConfig(ctx)
	if err != nil {
		log.Error(err, "failed to load telemetry configuration", "configMap", r.TelemetryConfigRef)
		return ctrl.Result{}, err
	}

	return v2.Reconcile(ctx, r.Client, r.Recorder, wandb, telemetryConfig)
}

// Delete was taken from original v1 reconciler
func (r *WeightsAndBiasesReconciler) Delete(e event.DeleteEvent) bool {
	return !e.DeleteStateUnknown
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var b = ctrl.NewControllerManagedBy(mgr).
		For(&apiv2.WeightsAndBiases{}).
		// Applications carry plain (non-controller) owner refs; without
		// MatchEveryOwner this watch never fires and app status changes stop
		// refreshing status.wandb.applications once the estate settles.
		Owns(&apiv2.Application{}, builder.MatchEveryOwner).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&networkingv1.Ingress{})
	if utils.IsRegistered(r.Scheme, &gatewayv1.Gateway{}) {
		b = b.Watches(&gatewayv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayToWandb))
	}
	if utils.IsRegistered(r.Scheme, &mocov1beta2.MySQLCluster{}) {
		b = b.Owns(&mocov1beta2.MySQLCluster{})
	}
	if r.TelemetryConfigRef.Name != "" {
		b = b.Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.mapTelemetryConfigToWandb),
			builder.WithPredicates(predicate.NewPredicateFuncs(r.isTelemetryConfig)),
		)
	}

	return b.Complete(r)
}

func (r *WeightsAndBiasesReconciler) loadTelemetryConfig(ctx context.Context) (v2.TelemetryRuntimeConfig, error) {
	if r.TelemetryConfigRef.Name == "" {
		return v2.DefaultTelemetryRuntimeConfig(), nil
	}

	return v2.LoadTelemetryRuntimeConfigFromConfigMap(
		ctx,
		r.Client,
		r.TelemetryConfigRef,
		v2.DefaultTelemetryRuntimeConfig(),
	)
}

func (r *WeightsAndBiasesReconciler) isTelemetryConfig(obj client.Object) bool {
	return client.ObjectKeyFromObject(obj) == r.TelemetryConfigRef
}

func (r *WeightsAndBiasesReconciler) mapTelemetryConfigToWandb(ctx context.Context, obj client.Object) []ctrl.Request {
	if !r.isTelemetryConfig(obj) {
		return nil
	}

	wandbList := &apiv2.WeightsAndBiasesList{}
	if err := r.List(ctx, wandbList); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0, len(wandbList.Items))
	for i := range wandbList.Items {
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(&wandbList.Items[i]),
		})
	}

	return requests
}

func (r *WeightsAndBiasesReconciler) mapGatewayToWandb(ctx context.Context, obj client.Object) []ctrl.Request {
	gateway, ok := obj.(*gatewayv1.Gateway)
	if !ok {
		return nil
	}

	wandbList := &apiv2.WeightsAndBiasesList{}
	if err := r.List(ctx, wandbList); err != nil {
		return nil
	}

	requests := make([]ctrl.Request, 0)
	for i := range wandbList.Items {
		wandb := &wandbList.Items[i]
		if !gatewayMatchesWandb(gateway, wandb) {
			continue
		}
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(wandb),
		})
	}

	return requests
}

func gatewayMatchesWandb(gateway *gatewayv1.Gateway, wandb *apiv2.WeightsAndBiases) bool {
	if wandb.Spec.Networking.Mode != apiv2.NetworkingModeGatewayAPI || wandb.Spec.Networking.GatewayAPI == nil {
		return false
	}

	if wandb.Spec.Networking.GatewayAPI.Gateway.Managed {
		return gateway.Namespace == wandb.Namespace &&
			gateway.Name == fmt.Sprintf("%s-gateway", wandb.Name)
	}

	ref := wandb.Spec.Networking.GatewayAPI.Gateway.GatewayRef
	if ref == nil {
		return false
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = wandb.Namespace
	}

	return gateway.Namespace == namespace && gateway.Name == ref.Name
}
