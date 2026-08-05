package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// WriteOwnedFields create-or-updates obj, writing ONLY the fields the operator
// owns and preserving everything a co-managing controller writes (finalizers,
// status, annotations).
//
// Use it for vendored CRs (e.g. the Altinity CHI/CHK) whose status holds
// unexported fields and uint64 values: controllerutil.CreateOrUpdate
// (apiequality.Semantic.DeepEqual) and CreateOrPatch (DeepCopyJSON of the
// status) both panic traversing that status. Here we fetch, mutate only owned
// fields, compare only those, and Update — which marshals via JSON and never
// deep-copies the status.
//
// applyOwned mutates the fetched object in place. ownedEqual must compare only
// owned fields (spec, labels, owner refs); excluding status keeps the
// panic-prone vendored status out of the comparison.
func WriteOwnedFields[T client.Object](
	ctx context.Context,
	cl client.Client,
	obj T,
	applyOwned func(obj T),
	ownedEqual func(a, b T) bool,
) (controllerutil.OperationResult, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := cl.Get(ctx, key, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		obj.SetName(key.Name)
		obj.SetNamespace(key.Namespace)
		applyOwned(obj)
		if err := cl.Create(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	before, ok := obj.DeepCopyObject().(T)
	if !ok {
		return controllerutil.OperationResultNone, fmt.Errorf("DeepCopyObject returned unexpected type for %T", obj)
	}
	applyOwned(obj)
	if ownedEqual(before, obj) {
		return controllerutil.OperationResultNone, nil
	}
	if err := cl.Update(ctx, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.OperationResultUpdated, nil
}

// JSONEqual reports whether a and b marshal to identical JSON. It is panic-safe
// for vendored types whose reflective deep-equal/deep-copy chokes on unexported
// or uint64 fields (json.Marshal skips unexported fields and accepts uint64). A
// marshal error is treated as not-equal so the caller writes rather than skips.
func JSONEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}
