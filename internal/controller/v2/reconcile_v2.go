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
	"fmt"
	"os"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/translator"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var defaultRequeueMinutes = 1
var defaultRequeueDuration = time.Duration(defaultRequeueMinutes) * time.Minute

// Reconcile for V2 of WandB as the assumed object
func Reconcile(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	var minioConnection *translator.InfraConnection
	var err error

	/////////////////////////
	// Write State
	if err = redisWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if minioConnection, err = minioWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseWriteState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	/////////////////////////
	// Status Update
	if err = redisReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = mysqlReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = kafkaReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}
	if err = minioReadState(ctx, client, wandb, minioConnection); err != nil {
		return ctrl.Result{}, err
	}
	if err = clickHouseReadState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	/////////////////////////

	if err = inferState(ctx, client, wandb); err != nil {
		return ctrl.Result{}, err
	}

	return reconcileWandbManifest(ctx, client, wandb)
}

func reconcileWandbManifest(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases) (ctrl.Result, error) {
	// Reconcile Wandb Manifest
	manifest := serverManifest.Manifest{}
	manifestData, err := os.ReadFile("0.76.1.yaml")
	if err != nil {
		return ctrl.Result{}, err
	}
	if err = yaml.Unmarshal(manifestData, &manifest); err != nil {
		return ctrl.Result{}, err
	}

	// Create Strimzi KafkaTopic resources for enabled topics
	if wandb.Spec.Kafka.Enabled {
		for _, topic := range manifest.Kafka {
			if !manifestTopicEnabled(topic.Features, manifest.Features) {
				continue
			}

			// Determine namespace and cluster name for Strimzi resources
			kafkaNS := wandb.Spec.Kafka.Namespace
			if kafkaNS == "" {
				kafkaNS = wandb.Namespace
			}
			clusterName := wandb.Spec.Kafka.Name
			if clusterName == "" {
				// Fallback to instance name if not explicitly configured
				clusterName = wandb.Name
			}

			// Build KafkaTopic unstructured object
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "kafka.strimzi.io",
				Version: "v1beta2",
				Kind:    "KafkaTopic",
			})
			// Use the logical topic name as the resource name
			resName := topic.Topic
			if resName == "" {
				// If not set, fallback to topic entry name
				resName = topic.Name
			}
			if resName == "" {
				// Nothing actionable without a name
				continue
			}
			u.SetName(resName)
			u.SetNamespace(kafkaNS)
			labels := map[string]string{
				"strimzi.io/cluster":           clusterName,
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/part-of":    "wandb",
				"app.kubernetes.io/instance":   wandb.Name,
			}
			u.SetLabels(labels)

			// Prepare spec
			partitions := int64(1)
			if topic.PartitionCount > 0 {
				partitions = int64(topic.PartitionCount)
			}
			replicas := int64(1)
			if wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor > 0 {
				replicas = int64(wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor)
			}

			_ = unstructured.SetNestedField(u.Object, topic.Topic, "spec", "topicName")
			_ = unstructured.SetNestedField(u.Object, partitions, "spec", "partitions")
			_ = unstructured.SetNestedField(u.Object, replicas, "spec", "replicas")

			// Create or Update
			existing := &unstructured.Unstructured{}
			existing.SetGroupVersionKind(u.GroupVersionKind())
			getErr := client.Get(ctx, types.NamespacedName{Name: u.GetName(), Namespace: kafkaNS}, existing)
			if getErr != nil {
				if apiErrors.IsNotFound(getErr) {
					// Set ownerRef only if same namespace
					if kafkaNS == wandb.Namespace {
						_ = controllerutil.SetOwnerReference(wandb, u, client.Scheme())
					}
					if err := client.Create(ctx, u); err != nil {
						return ctrl.Result{}, err
					}
				} else {
					return ctrl.Result{}, getErr
				}
			} else {
				// Update spec fields if they differ
				// We replace the spec fields we manage; keep other fields intact
				_ = unstructured.SetNestedField(existing.Object, topic.Topic, "spec", "topicName")
				_ = unstructured.SetNestedField(existing.Object, partitions, "spec", "partitions")
				_ = unstructured.SetNestedField(existing.Object, replicas, "spec", "replicas")
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

	// TODO lets run migrations here, need to add some logic for ensuring they run only once.
	//for name, migrationTask := range manifest.Migrations {
	//	// check for a currently running job
	//	jobName := fmt.Sprintf("%s-%s", wandb.Name, name)
	//	job := &batchv1.Job{}
	//	err := client.Get(
	//		ctx,
	//		types.NamespacedName{
	//			Namespace: wandb.Namespace,
	//			Name:      jobName,
	//		},
	//		job,
	//	)
	//	if err != nil && !apiErrors.IsNotFound(err) {
	//		return ctrl.Result{}, err
	//	}
	//	if apiErrors.IsNotFound(err) {
	//		job.ObjectMeta.Name = jobName
	//		job.ObjectMeta.Namespace = wandb.Namespace
	//		containerSpec := corev1.Container{
	//			Name:  name,
	//			Image: migrationTask.Image.GetImage(),
	//			Args:  migrationTask.Args,
	//		}
	//		job.Spec.Template.Spec.Containers = []corev1.Container{containerSpec}
	//		err = client.Create(ctx, job)
	//	} else {
	//		if job.Status.Succeeded == 0 {
	//			client.Delete(ctx, job)
	//		}
	//	}
	//}

	for _, app := range manifest.Applications {
		// If the application is gated behind features, only install it when
		// at least one of those features is enabled in the manifest.
		if len(app.Features) > 0 && !manifestTopicEnabled(app.Features, manifest.Features) {
			continue
		}
		application := &apiv2.Application{}
		applicationName := fmt.Sprintf("%s-%s", wandb.Name, app.Name)
		err := client.Get(ctx, types.NamespacedName{Name: applicationName, Namespace: wandb.Namespace}, application)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				application.ObjectMeta.Name = applicationName
				application.ObjectMeta.Namespace = wandb.Namespace
			} else {
				return ctrl.Result{}, err
			}
		}

		var combinedEnvs []serverManifest.EnvVar
		for _, commonVars := range app.CommonEnvs {
			if envvars, ok := manifest.CommonEnvvars[commonVars]; ok {
				for _, env := range envvars {
					combinedEnvs = append(combinedEnvs, env)
				}
			}
		}

		for _, env := range app.Env {
			combinedEnvs = append(combinedEnvs, env)
		}

		var envVars []corev1.EnvVar
		for _, env := range combinedEnvs {
			var secretKeySelector corev1.SecretKeySelector
			var envValue string

			if env.Value != "" {
				envValue = env.Value
			} else {
				switch env.Sources[0].Type {
				case "mysql":
					secretKeySelector = wandb.Status.MySQLStatus.Connection.URL
				case "redis":
					secretKeySelector = wandb.Status.RedisStatus.Connection.URL
				case "bucket":
					secretKeySelector = wandb.Status.MinioStatus.Connection.URL
				case "clickhouse":
					switch env.Sources[0].Field {
					case "host":
						secretKeySelector.LocalObjectReference = wandb.Status.ClickHouseStatus.Connection.URL.LocalObjectReference
						secretKeySelector.Key = "Host"
					case "port":
						secretKeySelector.LocalObjectReference = wandb.Status.ClickHouseStatus.Connection.URL.LocalObjectReference
						secretKeySelector.Key = "Port"
					case "user":
						secretKeySelector.LocalObjectReference = wandb.Status.ClickHouseStatus.Connection.URL.LocalObjectReference
						secretKeySelector.Key = "User"
					case "password":
						secretKeySelector.LocalObjectReference = wandb.Status.ClickHouseStatus.Connection.URL.LocalObjectReference
						secretKeySelector.Key = "Password"
					case "database":
						secretKeySelector.LocalObjectReference = wandb.Status.ClickHouseStatus.Connection.URL.LocalObjectReference
						secretKeySelector.Key = "Database"
					}
				case "service":
					serviceList := &corev1.ServiceList{}
					targetApplicationName := fmt.Sprintf("%s-%s", wandb.Name, env.Sources[0].Name)
					err := client.List(
						ctx,
						serviceList,
						ctrlClient.InNamespace(wandb.Namespace),
						ctrlClient.MatchingLabels{"app.kubernetes.io/name": targetApplicationName},
					)
					if err != nil {
						return ctrl.Result{}, err
					}
					if len(serviceList.Items) > 0 {
						proto := ""
						if env.Sources[0].Proto != "" {
							proto = fmt.Sprintf("%s://", env.Sources[0].Proto)
						}
						envValue = fmt.Sprintf("%s%s:%d%s", proto, serviceList.Items[0].Name, serviceList.Items[0].Spec.Ports[0].Port, env.Sources[0].Path)
					}
				}
			}
			if envValue != "" || secretKeySelector.Key != "" {
				envVar := corev1.EnvVar{}
				envVar.Name = env.Name
				if envValue != "" {
					envVar.Value = envValue
				} else if secretKeySelector.Key != "" {
					envVar.ValueFrom = &corev1.EnvVarSource{
						SecretKeyRef: &secretKeySelector,
					}
				}

				envVars = append(envVars, envVar)
			}
		}

		container := corev1.Container{
			Name:  app.Name,
			Image: app.Image.GetImage(),
			Env:   envVars,
			Args:  app.Args,
		}

		initContainers := []corev1.Container{}

		if app.InitContainers != nil {
			for _, initContainerSpec := range app.InitContainers {
				initContainer := corev1.Container{
					Name:    initContainerSpec.Name,
					Image:   initContainerSpec.Image.GetImage(),
					Env:     envVars,
					Args:    initContainerSpec.Args,
					Command: initContainerSpec.Command,
				}
				initContainers = append(initContainers, initContainer)
			}
		}

		application.Spec.Kind = "Deployment"
		application.Spec.PodTemplate.Spec.Containers = []corev1.Container{container}
		application.Spec.PodTemplate.Spec.InitContainers = initContainers

		if app.Service != nil && len(app.Service.Ports) > 0 {
			application.Spec.ServiceTemplate = &corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:     app.Service.Ports[0].Name,
					Port:     app.Service.Ports[0].Port,
					Protocol: app.Service.Ports[0].Protocol,
				}},
			}
		}

		err = controllerutil.SetOwnerReference(wandb, application, client.Scheme())
		if err != nil {
			return ctrl.Result{}, err
		}

		if application.ObjectMeta.CreationTimestamp.IsZero() {
			if err = client.Create(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err = client.Update(ctx, application); err != nil {
				return ctrl.Result{}, err
			}
		}

	}
	return ctrl.Result{}, nil
}

