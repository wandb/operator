package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// DeleteHandler decodes identity only. A v1 DELETE must not first translate a
// native v2 spec through the legacy values schema to reach this guard.
func DeleteHandler(reader client.Reader) admission.Handler {
	return admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
		w := &apiv2.WeightsAndBiases{}
		if err := json.Unmarshal(req.OldObject.Raw, w); err != nil {
			return admission.Denied("cannot decode installation identity: " + err.Error())
		}
		if err := ValidateDelete(ctx, reader, w); err != nil {
			return admission.Denied(err.Error())
		}
		return admission.Allowed("retention inventory verified")
	})
}

// ValidateDelete uses the storage version and an uncached reader for both APIs.
// It must never migrate or write resources from admission.
func ValidateDelete(ctx context.Context, reader client.Reader, request *apiv2.WeightsAndBiases) error {
	if reader == nil {
		return fmt.Errorf("retention deletion guard has no API reader")
	}
	w := &apiv2.WeightsAndBiases{}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(request), w); err != nil {
		return err
	}
	if w.UID != request.UID || (request.ResourceVersion != "" && request.ResourceVersion != w.ResourceVersion) {
		return fmt.Errorf("installation changed during deletion admission; retry deletion")
	}
	if !slices.Contains(w.Finalizers, Finalizer) {
		return fmt.Errorf("retention finalizer has not been installed; wait for reconciliation")
	}
	cm := &corev1.ConfigMap{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: w.Namespace, Name: LedgerName(w.UID)}, cm); err != nil {
		return fmt.Errorf("retention inventory is not ready: %w", err)
	}
	s := &Session{reader: reader, parent: w, cm: cm, fingerprint: Fingerprint(w)}
	if err := json.Unmarshal([]byte(cm.Data[ledgerDataKey]), &s.Ledger); err != nil {
		return fmt.Errorf("invalid retention inventory: %w", err)
	}
	if s.Ledger.Version != 1 || s.Ledger.InstallationUID != w.UID || s.Ledger.Resources == nil {
		return fmt.Errorf("retention inventory identity or version mismatch")
	}
	if !w.DeletionTimestamp.IsZero() && s.Ledger.Deleting {
		return nil
	}
	return s.CheckReady(ctx)
}
