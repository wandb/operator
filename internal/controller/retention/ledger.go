package retention

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ledgerDataKey = "inventory.json"

type Ledger struct {
	Version          int                 `json:"version"`
	InstallationUID  types.UID           `json:"installationUID"`
	ReadyFingerprint string              `json:"readyFingerprint,omitempty"`
	Deleting         bool                `json:"deleting,omitempty"`
	Resources        map[string]Resource `json:"resources"`
	Dependencies     map[string][]string `json:"dependencies,omitempty"`
}

// Session serializes retention writes with this installation's reconcile loop.
// Inventory is deliberately not owned by WandB so detach provenance survives it.
type Session struct {
	client.Client
	reader      client.Reader
	parent      *apiv2.WeightsAndBiases
	cm          *corev1.ConfigMap
	Ledger      Ledger
	fingerprint string
}

func LedgerName(uid types.UID) string {
	sum := sha256.Sum256([]byte(uid))
	return "wandb-retention-" + hex.EncodeToString(sum[:16])
}

func Fingerprint(w *apiv2.WeightsAndBiases) string {
	data, _ := json.Marshal(struct {
		Spec                   apiv2.WeightsAndBiasesSpec
		Tracking, Installation string
	}{w.Spec, w.Annotations["argocd.argoproj.io/tracking-id"], w.Annotations["argocd.argoproj.io/installation-id"]})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func Open(ctx context.Context, c client.Client, reader client.Reader, w *apiv2.WeightsAndBiases) (*Session, error) {
	if w.UID == "" {
		return nil, fmt.Errorf("retention requires an installation UID")
	}
	if reader == nil {
		reader = c
	}
	s := &Session{Client: c, reader: reader, parent: w.DeepCopy(), fingerprint: Fingerprint(w), cm: &corev1.ConfigMap{}}
	err := reader.Get(ctx, client.ObjectKey{Namespace: w.Namespace, Name: LedgerName(w.UID)}, s.cm)
	if apierrors.IsNotFound(err) {
		if !w.DeletionTimestamp.IsZero() {
			return nil, fmt.Errorf("deletion predates retention migration; restore or recover the installation before purging")
		}
		s.cm = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: LedgerName(w.UID), Namespace: w.Namespace}}
		associate(s.cm, w, Resource{Component: "retention", Instance: "inventory"})
		s.Ledger = Ledger{Version: 1, InstallationUID: w.UID, Resources: map[string]Resource{}}
	} else if err != nil {
		return nil, err
	} else if err = json.Unmarshal([]byte(s.cm.Data[ledgerDataKey]), &s.Ledger); err != nil {
		return nil, fmt.Errorf("decode retention inventory: %w", err)
	}
	if s.Ledger.Version != 1 || s.Ledger.InstallationUID != w.UID || s.Ledger.Resources == nil {
		return nil, fmt.Errorf("retention inventory identity or version mismatch")
	}
	if w.DeletionTimestamp.IsZero() && !s.Ledger.Deleting {
		resources, err := Inventory(w)
		if err != nil {
			return nil, err
		}
		policies := map[string]apiv2.OnDeletePolicy{}
		for _, r := range resources {
			policies[InstanceID(r.Component, r.Instance)] = r.Policy
			if old, ok := s.Ledger.Resources[r.Key()]; ok {
				if old.Component != r.Component || old.Instance != r.Instance {
					return nil, fmt.Errorf("resource %s cannot move between instances without readoption", r.Key())
				}
				r.UID = old.UID
				r.Supplied = old.Supplied
			}
			s.Ledger.Resources[r.Key()] = r
		}
		// Descendants and retired names inherit policy changes to their instance.
		for key, r := range s.Ledger.Resources {
			if p, ok := policies[InstanceID(r.Component, r.Instance)]; ok {
				r.Policy = p
				s.Ledger.Resources[key] = r
			}
		}
		if s.Ledger.Dependencies == nil {
			s.Ledger.Dependencies = map[string][]string{}
		}
		for consumer, dependencies := range Dependencies(w) {
			for _, dependency := range dependencies {
				if !slices.Contains(s.Ledger.Dependencies[consumer], dependency) {
					s.Ledger.Dependencies[consumer] = append(s.Ledger.Dependencies[consumer], dependency)
				}
			}
		}
		if s.Ledger.ReadyFingerprint != s.fingerprint {
			s.Ledger.ReadyFingerprint = ""
		}
	}
	if err := s.save(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) save(ctx context.Context) error {
	oldLabels := maps.Clone(s.cm.Labels)
	oldAnnotations := maps.Clone(s.cm.Annotations)
	associate(s.cm, s.parent, Resource{Component: "retention", Instance: "inventory"})
	data, err := json.Marshal(s.Ledger)
	if err != nil {
		return err
	}
	if len(data) > 900000 {
		return fmt.Errorf("retention inventory exceeds its safe storage limit")
	}
	if s.cm.Data[ledgerDataKey] == string(data) && s.cm.ResourceVersion != "" && maps.Equal(oldLabels, s.cm.Labels) && maps.Equal(oldAnnotations, s.cm.Annotations) {
		return nil
	}
	s.cm.Data = map[string]string{ledgerDataKey: string(data)}
	if s.cm.ResourceVersion == "" {
		return s.Client.Create(ctx, s.cm)
	}
	return s.Client.Update(ctx, s.cm)
}

