package keeper

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/common"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Keeper topology reconciliation", func() {
	It("creates a new ensemble at its requested size", func() {
		scheme := topologyScheme()
		desired := topologyKeeper(3, 0)
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()

		conditions := ReconcileState(context.Background(), cl, desired)

		Expect(conditions).To(ContainElement(HaveField("Message", "waiting for the ClickHouse Keeper ensemble")))
		actual := &chkv1.ClickHouseKeeperInstallation{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(desired), actual)).To(Succeed())
		Expect(expectedKeeperPodCount(actual)).To(Equal(3))
		Expect(actual.Annotations[StableReplicasAnnotation]).To(Equal("3"))
	})

	It("stages only one new replica", func() {
		scheme := topologyScheme()
		actual := topologyKeeper(1, 1)
		actual.Status = &chkv1.Status{Pods: []string{"keeper-0"}}
		desired := topologyKeeper(3, 0)
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actual, topologyReadyPod("keeper-0")).Build()

		conditions := ReconcileState(context.Background(), cl, desired)

		Expect(conditions).To(ContainElement(HaveField("Message", "adding ClickHouse Keeper replica 2 of 3")))
		updated := &chkv1.ClickHouseKeeperInstallation{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(actual), updated)).To(Succeed())
		Expect(expectedKeeperPodCount(updated)).To(Equal(2))
		Expect(updated.Annotations[StableReplicasAnnotation]).To(Equal("1"))
	})

	It("enables reconfiguration before scaling an existing ensemble", func() {
		scheme := topologyScheme()
		actual := topologyKeeper(1, 0)
		actual.Spec.Configuration.Settings = nil
		desired := topologyKeeper(3, 0)
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actual).Build()

		conditions := ReconcileState(context.Background(), cl, desired)

		Expect(conditions).To(ContainElement(HaveField("Message", "enabling safe ClickHouse Keeper scaling")))
		updated := &chkv1.ClickHouseKeeperInstallation{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(actual), updated)).To(Succeed())
		Expect(expectedKeeperPodCount(updated)).To(Equal(1))
		Expect(reconfigurationEnabled(updated)).To(BeTrue())
		Expect(updated.Annotations).NotTo(HaveKey(StableReplicasAnnotation))
	})

	It("creates a job that commits the staged member", func() {
		scheme := topologyScheme()
		actual := topologyKeeper(2, 1)
		desired := topologyKeeper(3, 0)
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(actual).Build()

		conditions := ReconcileState(context.Background(), cl, desired)

		Expect(conditions).To(ContainElement(HaveField(
			"Message",
			ContainSubstring("created ClickHouse Keeper membership job"),
		)))
		job := &batchv1.Job{}
		name := common.FitDefaultInfraName(actual.Name, "-add-1", 63)
		Expect(cl.Get(context.Background(), client.ObjectKey{Namespace: actual.Namespace, Name: name}, job)).To(Succeed())
		Expect(job.Spec.Template.Spec.Containers[0].Args[0]).To(ContainSubstring(
			"server.1=chk-wandb-clickhouse-chk-default-0-1:9444;participant;1",
		))
	})

	It("records the staged replica after membership and pods are ready", func() {
		scheme := topologyScheme()
		actual := topologyKeeper(2, 1)
		actual.Status = &chkv1.Status{Pods: []string{"keeper-0", "keeper-1"}}
		desired := topologyKeeper(3, 0)
		job, err := membershipJob(desired, common.FitDefaultInfraName(actual.Name, "-add-1", 63), 1)
		Expect(err).NotTo(HaveOccurred())
		job.Status.Conditions = []batchv1.JobCondition{{
			Type:   batchv1.JobComplete,
			Status: corev1.ConditionTrue,
		}}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			actual,
			job,
			topologyReadyPod("keeper-0"),
			topologyReadyPod("keeper-1"),
		).Build()

		conditions := ReconcileState(context.Background(), cl, desired)

		Expect(conditions).To(ContainElement(HaveField("Message", "recording 2 stable ClickHouse Keeper replicas")))
		updated := &chkv1.ClickHouseKeeperInstallation{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(actual), updated)).To(Succeed())
		Expect(expectedKeeperPodCount(updated)).To(Equal(2))
		Expect(updated.Annotations[StableReplicasAnnotation]).To(Equal("2"))
	})

	It("rejects a missing cluster layout", func() {
		desired := topologyKeeper(3, 0)
		desired.Spec.Configuration.Clusters = nil

		conditions := ReconcileState(context.Background(), fake.NewClientBuilder().WithScheme(topologyScheme()).Build(), desired)

		Expect(conditions).To(ContainElement(SatisfyAll(
			HaveField("Reason", "InvalidKeeperConfiguration"),
			HaveField("Message", "ClickHouse Keeper cluster layout is required"),
		)))
	})

	It("rejects a membership job template without a container", func() {
		desired := topologyKeeper(3, 0)
		desired.Spec.Templates.PodTemplates[0].Spec.Containers = nil

		job, err := membershipJob(desired, "membership", 1)

		Expect(job).To(BeNil())
		Expect(err).To(MatchError("ClickHouse Keeper pod template requires a container"))
	})
})

func topologyScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(batchv1.AddToScheme(scheme)).To(Succeed())
	Expect(chiv1.AddToScheme(scheme)).To(Succeed())
	Expect(chkv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func topologyKeeper(replicas, stable int) *chkv1.ClickHouseKeeperInstallation {
	settings := chiv1.NewSettings()
	settings.Set("keeper_server/enable_reconfiguration", chiv1.NewSettingScalar("true"))
	annotations := map[string]string{}
	if stable > 0 {
		annotations[StableReplicasAnnotation] = fmt.Sprint(stable)
	}
	return &chkv1.ClickHouseKeeperInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "wandb-clickhouse-chk",
			Namespace:   "wandb",
			Labels:      map[string]string{"app": "keeper"},
			Annotations: annotations,
		},
		Spec: chkv1.ChkSpec{
			Configuration: &chkv1.Configuration{
				Settings: settings,
				Clusters: []*chkv1.Cluster{{
					Name:   ClusterName,
					Layout: &chkv1.ChkClusterLayout{ReplicasCount: replicas},
				}},
			},
			Templates: &chiv1.Templates{
				PodTemplates: []chiv1.PodTemplate{{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  keeperContainerName,
							Image: defaultKeeperImage,
						}},
					},
				}},
			},
		},
	}
}

func topologyReadyPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "wandb"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}
}
