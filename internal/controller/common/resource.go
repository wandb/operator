package common

import (
	"context"
	"reflect"

	"github.com/wandb/operator/internal/logx"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	log := logx.GetSlog(ctx)

	err := c.Get(ctx, namespacedName, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug(
				"NotFound", "type", resourceTypeName,
				"namespace", namespacedName.Namespace, "name", namespacedName.Name,
			)
			return false, nil
		}
		log.Error(
			"GetResourceError", logx.ErrAttr(err), "type", resourceTypeName,
			"namespace", namespacedName.Namespace, "name", namespacedName.Name,
		)
		return false, err
	}
	log.Debug(
		"Found", "type", resourceTypeName,
		"namespace", namespacedName.Namespace, "name", namespacedName.Name,
	)
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
	log := logx.GetSlog(ctx)

	var err error
	var action CrudAction
	desiredExists := !IsNil(desired) && desired.GetName() != ""
	actualExists := !IsNil(actual) && actual.GetName() != ""

	if actualExists && desiredExists {
		// Avoid no-op updates while ignoring runtime-managed fields such as status and
		// resource metadata that differ between desired and actual.
		if desiredMatchesActual(desired, actual) {
			return NoAction, nil
		}

		action = UpdateAction
		desired.SetResourceVersion(actual.GetResourceVersion())
		err = c.Update(ctx, desired)
	}
	if !actualExists && desiredExists {
		action = CreateAction
		err = c.Create(ctx, desired)
	}
	if actualExists && !desiredExists {
		action = DeleteAction
		err = c.Delete(ctx, actual)
	}
	if action != NoAction {
		if desiredExists {
			log.Info(string(action), "namespace", desired.GetNamespace(), "name", desired.GetName())
		} else if actualExists {
			log.Info(string(action), "namespace", actual.GetNamespace(), "name", actual.GetName())
		}
	}
	if err != nil {
		log.Error("error on crud resource", logx.ErrAttr(err), "action", action)
	}
	return action, err
}

func desiredMatchesActual(desired client.Object, actual client.Object) bool {
	desiredMap, desiredErr := toComparableUnstructured(desired)
	actualMap, actualErr := toComparableUnstructured(actual)
	if desiredErr == nil && actualErr == nil {
		return equality.Semantic.DeepDerivative(desiredMap, actualMap)
	}

	// Fallback for unexpected conversion failures.
	return equality.Semantic.DeepDerivative(desired, actual)
}

func toComparableUnstructured(obj client.Object) (map[string]any, error) {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	delete(m, "status")

	metadata, ok := m["metadata"].(map[string]any)
	if ok {
		delete(metadata, "creationTimestamp")
		delete(metadata, "resourceVersion")
		delete(metadata, "generation")
		delete(metadata, "uid")
		delete(metadata, "managedFields")
		delete(metadata, "selfLink")
	}

	return m, nil
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
