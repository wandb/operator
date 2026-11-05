package v2

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/retention"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Retention deletion admission", func() {
	for _, version := range []string{"v1", "v2"} {
		for _, propagation := range []metav1.DeletionPropagation{metav1.DeletePropagationBackground, metav1.DeletePropagationForeground, metav1.DeletePropagationOrphan} {
			It(fmt.Sprintf("guards %s %s deletion before and after migration", version, propagation), func() {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "retention-"}}
				Expect(k8sClient.Create(ctx, ns)).To(Succeed())
				w := &appsv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: ns.Name, Finalizers: []string{retention.Finalizer}}}
				w.Spec.Wandb.Hostname = "https://wandb.example.com"
				w.Spec.Wandb.Version = testPinnedVersion
				w.Spec.MySQL = map[string]appsv2.MySQLSpec{"default": {ExternalMysql: &appsv2.MysqlConnection{}}}
				w.Spec.Redis = map[string]appsv2.RedisSpec{"default": {ExternalRedis: &appsv2.RedisConnection{Host: appsv2.ValueOrSecret{Value: "redis"}, Port: appsv2.ValueOrSecret{Value: "6379"}}}}
				w.Spec.ClickHouse = map[string]appsv2.ClickHouseSpec{"default": {ExternalClickHouse: &appsv2.ClickHouseConnection{}}}
				w.Spec.ObjectStore = map[string]appsv2.ObjectStoreSpec{"default": {ExternalObjectStore: &appsv2.ObjectStoreConnection{Bucket: appsv2.ValueOrSecret{Value: "test"}}}}
				Expect(k8sClient.Create(ctx, w)).To(Succeed())
				target := &unstructured.Unstructured{}
				target.SetAPIVersion("apps.wandb.com/" + version)
				target.SetKind("WeightsAndBiases")
				target.SetName(w.Name)
				target.SetNamespace(w.Namespace)
				Expect(k8sClient.Delete(ctx, target, client.PropagationPolicy(propagation))).To(MatchError(ContainSubstring("inventory is not ready")))

				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wandb-mysql-connection", Namespace: ns.Name, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases", Name: w.Name, UID: w.UID}}}}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
				session, err := retention.Open(ctx, k8sClient, k8sClient, w)
				Expect(err).NotTo(HaveOccurred())
				Expect(session.Reconcile(ctx)).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				Expect(secret.OwnerReferences).To(BeEmpty())
				Expect(k8sClient.Delete(ctx, target, client.PropagationPolicy(propagation))).To(Succeed())
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w), w)).To(Succeed())
				Expect(w.DeletionTimestamp.IsZero()).To(BeFalse())
				w.Spec.RetentionPolicy.OnDelete = appsv2.PurgeOnDelete
				Expect(k8sClient.Update(ctx, w)).To(MatchError(ContainSubstring("cannot change after deletion")))
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w), w)).To(Succeed())
				w.Labels = map[string]string{"test-progress": "allowed"}
				Expect(k8sClient.Update(ctx, w)).To(Succeed())
				// envtest has no garbage collector; only assert API admission and metadata.
			})
		}
	}
})
