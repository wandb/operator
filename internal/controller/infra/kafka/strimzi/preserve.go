package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PreserveFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, log := logx.WithSlog(ctx, logx.Kafka)

	var found bool
	var err error
	var actual = &corev1.Secret{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	nsName := nsnBuilder.ConnectionNsName()

	if found, err = common.GetResource(
		ctx, cl, nsName, AppConnTypeName, actual,
	); err != nil {
		return err
	}
	if !found {
		log.Info("abort preserve finalizer: no connection info found")
		return nil
	}

	beforeOwnerCount := len(actual.OwnerReferences)
	// remove wandbOwner as an OwnerReference to stop cascading deletion
	newOwnerRefs := utils.FilterFunc(actual.OwnerReferences, func(ref metav1.OwnerReference) bool {
		return ref.UID != wandbOwner.GetUID()
	})
	afterOwnerCount := len(newOwnerRefs)
	actual.SetOwnerReferences(newOwnerRefs)

	if err = cl.Update(ctx, actual); err != nil {
		if !errors.IsNotFound(err) {
			log.Error("error removing wandb owner reference during preserve", logx.ErrAttr(err))
			return err
		}
	}
	log.Debug("removed wandb owner reference during preserve",
		"uid", wandbOwner.GetUID(), "removalCount", beforeOwnerCount-afterOwnerCount,
	)

	return nil
}
