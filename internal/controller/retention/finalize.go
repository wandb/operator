package retention

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func purgeOrder(r Resource) int {
	base := 20
	switch r.Component {
	case "kafka":
		base = 0
	case "clickhouse":
		base = 10
	case "objectstore":
		base = 30
	}
	return base + r.Stage
}

// Finalize returns false while deletions are pending. Retained resources never
// receive DELETE, and explicit UID preconditions protect replacements.
func (s *Session) Finalize(ctx context.Context) (bool, error) {
	if !s.Ledger.Deleting {
		if err := s.CheckReady(ctx); err != nil {
			return false, err
		}
		if err := s.discover(ctx); err != nil {
			return false, err
		}
		if err := s.CheckReady(ctx); err != nil {
			return false, err
		}
		s.Ledger.Deleting = true
		if err := s.save(ctx); err != nil {
			return false, err
		}
	}
	// Vendor controllers can finish in-flight reconciles after deletion starts.
	// Persist late children/claims before attempting the next deletion stage.
	if err := s.discover(ctx); err != nil {
		return false, err
	}
	resources := s.sortedResources()
	for _, r := range resources {
		if r.Policy == apiv2.PurgeOnDelete {
			done, err := s.stopApplicationClients(ctx)
			if err != nil || !done {
				return false, err
			}
			break
		}
	}
	slices.SortFunc(resources, func(a, b Resource) int {
		if purgeOrder(a) != purgeOrder(b) {
			return purgeOrder(a) - purgeOrder(b)
		}
		return strings.Compare(a.Key(), b.Key())
	})
	for _, r := range resources {
		obj := resourceObject(r)
		err := s.reader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if absent(err, r) {
			continue
		}
		if err != nil {
			return false, err
		}
		if r.Supplied {
			continue
		}
		if r.Policy == apiv2.DetachOnDelete && r.Stage == 1 && !obj.GetDeletionTimestamp().IsZero() {
			continue
		}
		if r.UID == "" || r.UID != obj.GetUID() || (!associated(obj, s.parent, r) && obj.GetDeletionTimestamp().IsZero()) {
			return false, fmt.Errorf("refusing deletion of unverified or replaced resource %s", r.Key())
		}
		if r.Policy == apiv2.DetachOnDelete {
			if ownedBy(obj, s.parent.UID) || !obj.GetDeletionTimestamp().IsZero() {
				return false, fmt.Errorf("retained resource %s is terminating or still owned by WandB", r.Key())
			}
			continue
		}
		if r.Policy != apiv2.PurgeOnDelete {
			return false, fmt.Errorf("invalid stored retention policy for %s", r.Key())
		}
		if obj.GetDeletionTimestamp().IsZero() {
			uid := r.UID
			if err := s.Client.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground), client.Preconditions{UID: &uid}); err != nil {
				return false, ignoreMissing(err)
			}
		}
		// Observe termination before moving on to configuration, PVCs, or a
		// backing store. This also keeps the parent finalizer across retries.
		return false, nil
	}
	return true, nil
}

func (s *Session) stopApplicationClients(ctx context.Context) (bool, error) {
	apps := &apiv2.ApplicationList{}
	if err := s.reader.List(ctx, apps, client.InNamespace(s.parent.Namespace)); err != nil {
		return false, err
	}
	jobs := &batchv1.JobList{}
	if err := s.reader.List(ctx, jobs, client.InNamespace(s.parent.Namespace)); err != nil {
		return false, err
	}
	var objects []client.Object
	for i := range apps.Items {
		objects = append(objects, &apps.Items[i])
	}
	for i := range jobs.Items {
		objects = append(objects, &jobs.Items[i])
	}
	for _, obj := range objects {
		if ownedBy(obj, s.parent.UID) {
			if obj.GetDeletionTimestamp().IsZero() {
				uid := obj.GetUID()
				if err := s.Client.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground), client.Preconditions{UID: &uid}); err != nil {
					return false, ignoreMissing(err)
				}
			}
			return false, nil
		}
	}
	return true, nil
}
