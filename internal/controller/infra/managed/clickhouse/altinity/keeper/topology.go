package keeper

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wandb/operator/internal/controller/common"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const StableReplicasAnnotation = "operator.wandb.com/keeper-stable-replicas"

// ReconcileState grows an existing Keeper ensemble one committed Raft member
// at a time. A brand-new ensemble can safely start at its requested size.
func ReconcileState(
	ctx context.Context,
	cl client.Client,
	desired *chkv1.ClickHouseKeeperInstallation,
) []metav1.Condition {
	if desired.Spec.Configuration == nil ||
		len(desired.Spec.Configuration.Clusters) == 0 ||
		desired.Spec.Configuration.Clusters[0] == nil ||
		desired.Spec.Configuration.Clusters[0].Layout == nil {
		return topologyCondition(metav1.ConditionFalse, "InvalidKeeperConfiguration", "ClickHouse Keeper cluster layout is required")
	}
	nsName := client.ObjectKeyFromObject(desired)
	actual := &chkv1.ClickHouseKeeperInstallation{}
	found, err := common.GetResource(ctx, cl, nsName, ResourceTypeName, actual)
	if err != nil {
		return topologyCondition(metav1.ConditionUnknown, common.ApiErrorReason, err.Error())
	}

	target := expectedKeeperPodCount(desired)
	if !found {
		setStableReplicas(desired, target)
		return append(
			WriteState(ctx, cl, nsName, desired),
			topologyCondition(metav1.ConditionFalse, common.PendingCreateReason, "waiting for the ClickHouse Keeper ensemble")...,
		)
	}
	current := expectedKeeperPodCount(actual)
	if target < current {
		return topologyCondition(
			metav1.ConditionFalse,
			"ReplicaReductionUnsupported",
			fmt.Sprintf("refusing to reduce ClickHouse Keeper replicas from %d to %d without a member removal workflow", current, target),
		)
	}

	stable, adopted := stableReplicas(actual)
	if !reconfigurationEnabled(actual) || !adopted {
		staged := desired.DeepCopy()
		setKeeperReplicas(staged, current)
		if !keeperOwnedEqual(actual, staged) {
			return append(
				WriteState(ctx, cl, nsName, staged),
				topologyCondition(metav1.ConditionFalse, common.PendingCreateReason, "enabling safe ClickHouse Keeper scaling")...,
			)
		}
		ready := ReadState(ctx, cl, nsName)
		if !apimeta.IsStatusConditionTrue(ready, KeeperReportedReadyType) {
			return ready
		}
		setStableReplicas(staged, current)
		return append(
			WriteState(ctx, cl, nsName, staged),
			topologyCondition(metav1.ConditionFalse, common.PendingCreateReason, "recording the current ClickHouse Keeper topology")...,
		)
	}

	if current == stable {
		ready := ReadState(ctx, cl, nsName)
		if !apimeta.IsStatusConditionTrue(ready, KeeperReportedReadyType) {
			return ready
		}
		setStableReplicas(desired, stable)
		if current == target {
			if keeperOwnedEqual(actual, desired) {
				return ready
			}
			return append(
				WriteState(ctx, cl, nsName, desired),
				topologyCondition(metav1.ConditionFalse, common.PendingCreateReason, "waiting for the desired ClickHouse Keeper configuration")...,
			)
		}

		staged := desired.DeepCopy()
		setKeeperReplicas(staged, current+1)
		setStableReplicas(staged, stable)
		return append(
			WriteState(ctx, cl, nsName, staged),
			topologyCondition(
				metav1.ConditionFalse,
				common.PendingCreateReason,
				fmt.Sprintf("adding ClickHouse Keeper replica %d of %d", current+1, target),
			)...,
		)
	}

	if current != stable+1 {
		return topologyCondition(
			metav1.ConditionFalse,
			"UnexpectedKeeperTopology",
			fmt.Sprintf("ClickHouse Keeper has %d configured replicas but %d stable replicas", current, stable),
		)
	}

	jobConditions, complete := reconcileMembershipJob(ctx, cl, desired, stable)
	if !complete {
		return jobConditions
	}
	ready := ReadState(ctx, cl, nsName)
	if !apimeta.IsStatusConditionTrue(ready, KeeperReportedReadyType) {
		return ready
	}

	staged := desired.DeepCopy()
	setKeeperReplicas(staged, current)
	setStableReplicas(staged, current)
	return append(
		WriteState(ctx, cl, nsName, staged),
		topologyCondition(
			metav1.ConditionFalse,
			common.PendingCreateReason,
			fmt.Sprintf("recording %d stable ClickHouse Keeper replicas", current),
		)...,
	)
}

