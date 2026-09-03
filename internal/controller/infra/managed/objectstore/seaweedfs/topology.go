package seaweedfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/wandb/operator/internal/controller/common"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	SeaweedTopologyReadyType = "SeaweedTopologyReady"

	stableReplicationAnnotation = "operator.wandb.com/seaweed-stable-replication"
	s3RolloutAnnotation         = "operator.wandb.com/seaweed-s3-rollout-replication"

	topologyJobPreVerify  = "pre-verify"
	topologyJobReplicate  = "replicate"
	topologyJobPostVerify = "post-verify"
)

type topologyJobState int

type statefulSetTarget struct {
	suffix   string
	replicas int32
}

const (
	topologyJobPending topologyJobState = iota
	topologyJobSucceeded
	topologyJobFailed
)

func reconcileTopology(
	ctx context.Context,
	kubeClient client.Client,
	desired, actual *seaweedv1.Seaweed,
	owner client.Object,
) ([]metav1.Condition, error) {
	targetReplication := defaultReplication(desired)
	if actual == nil {
		setStableReplication(desired, targetReplication)
		return topologyResult(
			metav1.ConditionTrue,
			"TopologyInitialized",
			fmt.Sprintf("SeaweedFS topology initialized with %d volume servers and replication %s", desired.Spec.Volume.Replicas, targetReplication),
		), nil
	}

	stableReplication := actual.Annotations[stableReplicationAnnotation]
	if stableReplication == "" {
		return adoptTopology(ctx, kubeClient, desired, actual)
	}

	setStableReplication(desired, stableReplication)
	return migrateTopology(ctx, kubeClient, desired, actual, owner, stableReplication, targetReplication)
}

func adoptTopology(
	ctx context.Context,
	kubeClient client.Client,
	desired, actual *seaweedv1.Seaweed,
) ([]metav1.Condition, error) {
	currentReplicas := volumeReplicas(actual)
	stableReplication := defaultReplication(actual)

	desired.Spec.Volume.Replicas = currentReplicas
	setDefaultReplication(desired, stableReplication)

	ready, err := seaweedComponentsAndWorkloadsReady(ctx, kubeClient, actual)
	if err != nil {
		return nil, err
	}
	if !ready {
		return topologyResult(
			metav1.ConditionFalse,
			"WaitingForTopologyAdoption",
			"waiting for the existing SeaweedFS components before recording their topology",
		), nil
	}

	setStableReplication(desired, stableReplication)
	return topologyResult(
		metav1.ConditionFalse,
		"TopologyAdopted",
		fmt.Sprintf("recorded existing SeaweedFS topology at %d volume servers with replication %s", currentReplicas, stableReplication),
	), nil
}