// manifestTopicEnabled returns true if any of the topic's feature flags are enabled
// in the manifest's top-level Features section.
func manifestTopicEnabled(topicFeatures []string, mf *serverManifest.Features) bool {
	if len(topicFeatures) == 0 || mf == nil {
		return false
	}
	for _, f := range topicFeatures {
		switch f {
		case "runsV2":
			if mf.RunsV2 {
				return true
			}
		case "filestreamQueue":
			if mf.FilestreamQueue {
				return true
			}
		case "metricObserver":
			if mf.MetricObserver {
				return true
			}
		case "weaveTrace":
			if mf.WeaveTrace {
				return true
			}
		case "proxy":
			if mf.Proxy {
				return true
			}
		case "weaveTraceWorkers":
			if mf.WeaveTraceWorkers {
				return true
			}
		}
	}
	return false
}

func inferState(
	ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases,
) error {
	log := ctrl.LoggerFrom(ctx)

	// Infra is "ok" if either it is not enabled or if it is (enabled and) ready
	redisOk := !wandb.Spec.Redis.Enabled || wandb.Status.RedisStatus.Ready
	minioOk := !wandb.Spec.Minio.Enabled || wandb.Status.MinioStatus.Ready
	mysqlOk := !wandb.Spec.MySQL.Enabled || wandb.Status.MySQLStatus.Ready
	clickHouseOk := !wandb.Spec.ClickHouse.Enabled || wandb.Status.ClickHouseStatus.Ready
	kafkaOk := !wandb.Spec.Kafka.Enabled || wandb.Status.KafkaStatus.Ready

	if redisOk && minioOk && mysqlOk && clickHouseOk && kafkaOk {
		wandb.Status.State = "Ready"
	} else {
		wandb.Status.State = "NotReady"
	}

	if err := client.Status().Update(ctx, wandb); err != nil {
		log.Error(err, "Failed to update status")
		return err
	}
	return nil
}
