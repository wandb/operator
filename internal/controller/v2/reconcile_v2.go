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

package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	"github.com/wandb/operator/internal/controller/infra/managed/mysql/mysql"
	"github.com/wandb/operator/internal/logx"
	oputils "github.com/wandb/operator/pkg/utils"
	strimziv1 "github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const CleanupFinalizer = "wandb.apps.wandb.com/cleanup"

var defaultRequeueMinutes = 1
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

var managedWorkloadTelemetryApplications = map[string]struct{}{
	"api":                               {},
	"executor":                          {},
	"filemeta":                          {},
	"filestream":                        {},
	"flat-run-fields-updater":           {},
	"glue":                              {},
	"metric-observer":                   {},
	"nginx-proxy":                       {},
	"parquet":                           {},
	"weave":                             {},
	"weave-trace":                       {},
	"weave-trace-worker":                {},
	"weave-trace-evaluate-model-worker": {},
}

var managedWorkloadTelemetryEnvVars = []serverManifest.EnvVar{
	{
		Name: "OTEL_EXPORTER_OTLP_PROTOCOL",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "protocol"},
		},
	},
	{
		Name: "OTEL_TRACES_EXPORTER",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "tracesExporter"},
		},
	},
	{
		Name: "OTEL_METRICS_EXPORTER",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "metricsExporter"},
		},
	},
	{
		Name: "OTEL_LOGS_EXPORTER",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "logsExporter"},
		},
	},
	{
		Name: "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "metricsEndpoint"},
		},
	},
	{
		Name: "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "logsEndpoint"},
		},
	},
	{
		Name: "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "tracesEndpoint"},
		},
	},
	{
		Name: "OTEL_SERVICE_NAME",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "serviceName"},
		},
	},
	{
		Name: "OTEL_RESOURCE_ATTRIBUTES",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "resourceAttributes"},
		},
	},
	{
		Name: "GORILLA_TRACER",
		Sources: []serverManifest.EnvSource{
			{Type: "telemetry", Field: "gorillaTracer"},
		},
	},
}

type finalizerFunc func(context.Context, ctrlClient.Client, *apiv2.WeightsAndBiases) error

func runRetentionFinalizer(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	infraSpec apiv2.ManagedInfraSpec,
	purgeFn finalizerFunc,
	detachFn finalizerFunc,
) error {
	switch wandb.GetRetentionPolicy(infraSpec).OnDelete {
	case apiv2.PurgeOnDelete:
		return purgeFn(ctx, client, wandb)
	case apiv2.DetachOnDelete:
		return detachFn(ctx, client, wandb)
	}
	return nil
}

