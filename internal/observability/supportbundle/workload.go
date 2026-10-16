package supportbundle

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	collectorExecutable = "/app/bin/collector"
	collectorWorkingDir = "/app"

	collectionMountPath   = "/etc/support-bundle"
	stagingMountPath      = "/staging"
	tmpMountPath          = "/tmp"
	resultPath            = "/dev/termination-log"
	stagingSizeLimitValue = "30Gi"

	// Each run's bundle ID is its Job name, which Kubernetes stamps on the Job's pods.
	bundleIDEnv       = "SUPPORT_BUNDLE_ID"
	bundleIDFieldPath = "metadata.labels['batch.kubernetes.io/job-name']"

	collectionVolumeName = "collection"
	stagingVolumeName    = "staging"
	tmpVolumeName        = "tmp"

	runAsUser int64 = 65532

	activeDeadlineSeconds   int64 = 1800
	ttlSecondsAfterFinished int32 = 86400
	terminationGraceSeconds int64 = 30
)

var stagingSizeLimit = resource.MustParse(stagingSizeLimitValue)

// BucketConnection stores the Kubernetes Secret references for the ObjectStore
type BucketConnection struct {
	URL    *corev1.SecretKeySelector
	Region *corev1.SecretKeySelector
}

// DefaultBucketConnection retrieves the default object store's connection from status
func DefaultBucketConnection(wandb *apiv2.WeightsAndBiases) (BucketConnection, error) {
	status, ok := wandb.Status.ObjectStoreStatus[apiv2.DefaultInstanceName]
	if !ok {
		return BucketConnection{}, fmt.Errorf("default object store connection is not available yet")
	}
	conn := BucketConnection{
		URL:    status.Connection.URL.SecretKeyRef(),
		Region: status.Connection.Region.SecretKeyRef(),
	}
	if conn.URL == nil {
		return BucketConnection{}, fmt.Errorf("default object store connection has no url secret")
	}
	return conn, nil
}

// Application builds the CronJob Application for support-bundle
func Application(wandb *apiv2.WeightsAndBiases, bucket BucketConnection) *apiv2.Application {
	return &apiv2.Application{
		Name:      Name(wandb),
		Namespace: wandb.Namespace,
		Labels:    objectLabels(wandb),
		Spec: apiv2.ApplicationSpec{
			Kind:        "CronJob",
			PodTemplate: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{}}},
			CronJobs:    []batchv1.CronJob{cronJob(wandb, bucket)},
		},
	}
}

// cronJob creates the job template for support bundle
func cronJob(wandb *apiv2.WeightsAndBiases, bucket BucketConnection) batchv1.CronJob {
	return batchv1.CronJob{
		Name:   CronJobName(wandb),
		Labels: map[string]string{"app.kubernetes.io/version": Version(wandb)},
		Spec: batchv1.CronJobSpec{
			Schedule:                   supportBundleSchedule,
			Suspend:                    new(true),
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: new(int32(1)),
			FailedJobsHistoryLimit:     new(int32(1)),
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: SelectorLabels(wandb)},
				Spec: batchv1.JobSpec{
					BackoffLimit:            new(int32(0)),
					ActiveDeadlineSeconds:   new(activeDeadlineSeconds),
					TTLSecondsAfterFinished: new(ttlSecondsAfterFinished),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: SelectorLabels(wandb)},
						Spec:       podSpec(wandb, bucket),
					},
				},
			},
		},
	}
}

// podSpec creates the spec for the Pod that's created to run the job
func podSpec(wandb *apiv2.WeightsAndBiases, bucket BucketConnection) corev1.PodSpec {
	return corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyNever,
		ServiceAccountName:            Name(wandb),
		AutomountServiceAccountToken:  new(true),
		TerminationGracePeriodSeconds: new(terminationGraceSeconds),
		ImagePullSecrets:              wandb.Spec.Global.ImagePullSecrets,
		SecurityContext:               podSecurityContext(),
		Containers: []corev1.Container{{
			Name:       supportBundleContainerName,
			Image:      Image(wandb),
			Command:    []string{collectorExecutable},
			Args:       collectorArgs(),
			WorkingDir: collectorWorkingDir,
			Env:        collectorEnv(bucket),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: collectionVolumeName, MountPath: collectionMountPath, ReadOnly: true},
				{Name: stagingVolumeName, MountPath: stagingMountPath},
				{Name: tmpVolumeName, MountPath: tmpMountPath},
			},
			// Path to the pod termination message
			TerminationMessagePath:   resultPath,
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			SecurityContext:          containerSecurityContext(),
		}},
		Volumes: []corev1.Volume{
			{Name: collectionVolumeName,
				ConfigMap: &corev1.ConfigMapVolumeSource{Name: Name(wandb)}},
			{Name: stagingVolumeName,
				EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &stagingSizeLimit}},
			{Name: tmpVolumeName, EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}
}

// collectorArgs is what will run to retrieve the support bundle
func collectorArgs() []string {
	return []string{
		"--log-format=json",
		"agent",
		"--lumen-collection=false",
		"--support-bundle",
		"--support-bundle-spec=" + collectionMountPath + "/" + defaultPreset + ".yaml",
		"--staging-root=file://" + stagingMountPath,
		"--support-bundle-id=$(" + bundleIDEnv + ")",
	}
}

func collectorEnv(bucket BucketConnection) []corev1.EnvVar {
	env := []corev1.EnvVar{
		secretEnv("WANDB_DATA_ROOT", bucket.URL, false),
		{Name: "LUMEN_SUPPORT_BUNDLE_RESULT_PATH", Value: resultPath},
		{Name: bundleIDEnv, ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: bundleIDFieldPath}}},
	}
	if bucket.Region != nil {
		env = append(env, secretEnv("AWS_REGION", bucket.Region, true))
	}
	return env
}

func secretEnv(name string, ref *corev1.SecretKeySelector, optional bool) corev1.EnvVar {
	selector := ref.DeepCopy()
	if optional {
		selector.Optional = new(true)
	}
	return corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: selector}}
}

func podSecurityContext() *corev1.PodSecurityContext {
	securityContext := &corev1.PodSecurityContext{
		RunAsNonRoot:   new(true),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	if !utils.IsOpenShift() {
		securityContext.RunAsUser = new(runAsUser)
		securityContext.RunAsGroup = new(runAsUser)
		securityContext.FSGroup = new(runAsUser)
	}
	return securityContext
}

func containerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: new(false),
		ReadOnlyRootFilesystem:   new(true),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}
