package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/redis/opstree"
	"github.com/wandb/operator/pkg/wandb/manifest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Redis rendered security API", func() {
	It("strictly admits standalone, replication and Sentinel renderer output", func() {
		owner := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{
			Name: "redis-security-owner", Namespace: "default", UID: "redis-security-owner",
		}}
		spec := &apiv2.ManagedRedisSpec{
			Name: "redis-security", Namespace: "default", StorageSize: "1Gi",
		}
		standalone, err := opstree.ToRedisStandaloneVendorSpec(ctx, owner, spec, scheme.Scheme, manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		spec.Sentinel.Enabled = true
		replication, err := opstree.ToRedisReplicationVendorSpec(ctx, owner, spec, scheme.Scheme, manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())
		sentinel, err := opstree.ToRedisSentinelVendorSpec(ctx, owner, spec, scheme.Scheme, manifest.Manifest{})
		Expect(err).NotTo(HaveOccurred())

		for _, desired := range []client.Object{standalone, replication, sentinel} {
			Expect(k8sClient.Create(ctx, desired, &client.CreateOptions{FieldValidation: metav1.FieldValidationStrict})).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, desired)).To(Succeed()) })
		}
		actual := sentinel.DeepCopy()
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sentinel), actual)).To(Succeed())
		Expect(actual.Spec.VolumeMount).To(Equal(sentinel.Spec.VolumeMount))
		Expect(actual.Spec.SecurityContext.ReadOnlyRootFilesystem).To(HaveValue(BeTrue()))
	})
})