// Reconcile for V2 of WandB as the assumed object
func Reconcile(
	ctx context.Context,
	client ctrlClient.Client,
	recorder record.EventRecorder,
	wandb *apiv2.WeightsAndBiases,
	telemetryConfig TelemetryRuntimeConfig,
) (ctrl.Result, error) {
	ctx, log := logx.WithSlog(ctx, logx.ReconcileInfraV2)

	var err error

	var errorCount int

	/////////////////////////
	// Retention Finalizer

	isFlaggedForDeletion := !wandb.GetDeletionTimestamp().IsZero()

	// ensure finalizer if not present
	if !isFlaggedForDeletion && !ctrlqueue.ContainsString(wandb.GetFinalizers(), CleanupFinalizer) {
		wandb.SetFinalizers(append(wandb.GetFinalizers(), CleanupFinalizer))
		if err := client.Update(ctx, wandb); err != nil {
			log.Error(fmt.Sprintf("Failed to add finalizer '%s'", CleanupFinalizer), logx.ErrAttr(err))
			return ctrl.Result{}, err
		}
	}

	// if deleting and handle cleanup or preservation of config and data
	if isFlaggedForDeletion && !wandb.GetDeletionTimestamp().IsZero() {
		if ctrlqueue.ContainsString(wandb.GetFinalizers(), CleanupFinalizer) {

			if wandb.Spec.ObjectStore.ManagedObjectStore != nil {
				if err = runRetentionFinalizer(ctx, client, wandb, wandb.Spec.ObjectStore.ManagedObjectStore.ManagedInfraSpec, objectStorePurgeFinalizer, objectStoreDetachFinalizer); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.MySQL.ManagedMysql != nil {
				if err = runRetentionFinalizer(ctx, client, wandb, wandb.Spec.MySQL.ManagedMysql.ManagedInfraSpec, mysqlPurgeFinalizer, mysqlDetachFinalizer); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.Redis.ManagedRedis != nil {
				if err = runRetentionFinalizer(ctx, client, wandb, wandb.Spec.Redis.ManagedRedis.ManagedInfraSpec, redisPurgeFinalizer, redisDetachFinalizer); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.Kafka.ManagedKafka != nil {
				if err = runRetentionFinalizer(ctx, client, wandb, wandb.Spec.Kafka.ManagedKafka.ManagedInfraSpec, kafkaPurgeFinalizer, kafkaDetachFinalizer); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.ClickHouse.ManagedClickHouse != nil {
				if err = runRetentionFinalizer(ctx, client, wandb, wandb.Spec.ClickHouse.ManagedClickHouse.ManagedInfraSpec, clickHousePurgeFinalizer, clickHouseDetachFinalizer); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.ObjectStore.ExternalObjectStore != nil && wandb.Spec.RetentionPolicy.OnDelete == apiv2.PurgeOnDelete {
				if err = objectStorePurgeFinalizer(ctx, client, wandb); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.MySQL.ExternalMysql != nil && wandb.Spec.RetentionPolicy.OnDelete == apiv2.PurgeOnDelete {
				if err = mysqlPurgeFinalizer(ctx, client, wandb); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.Redis.ExternalRedis != nil && wandb.Spec.RetentionPolicy.OnDelete == apiv2.PurgeOnDelete {
				if err = redisPurgeFinalizer(ctx, client, wandb); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.Kafka.ExternalKafka != nil && wandb.Spec.RetentionPolicy.OnDelete == apiv2.PurgeOnDelete {
				if err = kafkaPurgeFinalizer(ctx, client, wandb); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.ClickHouse.ExternalClickHouse != nil && wandb.Spec.RetentionPolicy.OnDelete == apiv2.PurgeOnDelete {
				if err = clickHousePurgeFinalizer(ctx, client, wandb); err != nil {
					return ctrl.Result{}, err
				}
			}
			if err = deleteInfraHTTPRoutes(ctx, client, wandb); err != nil {
				return ctrl.Result{}, err
			}
			if wandb.Spec.Networking.Mode == apiv2.NetworkingModeIngress {
				if err = deleteConsolidatedIngress(ctx, client, wandb); err != nil {
					return ctrl.Result{}, err
				}
			}
			if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI &&
				wandb.Spec.Networking.GatewayAPI != nil &&
				wandb.Spec.Networking.GatewayAPI.Gateway.Managed {
				if err = deleteGateway(ctx, client, wandb); err != nil {
					return ctrl.Result{}, err
				}
			}
			if spec := wandb.Spec.Kafka.ManagedKafka; spec != nil {
				if wandb.GetRetentionPolicy(spec.ManagedInfraSpec).OnDelete == apiv2.DetachOnDelete {
					if err = kafkaDetachFinalizer(ctx, client, wandb); err != nil {
						return ctrl.Result{}, err
					}
				}
			}
			controllerutil.RemoveFinalizer(wandb, CleanupFinalizer)
			if err := client.Update(ctx, wandb); err != nil {
				log.Error("Failed to remove finalizer '%s'", logx.ErrAttr(err))
				return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
			}
		}
		// continue post-finalizer logic in a future pass of the reconciliation loop
		return ctrl.Result{}, nil
	}

	/////////////////////////
	// Fetch manifest early so infra sizing can be applied before provisioning
	manifest, err := serverManifest.GetServerManifest(ctx, wandb.Spec.Wandb.ManifestRepository, wandb.Spec.Wandb.Version)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Override features from CR spec if present
	for key, enabled := range wandb.Spec.Wandb.Features {
		manifest.Features[key] = enabled
	}

	// Apply manifest-derived infra sizing before provisioning
	ApplyInfraSizing(wandb, manifest)

	/////////////////////////
	// Write Infra State
	redisConditions := redisWriteState(ctx, client, wandb)
	mysqlConditions := mysqlWriteState(ctx, client, wandb)
	kafkaConditions := kafkaWriteState(ctx, client, wandb)
	objectStoreConditions, objectStoreConnection := objectStoreWriteState(ctx, client, wandb)
	clickHouseConditions := clickHouseWriteState(ctx, client, wandb)

	/////////////////////////
	// Read Infra State
	redisConditions, redisInfraConn := redisReadState(ctx, client, wandb, redisConditions)
	mysqlConditions, mysqlInfraConn := mysqlReadState(ctx, client, wandb, mysqlConditions)
	kafkaConditions, kafkaInfraConn := kafkaReadState(ctx, client, wandb, kafkaConditions)
	objectStoreConditions = objectStoreReadState(ctx, client, wandb, objectStoreConditions)
	clickHouseConditions, clickHouseInfraConn := clickHouseReadState(ctx, client, wandb, clickHouseConditions)

	/////////////////////////
	// WandB Status Inference
	var res ctrl.Result
	var ctrlResults []ctrl.Result

	if res, err = redisInferStatus(ctx, client, recorder, wandb, redisConditions, redisInfraConn); err != nil {
		errorCount++
	}
	ctrlResults = append(ctrlResults, res)

	if res, err = mysqlInferStatus(ctx, client, recorder, wandb, mysqlConditions, mysqlInfraConn); err != nil {
		errorCount++
	}
	ctrlResults = append(ctrlResults, res)

	if res, err = kafkaInferStatus(ctx, client, recorder, wandb, kafkaConditions, kafkaInfraConn); err != nil {
		errorCount++
	}
	ctrlResults = append(ctrlResults, res)

	if res, err = objectStoreInferStatus(ctx, client, recorder, wandb, objectStoreConditions, objectStoreConnection); err != nil {
		errorCount++
	}
	ctrlResults = append(ctrlResults, res)

	if res, err = clickHouseInferStatus(ctx, client, recorder, wandb, clickHouseConditions, clickHouseInfraConn); err != nil {
		errorCount++
	}
	ctrlResults = append(ctrlResults, res)

	if err = inferState(ctx, client, wandb); err != nil {
		errorCount++
	}

	if errorCount > 0 {
		return ctrl.Result{}, errors.New("infra state update errors")
	}

	if err := reconcileTelemetryConnectionSecret(ctx, client, wandb, telemetryConfig); err != nil {
		log.Error("failed to reconcile telemetry connection secret", logx.ErrAttr(err))
		return ctrl.Result{}, err
	}

	redisReady := wandb.Status.RedisStatus.Ready
	mysqlReady := wandb.Status.MySQLStatus.Ready
	kafkaReady := wandb.Status.KafkaStatus.Ready
	objectStoreReady := wandb.Status.ObjectStoreStatus.Ready
	clickHouseReady := wandb.Status.ClickHouseStatus.Ready

	if !redisReady || !mysqlReady || !kafkaReady || !objectStoreReady || !clickHouseReady {
		log := ctrl.LoggerFrom(ctx)
		log.Info("Infra not ready in V2.Reconcile",
			"redis", redisReady, "mysql", mysqlReady, "kafka", kafkaReady, "objectStore", objectStoreReady, "clickhouse", clickHouseReady)
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	res, err = ReconcileWandbManifest(ctx, client, wandb, manifest, telemetryConfig)
	// send up the manifest error for now
	if err != nil {
		return res, err
	}
	ctrlResults = append(ctrlResults, res)

	return consolidateResults(ctrlResults), nil
}

func consolidateResults(results []ctrl.Result) ctrl.Result {
	durations := lo.Filter(
		lo.Map(results, func(r ctrl.Result, _ int) time.Duration { return r.RequeueAfter }),
		func(d time.Duration, _ int) bool { return d > 0 },
	)
	// if there are no non-zero durations
	if len(durations) == 0 {
		return ctrl.Result{}
	}

	return ctrl.Result{
		RequeueAfter: lo.Min(durations),
	}
}

func ReconcileWandbManifest(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	manifest serverManifest.Manifest,
	telemetryConfig TelemetryRuntimeConfig,
) (ctrl.Result, error) {
	// Reconcile Wandb Manifest
	logger := ctrl.LoggerFrom(ctx).WithName("reconcileWandbManifest")
	logger.Info("Reconciling Wandb Manifest", "name", wandb.Name)
	var result ctrl.Result
	var err error

	redisReady := wandb.Status.RedisStatus.Ready
	mysqlReady := wandb.Status.MySQLStatus.Ready
	kafkaReady := wandb.Status.KafkaStatus.Ready
	objectStoreReady := wandb.Status.ObjectStoreStatus.Ready
	clickHouseReady := wandb.Status.ClickHouseStatus.Ready

	if !redisReady || !mysqlReady || !kafkaReady || !objectStoreReady || !clickHouseReady {
		logger.Info("Infra components not ready yet, requeuing for reconciliation",
			"redis", redisReady, "mysql", mysqlReady, "kafka", kafkaReady, "objectStore", objectStoreReady, "clickhouse", clickHouseReady)
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	logger.Info("Manifest Features", "features", manifest.Features)

	result, err = generateSecrets(ctx, client, wandb, manifest)
	if err != nil {
		return result, err
	}

	result, err = createKafkaTopics(ctx, client, wandb, manifest)
	if err != nil {
		return result, err
	}

	result, err = runMysqlInitJob(ctx, client, wandb, manifest)
	if err != nil {
		return result, err
	}

	if wandb.Spec.MySQL.ManagedMysql != nil && !wandb.Status.Wandb.MySQLInit.Succeeded {
		logger.Info("Mysql init not yet successful", "Message", wandb.Status.Wandb.MySQLInit.Message)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	serviceAccountName := wandb.Spec.Wandb.ServiceAccount.ServiceAccountName

	if *wandb.Spec.Wandb.ServiceAccount.Create {
		if err := createOrUpdateServiceAccount(ctx, client, wandb, serviceAccountName); err != nil {
			logger.Error(err, "Failed to create/update ServiceAccount")
			return ctrl.Result{}, err
		}
	}

	if err := createOrUpdateRole(ctx, client, wandb, serviceAccountName); err != nil {
		logger.Error(err, "Failed to create/update Role for service account")
		return ctrl.Result{}, err
	}

	if err := createOrUpdateRoleBinding(ctx, client, wandb, serviceAccountName); err != nil {
		logger.Error(err, "Failed to create/update RoleBinding for service account")
		return ctrl.Result{}, err
	}

	if err := cleanupNetworkingModeResources(ctx, client, wandb); err != nil {
		logger.Error(err, "Failed to clean up stale networking resources")
		return ctrl.Result{}, err
	}
	resetInactiveNetworkingStatus(wandb)

	if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI {
		wandb.Status.GatewayStatus = nil
		if err := reconcileGateway(ctx, client, wandb); err != nil {
			logger.Error(err, "Failed to reconcile Gateway")
			return ctrl.Result{}, err
		}
	}

	result, err = runMigrations(ctx, client, wandb, manifest)
	if err != nil {
		return result, err
	}

	if !wandb.Status.Wandb.Migration.Ready {
		logger.Info("Migration not yet successful for version", "version", wandb.Spec.Wandb.Version, "reason", wandb.Status.Wandb.Migration.Reason)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	result, err = reconcileApplications(ctx, client, wandb, manifest, telemetryConfig)
	if err != nil {
		return result, err
	}

	if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI {
		if err := reconcileInfraHTTPRoutes(ctx, client, wandb, manifest); err != nil {
			logger.Error(err, "Failed to reconcile infra HTTPRoutes")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func reconcileApplications(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	manifest serverManifest.Manifest,
	telemetryConfig TelemetryRuntimeConfig,
) (ctrl.Result, error) {
	logger := logx.GetSlog(ctx)
	logger.Info("Reconciling applications")
	serviceAccountName := wandb.Spec.Wandb.ServiceAccount.ServiceAccountName

	if wandb.Spec.Wandb.InternalServiceAuth.Enabled != nil && *wandb.Spec.Wandb.InternalServiceAuth.Enabled {
		if err := createOrUpdateOIDCDiscoveryClusterRoleBinding(ctx, client, wandb); err != nil {
			logger.Error("Failed to create ClusterRoleBinding for OIDC discovery. "+
				"This is required for JWT token validation between W&B services. "+
				"Either grant the operator ClusterRoleBinding permissions, or manually create the ClusterRoleBinding. "+
				"W&B will continue starting, but JWT authentication will fail until this is resolved.", "err", err)
			// Non-fatal: continue reconciliation even if ClusterRoleBinding creation fails
		}
	}

	desiredAppNames := make(map[string]bool)
	for _, app := range sortedManifestApplications(manifest) {
		if len(app.Features) > 0 && !manifestFeaturesEnabled(app.Features, manifest.Features) {
			continue
		}
		desiredAppNames[app.Name] = true
	}

	for _, app := range sortedManifestApplications(manifest) {
		// If the application is gated behind features, only install it when
		// at least one of those features is enabled in the manifest.
		if len(app.Features) > 0 && !manifestFeaturesEnabled(app.Features, manifest.Features) {
			continue
		}

		applicationName := fmt.Sprintf("%s-%s", wandb.Name, app.Name)
		envVars, err := resolveEnvvars(ctx, client, wandb, manifest, app.CommonEnvs, app.Env)
		if err != nil {
			return ctrl.Result{}, err
		}
		envVars, err = injectManagedWorkloadTelemetryEnvvars(ctx, client, wandb, manifest, app, envVars, telemetryConfig.Enabled)
		if err != nil {
			return ctrl.Result{}, err
		}
		envVars = applyWorkloadTelemetryDefaults(envVars, applicationName)

		volumes, volumeMounts, err := resolveVolumeMounts(ctx, manifest, app.CommonVolumeMounts, app.VolumeMounts)
		if err != nil {
			return ctrl.Result{}, err
		}

		// First, resolve any inline files and JWT token volumes at the Application level
		// so that volumeMounts/volumes are ready before constructing containers.
		if len(app.Files) > 0 {
			volumes, volumeMounts, err = resolveInlineFiles(ctx, client, wandb, app, volumes, volumeMounts)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		if len(app.JWTTokens) > 0 {
			// resolveJWTTokens appends mounts to the given container, but also returns volumes.
			// We only use the returned volumes here, consistent with previous behavior.
			volumes, volumeMounts = resolveJWTTokens(app, volumes, volumeMounts)
		}

		containers := resolveContainers(app, wandb, envVars, volumeMounts)

		initContainers := resolveInitContainers(app, envVars, volumeMounts)

		application := &apiv2.Application{}
		err = client.Get(ctx, types.NamespacedName{Name: applicationName, Namespace: wandb.Namespace}, application)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				application.SetName(applicationName)
				application.SetNamespace(wandb.Namespace)
			} else {
				return ctrl.Result{}, err
			}
		}

		application.Spec.Kind = "Deployment"
		application.Spec.PodTemplate.Spec.Containers = containers
		// Replace volumes entirely on each reconcile to avoid accumulating duplicates
		// across updates (e.g., duplicate "files-inline" volume names).
		application.Spec.PodTemplate.Spec.Volumes = volumes
		application.Spec.PodTemplate.Spec.InitContainers = initContainers
		application.Spec.PodTemplate.Spec.Affinity = wandb.Spec.Affinity
		application.Spec.PodTemplate.Spec.Tolerations = *wandb.Spec.Tolerations

		application.Spec.HpaTemplate = ResolveAutoscaling(app, wandb)

		// Set shared service account for all W&B applications
		application.Spec.PodTemplate.Spec.ServiceAccountName = serviceAccountName

		// Reconcile Service ports: fully replace the ServiceTemplate ports with
		// the ports declared in the manifest for this app. This ensures that any
		// change to port numbers, names, or protocols is propagated on each
		// reconcile. If no service ports are declared, clear the ServiceTemplate.
		if app.Service != nil && len(app.Service.Ports) > 0 {
			application.Spec.ServiceTemplate = &corev1.ServiceSpec{
				Type: app.Service.Type,
			}
			application.Spec.ServiceTemplate.Ports = app.Service.Ports
		} else {
			// No service declared in manifest; ensure we clear any previous template
			application.Spec.ServiceTemplate = nil
		}

		if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI && app.Ingress != nil &&
			wandb.Status.GatewayStatus != nil && wandb.Status.GatewayStatus.GatewayRef != nil {
			application.Spec.HTTPRouteTemplate = buildHTTPRouteTemplate(wandb, app)
		} else {
			application.Spec.HTTPRouteTemplate = nil
		}

		err = controllerutil.SetOwnerReference(wandb, application, client.Scheme())
		if err != nil {
			return ctrl.Result{}, err
		}

		if application.CreationTimestamp.IsZero() {
			if err = client.Create(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err = client.Update(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		}

		wandb.Status.Wandb.Applications[app.Name] = application.Status
	}

	existingApps := &apiv2.ApplicationList{}
	if err := client.List(ctx, existingApps, ctrlClient.InNamespace(wandb.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list existing applications: %w", err)
	}

	for _, app := range existingApps.Items {
		if !isOwnedBy(&app, wandb) {
			continue
		}

		appNamePrefix := fmt.Sprintf("%s-", wandb.Name)
		if !strings.HasPrefix(app.Name, appNamePrefix) {
			continue
		}
		appName := strings.TrimPrefix(app.Name, appNamePrefix)

		if !desiredAppNames[appName] {
			logger.Info("Deleting application no longer in manifest or disabled by feature", "application", app.Name)
			if err := client.Delete(ctx, &app); err != nil && !apiErrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("failed to delete application %s: %w", app.Name, err)
			}
			delete(wandb.Status.Wandb.Applications, appName)
		}
	}

	if wandb.Spec.Networking.Mode == apiv2.NetworkingModeIngress {
		wandb.Status.IngressStatus = nil
		if err := reconcileConsolidatedIngress(ctx, client, wandb, manifest); err != nil {
			logger.Error("Failed to reconcile consolidated Ingress", "err", err)
			return ctrl.Result{}, err
		}
	}

	hostname, err := url.Parse(wandb.Spec.Wandb.Hostname)
	if err != nil {
		logger.Error("Failed to parse provided hostname", "hostname", wandb.Spec.Wandb.Hostname, "err", err)
	} else {
		if wandb.Spec.Networking.Mode == apiv2.NetworkingModeNone {
			// Only override with NodePort if user didn't specify a port in the hostname
			if manifestFeaturesEnabled([]string{"proxy"}, manifest.Features) && hostname.Port() == "" {
				proxyService := &corev1.Service{}
				proxyServiceName := fmt.Sprintf("%s-%s", wandb.Name, "nginx-proxy")
				err := client.Get(ctx, types.NamespacedName{Name: proxyServiceName, Namespace: wandb.Namespace}, proxyService)
				if err != nil {
					logger.Error("Failed to get proxy service", "service", proxyServiceName, "err", err)
				} else {
					nodePort := proxyService.Spec.Ports[0].NodePort
					hostname.Host = fmt.Sprintf("%s:%d", hostname.Hostname(), nodePort)
				}
			}
		}

		if wandb.Status.Wandb.Hostname != hostname.String() {
			wandb.Status.Wandb.Hostname = hostname.String()
		}
	}

	if err := client.Status().Update(ctx, wandb); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func buildHTTPRouteTemplate(wandb *apiv2.WeightsAndBiases, app serverManifest.Application) *apiv2.HTTPRouteTemplateSpec {
	gwConfig := wandb.Spec.Networking.GatewayAPI

	ref := wandb.Status.GatewayStatus.GatewayRef
	parentRef := gatewayv1.ParentReference{
		Name: gatewayv1.ObjectName(ref.Name),
	}
	if ref.Namespace != "" && ref.Namespace != wandb.Namespace {
		ns := gatewayv1.Namespace(ref.Namespace)
		parentRef.Namespace = &ns
	}
	if gwConfig.ListenerName != nil {
		sectionName := gatewayv1.SectionName(*gwConfig.ListenerName)
		parentRef.SectionName = &sectionName
	}

	hostname := parseHostname(wandb.Spec.Wandb.Hostname)
	hostnames := []gatewayv1.Hostname{gatewayv1.Hostname(hostname)}
	for _, h := range wandb.Spec.Wandb.AdditionalHostnames {
		hostnames = append(hostnames, gatewayv1.Hostname(h))
	}

	var paths []string
	var pathType string
	if app.Ingress != nil {
		paths = app.Ingress.Paths
		pathType = app.Ingress.PathType
	}

	return &apiv2.HTTPRouteTemplateSpec{
		ParentRefs:  []gatewayv1.ParentReference{parentRef},
		Hostnames:   hostnames,
		Paths:       paths,
		PathType:    pathType,
		ServicePort: resolveHTTPRouteServicePort(app),
	}
}

func resolveHTTPRouteServicePort(app serverManifest.Application) *gatewayv1.PortNumber {
	if app.Ingress != nil && app.Ingress.ServicePort != "" {
		port := intstr.Parse(app.Ingress.ServicePort)
		if port.Type == intstr.Int {
			p := gatewayv1.PortNumber(port.IntVal)
			return &p
		}
		if port.Type == intstr.String && app.Service != nil {
			for _, servicePort := range app.Service.Ports {
				if servicePort.Name == port.StrVal {
					p := gatewayv1.PortNumber(servicePort.Port)
					return &p
				}
			}
		}
	}
	return nil
}

func resolveInitContainers(app serverManifest.Application, envVars []corev1.EnvVar, volumeMounts []corev1.VolumeMount) []corev1.Container {
	initContainers := []corev1.Container{}

	if app.InitContainers != nil {
		for _, initContainerSpec := range app.InitContainers {
			if initContainerSpec.Name == "migrate" {
				continue
			}
			initContainer := corev1.Container{
				Name:         initContainerSpec.Name,
				Image:        initContainerSpec.Image.GetImage(),
				Env:          envVars,
				Args:         initContainerSpec.Args,
				Command:      initContainerSpec.Command,
				VolumeMounts: volumeMounts,
			}
			initContainers = append(initContainers, initContainer)
		}
	}
	return initContainers
}

func ResolveResources(app serverManifest.Application, wandb *apiv2.WeightsAndBiases, containerResources *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	var resources *corev1.ResourceRequirements

	// check if there is "default" in the sizing map and apply those values
	if defaultConfig, ok := app.Sizing["default"]; ok && defaultConfig.Resources != nil {
		resources = mergeResources(resources, defaultConfig.Resources, wandb.Spec.RequireLimits)
	}

	// check if there is a sizing config in the map that corresponds to the size in the wandb spec and apply that
	if sizeConfig, ok := app.Sizing[wandb.Spec.Size]; ok && sizeConfig.Resources != nil {
		resources = mergeResources(resources, sizeConfig.Resources, wandb.Spec.RequireLimits)
	}

	// check if the container has a resource and if so apply those settings
	resources = mergeResources(resources, containerResources, wandb.Spec.RequireLimits)

	if resources == nil {
		return nil
	}

	if len(resources.Limits) == 0 && len(resources.Requests) == 0 {
		return nil
	}

	return resources
}

func ResolveAutoscaling(app serverManifest.Application, wandb *apiv2.WeightsAndBiases) *autoscalingv2.HorizontalPodAutoscalerSpec {
	hpa := &autoscalingv2.HorizontalPodAutoscalerSpec{}

	// check if there is "default" in the sizing map and apply those values
	if defaultConfig, ok := app.Sizing["default"]; ok && defaultConfig.Autoscaling != nil {
		hpa.MinReplicas = defaultConfig.Autoscaling.Horizontal.MinReplicas
		hpa.MaxReplicas = defaultConfig.Autoscaling.Horizontal.MaxReplicas
		hpa.Metrics = defaultConfig.Autoscaling.Horizontal.Metrics
		hpa.Behavior = defaultConfig.Autoscaling.Horizontal.Behavior
		hpa.ScaleTargetRef = defaultConfig.Autoscaling.Horizontal.ScaleTargetRef
	}

	// check if there is a sizing config in the map that corresponds to the size in the wandb spec and apply that
	if sizeConfig, ok := app.Sizing[wandb.Spec.Size]; ok && sizeConfig.Autoscaling != nil {
		if sizeConfig.Autoscaling.Horizontal.MinReplicas != nil {
			hpa.MinReplicas = sizeConfig.Autoscaling.Horizontal.MinReplicas
		}
		if sizeConfig.Autoscaling.Horizontal.MaxReplicas != 0 {
			hpa.MaxReplicas = sizeConfig.Autoscaling.Horizontal.MaxReplicas
		}
		if len(sizeConfig.Autoscaling.Horizontal.Metrics) > 0 {
			hpa.Metrics = sizeConfig.Autoscaling.Horizontal.Metrics
		}
		if sizeConfig.Autoscaling.Horizontal.Behavior != nil {
			hpa.Behavior = sizeConfig.Autoscaling.Horizontal.Behavior
		}
		if sizeConfig.Autoscaling.Horizontal.ScaleTargetRef.Name != "" {
			hpa.ScaleTargetRef = sizeConfig.Autoscaling.Horizontal.ScaleTargetRef
		}
	}

	if hpa.MaxReplicas == 0 {
		return nil
	}

	return hpa
}

// mergeResources merges an overlay ResourceRequirements into a base, with
// overlay values taking precedence on a per-resource-name basis.
func mergeResources(base, overlay *corev1.ResourceRequirements, requireLimits bool) *corev1.ResourceRequirements {
	if base == nil && overlay == nil {
		return nil
	}
	result := &corev1.ResourceRequirements{}
	if base != nil {
		if base.Limits != nil {
			result.Limits = make(corev1.ResourceList)
			for k, v := range base.Limits {
				result.Limits[k] = v
			}
		}
		if base.Requests != nil {
			result.Requests = make(corev1.ResourceList)
			for k, v := range base.Requests {
				result.Requests[k] = v
			}
		}
	}
	if overlay != nil {
		if overlay.Limits != nil {
			if result.Limits == nil {
				result.Limits = make(corev1.ResourceList)
			}
			for k, v := range overlay.Limits {
				result.Limits[k] = v
			}
		}
		if overlay.Requests != nil {
			if result.Requests == nil {
				result.Requests = make(corev1.ResourceList)
			}
			for k, v := range overlay.Requests {
				result.Requests[k] = v
			}
		}
	}

	if !requireLimits {
		result.Limits = nil
	}
	return result
}

// ResolveInfraSizing resolves a SizingConfig from an InfraConfig map for the
// given Size. It merges the "default" sizing with the size-specific sizing,
// where size-specific values override defaults.
func ResolveInfraSizing(sizing map[apiv2.Size]serverManifest.SizingConfig, size apiv2.Size, requireLimits bool) *serverManifest.SizingConfig {
	result := &serverManifest.SizingConfig{}

	// Apply "default" sizing baseline
	if defaultSizing, ok := sizing["default"]; ok {
		result.Replicas = defaultSizing.Replicas
		result.Shards = defaultSizing.Shards
		result.VolumeSize = defaultSizing.VolumeSize
		if defaultSizing.Resources != nil {
			result.Resources = defaultSizing.Resources.DeepCopy()
		}
	}

	// Override with size-specific sizing, merging resources
	if sizeSizing, ok := sizing[size]; ok {
		if sizeSizing.Replicas != 0 {
			result.Replicas = sizeSizing.Replicas
		}
		if sizeSizing.Shards != 0 {
			result.Shards = sizeSizing.Shards
		}
		if sizeSizing.VolumeSize != "" {
			result.VolumeSize = sizeSizing.VolumeSize
		}
		result.Resources = mergeResources(result.Resources, sizeSizing.Resources, requireLimits)
	}

	return result
}

// ResolveKafkaSizing resolves a SizingConfig from the KafkaConfig for the given Size.
func ResolveKafkaSizing(sizing map[apiv2.Size]serverManifest.KafkaSizingConfig, size apiv2.Size, requireLimits bool) *serverManifest.KafkaSizingConfig {
	result := &serverManifest.KafkaSizingConfig{}

	if defaultSizing, ok := sizing["default"]; ok {
		result.Replicas = defaultSizing.Replicas
		result.VolumeSize = defaultSizing.VolumeSize
		result.ReplicationFactor = defaultSizing.ReplicationFactor
		result.MinInSyncReplicas = defaultSizing.MinInSyncReplicas
		result.OffsetsTopicRF = defaultSizing.OffsetsTopicRF
		result.TransactionStateRF = defaultSizing.TransactionStateRF
		result.TransactionStateISR = defaultSizing.TransactionStateISR
		if defaultSizing.Resources != nil {
			result.Resources = defaultSizing.Resources.DeepCopy()
		}
	}

	if sizeSizing, ok := sizing[size]; ok {
		if sizeSizing.Replicas != 0 {
			result.Replicas = sizeSizing.Replicas
		}
		if sizeSizing.VolumeSize != "" {
			result.VolumeSize = sizeSizing.VolumeSize
		}
		if sizeSizing.ReplicationFactor != 0 {
			result.ReplicationFactor = sizeSizing.ReplicationFactor
		}
		if sizeSizing.MinInSyncReplicas != 0 {
			result.MinInSyncReplicas = sizeSizing.MinInSyncReplicas
		}
		if sizeSizing.OffsetsTopicRF != 0 {
			result.OffsetsTopicRF = sizeSizing.OffsetsTopicRF
		}
		if sizeSizing.TransactionStateRF != 0 {
			result.TransactionStateRF = sizeSizing.TransactionStateRF
		}
		if sizeSizing.TransactionStateISR != 0 {
			result.TransactionStateISR = sizeSizing.TransactionStateISR
		}
		result.Resources = mergeResources(result.Resources, sizeSizing.Resources, requireLimits)
	}

	return result
}

// ApplyInfraSizing applies manifest-derived sizing to the wandb spec's infra
// components. Values from the manifest are only applied when the corresponding
// spec field has not been explicitly set by the user (i.e., is zero-valued).
func ApplyInfraSizing(wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) {
	size := wandb.Spec.Size

	// Default MySQL
	if wandb.Spec.MySQL.ManagedMysql != nil {
		if mysqlConfig, ok := manifest.Mysql["default"]; ok {
			sizing := ResolveInfraSizing(mysqlConfig.Sizing, size, wandb.Spec.RequireLimits)
			spec := wandb.Spec.MySQL.ManagedMysql
			if spec.Replicas == 0 && sizing.Replicas != 0 {
				spec.Replicas = sizing.Replicas
			}
			if spec.StorageSize == "" && sizing.VolumeSize != "" {
				spec.StorageSize = sizing.VolumeSize
			}
			if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
				spec.Config.Resources = *sizing.Resources
			}
		}
	}

	// Default Redis
	if wandb.Spec.Redis.ManagedRedis != nil {
		if redisConfig, ok := manifest.Redis["default"]; ok {
			sizing := ResolveInfraSizing(redisConfig.Sizing, size, wandb.Spec.RequireLimits)
			spec := wandb.Spec.Redis.ManagedRedis
			if spec.StorageSize == "" && sizing.VolumeSize != "" {
				spec.StorageSize = sizing.VolumeSize
			}
			if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
				spec.Config.Resources = *sizing.Resources
			}
		}
	}

	// Default ClickHouse
	if wandb.Spec.ClickHouse.ManagedClickHouse != nil {
		if clickhouseConfig, ok := manifest.Redis["default"]; ok {
			sizing := ResolveInfraSizing(clickhouseConfig.Sizing, size, wandb.Spec.RequireLimits)
			spec := wandb.Spec.ClickHouse.ManagedClickHouse
			if spec.Replicas == 0 && sizing.Replicas != 0 {
				spec.Replicas = sizing.Replicas
			}
			if spec.StorageSize == "" && sizing.VolumeSize != "" {
				spec.StorageSize = sizing.VolumeSize
			}
			if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
				spec.Config.Resources = *sizing.Resources
			}
		}
	}

	// Default ObjectStore (bucket)
	if wandb.Spec.ObjectStore.ManagedObjectStore != nil {
		if objectStoreConfig, ok := manifest.Redis["default"]; ok {
			sizing := ResolveInfraSizing(objectStoreConfig.Sizing, size, wandb.Spec.RequireLimits)
			spec := wandb.Spec.ObjectStore.ManagedObjectStore
			if spec.Replicas == 0 && sizing.Replicas != 0 {
				spec.Replicas = sizing.Replicas
			}
			if spec.StorageSize == "" && sizing.VolumeSize != "" {
				spec.StorageSize = sizing.VolumeSize
			}
			if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
				spec.Config.Resources = *sizing.Resources
			}
		}
	}

	// Kafka
	if wandb.Spec.Kafka.ManagedKafka != nil {
		if sizing := ResolveKafkaSizing(manifest.Kafka.Sizing, size, wandb.Spec.RequireLimits); sizing != nil {
			spec := wandb.Spec.Kafka.ManagedKafka
			if spec.Replicas == 0 && sizing.Replicas != 0 {
				spec.Replicas = sizing.Replicas
			}
			if spec.StorageSize == "" && sizing.VolumeSize != "" {
				spec.StorageSize = sizing.VolumeSize
			}
			if sizing.Resources != nil && len(spec.Config.Resources.Requests) == 0 && len(spec.Config.Resources.Limits) == 0 {
				spec.Config.Resources = *sizing.Resources
			}
			if spec.Config.ReplicationConfig.DefaultReplicationFactor == 0 && sizing.ReplicationFactor != 0 {
				spec.Config.ReplicationConfig.DefaultReplicationFactor = sizing.ReplicationFactor
			}
			if spec.Config.ReplicationConfig.MinInSyncReplicas == 0 && sizing.MinInSyncReplicas != 0 {
				spec.Config.ReplicationConfig.MinInSyncReplicas = sizing.MinInSyncReplicas
			}
			if spec.Config.ReplicationConfig.OffsetsTopicRF == 0 && sizing.OffsetsTopicRF != 0 {
				spec.Config.ReplicationConfig.OffsetsTopicRF = sizing.OffsetsTopicRF
			}
			if spec.Config.ReplicationConfig.TransactionStateRF == 0 && sizing.TransactionStateRF != 0 {
				spec.Config.ReplicationConfig.TransactionStateRF = sizing.TransactionStateRF
			}
			if spec.Config.ReplicationConfig.TransactionStateISR == 0 && sizing.TransactionStateISR != 0 {
				spec.Config.ReplicationConfig.TransactionStateISR = sizing.TransactionStateISR
			}
		}
	}
}

func resolveContainers(app serverManifest.Application, wandb *apiv2.WeightsAndBiases, envVars []corev1.EnvVar, volumeMounts []corev1.VolumeMount) []corev1.Container {
	// Build containers: support multi-container apps via app.Containers; fall back to a single
	// default container when no explicit containers are provided.
	containers := []corev1.Container{}
	if len(app.Containers) > 0 {
		for _, container := range app.Containers {
			// Convert ports
			var containerPorts []corev1.ContainerPort
			for _, p := range container.Ports {
				containerPorts = append(containerPorts, corev1.ContainerPort{
					Name:          p.Name,
					ContainerPort: p.ContainerPort,
					Protocol:      p.Protocol,
				})
			}

			// Choose image/args/command with sensible fallbacks to app-level values
			img := app.Image.GetImage()
			if container.Image.Repository != "" {
				img = container.Image.GetImage()
			}
			args := app.Args
			if len(container.Args) > 0 {
				args = container.Args
			}
			cmd := app.Command
			if len(container.Command) > 0 {
				cmd = container.Command
			}

			c := corev1.Container{
				Name:         container.Name,
				Image:        img,
				Env:          envVars,
				Args:         args,
				Command:      cmd,
				Ports:        containerPorts,
				VolumeMounts: volumeMounts,
			}

			if resources := ResolveResources(app, wandb, container.Resources); resources != nil {
				c.Resources = *resources
			}

			// Default HTTPGet probe ports to the first declared port name when missing
			if container.StartupProbe != nil && container.StartupProbe.HTTPGet != nil {
				if container.StartupProbe.HTTPGet.Port.StrVal == "" && container.StartupProbe.HTTPGet.Port.IntVal == 0 {
					if len(container.Ports) > 0 && container.StartupProbe.HTTPGet.Path != "" {
						container.StartupProbe.HTTPGet = &corev1.HTTPGetAction{Path: container.StartupProbe.HTTPGet.Path, Port: intstr.FromString(container.Ports[0].Name)}
					}
				}
				c.StartupProbe = container.StartupProbe
			}
			if container.LivenessProbe != nil && container.LivenessProbe.HTTPGet != nil {
				if container.LivenessProbe.HTTPGet.Port.StrVal == "" && container.LivenessProbe.HTTPGet.Port.IntVal == 0 {
					if len(container.Ports) > 0 && container.LivenessProbe.HTTPGet.Path != "" {
						container.LivenessProbe.HTTPGet = &corev1.HTTPGetAction{Path: container.LivenessProbe.HTTPGet.Path, Port: intstr.FromString(container.Ports[0].Name)}
					}
				}
				c.LivenessProbe = container.LivenessProbe
			}
			if container.ReadinessProbe != nil && container.ReadinessProbe.HTTPGet != nil {
				if container.ReadinessProbe.HTTPGet.Port.StrVal == "" && container.ReadinessProbe.HTTPGet.Port.IntVal == 0 {
					if len(container.Ports) > 0 && container.ReadinessProbe.HTTPGet.Path != "" {
						container.ReadinessProbe.HTTPGet = &corev1.HTTPGetAction{Path: container.ReadinessProbe.HTTPGet.Path, Port: intstr.FromString(container.Ports[0].Name)}
					}
				}
				c.ReadinessProbe = container.ReadinessProbe
			}

			containers = append(containers, c)
		}
	} else {
		// Backward-compatible single-container behavior
		c := corev1.Container{
			Name:         app.Name,
			Image:        app.Image.GetImage(),
			Env:          envVars,
			Args:         app.Args,
			Command:      app.Command,
			VolumeMounts: volumeMounts,
		}

		if resources := ResolveResources(app, wandb, nil); resources != nil {
			c.Resources = *resources
		}
		containers = append(containers, c)
	}
	return containers
}

func applyWorkloadTelemetryDefaults(envVars []corev1.EnvVar, applicationName string) []corev1.EnvVar {
	if applicationName == "" || !hasWorkloadTelemetryConfig(envVars) {
		return envVars
	}

	serviceNameIndex := -1
	for i, envVar := range envVars {
		if envVar.Name != "OTEL_SERVICE_NAME" {
			continue
		}
		serviceNameIndex = i
		if envVar.Value != "" {
			return envVars
		}
		break
	}

	serviceNameEnv := corev1.EnvVar{
		Name:  "OTEL_SERVICE_NAME",
		Value: applicationName,
	}

	if serviceNameIndex == -1 {
		return append(envVars, serviceNameEnv)
	}

	envVars[serviceNameIndex] = serviceNameEnv
	return envVars
}

func injectManagedWorkloadTelemetryEnvvars(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	manifest serverManifest.Manifest,
	app serverManifest.Application,
	envVars []corev1.EnvVar,
	telemetryEnabled bool,
) ([]corev1.EnvVar, error) {
	if !telemetryEnabled || !shouldInjectManagedWorkloadTelemetry(app.Name) {
		return envVars, nil
	}

	telemetryEnvVars, err := resolveEnvvars(ctx, client, wandb, manifest, nil, managedWorkloadTelemetryEnvVars)
	if err != nil {
		return nil, err
	}

	return appendMissingEnvVars(envVars, telemetryEnvVars), nil
}

func shouldInjectManagedWorkloadTelemetry(appName string) bool {
	_, ok := managedWorkloadTelemetryApplications[appName]
	return ok
}

func appendMissingEnvVars(existing []corev1.EnvVar, additions []corev1.EnvVar) []corev1.EnvVar {
	seen := make(map[string]struct{}, len(existing))
	for _, envVar := range existing {
		seen[envVar.Name] = struct{}{}
	}

	for _, envVar := range additions {
		if _, ok := seen[envVar.Name]; ok {
			continue
		}
		existing = append(existing, envVar)
		seen[envVar.Name] = struct{}{}
	}

	return existing
}

func hasWorkloadTelemetryConfig(envVars []corev1.EnvVar) bool {
	for _, envVar := range envVars {
		switch envVar.Name {
		case "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
			"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
			"OTEL_METRICS_EXPORTER",
			"OTEL_LOGS_EXPORTER",
			"OTEL_TRACES_EXPORTER",
			"OTEL_SERVICE_NAME",
			"GORILLA_TRACER":
			return true
		}
	}
	return false
}

func resolveJWTTokens(app serverManifest.Application, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount) {
	for _, jwtToken := range app.JWTTokens {
		var volume corev1.Volume
		volumeName := fmt.Sprintf("%s-%s", app.Name, jwtToken.Name)

		// Create volume based on source type
		switch {
		case jwtToken.Source.KubernetesServiceAccount != nil:
			// Projected volume with service account token
			expirationSeconds := jwtToken.Source.KubernetesServiceAccount.ExpirationSeconds
			if expirationSeconds == 0 {
				expirationSeconds = 3607 // Default 1 hour + 7 seconds
			}
			volume = corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
									Audience:          jwtToken.Source.KubernetesServiceAccount.Audience,
									Path:              "token",
									ExpirationSeconds: &expirationSeconds,
								},
							},
						},
					},
				},
			}

		case jwtToken.Source.SecretRef != nil:
			// Secret volume
			key := jwtToken.Source.SecretRef.Key
			if key == "" {
				key = "token" // Default key name
			}
			volume = corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: jwtToken.Source.SecretRef.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  key,
								Path: "token",
							},
						},
					},
				},
			}

		case jwtToken.Source.CSIProvider != nil:
			// CSI volume
			volume = corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					CSI: &corev1.CSIVolumeSource{
						Driver:           jwtToken.Source.CSIProvider.Driver,
						VolumeAttributes: jwtToken.Source.CSIProvider.Parameters,
						ReadOnly:         func() *bool { b := true; return &b }(),
					},
				},
			}

		default:
			// No valid source specified, skip this token
			continue
		}

		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: jwtToken.MountPath,
			ReadOnly:  true,
		})
	}
	return volumes, volumeMounts
}