func migrateTopology(
	ctx context.Context,
	kubeClient client.Client,
	desired, actual *seaweedv1.Seaweed,
	owner client.Object,
	stableReplication, targetReplication string,
) ([]metav1.Condition, error) {
	targetReplicas := desired.Spec.Volume.Replicas
	actualReplicas := volumeReplicas(actual)
	readyReplicas := actual.Status.Volume.ReadyReplicas

	if targetReplicas < actualReplicas {
		desired.Spec.Volume.Replicas = actualReplicas
		setDefaultReplication(desired, stableReplication)
		return topologyResult(
			metav1.ConditionFalse,
			"ReplicaReductionUnsupported",
			fmt.Sprintf("refusing to reduce SeaweedFS volume servers from %d to %d without an evacuation workflow", actualReplicas, targetReplicas),
		), nil
	}

	if actualReplicas < targetReplicas || readyReplicas < targetReplicas {
		setDefaultReplication(desired, stableReplication)
		return topologyResult(
			metav1.ConditionFalse,
			"ScalingVolumeServers",
			fmt.Sprintf("scaling SeaweedFS volume servers from %d to %d before changing replication", actualReplicas, targetReplicas),
		), nil
	}

	actualReplication := defaultReplication(actual)
	ready, err := seaweedComponentsAndWorkloadsReady(ctx, kubeClient, actual)
	if err != nil {
		return nil, err
	}
	if !ready {
		setDefaultReplication(desired, actualReplication)
		return topologyResult(
			metav1.ConditionFalse,
			"WaitingForSeaweedComponents",
			"waiting for SeaweedFS master, volume, filer, and S3 components before continuing topology migration",
		), nil
	}
	if targetReplication == stableReplication && actualReplication == targetReplication {
		return topologyResult(
			metav1.ConditionTrue,
			"TopologyReady",
			fmt.Sprintf("SeaweedFS has %d ready volume servers with replication %s", readyReplicas, targetReplication),
		), nil
	}

	if actualReplication == stableReplication {
		setDefaultReplication(desired, stableReplication)
		state, message, err := reconcileTopologyJob(
			ctx, kubeClient, desired, owner, topologyJobPreVerify, targetReplication,
			verificationScript(desired),
		)
		if err != nil {
			return nil, err
		}
		if result := topologyJobResult(
			state,
			"PreMigrationVerificationFailed",
			"PreMigrationVerificationRunning",
			message,
		); result != nil {
			return result, nil
		}

		setDefaultReplication(desired, targetReplication)
		return topologyResult(
			metav1.ConditionFalse,
			"UpdatingReplicationPolicy",
			fmt.Sprintf("existing data verified; changing SeaweedFS replication from %s to %s", stableReplication, targetReplication),
		), nil
	}

	if actualReplication != targetReplication {
		setDefaultReplication(desired, stableReplication)
		return topologyResult(
			metav1.ConditionFalse,
			"UnexpectedReplicationPolicy",
			fmt.Sprintf("SeaweedFS replication is %s; expected stable %s or target %s", actualReplication, stableReplication, targetReplication),
		), nil
	}

	state, message, err := reconcileTopologyJob(
		ctx, kubeClient, desired, owner, topologyJobReplicate, targetReplication,
		replicationScript(desired, targetReplication),
	)
	if err != nil {
		return nil, err
	}
	if result := topologyJobResult(
		state,
		"ReplicationFailed",
		"ReplicationRunning",
		message,
	); result != nil {
		return result, nil
	}

	s3Ready, err := s3GatewayRolloutReady(ctx, kubeClient, desired, targetReplication)
	if err != nil {
		return nil, err
	}
	if !s3Ready {
		if err := rolloutS3Gateway(ctx, kubeClient, desired, targetReplication); err != nil {
			return nil, err
		}
		return topologyResult(
			metav1.ConditionFalse,
			"RestartingS3Gateway",
			"restarting the SeaweedFS S3 gateway after volume replication",
		), nil
	}

	state, message, err = reconcileTopologyJob(
		ctx, kubeClient, desired, owner, topologyJobPostVerify, targetReplication,
		verificationScript(desired),
	)
	if err != nil {
		return nil, err
	}
	if result := topologyJobResult(
		state,
		"PostMigrationVerificationFailed",
		"PostMigrationVerificationRunning",
		message,
	); result != nil {
		return result, nil
	}

	setStableReplication(desired, targetReplication)
	return topologyResult(
		metav1.ConditionTrue,
		"TopologyMigrated",
		fmt.Sprintf("SeaweedFS data verified after replication changed from %s to %s", stableReplication, targetReplication),
	), nil
}

func topologyJobResult(
	state topologyJobState,
	failedReason, pendingReason, message string,
) []metav1.Condition {
	switch state {
	case topologyJobFailed:
		return topologyResult(metav1.ConditionFalse, failedReason, message)
	case topologyJobPending:
		return topologyResult(metav1.ConditionFalse, pendingReason, message)
	default:
		return nil
	}
}

func reconcileTopologyJob(
	ctx context.Context,
	kubeClient client.Client,
	seaweed *seaweedv1.Seaweed,
	owner client.Object,
	stage, targetReplication, script string,
) (topologyJobState, string, error) {
	name := topologyJobName(seaweed.Name, stage, targetReplication)
	job := &batchv1.Job{}
	err := kubeClient.Get(ctx, types.NamespacedName{Namespace: seaweed.Namespace, Name: name}, job)
	if err != nil && !apierrors.IsNotFound(err) {
		return topologyJobPending, "", err
	}
	if apierrors.IsNotFound(err) {
		job = topologyJob(seaweed, name, stage, targetReplication, script)
		if err := controllerutil.SetOwnerReference(owner, job, kubeClient.Scheme()); err != nil {
			return topologyJobPending, "", err
		}
		if err := kubeClient.Create(ctx, job); err != nil {
			return topologyJobPending, "", err
		}
		return topologyJobPending, fmt.Sprintf("SeaweedFS topology %s job %s created", stage, name), nil
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			message := condition.Message
			if message == "" {
				message = fmt.Sprintf("SeaweedFS topology %s job %s failed", stage, name)
			}
			return topologyJobFailed, message, nil
		}
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return topologyJobSucceeded, fmt.Sprintf("SeaweedFS topology %s job %s succeeded", stage, name), nil
		}
	}
	return topologyJobPending, fmt.Sprintf("waiting for SeaweedFS topology %s job %s", stage, name), nil
}