func resourceObject(r Resource) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.GVK())
	obj.SetNamespace(r.Namespace)
	obj.SetName(r.Name)
	return obj
}

func absent(err error, r Resource) bool {
	return apierrors.IsNotFound(err) || (r.OptionalAPI && meta.IsNoMatchError(err))
}

func (s *Session) objectResource(obj client.Object) (Resource, bool, error) {
	gvk, err := s.GroupVersionKindFor(obj)
	if err != nil {
		return Resource{}, false, err
	}
	key := (Resource{APIVersion: gvk.GroupVersion().String(), Kind: gvk.Kind, Namespace: obj.GetNamespace(), Name: obj.GetName()}).Key()
	r, ok := s.Ledger.Resources[key]
	return r, ok, nil
}

func (s *Session) trusted(obj client.Object, r Resource) bool {
	if r.UID != "" {
		if foreign := obj.GetLabels()[UIDLabel]; foreign != "" && !associated(obj, s.parent, r) {
			return false
		}
		if r.UID == obj.GetUID() {
			return true
		}
		if r.Stage == 0 || r.Stage == 3 {
			return false
		}
		for _, parent := range s.Ledger.Resources {
			if parent.UID != "" && !parent.Supplied && ownedBy(obj, parent.UID) {
				return true
			}
		}
		return false
	}
	if foreign := obj.GetLabels()[UIDLabel]; foreign != "" && foreign != string(s.parent.UID) {
		return false
	}
	if associated(obj, s.parent, r) || ownedBy(obj, s.parent.UID) {
		return true
	}
	for _, parent := range s.Ledger.Resources {
		if parent.UID != "" && !parent.Supplied && ownedBy(obj, parent.UID) {
			return true
		}
	}
	return false
}