func resolveInlineFiles(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases, app serverManifest.Application, volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount, error) {
	// Collect inline files into a single operator-managed ConfigMap
	inlineData := map[string]string{}
	inlineCMName := fmt.Sprintf("%s-%s-files", wandb.Name, app.Name)
	// Track external ConfigMap refs and create one Volume per unique ref
	cmRefVolumeNames := map[string]string{}

	for _, f := range app.Files {
		key := f.Name
		fileName := f.FileName
		if fileName == "" {
			fileName = key
		}

		var volName string
		if f.Inline != "" {
			// Accumulate into inline CM data
			inlineData[key] = f.Inline
			volName = "files-inline"
		} else if f.ConfigMapRef != "" {
			// external ConfigMap reference
			if existing, ok := cmRefVolumeNames[f.ConfigMapRef]; ok {
				volName = existing
			} else {
				volName = fmt.Sprintf("cm-%s", f.ConfigMapRef)
				cmRefVolumeNames[f.ConfigMapRef] = volName
			}
		} else {
			// neither inline nor ref provided; skip
			continue
		}

		// Mount each file as a single file using subPath into the specified directory
		mountPath := f.MountPath
		if mountPath == "" {
			// require a mountPath; skip if not provided
			continue
		}
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: fmt.Sprintf("%s/%s", mountPath, fileName),
			SubPath:   key,
			ReadOnly:  true,
		})
	}

	// Create/update inline ConfigMap if we have any inline data
	if len(inlineData) > 0 {
		cm := &corev1.ConfigMap{}
		cm.Namespace = wandb.Namespace
		cm.Name = inlineCMName
		// Try to get existing
		err := client.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, cm)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				cm.Data = inlineData
				if err := controllerutil.SetOwnerReference(wandb, cm, client.Scheme()); err != nil {
					return volumes, volumeMounts, err
				}
				if err := client.Create(ctx, cm); err != nil {
					return volumes, volumeMounts, err
				}
			} else {
				return volumes, volumeMounts, err
			}
		} else {
			// Update data if changed
			if cm.Data == nil {
				cm.Data = map[string]string{}
			}
			cm.Data = inlineData
			if err := client.Update(ctx, cm); err != nil {
				return volumes, volumeMounts, err
			}
		}

		// Add a volume for the inline CM
		volumes = append(volumes, corev1.Volume{
			Name: "files-inline",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: inlineCMName}},
			},
		})
	}

	// Add volumes for each external ConfigMap ref
	for ref, volName := range cmRefVolumeNames {
		volumes = append(volumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: ref}},
			},
		})
	}

	return volumes, volumeMounts, nil
}