func topologyJob(
	seaweed *seaweedv1.Seaweed,
	name, stage, targetReplication, script string,
) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: seaweed.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   seaweed.Name,
				"app.kubernetes.io/component":  "seaweedfs-topology",
				"operator.wandb.com/stage":     stage,
			},
			Annotations: map[string]string{
				"operator.wandb.com/target-replication": targetReplication,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          ptr.To[int32](0),
			ActiveDeadlineSeconds: ptr.To[int64](86400),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "wandb-operator",
						"app.kubernetes.io/instance":   seaweed.Name,
						"app.kubernetes.io/component":  "seaweedfs-topology",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: seaweed.Spec.ImagePullSecrets,
					Containers: []corev1.Container{{
						Name:            "topology",
						Image:           seaweed.Spec.Image,
						ImagePullPolicy: seaweed.Spec.ImagePullPolicy,
						Command:         []string{"/bin/sh", "-c"},
						Args:            []string{script},
					}},
				},
			},
		},
	}
}

func verificationScript(seaweed *seaweedv1.Seaweed) string {
	master := fmt.Sprintf("%s-master:%d", seaweed.Name, seaweedv1.MasterHTTPPort)
	filer := fmt.Sprintf("%s-filer:%d", seaweed.Name, seaweedv1.FilerHTTPPort)
	return fmt.Sprintf(`set -eu
set -o pipefail
echo "fs.verify -concurrency 8 /buckets" | weed shell -master=%s -filer=%s | awk '
  /failed verify/ {
    failures++
    if (failures <= 20) print
  }
  /^total / { print }
  /^verified / {
    print
    summary=$0
  }
  END {
    if (summary !~ /^verified [0-9]+ files, error 0 files[[:space:]]*$/) exit 1
  }
'
`, master, filer)
}

func replicationScript(seaweed *seaweedv1.Seaweed, targetReplication string) string {
	master := fmt.Sprintf("%s-master:%d", seaweed.Name, seaweedv1.MasterHTTPPort)
	filer := fmt.Sprintf("%s-filer:%d", seaweed.Name, seaweedv1.FilerHTTPPort)
	return fmt.Sprintf(`set -eu
set -o pipefail
printf 'lock\nvolume.configure.replication -replication=%s\nvolume.fix.replication -apply -doDelete=false\nunlock\n' | weed shell -master=%s -filer=%s
output=/tmp/seaweed-replication-check.out
echo "volume.fix.replication -verbose -doDelete=false" | weed shell -master=%s -filer=%s | tee "$output"
if grep -Eq 'under replicated|failed to place|not well placed|mismatch in topology' "$output"; then
  exit 1
fi
`, targetReplication, master, filer, master, filer)
}

func topologyResult(status metav1.ConditionStatus, reason, message string) []metav1.Condition {
	return []metav1.Condition{{
		Type:    SeaweedTopologyReadyType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}}
}

func defaultReplication(seaweed *seaweedv1.Seaweed) string {
	if seaweed != nil && seaweed.Spec.Master != nil && seaweed.Spec.Master.DefaultReplication != nil {
		return *seaweed.Spec.Master.DefaultReplication
	}
	return "000"
}

func volumeReplicas(seaweed *seaweedv1.Seaweed) int32 {
	if seaweed == nil || seaweed.Spec.Volume == nil {
		return 0
	}
	return seaweed.Spec.Volume.Replicas
}

func setDefaultReplication(seaweed *seaweedv1.Seaweed, replication string) {
	if seaweed.Spec.Master != nil {
		seaweed.Spec.Master.DefaultReplication = ptr.To(replication)
	}
}

func setStableReplication(seaweed *seaweedv1.Seaweed, replication string) {
	if seaweed.Annotations == nil {
		seaweed.Annotations = map[string]string{}
	}
	seaweed.Annotations[stableReplicationAnnotation] = replication
}

func rolloutS3Gateway(
	ctx context.Context,
	kubeClient client.Client,
	seaweed *seaweedv1.Seaweed,
	replication string,
) error {
	deployment := &appsv1.Deployment{}
	if err := kubeClient.Get(ctx, types.NamespacedName{
		Namespace: seaweed.Namespace,
		Name:      fmt.Sprintf("%s-s3", seaweed.Name),
	}, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if deployment.Spec.Template.Annotations[s3RolloutAnnotation] == replication {
		return nil
	}

	base := deployment.DeepCopy()
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = map[string]string{}
	}
	deployment.Spec.Template.Annotations[s3RolloutAnnotation] = replication
	return kubeClient.Patch(ctx, deployment, client.MergeFrom(base))
}

