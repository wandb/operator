package common

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetResource is a generic function that retrieves a Kubernetes resource.
// Returns (false, nil) if the resource is not found, or (true, nil) if found.
// Returns (false, error) for any other error.
func GetResource[T client.Object](
	ctx context.Context,
	c client.Client,
	namespacedName types.NamespacedName,
	resourceTypeName string,
	obj T,
) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info(fmt.Sprintf("get %s.%s", namespacedName.Namespace, namespacedName.Name))

	err := c.Get(ctx, namespacedName, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(fmt.Sprintf("not found %s.%s.%s",
				resourceTypeName, namespacedName.Namespace, namespacedName.Name),
			)
			return false, nil
		}
		log.Error(err, "error getting resource", "type", resourceTypeName)
		return false, err
	}
	return true, nil
}

type CrudAction string

const (
	NoAction     = ""
	CreateAction = "Create"
	UpdateAction = "Update"
	DeleteAction = "Delete"
)

// CrudResource is a generic function that gets a resource, and creates it if not found, or updates it if it exists.
// The getter function should return (nil, nil) if the resource is not found.
func CrudResource[T client.Object](ctx context.Context, c client.Client, desired T, actual T) (CrudAction, error) {
	log := ctrl.LoggerFrom(ctx)

	var err error
	var action CrudAction
	desiredExists := !IsNil(desired) && desired.GetName() != ""
	actualExists := !IsNil(actual) && actual.GetName() != ""

	if actualExists && desiredExists {
		action = UpdateAction
		log.Info(fmt.Sprintf("update %s.%s", desired.GetNamespace(), desired.GetName()))
		desired.SetResourceVersion(actual.GetResourceVersion())
		err = c.Update(ctx, desired)
	}
	if !actualExists && desiredExists {
		action = CreateAction
		log.Info(fmt.Sprintf("create %s.%s", desired.GetNamespace(), desired.GetName()))
		err = c.Create(ctx, desired)
	}
	if actualExists && !desiredExists {
		action = DeleteAction
		log.Info(fmt.Sprintf("delete %s.%s", actual.GetNamespace(), actual.GetName()))
		err = c.Delete(ctx, actual)
	}
	if err != nil {
		log.Error(err, "error on crud resource", "action", action)
	}
	return action, err
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