func resolveEnvvars(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest, commonEnvs []string, envs []serverManifest.EnvVar) ([]corev1.EnvVar, error) {
	logger := logx.GetSlog(ctx)
	var combinedEnvs []serverManifest.EnvVar
	for _, commonVars := range commonEnvs {
		if envvars, ok := manifest.CommonEnvvars[commonVars]; ok {
			combinedEnvs = append(combinedEnvs, envvars...)
		}
	}

	combinedEnvs = append(combinedEnvs, envs...)

	var envVars []corev1.EnvVar
	for _, env := range combinedEnvs {
		// If a literal value is provided, it's a simple case.
		if env.Value != "" {
			envVars = append(envVars, corev1.EnvVar{Name: env.Name, Value: env.Value})
			continue
		}
		if env.ValueFrom != nil {
			envVars = append(envVars, corev1.EnvVar{Name: env.Name, ValueFrom: env.ValueFrom})
		}

		// Multi-source composition: build a comma-separated value from all resolvable sources.
		// Secret-backed sources are exposed via intermediate env vars and referenced with $(VAR) expansion.
		// If there is exactly one secret-backed source and no literals, keep direct SecretKeyRef for back-compat.

		// Temporary slices to build the final env value and intermediates
		var components []string
		var intermediateVars []corev1.EnvVar

		// Helper to add a secret-backed component via an intermediate env var
		addSecretComponent := func(selector corev1.SecretKeySelector, idx int) {
			// Deterministic name based on target env and source index
			ivName := fmt.Sprintf("%s_%d", env.Name, idx)
			// K8s env var names must be alphanumeric + _ and not start with a number
			// The env.Name in manifest follows standard patterns; idx ensures uniqueness.
			intermediateVars = append(intermediateVars, corev1.EnvVar{
				Name: ivName,
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: selector.LocalObjectReference,
					Key:                  selector.Key,
					Optional:             selector.Optional,
				}},
			})
			components = append(components, fmt.Sprintf("$(%s)", ivName))
		}

		// Track if we only have a single secret-backed component
		singleSecretSelector := corev1.SecretKeySelector{}
		secretOnlyCount := 0

		for idx, src := range env.Sources {
			switch src.Type {
			case "generatedSecret":
				if sel, ok := wandb.Status.GeneratedSecrets[src.Name]; ok {
					singleSecretSelector = sel
					secretOnlyCount++
					addSecretComponent(sel, idx)
				}
			case "mysql":
				// mysql connection URL as a secret ref
				selector := wandb.Status.MySQLStatus.Connection.URL
				// Record for potential direct assignment case
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "redis":
				selector := wandb.Status.RedisStatus.Connection.URL
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "bucket":
				selector := corev1.SecretKeySelector{
					LocalObjectReference: wandb.Status.ObjectStoreStatus.Connection.URL.LocalObjectReference,
				}
				switch src.Field {
				case "host":
					selector.Key = "Host"
				case "port":
					selector.Key = "Port"
				case "region":
					selector.Key = "Region"
				default:
					selector.Key = "url"
				}
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "clickhouse":
				// clickhouse fields are provided as separate keys in the same secret
				selector := corev1.SecretKeySelector{
					LocalObjectReference: wandb.Status.ClickHouseStatus.Connection.URL.LocalObjectReference,
				}
				switch src.Field {
				case "host":
					selector.Key = "Host"
				case "port":
					selector.Key = "Port"
				case "user":
					selector.Key = "User"
				case "password":
					selector.Key = "Password"
				case "database":
					selector.Key = "Database"
				default:
					// Unrecognized field; skip
					continue
				}
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "kafka":
				// kafka can be referenced as a full URL (no field) or by specific fields (host/port)
				if src.Field == "" {
					selector := wandb.Status.KafkaStatus.Connection.URL
					singleSecretSelector = selector
					secretOnlyCount++
					addSecretComponent(selector, idx)
					break
				}
				selector := corev1.SecretKeySelector{
					LocalObjectReference: wandb.Status.KafkaStatus.Connection.URL.LocalObjectReference,
				}
				switch src.Field {
				case "host":
					selector.Key = "Host"
				case "port":
					selector.Key = "Port"
				default:
					selector.Key = "url"
				}
				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "telemetry":
				secretName := src.Name
				if secretName == "" {
					secretName = "wandb-otel-connection"
				}

				selector := corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
				}
				switch src.Field {
				case "", "metrics", "metricsEndpoint":
					selector.Key = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
				case "logs", "logsEndpoint":
					selector.Key = "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"
				case "traces", "tracesEndpoint":
					selector.Key = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
				case "metricsExporter":
					selector.Key = "OTEL_METRICS_EXPORTER"
				case "logsExporter":
					selector.Key = "OTEL_LOGS_EXPORTER"
				case "tracesExporter":
					selector.Key = "OTEL_TRACES_EXPORTER"
				case "protocol":
					selector.Key = "OTEL_EXPORTER_OTLP_PROTOCOL"
				case "serviceName":
					selector.Key = "OTEL_SERVICE_NAME"
				case "resourceAttributes":
					selector.Key = "OTEL_RESOURCE_ATTRIBUTES"
				case "gorillaTracer", "tracer":
					selector.Key = "GORILLA_TRACER"
				default:
					if strings.HasPrefix(src.Field, "OTEL_") {
						selector.Key = src.Field
					} else {
						continue
					}
				}

				singleSecretSelector = selector
				secretOnlyCount++
				addSecretComponent(selector, idx)
			case "service":
				// Prefer deterministic manifest-derived service resolution to avoid startup races
				// where the Service object has not been created yet.
				if resolved, ok := resolveServiceURLFromManifest(wandb.Name, manifest, src); ok {
					components = append(components, resolved)
					continue
				}

				// Fallback: resolve from live Service object (back-compat).
				serviceList := &corev1.ServiceList{}
				targetApplicationName := fmt.Sprintf("%s-%s", wandb.Name, src.Name)
				err := client.List(
					ctx,
					serviceList,
					ctrlClient.InNamespace(wandb.Namespace),
					ctrlClient.MatchingLabels{"app.kubernetes.io/name": targetApplicationName},
				)
				if err != nil {
					return nil, err
				}
				if len(serviceList.Items) == 0 || len(serviceList.Items[0].Spec.Ports) == 0 {
					// Can't resolve; skip this component
					continue
				}
				proto := ""
				if src.Proto != "" {
					proto = fmt.Sprintf("%s://", src.Proto)
				}
				// Choose a port: prefer named match if provided; else pick the first port
				selectedPort := serviceList.Items[0].Spec.Ports[0].Port
				if src.Port != "" {
					for _, servicePort := range serviceList.Items[0].Spec.Ports {
						if servicePort.Name == src.Port {
							selectedPort = servicePort.Port
							break
						}
					}
				}
				components = append(components, fmt.Sprintf("%s%s:%d%s", proto, serviceList.Items[0].Name, selectedPort, src.Path))
			case "jwt-issuer-map":
				if *wandb.Spec.Wandb.InternalServiceAuth.Enabled {
					// TODO Get real OIDC Issuer
					issuer := "https://kubernetes.default.svc.cluster.local"
					if wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer != "" {
						issuer = wandb.Spec.Wandb.InternalServiceAuth.OIDCIssuer
					}
					components = append(
						components,
						fmt.Sprintf(
							"{\"system:serviceaccount:%s:%s\": \"%s\" }",
							wandb.Namespace,
							wandb.Spec.Wandb.ServiceAccount.ServiceAccountName,
							issuer,
						),
					)
				}
			case "custom-resource":
				// Read a value from the current WandB custom resource via dotted field path
				if src.Field == "" {
					// No field specified; nothing to resolve
					continue
				}
				if val, ok := resolveCRFieldString(wandb, src.Field); ok {
					// Treat as a literal component (not secret-backed)
					logger.Debug("field found in CR", "cr", wandb.Name, "field", src.Field, "value", val)
					components = append(components, val)
				} else {
					logger.Debug("field not found in CR", "cr", wandb.Name, "field", src.Field)
				}
			default:
				// Unknown source type; skip
				continue
			}
		}

		// If we built no components, skip emitting this env var
		if len(components) == 0 {
			if env.DefaultValue != "" {
				envVars = append(envVars, corev1.EnvVar{Name: env.Name, Value: env.DefaultValue})
			}
			continue
		}

		// Optimization/back-compat: if there's exactly one component and it is secret-backed, emit ValueFrom directly
		if len(components) == 1 && secretOnlyCount == 1 && components[0] != "" && intermediateVars != nil {
			// Emit the single env var directly from the secret without intermediate
			envVars = append(envVars, corev1.EnvVar{
				Name:      env.Name,
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &singleSecretSelector},
			})
			continue
		}

		// Otherwise, add all intermediate vars first to ensure $(VAR) expansion works
		envVars = append(envVars, intermediateVars...)
		// Then add the final composed env var
		envVars = append(envVars, corev1.EnvVar{
			Name:  env.Name,
			Value: strings.Join(components, ","),
		})
	}
	return envVars, nil
}

