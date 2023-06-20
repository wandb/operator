package kubeclient

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object) error {
	if err := c.Create(ctx, obj); err != nil {
		if errors.IsAlreadyExists(err) {
			return c.Update(ctx, obj)
		}
		return err
	}
	return nil
}

func DeleteIfExists(ctx context.Context, c client.Client, obj client.Object) error {
	if err := c.Delete(ctx, obj); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func GetOrCreate(ctx context.Context, c client.Client, obj client.Object) error {
	objKey := client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	err := c.Get(ctx, objKey, obj)

	if err != nil {
		if errors.IsNotFound(err) {
			err := c.Create(ctx, obj)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}
