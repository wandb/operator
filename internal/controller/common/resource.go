package common

import (
	"context"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetResource is a generic function that retrieves a Kubernetes resource.
// Returns (nil, nil) if the resource is not found, or (resource, nil) if found.
// Returns (nil, error) for any other error.
func GetResource[T client.Object](
	ctx context.Context,
	c client.Client,
	namespacedName types.NamespacedName,
	resourceTypeName string,
	obj T,
) error {
	log := ctrl.LoggerFrom(ctx)

	err := c.Get(ctx, namespacedName, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "error getting resource", "type", resourceTypeName)
		return err
	}
	return nil
}

// CrudResource is a generic function that gets a resource, and creates it if not found, or updates it if it exists.
// The getter function should return (nil, nil) if the resource is not found.
func CrudResource[T client.Object](
	ctx context.Context,
	c client.Client,
	desired T,
	actual T,
) error {
	var err error
	desiredExists := !IsNil(desired) && desired.GetName() != ""
	actualExists := !IsNil(actual) && actual.GetName() != ""

	if actualExists && desiredExists {
		err = c.Update(ctx, desired)
	}
	if !actualExists && desiredExists {
		err = c.Create(ctx, desired)
	}
	if actualExists && !desiredExists {
		err = c.Delete(ctx, actual)
	}
	return err
}

// IsNil checks if the generic value v is a pointer and if that pointer is nil.
// It returns false if true is a non-pointer type, or if it's a non-nil pointer.
func IsNil[T any](v T) bool {
	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Pointer {
		return false
	}

	// Since we've already checked that the Kind is Pointer, we can safely call IsNil().
	return val.IsNil()
}