func resolveVolumeMounts(ctx context.Context, manifest serverManifest.Manifest, commonvms []string, vms []serverManifest.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount, error) {
	log := logx.GetSlog(ctx)

	var combinedVolumeMounts []serverManifest.VolumeMount
	for _, commonVolumeMounts := range commonvms {
		if volumeMounts, ok := manifest.CommonVolumeMounts[commonVolumeMounts]; ok {
			combinedVolumeMounts = append(combinedVolumeMounts, volumeMounts...)
		}
	}

	combinedVolumeMounts = append(combinedVolumeMounts, vms...)

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	for _, manifestVM := range combinedVolumeMounts {
		volume := corev1.Volume{
			Name: manifestVM.Name,
		}
		switch manifestVM.Source.Type {
		case "secret":
			volume.Secret = &corev1.SecretVolumeSource{
				SecretName: manifestVM.Source.Name,
			}
		case "configMap":
			volume.ConfigMap = &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: manifestVM.Source.Name,
				},
			}
		case "emptyDir":
			volume.EmptyDir = &corev1.EmptyDirVolumeSource{}
		default:
			log.Error("unsupported volume source type", "type", manifestVM.Source.Type)
			return nil, nil, fmt.Errorf("unsupported volume source type: %s", manifestVM.Source.Type)
		}
		volumeMount := corev1.VolumeMount{
			MountPath: manifestVM.MountPath,
			Name:      manifestVM.Name,
		}
		volumes = append(volumes, volume)
		volumeMounts = append(volumeMounts, volumeMount)
	}
	return volumes, volumeMounts, nil
}