func (s *Session) reconcileObject(ctx context.Context, r Resource, obj client.Object) error {
	if !s.trusted(obj, r) {
		adopt, err := s.canAdopt(ctx, obj, r)
		if err != nil {
			return err
		}
		// Older versions reused this Secret without recording who created it.
		if adopt {
			r.UID = obj.GetUID()
		} else if r.Component == "mysql" && r.Kind == "Secret" && strings.HasSuffix(r.Name, "-db-password") && len(obj.GetOwnerReferences()) == 0 && obj.GetLabels()[UIDLabel] == "" {
			r.Supplied = true
		} else {
			return fmt.Errorf("refusing to adopt unverified resource %s (UID %s)", r.Key(), obj.GetUID())
		}
	}
	if !obj.GetDeletionTimestamp().IsZero() {
		if r.Stage == 1 {
			return nil
		}
		return fmt.Errorf("resource %s is already terminating; retention cannot be guaranteed", r.Key())
	}
	r.UID = obj.GetUID()
	if u, ok := obj.(*unstructured.Unstructured); ok && r.Kind == "StatefulSet" {
		r.ClaimTemplates = claimTemplates(u)
	}
	s.Ledger.Resources[r.Key()] = r
	// Record identity before changing ownership, so a retry cannot mistake an
	// already-detached resource for an unrecognized object.
	if err := s.save(ctx); err != nil {
		return err
	}
	if r.Supplied {
		return nil
	}
	before := obj.DeepCopyObject().(client.Object)
	associate(obj, s.parent, r)
	if !reflect.DeepEqual(before, obj) {
		return s.Client.Update(ctx, obj)
	}
	return nil
}

func (s *Session) Reconcile(ctx context.Context) error {
	resources := s.sortedResources()
	for _, r := range resources {
		obj := resourceObject(r)
		err := s.reader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if absent(err, r) {
			continue
		}
		if err != nil {
			return err
		}
		if err = s.reconcileObject(ctx, r, obj); err != nil {
			return err
		}
	}
	if err := s.discover(ctx); err != nil {
		return err
	}
	for _, r := range s.Ledger.Resources {
		obj := resourceObject(r)
		err := s.reader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if absent(err, r) {
			continue
		}
		if err != nil {
			return err
		}
		if r.Stage == 1 && !obj.GetDeletionTimestamp().IsZero() {
			continue
		}
		if err = s.verifyOwnerPath(ctx, obj, r.Policy == apiv2.DetachOnDelete, map[types.UID]bool{}); err != nil {
			return err
		}
		if err = s.verifyReferences(ctx, obj, r); err != nil {
			return err
		}
	}
	if err := s.validateDependencies(); err != nil {
		return err
	}
	s.Ledger.ReadyFingerprint = s.fingerprint
	return s.save(ctx)
}

func (s *Session) sortedResources() []Resource {
	resources := make([]Resource, 0, len(s.Ledger.Resources))
	for _, r := range s.Ledger.Resources {
		resources = append(resources, r)
	}
	slices.SortFunc(resources, func(a, b Resource) int {
		if a.Stage != b.Stage {
			return a.Stage - b.Stage
		}
		return strings.Compare(a.Key(), b.Key())
	})
	return resources
}

func (s *Session) CheckReady(ctx context.Context) error {
	if !argoProtected(s.cm, s.parent) {
		return fmt.Errorf("retention inventory is missing Argo CD deletion protection")
	}
	if s.Ledger.ReadyFingerprint != s.fingerprint {
		return fmt.Errorf("retention migration is not ready for the current configuration")
	}
	for _, r := range s.Ledger.Resources {
		obj := resourceObject(r)
		err := s.reader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if absent(err, r) {
			continue
		}
		if err != nil {
			return err
		}
		if r.Stage == 1 && !obj.GetDeletionTimestamp().IsZero() {
			continue
		}
		if !r.Supplied && !argoProtected(obj, s.parent) {
			return fmt.Errorf("resource %s is missing Argo CD deletion protection", r.Key())
		}
		if r.UID == "" || r.UID != obj.GetUID() || !s.trusted(obj, r) || ownedBy(obj, s.parent.UID) || !obj.GetDeletionTimestamp().IsZero() {
			return fmt.Errorf("resource %s has not completed retention migration", r.Key())
		}
		if err = s.verifyOwnerPath(ctx, obj, r.Policy == apiv2.DetachOnDelete, map[types.UID]bool{}); err != nil {
			return err
		}
		if err = s.verifyReferences(ctx, obj, r); err != nil {
			return err
		}
	}
	return s.validateDependencies()
}
