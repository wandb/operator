package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	v1 "github.com/wandb/operator/pkg/vendored/strimzi-kafka/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, _ = logx.WithSlog(ctx, logx.Kafka)
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	ownerUID := wandbOwner.GetUID()

	if err := common.DetachOwnerReference(ctx, cl, nsnBuilder.KafkaNsName(), KafkaResourceType, &v1.Kafka{}, ownerUID); err != nil {
		return err
	}
	return common.DetachOwnerReference(ctx, cl, nsnBuilder.NodePoolNsName(), NodePoolResourceType, &v1.KafkaNodePool{}, ownerUID)
}