func s3GatewayRolloutReady(
	ctx context.Context,
	kubeClient client.Client,
	seaweed *seaweedv1.Seaweed,
	replication string,
) (bool, error) {
	if seaweed.Spec.S3 == nil || seaweed.Spec.S3.Replicas == 0 {
		return true, nil
	}
	deployment := &appsv1.Deployment{}
	err := kubeClient.Get(ctx, types.NamespacedName{
		Namespace: seaweed.Namespace,
		Name:      fmt.Sprintf("%s-s3", seaweed.Name),
	}, deployment)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if deployment.Spec.Template.Annotations[s3RolloutAnnotation] != replication {
		return false, nil
	}
	return deploymentReady(deployment, seaweed.Spec.S3.Replicas), nil
}

func targetJobToken(replication string) string {
	return strings.ReplaceAll(replication, "0", "z")
}

func topologyJobName(seaweedName, stage, replication string) string {
	stageToken := map[string]string{
		topologyJobPreVerify:  "p",
		topologyJobReplicate:  "r",
		topologyJobPostVerify: "v",
	}[stage]
	suffix := fmt.Sprintf("-swt-%s-%s", stageToken, targetJobToken(replication))
	return common.FitDefaultInfraName(seaweedName, suffix, 63)
}

func seaweedComponentsReady(seaweed *seaweedv1.Seaweed) bool {
	components := []seaweedv1.ComponentStatus{
		seaweed.Status.Master,
		seaweed.Status.Volume,
		seaweed.Status.Filer,
		seaweed.Status.S3,
	}
	for _, component := range components {
		if component.Replicas > 0 && component.ReadyReplicas != component.Replicas {
			return false
		}
	}
	return true
}

func seaweedComponentsAndWorkloadsReady(
	ctx context.Context,
	kubeClient client.Client,
	seaweed *seaweedv1.Seaweed,
) (bool, error) {
	if !seaweedComponentsReady(seaweed) {
		return false, nil
	}

	statefulSets := make([]statefulSetTarget, 0, 3)
	if seaweed.Spec.Master != nil {
		statefulSets = append(statefulSets, statefulSetTarget{suffix: "master", replicas: seaweed.Spec.Master.Replicas})
	}
	if seaweed.Spec.Volume != nil {
		statefulSets = append(statefulSets, statefulSetTarget{suffix: "volume", replicas: seaweed.Spec.Volume.Replicas})
	}
	if seaweed.Spec.Filer != nil {
		statefulSets = append(statefulSets, statefulSetTarget{suffix: "filer", replicas: seaweed.Spec.Filer.Replicas})
	}
	for _, component := range statefulSets {
		if component.replicas == 0 {
			continue
		}
		statefulSet := &appsv1.StatefulSet{}
		err := kubeClient.Get(ctx, types.NamespacedName{
			Namespace: seaweed.Namespace,
			Name:      fmt.Sprintf("%s-%s", seaweed.Name, component.suffix),
		}, statefulSet)
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !statefulSetReady(statefulSet, component.replicas) {
			return false, nil
		}
	}

	if seaweed.Spec.S3 == nil || seaweed.Spec.S3.Replicas == 0 {
		return true, nil
	}
	deployment := &appsv1.Deployment{}
	err := kubeClient.Get(ctx, types.NamespacedName{
		Namespace: seaweed.Namespace,
		Name:      fmt.Sprintf("%s-s3", seaweed.Name),
	}, deployment)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return deploymentReady(deployment, seaweed.Spec.S3.Replicas), nil
}

func statefulSetReady(statefulSet *appsv1.StatefulSet, expected int32) bool {
	return statefulSet.Generation <= statefulSet.Status.ObservedGeneration &&
		ptr.Deref(statefulSet.Spec.Replicas, 1) == expected &&
		statefulSet.Status.Replicas == expected &&
		statefulSet.Status.CurrentReplicas == expected &&
		statefulSet.Status.UpdatedReplicas == expected &&
		statefulSet.Status.ReadyReplicas == expected &&
		statefulSet.Status.CurrentRevision == statefulSet.Status.UpdateRevision
}

func deploymentReady(deployment *appsv1.Deployment, expected int32) bool {
	return deployment.Generation <= deployment.Status.ObservedGeneration &&
		ptr.Deref(deployment.Spec.Replicas, 1) == expected &&
		deployment.Status.Replicas == expected &&
		deployment.Status.UpdatedReplicas == expected &&
		deployment.Status.AvailableReplicas == expected &&
		deployment.Status.ReadyReplicas == expected
}
