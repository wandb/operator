package opstree

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var _ = Describe("Redis purge", func() {
	var ownerLabels map[string]string
	var rule common.OnDeleteRule
	var instance types.NamespacedName
	BeforeEach(func() {
		ownerLabels = BuildWandbRedisLabels(redisWandb(false))
		rule = common.OnDeleteRule{Policy: common.Purge, Selector: labels.SelectorFromSet(ownerLabels)}
		instance = types.NamespacedName{Namespace: "wandb", Name: "redis"}
	})
	claim := func(name string, pvcLabels map[string]string) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "wandb", Labels: pvcLabels}}
	}
	statefulSet := func(name, template string) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "wandb"}, Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: template}}},
		}}
	}
	DescribeTable("purges claims before ownership labeling",
		func(sts *appsv1.StatefulSet, name string, pvcLabels map[string]string) {
			pvc := claim(name, pvcLabels)
			objects := []client.Object{pvc}
			if sts != nil {
				objects = append(objects, sts)
			}
			cl := fake.NewClientBuilder().WithScheme(redisScheme()).WithObjects(objects...).Build()
			Expect(PurgeFinalizer(context.Background(), cl, instance, rule)).To(Succeed())
			Expect(apierrors.IsNotFound(cl.Get(context.Background(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{}))).To(BeTrue())
		},
		Entry("standalone", statefulSet("redis", "redis"), "redis-redis-0", nil),
		Entry("replication", statefulSet("redis-replica", "redis-replica"), "redis-replica-redis-replica-0", nil),
		Entry("custom template and retained ordinal", statefulSet("redis-replica", "custom-data"), "custom-data-redis-replica-3", nil),
		Entry("standalone after StatefulSet deletion", nil, "redis-redis-0", nil),
		Entry("replication after StatefulSet deletion", nil, "redis-replica-redis-replica-2", nil),
		Entry("legacy template after StatefulSet deletion", nil, "redis-data-redis-0", nil),
		Entry("custom template after StatefulSet deletion", nil, "custom-data-redis-replica-3", map[string]string{"app": "redis-replica", "redis_setup_type": "replication"}),
	)
	It("preserves unrelated claims, namespaces and conflicting ownership", func() {
		otherNamespace := claim("redis-redis-0", nil)
		otherNamespace.Namespace = "another"
		foreignOwner := BuildWandbRedisLabels(redisWandb(false))
		foreignOwner["weightsandbiases.apps.wandb.com/name"] = "another"
		claims := []*corev1.PersistentVolumeClaim{
			claim("redis-redis-other-0", nil), claim("redis-redis-01", nil),
			claim("redis-redis--1", nil), claim("custom-redis-0", nil),
			claim("custom-redis-1", map[string]string{"app": "redis", "redis_setup_type": "cluster"}),
			claim("redis-redis-9", foreignOwner),
			claim("other-other-0", ownerLabels), otherNamespace,
		}
		objects := []client.Object{statefulSet("redis", "redis")}
		for _, pvc := range claims {
			objects = append(objects, pvc)
		}
		cl := fake.NewClientBuilder().WithScheme(redisScheme()).WithObjects(objects...).Build()
		Expect(PurgeFinalizer(context.Background(), cl, instance, rule)).To(Succeed())
		for _, pvc := range claims {
			Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})).To(Succeed())
		}
	})
	It("purges orphan claims through ReadState after vendor CR deletion", func() {
		pvc := claim("redis-redis-0", nil)
		custom := claim("custom-data-redis-replica-3", ownerLabels)
		cl := fake.NewClientBuilder().WithScheme(redisScheme()).WithObjects(pvc, custom).Build()
		ReadState(context.Background(), cl, instance, redisWandb(false), rule)
		for _, pvc := range []*corev1.PersistentVolumeClaim{pvc, custom} {
			Expect(apierrors.IsNotFound(cl.Get(context.Background(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{}))).To(BeTrue())
		}
	})

	It("leaves detach claims intact", func() {
		pvc := claim("redis-redis-0", ownerLabels)
		cl := fake.NewClientBuilder().WithScheme(redisScheme()).WithObjects(pvc).Build()
		rule.Policy = common.Detach
		Expect(PurgeFinalizer(context.Background(), cl, instance, rule)).To(Succeed())
		Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})).To(Succeed())
	})
	It("returns discovery errors without deleting claims", func() {
		pvc := claim("redis-redis-0", ownerLabels)
		cl := fake.NewClientBuilder().WithScheme(redisScheme()).WithObjects(pvc).WithInterceptorFuncs(interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				return fmt.Errorf("discovery unavailable")
			},
		}).Build()
		Expect(PurgeFinalizer(context.Background(), cl, instance, rule)).To(MatchError("discovery unavailable"))
		remaining := &corev1.PersistentVolumeClaimList{}
		Expect(cl.List(context.Background(), remaining)).To(Succeed())
		Expect(remaining.Items).To(HaveLen(1))
	})
})
