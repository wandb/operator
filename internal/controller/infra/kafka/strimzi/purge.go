package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/translator"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PurgeFinalizer is a no-op for Kafka because PVC deletion is handled by
// Strimzi via KafkaNodePool's Spec.Storage.Volumes[].DeleteClaim == true,
// which is set in the translator when the retention policy is WBPurgeOnDelete.
func PurgeFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	onDeleteRule translator.OnDeleteRule,
) error {
	return nil
}