func runMysqlInitJob(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (ctrl.Result, error) {
	if wandb.Spec.MySQL.ManagedMysql == nil {
		return ctrl.Result{}, nil
	}

	if wandb.Status.Wandb.MySQLInit.Succeeded {
		return ctrl.Result{}, nil
	}

	logger := ctrl.LoggerFrom(ctx).WithName("mysqlInit")

	jobName := fmt.Sprintf("%s-mysql-init", wandb.Name)
	logger.Info("Checking for MySQL init job", "job", jobName)
	job := &batchv1.Job{}
	err := client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: wandb.Namespace}, job)

	if err != nil && !apiErrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apiErrors.IsNotFound(err) {
		logger.Info("Creating MySQL init job")

		specNamespacedName := managedMysqlSpecNamespacedName(wandb.Spec.MySQL.ManagedMysql)
		nsnBuilder := mysql.CreateNsNameBuilder(types.NamespacedName{
			Name:      specNamespacedName.Name,
			Namespace: specNamespacedName.Namespace,
		})
		secretName := fmt.Sprintf("%s-db-password", specNamespacedName.Name)

		mysqlCmd := "mysql -h $MYSQL_HOST -u root -p\"${MYSQL_ROOT_PASSWORD}\" -e " +
			"\"CREATE DATABASE IF NOT EXISTS wandb_local; " +
			"CREATE USER IF NOT EXISTS 'wandb_local'@'%%' IDENTIFIED BY '${MYSQL_PASSWORD}'; " +
			"GRANT ALL PRIVILEGES ON wandb_local.* TO 'wandb_local'@'%%'; FLUSH PRIVILEGES;\""

		// For InnoDBCluster, the service host is {name}.{namespace}.svc.cluster.local
		mysqlHost := fmt.Sprintf("%s.%s.svc.cluster.local", nsnBuilder.ClusterName(), specNamespacedName.Namespace)

		job = &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: wandb.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "wandb-operator",
					"app.kubernetes.io/instance":   wandb.Name,
					"app.kubernetes.io/component":  "mysql-init",
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyOnFailure,
						Containers: []corev1.Container{
							{
								Name:    "mysql-init",
								Image:   "mysql:8.0", // Use a standard mysql image
								Command: []string{"/bin/sh", "-c", mysqlCmd},
								Env: []corev1.EnvVar{
									{
										Name:  "MYSQL_HOST",
										Value: mysqlHost,
									},
									{
										Name: "MYSQL_ROOT_PASSWORD",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
												Key:                  "rootPassword",
											},
										},
									},
									{
										Name: "MYSQL_PASSWORD",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
												Key:                  "password",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		if err := controllerutil.SetOwnerReference(wandb, job, client.Scheme()); err != nil {
			return ctrl.Result{}, err
		}

		if err := client.Create(ctx, job); err != nil {
			return ctrl.Result{}, err
		}

		wandb.Status.Wandb.MySQLInit.Name = jobName
		wandb.Status.Wandb.MySQLInit.Succeeded = false
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if job.Status.Succeeded > 0 {
		logger.Info("MySQL init job succeeded")
		wandb.Status.Wandb.MySQLInit.Succeeded = true
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if job.Status.Failed > 0 {
		logger.Info("MySQL init job failed")
		wandb.Status.Wandb.MySQLInit.Failed = true
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
		// We might want to return an error or just requeue
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	logger.Info("MySQL init job still running")
	return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
}

func runMigrations(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (ctrl.Result, error) {
	version := wandb.Spec.Wandb.Version

	if wandb.Status.Wandb.Migration.Ready && wandb.Status.Wandb.Migration.Version == version {
		return ctrl.Result{}, nil
	}

	if wandb.Status.Wandb.Migration.Version != version {
		wandb.Status.Wandb.Migration.Version = version
		wandb.Status.Wandb.Migration.Ready = false
		wandb.Status.Wandb.Migration.Reason = "Running"
		wandb.Status.Wandb.Migration.Jobs = make(map[string]apiv2.MigrationJobStatus)
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
	}

	if len(manifest.Migrations) == 0 {
		wandb.Status.Wandb.Migration.Ready = true
		wandb.Status.Wandb.Migration.Reason = "Complete"
		wandb.Status.Wandb.Migration.LastSuccessVersion = version
		if err := client.Status().Update(ctx, wandb); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if wandb.Status.Wandb.Migration.Jobs == nil {
		wandb.Status.Wandb.Migration.Jobs = make(map[string]apiv2.MigrationJobStatus)
	}

	allSucceeded := true
	anyFailed := false
	anyRunning := false

	for name, migrationTask := range manifest.Migrations {
		jobName := fmt.Sprintf("%s-%s", wandb.Name, name)
		job := &batchv1.Job{}
		err := client.Get(ctx, types.NamespacedName{Name: jobName, Namespace: wandb.Namespace}, job)

		jobStatus := apiv2.MigrationJobStatus{
			Name: jobName,
		}

		if err != nil && !apiErrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		if apiErrors.IsNotFound(err) {

			envVars, err := resolveEnvvars(ctx, client, wandb, manifest, migrationTask.CommonEnvs, migrationTask.Env)
			if err != nil {
				return ctrl.Result{}, err
			}

			volumes, volumeMounts, err := resolveVolumeMounts(ctx, manifest, migrationTask.CommonVolumeMounts, migrationTask.VolumeMounts)
			if err != nil {
				return ctrl.Result{}, err
			}

			job = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: wandb.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "wandb-operator",
						"app.kubernetes.io/instance":   wandb.Name,
						"app.kubernetes.io/component":  "migration",
					},
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:         "migrate",
									Image:        migrationTask.Image.GetImage(),
									Args:         migrationTask.Args,
									Command:      migrationTask.Command,
									Env:          envVars,
									VolumeMounts: volumeMounts,
								},
							},
							Volumes:            volumes,
							ServiceAccountName: wandb.Spec.Wandb.ServiceAccount.ServiceAccountName,
						},
					},
				},
			}

			if err := controllerutil.SetOwnerReference(wandb, job, client.Scheme()); err != nil {
				return ctrl.Result{}, err
			}

			if err := client.Create(ctx, job); err != nil {
				return ctrl.Result{}, err
			}

			jobStatus.Succeeded = false
			wandb.Status.Wandb.Migration.Jobs[name] = jobStatus
			wandb.Status.Wandb.Migration.Reason = "Running"
			wandb.Status.Wandb.Migration.Ready = false
			if err := client.Status().Update(ctx, wandb); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}

		if job.Status.Succeeded > 0 {
			jobStatus.Succeeded = true
		} else {
			allSucceeded = false
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					jobStatus.Failed = true
					jobStatus.Message = cond.Message
					anyFailed = true
					break
				}
			}
			if !jobStatus.Failed {
				anyRunning = true
			}
		}

		wandb.Status.Wandb.Migration.Jobs[name] = jobStatus
	}

	if anyFailed {
		wandb.Status.Wandb.Migration.Reason = "Failed"
		wandb.Status.Wandb.Migration.Ready = false
	} else if anyRunning || !allSucceeded {
		wandb.Status.Wandb.Migration.Reason = "Running"
		wandb.Status.Wandb.Migration.Ready = false
	} else if allSucceeded {
		wandb.Status.Wandb.Migration.Reason = "Complete"
		wandb.Status.Wandb.Migration.Ready = true
		if wandb.Status.Wandb.Migration.LastSuccessVersion != version {
			wandb.Status.Wandb.Migration.LastSuccessVersion = version
		}
	} else {
		wandb.Status.Wandb.Migration.Reason = "Unknown"
		wandb.Status.Wandb.Migration.Ready = false
	}

	if err := client.Status().Update(ctx, wandb); err != nil {
		return ctrl.Result{}, err
	}

	if anyFailed {
		return ctrl.Result{}, fmt.Errorf("one or more migration jobs failed")
	}

	if allSucceeded {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func createKafkaTopics(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (ctrl.Result, error) {
	// Create Strimzi KafkaTopic resources for enabled topics
	if wandb.Spec.Kafka.ManagedKafka != nil {
		kafkaSpec := wandb.Spec.Kafka.ManagedKafka
		for _, topic := range manifest.Kafka.Topics {
			if len(topic.Features) > 0 && !manifestFeaturesEnabled(topic.Features, manifest.Features) {
				continue
			}

			kafkaNS := kafkaSpec.Namespace
			if kafkaNS == "" {
				kafkaNS = wandb.Namespace
			}
			clusterName := kafkaSpec.Name
			if clusterName == "" {
				clusterName = wandb.Name
			}

			// Use the logical topic name as the resource name
			resName := topic.Name
			if resName == "" {
				// If not set, fallback to topic entry name
				resName = topic.Topic
			}
			if resName == "" {
				// Nothing actionable without a name
				continue
			}
			labels := map[string]string{
				"strimzi.io/cluster":           clusterName,
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/part-of":    "wandb",
				"app.kubernetes.io/instance":   wandb.Name,
			}

			// Prepare spec
			partitions := int32(1)
			if topic.PartitionCount > 0 {
				partitions = int32(topic.PartitionCount)
			}
			replicas := int32(1)
			if kafkaSpec.Config.ReplicationConfig.DefaultReplicationFactor > 0 {
				replicas = int32(kafkaSpec.Config.ReplicationConfig.DefaultReplicationFactor)
			}

			// Build typed KafkaTopic object
			kt := &strimziv1.KafkaTopic{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resName,
					Namespace: kafkaNS,
					Labels:    labels,
				},
				Spec: strimziv1.KafkaTopicSpec{
					TopicName:  topic.Topic,
					Partitions: partitions,
					Replicas:   replicas,
				},
			}

			// Create or Update
			existing := &strimziv1.KafkaTopic{}
			getErr := client.Get(ctx, types.NamespacedName{Name: kt.Name, Namespace: kafkaNS}, existing)
			if getErr != nil {
				if apiErrors.IsNotFound(getErr) {
					// Set ownerRef only if same namespace
					if kafkaNS == wandb.Namespace {
						_ = controllerutil.SetOwnerReference(wandb, kt, client.Scheme())
					}
					if err := client.Create(ctx, kt); err != nil {
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, getErr
				}
			} else {
				// Update managed spec fields and preserve other fields
				existing.Spec.TopicName = topic.Topic
				existing.Spec.Partitions = partitions
				existing.Spec.Replicas = replicas
				// Preserve/ensure labels
				exLabels := existing.GetLabels()
				if exLabels == nil {
					exLabels = map[string]string{}
				}
				for k, v := range labels {
					exLabels[k] = v
				}
				existing.SetLabels(exLabels)
				if err := client.Update(ctx, existing); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func generateSecrets(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (ctrl.Result, error) {
	// Ensure any manifest-declared generated secrets exist and capture their selectors in status
	if wandb.Status.GeneratedSecrets == nil {
		wandb.Status.GeneratedSecrets = map[string]corev1.SecretKeySelector{}
	}
	for _, gs := range manifest.GeneratedSecrets {
		// Deterministic secret name scoped to the CR instance
		// If UseExactName is true, use the exact name without prefixing
		secretName := gs.Name
		if !gs.UseExactName {
			secretName = fmt.Sprintf("%s-%s", wandb.Name, gs.Name)
		}
		keyName := "key"
		sec := &corev1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: wandb.Namespace}, sec)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				// Create new secret with generated value
				valueLen := gs.Length
				if valueLen <= 0 {
					valueLen = 32
				}
				pw, err := oputils.GenerateRandomPassword(valueLen)
				if err != nil {
					return ctrl.Result{}, err
				}
				sec = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: wandb.Namespace,
						Labels: map[string]string{
							"app.kubernetes.io/managed-by": "wandb-operator",
							"app.kubernetes.io/instance":   wandb.Name,
							"app.kubernetes.io/part-of":    "wandb",
						},
					},
					StringData: map[string]string{keyName: pw},
					Type:       corev1.SecretTypeOpaque,
				}
				if err := controllerutil.SetOwnerReference(wandb, sec, client.Scheme()); err != nil {
					return ctrl.Result{}, err
				}
				if err := client.Create(ctx, sec); err != nil {
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, err
			}
		} else {
			// Secret exists. Ensure it has the expected key; do not overwrite existing value.
			if sec.Data == nil || (sec.Data != nil && sec.Data[keyName] == nil && sec.StringData == nil) {
				if sec.StringData == nil {
					sec.StringData = map[string]string{}
				}
				// Generate a value only if missing
				valueLen := gs.Length
				if valueLen <= 0 {
					valueLen = 32
				}
				pw, err := oputils.GenerateRandomPassword(valueLen)
				if err != nil {
					return ctrl.Result{}, err
				}
				sec.StringData[keyName] = pw
				if err := client.Update(ctx, sec); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
		// Record selector in status
		wandb.Status.GeneratedSecrets[gs.Name] = corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  keyName,
		}
	}
	// Persist status after updating generated secret selectors
	if err := client.Status().Update(ctx, wandb); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// manifestFeaturesEnabled returns true if any of the topic's feature flags are enabled
// in the manifest's top-level Features section.
func manifestFeaturesEnabled(topicFeatures []string, mf map[string]bool) bool {
	if len(topicFeatures) == 0 || mf == nil {
		return false
	}
	for _, f := range topicFeatures {
		if enabled, ok := mf[f]; ok && enabled {
			return true
		}
	}
	return false
}

func resolveServiceURLFromManifest(wandbName string, manifest serverManifest.Manifest, src serverManifest.EnvSource) (string, bool) {
	if src.Name == "" {
		return "", false
	}

	app, ok := manifest.Applications[src.Name]
	if !ok {
		return "", false
	}

	port, ok := resolveServicePortFromManifest(app, src.Port)
	if !ok {
		return "", false
	}

	protoPrefix := ""
	if src.Proto != "" {
		protoPrefix = fmt.Sprintf("%s://", src.Proto)
	}
	serviceName := fmt.Sprintf("%s-%s", wandbName, src.Name)
	return fmt.Sprintf("%s%s:%d%s", protoPrefix, serviceName, port, src.Path), true
}

func resolveServicePortFromManifest(app serverManifest.Application, requestedPort string) (int32, bool) {
	if requestedPort != "" {
		if n, err := strconv.ParseInt(requestedPort, 10, 32); err == nil {
			return int32(n), true
		}
	}

	if app.Service != nil {
		if requestedPort == "" && len(app.Service.Ports) > 0 {
			return app.Service.Ports[0].Port, true
		}
		for _, p := range app.Service.Ports {
			if p.Name == requestedPort {
				return p.Port, true
			}
		}
	}

	for _, container := range app.Containers {
		if requestedPort == "" && len(container.Ports) > 0 {
			return container.Ports[0].ContainerPort, true
		}
		for _, p := range container.Ports {
			if p.Name == requestedPort {
				return p.ContainerPort, true
			}
		}
	}
	for _, container := range app.InitContainers {
		if requestedPort == "" && len(container.Ports) > 0 {
			return container.Ports[0].ContainerPort, true
		}
		for _, p := range container.Ports {
			if p.Name == requestedPort {
				return p.ContainerPort, true
			}
		}
	}

	return 0, false
}

// resolveCRFieldString resolves a dotted field path (e.g., "spec.wandb.license")
// from the provided custom resource object, returning the string value if present.
// Non-string terminal values are treated as not found.
func resolveCRFieldString(obj any, path string) (string, bool) {
	if obj == nil || path == "" {
		return "", false
	}
	// Marshal to JSON then unmarshal into a generic map for easy traversal.
	b, err := json.Marshal(obj)
	if err != nil {
		return "", false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", false
	}
	cur := any(m)
	for _, seg := range strings.Split(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		next, ok := mm[seg]
		if !ok {
			return "", false
		}
		cur = next
	}
	if s, ok := cur.(string); ok {
		return s, true
	}
	return "", false
}

func inferState(
	ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	redisOk := wandb.Spec.Redis.ManagedRedis == nil || wandb.Status.RedisStatus.Ready
	objectStoreOk := wandb.Spec.ObjectStore.ManagedObjectStore == nil || wandb.Status.ObjectStoreStatus.Ready
	mysqlOk := wandb.Spec.MySQL.ManagedMysql == nil || wandb.Status.MySQLStatus.Ready
	clickHouseOk := wandb.Spec.ClickHouse.ManagedClickHouse == nil || wandb.Status.ClickHouseStatus.Ready
	kafkaOk := wandb.Spec.Kafka.ManagedKafka == nil || wandb.Status.KafkaStatus.Ready

	if redisOk && objectStoreOk && mysqlOk && clickHouseOk && kafkaOk {
		wandb.Status.Ready = true
	} else {
		wandb.Status.Ready = false
	}

	log.Info("About to update status", "apiVersion", wandb.APIVersion, "kind", wandb.Kind)
	if err := client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status")
		return err
	}
	log.Info("Status update successful")
	return nil
}

// createOrUpdateServiceAccount creates or updates the ServiceAccount for the W&B applications
func createOrUpdateServiceAccount(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	serviceAccountName string,
) error {
	log := ctrl.LoggerFrom(ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
			Annotations: wandb.Spec.Wandb.ServiceAccount.Annotations,
		},
		AutomountServiceAccountToken: ptr.To(true),
	}

	if err := controllerutil.SetControllerReference(wandb, serviceAccount, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference on ServiceAccount: %w", err)
	}

	existingServiceAccount := &corev1.ServiceAccount{}
	if err := client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: wandb.Namespace}, existingServiceAccount); err != nil {
		if apiErrors.IsNotFound(err) {
			log.Info("Creating a new ServiceAccount", "Namespace", wandb.Namespace, "Name", serviceAccountName)
			if err := client.Create(ctx, serviceAccount); err != nil {
				return fmt.Errorf("failed to create ServiceAccount: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get existing ServiceAccount: %w", err)
	}

	existingServiceAccount.Annotations = serviceAccount.Annotations
	existingServiceAccount.Labels = serviceAccount.Labels
	existingServiceAccount.OwnerReferences = serviceAccount.OwnerReferences
	existingServiceAccount.AutomountServiceAccountToken = serviceAccount.AutomountServiceAccountToken
	log.Info("Updating existing ServiceAccount", "Namespace", wandb.Namespace, "Name", serviceAccountName)
	if err := client.Update(ctx, existingServiceAccount); err != nil {
		return fmt.Errorf("failed to update ServiceAccount: %w", err)
	}

	return nil
}

// createOrUpdateRole creates or updates the Role for the W&B service account
func createOrUpdateRole(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	serviceAccountName string,
) error {
	log := ctrl.LoggerFrom(ctx)

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "create", "update", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(wandb, role, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference on Role: %w", err)
	}

	existingRole := &rbacv1.Role{}
	err := client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: wandb.Namespace}, existingRole)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.Info("Creating Role", "name", serviceAccountName, "namespace", wandb.Namespace)
			if err := client.Create(ctx, role); err != nil {
				return fmt.Errorf("failed to create Role: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Role: %w", err)
	}

	// Update existing role
	existingRole.Rules = role.Rules
	existingRole.Labels = role.Labels
	log.Info("Updating Role", "name", serviceAccountName, "namespace", wandb.Namespace)
	if err := client.Update(ctx, existingRole); err != nil {
		return fmt.Errorf("failed to update Role: %w", err)
	}

	return nil
}

