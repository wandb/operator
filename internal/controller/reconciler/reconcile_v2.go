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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/samber/lo"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/ctrlqueue"
	"github.com/wandb/operator/internal/logx"
	oputils "github.com/wandb/operator/pkg/utils"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
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

	if wandb.Spec.Wandb.ServiceAccount.Create != nil && *wandb.Spec.Wandb.ServiceAccount.Create {
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
		if len(app.Features) > 0 && !manifest.FeaturesEnabled(app.Features) {
			continue
		}
		desiredAppNames[app.Name] = true
	}

	for _, app := range sortedManifestApplications(manifest) {
		// If the application is gated behind features, only install it when
		// at least one of those features is enabled in the manifest.
		if len(app.Features) > 0 && !manifest.FeaturesEnabled(app.Features) {
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
			if manifest.FeaturesEnabled([]string{"proxy"}) && hostname.Port() == "" {
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

func runMigrations(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (ctrl.Result, error) {
	version := wandb.Spec.Wandb.Version

	if wandb.Status.Wandb.Migration.Ready && wandb.Status.Wandb.Migration.Version == version {
		for name := range manifest.Migrations {
			jobName := fmt.Sprintf("%s-%s", wandb.Name, name)
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: wandb.Namespace,
				},
			}
			propagation := metav1.DeletePropagationBackground
			deleteOptions := &ctrlClient.DeleteOptions{PropagationPolicy: &propagation}
			err := client.Delete(ctx, job, deleteOptions)
			if err != nil {
				if !apiErrors.IsNotFound(err) {
					return ctrl.Result{}, fmt.Errorf("failed to delete migration job %s: %v", jobName, err)
				}
			}
		}
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

func isOwnedBy(obj ctrlClient.Object, owner *apiv2.WeightsAndBiases) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == owner.UID {
			return true
		}
	}
	return false
}
