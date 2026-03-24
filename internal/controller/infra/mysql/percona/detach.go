package percona

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	pxcv1 "github.com/wandb/operator/pkg/vendored/percona-operator/pxc/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, log := logx.WithSlog(ctx, logx.Mysql)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	var actual = &pxcv1.PerconaXtraDBCluster{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.ClusterNsName(), ResourceTypeName, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach finalizer: PerconaXtraDBCluster CR not found")
		return nil
	}

	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("PerconaXtraDBCluster CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching PerconaXtraDBCluster CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached PerconaXtraDBCluster CR", "name", actual.Name)
	return nil
}
