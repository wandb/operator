package reconciler

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/managed/kafka/bufstream"
	"github.com/wandb/operator/internal/observability/telemetry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestReconcileKafkaRetentionOnWandbDeletion(t *testing.T) {
	for _, tc := range []struct {
		name                string
		policy              apiv2.OnDeletePolicy
		override            *apiv2.RetentionPolicy
		applicationsMissing bool
		failDelete          bool
		wantPurged          bool
	}{
		{name: "purge with applications present", policy: apiv2.PurgeOnDelete, wantPurged: true},
		{name: "purge with applications missing", policy: apiv2.PurgeOnDelete, applicationsMissing: true, wantPurged: true},
		{name: "detach", policy: apiv2.DetachOnDelete},
		{name: "detach override", policy: apiv2.PurgeOnDelete, override: &apiv2.RetentionPolicy{OnDelete: apiv2.DetachOnDelete}},
		{name: "purge override", policy: apiv2.DetachOnDelete, override: &apiv2.RetentionPolicy{OnDelete: apiv2.PurgeOnDelete}, wantPurged: true},
		{name: "purge retries after delete failure", policy: apiv2.PurgeOnDelete, failDelete: true, wantPurged: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			require.NoError(t, apiv2.AddToScheme(scheme))
			now := metav1.Now()
			wandb := &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name: "wandb", Namespace: "default", UID: "wandb-uid", DeletionTimestamp: &now,
					Finalizers: []string{CleanupFinalizer, "example.com/hold"},
				},
				Spec: apiv2.WeightsAndBiasesSpec{
					RetentionPolicy: apiv2.RetentionPolicy{OnDelete: tc.policy},
					Kafka: apiv2.KafkaSpec{ManagedKafka: &apiv2.ManagedKafkaSpec{
						Name: "wandb-kafka", Namespace: "default",
						ManagedInfraSpec: apiv2.ManagedInfraSpec{RetentionPolicy: tc.override},
					}},
				},
			}
			claim := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
				Name: "data-wandb-kafka-etcd-0", Namespace: wandb.Namespace,
				Labels: bufstream.BuildWandbKafkaLabels(wandb),
			}}
			legacyClaim := claim.DeepCopy()
			legacyClaim.Name = "data-wandb-kafka-etcd-1"
			legacyClaim.Labels = map[string]string{
				"app.kubernetes.io/name": "wandb-kafka-etcd", "app.kubernetes.io/instance": wandb.Namespace,
			}
			objects := []client.Object{wandb, claim, legacyClaim}
			if !tc.applicationsMissing {
				for _, name := range []string{"wandb-kafka", "wandb-kafka-etcd"} {
					objects = append(objects, &apiv2.Application{ObjectMeta: metav1.ObjectMeta{
						Name: name, Namespace: wandb.Namespace,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(wandb, apiv2.GroupVersion.WithKind("WeightsAndBiases"))},
					}})
				}
			}
			deleteErr := errors.New("PVC deletion failed")
			failDelete := tc.failDelete
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if _, ok := obj.(*corev1.PersistentVolumeClaim); ok {
						stored := &apiv2.WeightsAndBiases{}
						require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(wandb), stored))
						require.Contains(t, stored.Finalizers, CleanupFinalizer, "purge must run before releasing W&B deletion")
						if failDelete {
							return deleteErr
						}
					}
					return cl.Delete(ctx, obj, opts...)
				},
			}).Build()
			recorder := record.NewFakeRecorder(10)
			_, err := Reconcile(ctx, cl, recorder, wandb, telemetry.TelemetryRuntimeConfig{})
			if tc.failDelete {
				require.ErrorIs(t, err, deleteErr)
				require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(wandb), wandb))
				require.Contains(t, wandb.Finalizers, CleanupFinalizer)
				failDelete = false
				_, err = Reconcile(ctx, cl, recorder, wandb, telemetry.TelemetryRuntimeConfig{})
			}
			require.NoError(t, err)
			require.NoError(t, cl.Get(ctx, client.ObjectKeyFromObject(wandb), wandb))
			require.NotContains(t, wandb.Finalizers, CleanupFinalizer)
			for _, pvc := range []*corev1.PersistentVolumeClaim{claim, legacyClaim} {
				err := cl.Get(ctx, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})
				if tc.wantPurged {
					require.True(t, apierrors.IsNotFound(err), "claim not purged during W&B deletion: %v", err)
				} else {
					require.NoError(t, err)
				}
			}
			if !tc.wantPurged && !tc.applicationsMissing {
				for _, name := range []string{"wandb-kafka", "wandb-kafka-etcd"} {
					app := &apiv2.Application{}
					require.NoError(t, cl.Get(ctx, client.ObjectKey{Name: name, Namespace: wandb.Namespace}, app))
					require.Empty(t, app.OwnerReferences)
				}
			}
		})
	}
}
