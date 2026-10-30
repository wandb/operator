package retention

import (
	"context"
	"encoding/json"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const AdoptFromAnnotation = "weightsandbiases.apps.wandb.com/adopt-from-uid"

func (s *Session) canAdopt(ctx context.Context, obj client.Object, r Resource) (bool, error) {
	uid := types.UID(s.parent.Annotations[AdoptFromAnnotation])
	if uid == "" || uid == s.parent.UID || s.Ledger.Deleting {
		return false, nil
	}
	cm := &corev1.ConfigMap{}
	if err := s.reader.Get(ctx, client.ObjectKey{Namespace: s.parent.Namespace, Name: LedgerName(uid)}, cm); err != nil {
		return false, err
	}
	var donor Ledger
	if err := json.Unmarshal([]byte(cm.Data[ledgerDataKey]), &donor); err != nil {
		return false, err
	}
	if donor.InstallationUID != uid || donor.Version != 1 {
		return false, fmt.Errorf("invalid readoption inventory")
	}
	old := &apiv2.WeightsAndBiases{}
	if err := s.reader.Get(ctx, client.ObjectKey{Namespace: cm.Labels[common.WandbNamespaceLabel], Name: cm.Labels[common.WandbNameLabel]}, old); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	} else if err == nil && old.UID == uid {
		return false, fmt.Errorf("cannot readopt from an installation that still exists")
	}
	previous, ok := donor.Resources[r.Key()]
	if ok && (previous.Policy != apiv2.DetachOnDelete || previous.Supplied || previous.Component != r.Component || previous.Instance != r.Instance) {
		return false, nil
	}
	owner := s.parent.DeepCopy()
	owner.UID = uid
	if ownedBy(obj, uid) || (obj.GetLabels()[UIDLabel] != "" && obj.GetLabels()[UIDLabel] != string(uid)) {
		return false, nil
	}
	exact := ok && previous.UID == obj.GetUID() && previous.Policy == apiv2.DetachOnDelete && !previous.Supplied && previous.Component == r.Component && previous.Instance == r.Instance
	if exact && associated(obj, owner, previous) {
		return true, nil
	}
	// Vendor-managed metadata may be reset, and Pods can restart while detached.
	// Require a parent already verified by this adopter before accepting either.
	for _, parent := range s.Ledger.Resources {
		if parent.Key() == r.Key() || parent.UID == "" || parent.Supplied || parent.Component != r.Component || parent.Instance != r.Instance {
			continue
		}
		if ownedBy(obj, parent.UID) && (exact || r.Stage == 1 || r.Stage == 2) {
			return true, nil
		}
		if exact && r.Stage == 3 && parent.Stage == 0 {
			oldParent, known := donor.Resources[parent.Key()]
			if known && oldParent.UID == parent.UID && oldParent.Policy == apiv2.DetachOnDelete {
				return true, nil
			}
		}
	}
	return false, nil
}