func reconcileMembershipJob(
	ctx context.Context,
	cl client.Client,
	desired *chkv1.ClickHouseKeeperInstallation,
	replica int,
) ([]metav1.Condition, bool) {
	name := common.FitDefaultInfraName(desired.Name, fmt.Sprintf("-add-%d", replica), 63)
	job := &batchv1.Job{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: desired.Namespace, Name: name}, job)
	if err != nil && !apierrors.IsNotFound(err) {
		return topologyCondition(metav1.ConditionUnknown, common.ApiErrorReason, err.Error()), false
	}
	if apierrors.IsNotFound(err) {
		job, err = membershipJob(desired, name, replica)
		if err != nil {
			return topologyCondition(metav1.ConditionFalse, "InvalidKeeperConfiguration", err.Error()), false
		}
		if err := cl.Create(ctx, job); err != nil {
			return topologyCondition(metav1.ConditionUnknown, common.ApiErrorReason, err.Error()), false
		}
		return topologyCondition(
			metav1.ConditionFalse,
			common.PendingCreateReason,
			fmt.Sprintf("created ClickHouse Keeper membership job %s", name),
		), false
	}

	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			message := condition.Message
			if message == "" {
				message = fmt.Sprintf("ClickHouse Keeper membership job %s failed", name)
			}
			return topologyCondition(metav1.ConditionFalse, "KeeperMembershipFailed", message), false
		}
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return nil, true
		}
	}
	return topologyCondition(
		metav1.ConditionFalse,
		common.PendingCreateReason,
		fmt.Sprintf("waiting for ClickHouse Keeper membership job %s", name),
	), false
}

func membershipJob(
	desired *chkv1.ClickHouseKeeperInstallation,
	name string,
	replica int,
) (*batchv1.Job, error) {
	if desired == nil || desired.Spec.Templates == nil || len(desired.Spec.Templates.PodTemplates) == 0 {
		return nil, fmt.Errorf("ClickHouse Keeper pod template is required")
	}
	podSpec := desired.Spec.Templates.PodTemplates[0].Spec
	if len(podSpec.Containers) == 0 {
		return nil, fmt.Errorf("ClickHouse Keeper pod template requires a container")
	}
	memberHost := keeperHostServiceName(desired.Name, replica)
	clientHost := "keeper-" + desired.Name
	member := fmt.Sprintf("server.%d=%s:9444;participant;1", replica, memberHost)
	script := fmt.Sprintf(`set -u
member=%q
for attempt in $(seq 1 150); do
  config=$(/usr/bin/clickhouse-keeper keeper-client -h %q -p %d -q 'get "/keeper/config"' 2>/dev/null || true)
  if printf '%%s\n' "$config" | grep -Fqx "$member"; then
    exit 0
  fi
  /usr/bin/clickhouse-keeper keeper-client -h %q -p %d -q "reconfig add \"$member\"" >/dev/null 2>&1 || true
  sleep 2
done
exit 1
`, member, clientHost, KeeperClientPort, clientHost, KeeperClientPort)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       desired.Namespace,
			Labels:          desired.Labels,
			OwnerReferences: desired.OwnerReferences,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          ptr.To[int32](0),
			ActiveDeadlineSeconds: ptr.To[int64](360),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: desired.Labels},
				Spec: corev1.PodSpec{
					Affinity:         podSpec.Affinity,
					Tolerations:      podSpec.Tolerations,
					ImagePullSecrets: podSpec.ImagePullSecrets,
					RestartPolicy:    corev1.RestartPolicyNever,
					SecurityContext:  podSpec.SecurityContext,
					Containers: []corev1.Container{{
						Name:            "membership",
						Image:           podSpec.Containers[0].Image,
						ImagePullPolicy: podSpec.Containers[0].ImagePullPolicy,
						SecurityContext: podSpec.Containers[0].SecurityContext,
						Command:         []string{"/bin/sh", "-c"},
						Args:            []string{script},
					}},
				},
			},
		},
	}, nil
}

func topologyCondition(status metav1.ConditionStatus, reason, message string) []metav1.Condition {
	return []metav1.Condition{{
		Type:    KeeperReportedReadyType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}}
}

func keeperHostServiceName(installationName string, replica int) string {
	return fmt.Sprintf("chk-%s-%s-0-%d", installationName, ClusterName, replica)
}

func stableReplicas(chk *chkv1.ClickHouseKeeperInstallation) (int, bool) {
	value, ok := chk.GetAnnotations()[StableReplicasAnnotation]
	if !ok {
		return 0, false
	}
	replicas, err := strconv.Atoi(value)
	return replicas, err == nil && replicas > 0
}

func setStableReplicas(chk *chkv1.ClickHouseKeeperInstallation, replicas int) {
	annotations := chk.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[StableReplicasAnnotation] = strconv.Itoa(replicas)
	chk.SetAnnotations(annotations)
}

func setKeeperReplicas(chk *chkv1.ClickHouseKeeperInstallation, replicas int) {
	chk.Spec.Configuration.Clusters[0].Layout.ReplicasCount = replicas
}

func reconfigurationEnabled(chk *chkv1.ClickHouseKeeperInstallation) bool {
	if chk.Spec.Configuration == nil || chk.Spec.Configuration.Settings == nil {
		return false
	}
	return chk.Spec.Configuration.Settings.Get("keeper_server/enable_reconfiguration").String() == "true"
}
