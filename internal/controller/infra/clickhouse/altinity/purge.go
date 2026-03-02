package altinity

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PurgeFinalizer is a no-op for ClickHouse because PVC deletion is handled by
// the Altinity operator via ClickHouseInstallation's PVCReclaimPolicy == Delete,
// which is set in the translator when the retention policy is WBPurgeOnDelete.
func PurgeFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule translator.OnDeleteRule,
) error {
	return nil
}
