package strimzi

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PreserveFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	log := ctrl.LoggerFrom(ctx)

	var found bool
	var err error
	var actual = &corev1.Secret{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	nsName := nsNameBldr.ConnectionNsName()

	if found, err = common.GetResource(
		ctx, cl, nsName, AppConnTypeName, actual,
	); err != nil {
		return err
	}
	if !found {
		return nil
	}

	// remove wandbOwner as an OwnerReference to stop cascading deletion
	newOwnerRefs := utils.FilterFunc(actual.OwnerReferences, func(ref metav1.OwnerReference) bool {
		return ref.UID != wandbOwner.GetUID()
	})

	actual.SetOwnerReferences(newOwnerRefs)

	if err = cl.Update(ctx, actual); err != nil {
		if !errors.IsNotFound(err) {
			log.Info("error removing WandB obj ref from kafka connection info secret")
			return err
		}
	}

	return nil
}
