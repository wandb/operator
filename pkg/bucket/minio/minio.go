package minio

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func upsertNamespace(ctx context.Context, c client.Client, namespace string) error {
	return c.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "example-namespace",
		},
	})
}

func Reconcile(ctx context.Context, c client.Client) error {
	if err := upsertNamespace(ctx, c, "wandb"); err != nil {
		return err
	}

	return nil
}
