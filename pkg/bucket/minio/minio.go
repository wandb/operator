package minio

import (
	"context"

	"github.com/wandb/operator/pkg/utils/kubeclient"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Namespace = "wandb"

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
	if err := kubeclient.UpsertNamespace(ctx, c, Namespace); err != nil {
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
