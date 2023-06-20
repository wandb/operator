package minio

import (
	"context"

	"github.com/wandb/operator/pkg/utils/kubeclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func upsertNamespace(ctx context.Context, c client.Client, namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if err := c.Create(ctx, ns); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func ReconcileDelete(ctx context.Context, c client.Client) error {
	pvc := PersistanceVolumeClaim()
	kubeclient.DeleteIfExists(ctx, c, pvc)

	deployment := Deployment()
	kubeclient.DeleteIfExists(ctx, c, deployment)

	service := Service()
	kubeclient.DeleteIfExists(ctx, c, service)

	job := CreateBucketJob(service.GetName())
	kubeclient.DeleteIfExists(ctx, c, job)

	return nil
}

func ReconcileCreate(ctx context.Context, c client.Client) error {
	if err := upsertNamespace(ctx, c, "wandb"); err != nil {
		return err
	}

	// getting cannot change certain fields on update
	pvc := PersistanceVolumeClaim()
	kubeclient.CreateOrUpdate(ctx, c, pvc)

	deployment := Deployment()
	if err := kubeclient.CreateOrUpdate(ctx, c, deployment); err != nil {
		return err
	}

	// getting port already exist error on create
	service := Service()
	kubeclient.CreateOrUpdate(ctx, c, service)

	// getting cannot change certain fields on update
	job := CreateBucketJob(service.GetName())
	kubeclient.CreateOrUpdate(ctx, c, job)

	return nil
}

func GetConnectionString(ctx context.Context, c client.Client) (string, error) {
	endpoint, err := GetHost(ctx, c)
	if err != nil {
		return "", err
	}
	cs := "s3://" + RootUser + ":" + RootPassword + "@" + endpoint + ":31300/" + BucketName
	return cs, nil
}
