package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/redis/opstree"
	"github.com/wandb/operator/pkg/wandb/manifest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	It("purges unlabeled claims with live and deleted StatefulSets through the API", func() {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "redis-purge-security"}}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, namespace)).To(Succeed()) })
		nsn := types.NamespacedName{Namespace: namespace.Name, Name: "redis"}
		claim := func(name string, labels map[string]string) *corev1.PersistentVolumeClaim {
			return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsn.Namespace, Labels: labels}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}
		}
		statefulSet := func(name, template string) *appsv1.StatefulSet {
			podLabels := map[string]string{"app": name}
			return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsn.Namespace}, Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: podLabels}, ServiceName: name,
				Template:             corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: podLabels}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "redis", Image: "redis:7"}}}},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{*claim(template, nil)},
			}}
		}
		live := statefulSet("redis", "custom-data")
		gone := statefulSet("redis-replica", "archive-data")
		Expect(k8sClient.Create(ctx, live)).To(Succeed())
		Expect(k8sClient.Create(ctx, gone)).To(Succeed())
		pvcs := []*corev1.PersistentVolumeClaim{
			claim("custom-data-redis-0", nil),
			claim("archive-data-redis-replica-3", map[string]string{"app": "redis-replica", "redis_setup_type": "replication"}),
			claim("redis-data-redis-4", nil),
		}
		unrelated := claim("custom-data-redis-other-0", nil)
		for _, pvc := range append(pvcs, unrelated) {
			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())
		}
		Expect(k8sClient.Delete(ctx, gone)).To(Succeed())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(gone), &appsv1.StatefulSet{}))).To(BeTrue())
		owner := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: nsn.Namespace}}
		rule := common.ToOnDeleteRule(owner, apiv2.RetentionPolicy{OnDelete: apiv2.PurgeOnDelete}, opstree.RedisModuleName)
		Expect(opstree.PurgeFinalizer(ctx, k8sClient, nsn, rule)).To(Succeed())
		for _, pvc := range pvcs {
			actual := &corev1.PersistentVolumeClaim{}
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pvc), actual)
			if !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
				// envtest has admission but no PVC protection controller.
				Expect(actual.DeletionTimestamp.IsZero()).To(BeFalse())
				Expect(actual.Finalizers).To(ContainElement("kubernetes.io/pvc-protection"))
			}
		}
		actual := &corev1.PersistentVolumeClaim{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(unrelated), actual)).To(Succeed())
		Expect(actual.DeletionTimestamp.IsZero()).To(BeTrue())
	})
})
