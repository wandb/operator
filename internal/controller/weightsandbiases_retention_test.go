package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/retention"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRetentionWatchUsesInstallationUID(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, apiv2.AddToScheme(scheme))
	w := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "app", UID: "current"}}
	r := &WeightsAndBiasesReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(w).Build()}
	obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "other-namespace", Labels: map[string]string{common.WandbNameLabel: w.Name, common.WandbNamespaceLabel: w.Namespace, retention.UIDLabel: string(w.UID)}}}
	requests := r.mapRetainedResourceToWandb(context.Background(), obj)
	require.Len(t, requests, 1)
	require.Equal(t, w.Namespace, requests[0].Namespace)
	obj.Labels[retention.UIDLabel] = "previous-installation"
	require.Empty(t, r.mapRetainedResourceToWandb(context.Background(), obj))
	delete(obj.Labels, retention.UIDLabel)
	require.Empty(t, r.mapRetainedResourceToWandb(context.Background(), obj))
}
