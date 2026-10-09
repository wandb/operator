package reconciler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const workloadInputsAnnotation = "apps.wandb.com/workload-inputs"

func hasConditionalEnvs(manifest serverManifest.Manifest, common []string, envs []serverManifest.EnvVar) bool {
	for _, group := range common {
		for _, env := range manifest.CommonEnvvars[group] {
			if len(env.Features) > 0 {
				return true
			}
		}
	}
	for _, env := range envs {
		if len(env.Features) > 0 {
			return true
		}
	}
	return false
}

func workloadInputHash(template corev1.PodTemplateSpec, version, credentials string) (string, error) {
	data, err := json.Marshal(struct {
		Template    corev1.PodTemplateSpec
		Version     string
		Credentials string
	}{template, version, credentials})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func migrationInputHash(template corev1.PodTemplateSpec, version, credentials string) (string, error) {
	// Security profiles apply when a job next runs; changing a profile alone
	// must not rerun a completed schema migration at the same server version.
	inputs := template.DeepCopy()
	inputs.Spec.SecurityContext = nil
	for i := range inputs.Spec.Containers {
		inputs.Spec.Containers[i].SecurityContext = nil
	}
	for i := range inputs.Spec.InitContainers {
		inputs.Spec.InitContainers[i].SecurityContext = nil
	}
	return workloadInputHash(*inputs, version, credentials)
}

func runMigrations(ctx context.Context, client ctrlclient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (ctrl.Result, error) {
	before := wandb.DeepCopy().Status
	version := wandb.Spec.Wandb.Version
	previous := wandb.Status.Wandb.Migration
	legacyComplete := previous.Ready && previous.Version == version
	if previous.Version != version {
		wandb.Status.Wandb.Migration.Jobs = nil
	}
	status := &wandb.Status.Wandb.Migration
	status.Version = version
	if status.Jobs == nil {
		status.Jobs = map[string]apiv2.MigrationJobStatus{}
	}
	allSucceeded, anyFailed := true, false
	for _, name := range slices.Sorted(maps.Keys(manifest.Migrations)) {
		task := manifest.Migrations[name]
		template, err := migrationPodTemplate(ctx, client, wandb, manifest, task)
		if err != nil {
			return ctrl.Result{}, err
		}
		credentials, err := generatedSecretChecksum(ctx, client, wandb, template.Spec.Containers[0].Env)
		if err != nil {
			return ctrl.Result{}, err
		}
		hash, err := migrationInputHash(template, version, credentials)
		if err != nil {
			return ctrl.Result{}, err
		}
		jobName := fmt.Sprintf("%s-%s", wandb.Name, name)
		old := status.Jobs[name]
		// Adopt the old version-only cache for unaffected jobs. Conditional envs
		// must run once because their previous effective inputs are unknown.
		if legacyComplete && old.InputHash == "" && !hasConditionalEnvs(manifest, task.CommonEnvs, task.Env) {
			old = apiv2.MigrationJobStatus{Name: jobName, InputHash: hash, Succeeded: true, Phase: migrationPhaseSucceeded, Reason: "Complete"}
			status.Jobs[name] = old
		}
		job := &batchv1.Job{}
		err = client.Get(ctx, ctrlclient.ObjectKey{Namespace: wandb.Namespace, Name: jobName}, job)
		exists := err == nil
		if err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if exists && !isOwnedBy(job, wandb) {
			return ctrl.Result{}, fmt.Errorf("migration job %q is not owned by this WeightsAndBiases CR", jobName)
		}
		complete, failed := migrationJobTerminal(job)
		if old.Succeeded && old.InputHash == hash && (!exists || complete || failed) {
			if exists && job.DeletionTimestamp.IsZero() {
				if err := client.Delete(ctx, job, ctrlclient.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
			continue
		}
		current := apiv2.MigrationJobStatus{Name: jobName, InputHash: hash, Phase: migrationPhaseRunning, Reason: "JobPending"}
		if exists && (job.Annotations[workloadInputsAnnotation] != hash || !job.DeletionTimestamp.IsZero()) {
			// Foreground deletion keeps the name occupied until old pods are gone.
			// Never terminate an active schema migration to apply a newer input.
			if (complete || failed) && job.DeletionTimestamp.IsZero() {
				if err := client.Delete(ctx, job, ctrlclient.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
			current.Reason = "WaitingForPreviousJob"
		} else if !exists {
			job = &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name: jobName, Namespace: wandb.Namespace,
					Annotations: map[string]string{workloadInputsAnnotation: hash},
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "wandb-operator",
						"app.kubernetes.io/instance":   wandb.Name,
						"app.kubernetes.io/component":  "migration",
					},
				},
				Spec: batchv1.JobSpec{Template: template},
			}
			if err := controllerutil.SetOwnerReference(wandb, job, client.Scheme()); err != nil {
				return ctrl.Result{}, err
			}
			if err := client.Create(ctx, job); err != nil {
				return ctrl.Result{}, err
			}
			current.Reason = "JobCreated"
		} else if complete {
			current.Succeeded, current.Phase, current.Reason = true, migrationPhaseSucceeded, "JobSucceeded"
		} else if failed {
			current.Failed, current.Phase, current.Reason = true, migrationPhaseFailed, "JobFailed"
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					current.Message = cond.Message
					if cond.Reason != "" {
						current.Reason = cond.Reason
					}
				}
			}
		} else if job.Status.Active > 0 {
			current.Reason = "JobRunning"
		}
		status.Jobs[name] = current
		allSucceeded = allSucceeded && current.Succeeded
		anyFailed = anyFailed || current.Failed
	}
	status.Ready = allSucceeded
	status.Phase, status.Reason = migrationPhaseRunning, "Running"
	if allSucceeded {
		status.Phase, status.Reason = migrationPhaseSucceeded, "Complete"
		status.LastSuccessVersion = version
	} else if anyFailed {
		status.Phase, status.Reason = migrationPhaseFailed, "Failed"
	}
	if !allSucceeded {
		reason, message := migrationReadiness(wandb)
		setReadyStatus(wandb, false, reason, message)
	}
	if err := updateWandbStatusIfChanged(ctx, client, wandb, before); err != nil {
		return ctrl.Result{}, err
	}
	if !allSucceeded {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

func migrationJobTerminal(job *batchv1.Job) (complete, failed bool) {
	for _, condition := range job.Status.Conditions {
		if condition.Status == corev1.ConditionTrue {
			complete = complete || condition.Type == batchv1.JobComplete
			failed = failed || condition.Type == batchv1.JobFailed
		}
	}
	return complete, failed
}

func migrationPodTemplate(ctx context.Context, client ctrlclient.Client, wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest, task serverManifest.MigrationJob) (corev1.PodTemplateSpec, error) {
	envVars, err := resolveEnvvars(ctx, client, wandb, manifest, task.CommonEnvs, task.Env)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	volumes, volumeMounts, err := resolveVolumeMounts(ctx, manifest, task.CommonVolumeMounts, task.VolumeMounts)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	var caChecksum string
	envVars, volumes, volumeMounts, caChecksum, err = applyCustomCACertsToWorkload(ctx, client, wandb, envVars, volumes, volumeMounts)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	// spec.global.proxy env (migration Jobs egress too — v1 parity); before
	// legacy overrides so the escape hatch still wins.
	envVars = applyProxyToWorkload(wandb, envVars)

	// v1's global env reached job pods too (e.g. HTTP_PROXY); per-app entries don't apply here.
	envVars = overrideEnvVars(ctx, envVars, wandb.Spec.Wandb.LegacyOverrides[apiv2.LegacyOverridesGlobalKey].Env)

	podTemplate := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:         "migrate",
					Image:        task.Image.GetImage(wandb.Spec.Global.ImageRegistry),
					Args:         task.Args,
					Command:      task.Command,
					Env:          envVars,
					VolumeMounts: volumeMounts,
				},
			},
			Volumes:            volumes,
			ServiceAccountName: wandb.Spec.Wandb.ServiceAccount.ServiceAccountName,
			ImagePullSecrets:   wandb.Spec.Global.ImagePullSecrets,
		},
	}
	if hasWorkloadSecurityProfile(task.SecurityProfile) {
		applyWorkloadSecurityProfile(&podTemplate.Spec, task.SecurityProfile)
	}
	setCustomCACertsChecksumAnnotation(&podTemplate, caChecksum)
	return podTemplate, nil
}
