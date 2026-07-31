package altinity

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	chkv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse-keeper.altinity.com/v1"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ClickHouse staged writes", func() {
	keeperWithReplicas := func(replicas int) *chkv1.ClickHouseKeeperInstallation {
		settings := chiv1.NewSettings()
		settings.Set("keeper_server/enable_reconfiguration", chiv1.NewSettingScalar("true"))
		return &chkv1.ClickHouseKeeperInstallation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wandb-clickhouse-chk",
				Namespace: "wandb",
				Annotations: map[string]string{
					keeper.StableReplicasAnnotation: fmt.Sprint(replicas),
				},
			},
			Spec: chkv1.ChkSpec{
				Configuration: &chkv1.Configuration{
					Settings: settings,
					Clusters: []*chkv1.Cluster{{
						Layout: &chkv1.ChkClusterLayout{ReplicasCount: replicas},
					}},
				},
			},
		}
	}
	clickHouseWithReplicas := func(replicas int) *chiv1.ClickHouseInstallation {
		return &chiv1.ClickHouseInstallation{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb-clickhouse", Namespace: "wandb"},
			Spec: chiv1.ChiSpec{
				Configuration: &chiv1.Configuration{
					Clusters: []*chiv1.Cluster{{
						Layout: &chiv1.ChiClusterLayout{ShardsCount: 1, ReplicasCount: replicas},
					}},
				},
			},
		}
	}
	readyPod := func(name string, ready bool) *corev1.Pod {
		status := corev1.ConditionFalse
		if ready {
			status = corev1.ConditionTrue
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "wandb"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: status,
				}},
			},
		}
	}

	It("does not update ClickHouse in the reconcile that changes Keeper topology", func() {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())
		Expect(chkv1.AddToScheme(scheme)).To(Succeed())
		Expect(chiv1.AddToScheme(scheme)).To(Succeed())
		actualKeeper := keeperWithReplicas(1)
		actualKeeper.Status = &chkv1.Status{Pods: []string{"keeper-0"}}
		desiredKeeper := keeperWithReplicas(3)
		desiredKeeper.Annotations = nil
		desiredClickHouse := clickHouseWithReplicas(3)
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			actualKeeper,
			readyPod("keeper-0", true),
		).Build()

		conditions := WriteState(
			context.Background(),
			cl,
			types.NamespacedName{Name: "wandb-clickhouse", Namespace: "wandb"},
			nil,
			desiredKeeper,
			desiredClickHouse,
		)

		Expect(conditions).To(ContainElement(HaveField("Message", "adding ClickHouse Keeper replica 2 of 3")))
		updatedKeeper := &chkv1.ClickHouseKeeperInstallation{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(desiredKeeper), updatedKeeper)).To(Succeed())
		Expect(updatedKeeper.Spec.Configuration.Clusters[0].Layout.ReplicasCount).To(Equal(2))
		Expect(updatedKeeper.Annotations[keeper.StableReplicasAnnotation]).To(Equal("1"))
		clickHouse := &chiv1.ClickHouseInstallation{}
		err := cl.Get(context.Background(), client.ObjectKeyFromObject(desiredClickHouse), clickHouse)
		Expect(err).To(HaveOccurred())
	})

	It("does not scale ClickHouse while its current topology is degraded", func() {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(batchv1.AddToScheme(scheme)).To(Succeed())
		Expect(chkv1.AddToScheme(scheme)).To(Succeed())
		Expect(chiv1.AddToScheme(scheme)).To(Succeed())

		actualKeeper := keeperWithReplicas(3)
		actualKeeper.Status = &chkv1.Status{Pods: []string{"keeper-0", "keeper-1", "keeper-2"}}
		actualClickHouse := clickHouseWithReplicas(1)
		actualClickHouse.Status = &chiv1.Status{Pods: []string{"clickhouse-0"}}
		desiredClickHouse := clickHouseWithReplicas(3)
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			actualKeeper,
			actualClickHouse,
			readyPod("keeper-0", true),
			readyPod("keeper-1", true),
			readyPod("keeper-2", true),
			readyPod("clickhouse-0", false),
		).Build()

		conditions := WriteState(
			context.Background(),
			cl,
			types.NamespacedName{Name: "wandb-clickhouse", Namespace: "wandb"},
			nil,
			keeperWithReplicas(3),
			desiredClickHouse,
		)

		Expect(conditions).To(ContainElement(HaveField(
			"Message",
			"waiting for the current ClickHouse topology before applying the desired topology",
		)))
		updatedClickHouse := &chiv1.ClickHouseInstallation{}
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(actualClickHouse), updatedClickHouse)).To(Succeed())
		Expect(updatedClickHouse.Spec.Configuration.Clusters[0].Layout.ReplicasCount).To(Equal(1))
	})
})