// createOrUpdateRoleBinding creates or updates the RoleBinding for the W&B service account
func createOrUpdateRoleBinding(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	serviceAccountName string,
) error {
	log := ctrl.LoggerFrom(ctx)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     serviceAccountName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: wandb.Namespace,
			},
		},
	}

	if err := controllerutil.SetOwnerReference(wandb, roleBinding, client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference on RoleBinding: %w", err)
	}

	existingRoleBinding := &rbacv1.RoleBinding{}
	err := client.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: wandb.Namespace}, existingRoleBinding)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.Info("Creating RoleBinding", "name", serviceAccountName, "namespace", wandb.Namespace)
			if err := client.Create(ctx, roleBinding); err != nil {
				return fmt.Errorf("failed to create RoleBinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get RoleBinding: %w", err)
	}

	// Update existing rolebinding
	existingRoleBinding.RoleRef = roleBinding.RoleRef
	existingRoleBinding.Subjects = roleBinding.Subjects
	existingRoleBinding.Labels = roleBinding.Labels
	log.Info("Updating RoleBinding", "name", serviceAccountName, "namespace", wandb.Namespace)
	if err := client.Update(ctx, existingRoleBinding); err != nil {
		return fmt.Errorf("failed to update RoleBinding: %w", err)
	}

	return nil
}

