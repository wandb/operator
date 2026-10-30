package retention

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Session) live(ctx context.Context) error {
	w := &apiv2.WeightsAndBiases{}
	if err := s.reader.Get(ctx, client.ObjectKeyFromObject(s.parent), w); err != nil {
		return err
	}
	if w.UID != s.parent.UID || !w.DeletionTimestamp.IsZero() || s.Ledger.Deleting {
		return fmt.Errorf("installation is deleting or has been replaced")
	}
	return nil
}

func (s *Session) PrepareResource(ctx context.Context, desired, actual client.Object) (bool, error) {
	r, ok, err := s.objectResource(desired)
	if err != nil || !ok {
		return false, err
	}
	if err = s.live(ctx); err != nil {
		return false, err
	}
	if actual != nil && !common.IsNil(actual) && actual.GetName() != "" {
		if !s.trusted(actual, r) {
			return false, fmt.Errorf("refusing to overwrite unverified resource %s", r.Key())
		}
		if r.Supplied {
			return false, fmt.Errorf("refusing to modify supplied resource %s", r.Key())
		}
		if !actual.GetDeletionTimestamp().IsZero() {
			return false, fmt.Errorf("resource %s is terminating", r.Key())
		}
		r.UID = actual.GetUID()
		s.Ledger.Resources[r.Key()] = r
	}
	associate(desired, s.parent, r)
	if err = s.save(ctx); err != nil {
		return false, err
	}
	return actual != nil && !common.IsNil(actual) && ownedBy(actual, s.parent.UID), nil
}

func (s *Session) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	r, ok, err := s.objectResource(obj)
	if err != nil || !ok {
		if err != nil {
			return err
		}
		return s.Client.Create(ctx, obj, opts...)
	}
	if _, err = s.PrepareResource(ctx, obj, nil); err != nil {
		return err
	}
	if err = s.Client.Create(ctx, obj, opts...); err != nil {
		return err
	}
	r.UID = obj.GetUID()
	s.Ledger.Resources[r.Key()] = r
	return s.save(ctx)
}

func (s *Session) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	r, ok, err := s.objectResource(obj)
	if err != nil || !ok {
		if err != nil {
			return err
		}
		return s.Client.Update(ctx, obj, opts...)
	}
	actual := resourceObject(r)
	if err = s.reader.Get(ctx, client.ObjectKeyFromObject(obj), actual); err != nil {
		return err
	}
	if _, err = s.PrepareResource(ctx, obj, actual); err != nil {
		return err
	}
	return s.Client.Update(ctx, obj, opts...)
}

func (s *Session) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	_, ok, err := s.objectResource(obj)
	if err != nil {
		return err
	}
	// Removing an instance or switching Redis mode is not authorization to purge.
	if ok {
		return nil
	}
	// Legacy vendor readers can attempt selector-based cleanup when a root is
	// absent. An object outside this inventory has no verified deletion authority.
	return fmt.Errorf("refusing infrastructure deletion outside the retention inventory: %s/%s", obj.GetNamespace(), obj.GetName())
}

// Inventory resources are patched only by the lifecycle using an observed
// resourceVersion. Vendor PVC/pod label stamping is allowed for undiscovered
// children; discovery records and verifies those children before finalization.
func (s *Session) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	r, ok, err := s.objectResource(obj)
	if err != nil {
		return err
	}
	if !ok {
		return s.Client.Patch(ctx, obj, patch, opts...)
	}
	if err = s.live(ctx); err != nil {
		return err
	}
	actual := resourceObject(r)
	if err = s.reader.Get(ctx, client.ObjectKeyFromObject(obj), actual); err != nil {
		return err
	}
	if !s.trusted(actual, r) || r.Supplied {
		return fmt.Errorf("refusing to patch unverified resource %s", r.Key())
	}
	// These writers only stamp labels. Applying a caller's broader patch could
	// restore the legacy owner reference or change a replacement at the same name.
	data, err := patch.Data(obj)
	if err != nil {
		return err
	}
	return s.patchLabels(ctx, actual, r, data)
}

func (s *Session) DeleteAllOf(context.Context, client.Object, ...client.DeleteAllOfOption) error {
	return fmt.Errorf("bulk deletion is not permitted for retained infrastructure")
}

func ignoreMissing(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
