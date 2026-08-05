package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"maps"
	"reflect"

	"github.com/wandb/operator/internal/logx"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	NoAction        = ""
	CreateAction    = "Create"
	UpdateAction    = "Update"
	UnchangedAction = "Unchanged"
	DeleteAction    = "Delete"
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
		if resourceManagedFieldsEqual(desired, actual) {
			action = UnchangedAction
		} else {
			action = UpdateAction
			prepareResourceUpdate(desired, actual)
			err = c.Update(ctx, desired)
		}
	}
	if !actualExists && desiredExists {
		action = CreateAction
		err = c.Create(ctx, desired)
	}
	if actualExists && !desiredExists {
		action = DeleteAction
		err = c.Delete(ctx, actual)
	}
	if action != NoAction && action != UnchangedAction {
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

func resourceManagedFieldsEqual(desired, actual client.Object) bool {
	if !mapContains(actual.GetLabels(), desired.GetLabels()) ||
		!mapContains(actual.GetAnnotations(), desired.GetAnnotations()) ||
		!ownerReferencesContain(actual.GetOwnerReferences(), desired.GetOwnerReferences()) {
		return false
	}

	return resourceContentEqual(desired, actual)
}

func resourceContentEqual(desired, actual client.Object) bool {
	toContent := func(obj client.Object) (map[string]any, bool) {
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, false
		}
		content := map[string]any{}
		if err := json.Unmarshal(data, &content); err != nil {
			return nil, false
		}
		delete(content, "apiVersion")
		delete(content, "kind")
		delete(content, "metadata")
		delete(content, "status")
		normalizeSecretStringData(content)
		return content, true
	}

	desiredContent, desiredOK := toContent(desired)
	actualContent, actualOK := toContent(actual)
	return desiredOK && actualOK && reflect.DeepEqual(desiredContent, actualContent)
}

func normalizeSecretStringData(content map[string]any) {
	stringData, ok := content["stringData"].(map[string]any)
	if !ok {
		return
	}
	data, _ := content["data"].(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	for key, value := range stringData {
		text, ok := value.(string)
		if !ok {
			continue
		}
		data[key] = base64.StdEncoding.EncodeToString([]byte(text))
	}
	content["data"] = data
	delete(content, "stringData")
}

func mapContains(actual, desired map[string]string) bool {
	for key, value := range desired {
		if actual[key] != value {
			return false
		}
	}
	return true
}

func ownerReferencesContain(actual, desired []metav1.OwnerReference) bool {
	for _, desiredRef := range desired {
		found := false
		for _, actualRef := range actual {
			if reflect.DeepEqual(actualRef, desiredRef) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func prepareResourceUpdate(desired, actual client.Object) {
	desired.SetResourceVersion(actual.GetResourceVersion())
	desired.SetUID(actual.GetUID())
	desired.SetCreationTimestamp(actual.GetCreationTimestamp())
	desired.SetGeneration(actual.GetGeneration())
	desired.SetManagedFields(actual.GetManagedFields())
	desired.SetFinalizers(actual.GetFinalizers())
	desired.SetDeletionTimestamp(actual.GetDeletionTimestamp())

	labels := maps.Clone(actual.GetLabels())
	if labels == nil {
		labels = map[string]string{}
	}
	maps.Copy(labels, desired.GetLabels())
	desired.SetLabels(labels)

	annotations := maps.Clone(actual.GetAnnotations())
	if annotations == nil {
		annotations = map[string]string{}
	}
	maps.Copy(annotations, desired.GetAnnotations())
	desired.SetAnnotations(annotations)

	ownerReferences := append([]metav1.OwnerReference(nil), actual.GetOwnerReferences()...)
	for _, desiredRef := range desired.GetOwnerReferences() {
		if !ownerReferencesContain(ownerReferences, []metav1.OwnerReference{desiredRef}) {
			ownerReferences = append(ownerReferences, desiredRef)
		}
	}
	desired.SetOwnerReferences(ownerReferences)
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
