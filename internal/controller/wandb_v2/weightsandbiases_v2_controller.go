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

package wandb_v2

import (
	"context"
	"errors"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const dbFinalizer = "db.app.wandb.com"

// WeightsAndBiasesV2Reconciler reconciles a WeightsAndBiases object
type WeightsAndBiasesV2Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	DryRun   bool
	Debug    bool
}

//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.wandb.com,resources=weightsandbiases/finalizers,verbs=update
//+kubebuilder:rbac:groups=mysql.oracle.com,resources=ndbclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mysql.oracle.com,resources=ndbclusters/status,verbs=get
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusters/status,verbs=get
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusterbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pxc.percona.com,resources=perconaxtradbclusterbackups/status,verbs=get
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=redis.redis.opstreelabs.in,resources=redis/status,verbs=get
//+kubebuilder:rbac:groups="",resources=configmaps;events;persistentvolumeclaims;secrets;serviceaccounts;services,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups="",resources=endpoints;ingresses;nodes;nodes/spec;nodes/stats;nodes/metrics;nodes/proxy;namespaces;namespaces/status;replicationcontrollers;replicationcontrollers/status;resourcequotas;pods;pods/log;pods/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments;controllerrevisions;daemonsets;replicasets;statefulsets,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=apps,resources=deployments/status;daemonsets/status;replicasets/status;statefulsets/status,verbs=get
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=batch,resources=cronjobs;jobs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=list;watch
//+kubebuilder:rbac:groups=cloud.google.com,resources=backendconfigs,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;ingresses/status;networkpolicies,verbs=update;delete;get;list;create;patch;watch
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings;clusterroles;clusterrolebindings,verbs=update;delete;get;list;patch;create;watch
//+kubebuilder:rbac:urls=/metrics,verbs=get

// Deprecated/Erroneously required RBAC rules
//+kubebuilder:rbac:groups=extensions,resources=daemonsets;deployments;replicasets;ingresses;ingresses/status,verbs=get;list;watch

var defaultRequeueMinutes = 2
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WeightsAndBiasesV2Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info(
		"=== Reconciling Weights & Biases V2 instance...",
		"NamespacedName", req.NamespacedName,
		"Name", req.Name,
		"start", true,
	)

	var ctrlState CtrlState

	wandb := &apiv2.WeightsAndBiases{}
	if err := r.Client.Get(ctx, req.NamespacedName, wandb); err != nil {
		if machErrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info(
		"Found Weights & Biases V2 instance, processing the spec...",
		"Spec", wandb.Spec, "Name", wandb.Name, "UID", wandb.UID, "Generation", wandb.Generation,
	)

	ctrlState = r.handleDatabase(ctx, wandb, req)
	if ctrlState.isDone() {
		return ctrlState.reconcileResult()
	}

	ctrlState = r.handleRedis(ctx, wandb, req)
	if ctrlState.isDone() {
		return ctrlState.reconcileResult()
	}

	ctrlState = r.inferState(ctx, wandb)
	if ctrlState.isDone() {
		return ctrlState.reconcileResult()
	}

	return ctrl.Result{}, nil
}

func (r *WeightsAndBiasesV2Reconciler) handleDatabase(
	ctx context.Context, wandb *apiv2.WeightsAndBiases, req ctrl.Request,
) CtrlState {
	log := ctrllog.FromContext(ctx)

	if !wandb.Spec.Database.Enabled {
		log.Info("Database not enabled, skipping")
		return CtrlContinue()
	}

	dbType := wandb.Spec.Database.Type
	if dbType == "" {
		dbType = apiv2.WBDatabaseTypeNDB
		log.Info("Database type not specified, defaulting to NDB")
	}

	switch dbType {
	case apiv2.WBDatabaseTypeNDB:
		log.Info("Handling NDB MySQL database")
		return r.handleNdkMysql(ctx, wandb, req)

	case apiv2.WBDatabaseTypePercona:
		log.Info("Handling Percona XtraDB Cluster database")
		return r.handlePerconaMysql(ctx, wandb, req)

	default:
		log.Error(nil, "Unknown database type", "type", dbType)
		return CtrlError(errors.New("unknown database type: " + string(dbType)))
	}
}

func (r *WeightsAndBiasesV2Reconciler) inferState(
	ctx context.Context, wandb *apiv2.WeightsAndBiases,
) CtrlState {
	newState := wandb.Status.State
	curState := wandb.Status.State
	log := ctrl.LoggerFrom(ctx)
	databaseStatus := wandb.Status.DatabaseStatus
	redisStatus := wandb.Status.RedisStatus

	databaseReady := !wandb.Spec.Database.Enabled || databaseStatus.State == "ready"
	redisReady := !wandb.Spec.Redis.Enabled || redisStatus.State == "ready"

	if databaseReady && redisReady {
		newState = apiv2.WBStateReady
	}

	if curState != newState {
		wandb.Status.State = newState
		if err := r.Status().Update(ctx, wandb); err != nil {
			log.Error(err, "Failed to update Weights & Biases state", "from", curState, "to", newState)
			return CtrlError(err)
		}
		return CtrlDone(ctrl.Result{RequeueAfter: defaultRequeueDuration})
	}
	return CtrlContinue()
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeightsAndBiasesV2Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&apiv2.WeightsAndBiases{} /*, builder.WithPredicates(filterWBEvents{})*/).
		Owns(&corev1.Secret{}, builder.WithPredicates(filterSecretEvents{})).
		Owns(&corev1.ConfigMap{})
	return builder.Complete(r)
}

func (r *WeightsAndBiasesV2Reconciler) updateDbBackupStatus(ctx context.Context, wandb *apiv2.WeightsAndBiases, state, message string) {
	log := ctrl.LoggerFrom(ctx)
	now := metav1.Now()

	wandb.Status.DatabaseStatus.BackupStatus = apiv2.WBBackupStatus{
		LastBackupTime: &now,
		State:          state,
		Message:        message,
	}

	if err := r.Client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update backup status")
	}
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