// createOrUpdateOIDCDiscoveryClusterRoleBinding creates or updates the ClusterRoleBinding
// for OIDC discovery. This is required for JWT token validation between W&B services.
// Returns error if creation fails, but this is non-fatal for reconciliation.
func createOrUpdateOIDCDiscoveryClusterRoleBinding(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	clusterRoleBindingName := fmt.Sprintf("%s-oidc-discovery", wandb.Name)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:service-account-issuer-discovery",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:unauthenticated",
			},
		},
	}

	// Note: ClusterRoleBinding cannot have ownerReferences to namespaced resources
	// It will be cleaned up manually or left as cluster-scoped resource

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err := client.Get(ctx, types.NamespacedName{Name: clusterRoleBindingName}, existingClusterRoleBinding)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.Info("Creating ClusterRoleBinding for OIDC discovery", "name", clusterRoleBindingName)
			if err := client.Create(ctx, clusterRoleBinding); err != nil {
				return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ClusterRoleBinding: %w", err)
	}

	// Update existing clusterrolebinding
	existingClusterRoleBinding.RoleRef = clusterRoleBinding.RoleRef
	existingClusterRoleBinding.Subjects = clusterRoleBinding.Subjects
	existingClusterRoleBinding.Labels = clusterRoleBinding.Labels
	log.Info("Updating ClusterRoleBinding for OIDC discovery", "name", clusterRoleBindingName)
	if err := client.Update(ctx, existingClusterRoleBinding); err != nil {
		return fmt.Errorf("failed to update ClusterRoleBinding: %w", err)
	}

	return nil
}

func isOwnedBy(obj ctrlClient.Object, owner *apiv2.WeightsAndBiases) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner.UID {
			return true
		}
	}
	return false
}
