package altinity

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CheckDetached(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbUID types.UID,
) []metav1.Condition {
	nsnBuilder := createNsNameBuilder(specNamespacedName)
	actual := &chiv1.ClickHouseInstallation{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.InstallationNsName(), ResourceTypeName, actual)
	if err != nil || !found {
		return nil
	}
	if !common.IsDetached(actual, wandbUID) {
		return nil
	}
	return nil
}

func DetachFinalizer(
	ctx context.Context,
	cl client.Client,
	specNamespacedName types.NamespacedName,
	wandbOwner client.Object,
) error {
	ctx, log := logx.WithSlog(ctx, logx.ClickHouse)
	nsnBuilder := createNsNameBuilder(specNamespacedName)

	var actual = &chiv1.ClickHouseInstallation{}
	found, err := common.GetResource(ctx, cl, nsnBuilder.InstallationNsName(), ResourceTypeName, actual)
	if err != nil {
		return err
	}
	if !found {
		log.Info("abort detach finalizer: ClickHouseInstallation CR not found")
		return nil
	}

	if common.IsDetached(actual, wandbOwner.GetUID()) {
		log.Debug("ClickHouseInstallation CR already detached")
		return nil
	}

	common.RemoveOwnerReference(actual, wandbOwner.GetUID())
	if err = cl.Update(ctx, actual); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		log.Error("error detaching ClickHouseInstallation CR", logx.ErrAttr(err))
		return err
	}
	log.Info("detached ClickHouseInstallation CR", "name", actual.Name)

	secret := &corev1.Secret{}
	found, err = common.GetResource(ctx, cl, nsnBuilder.ConnectionNsName(), "Secret", secret)
	if err != nil || !found {
		return err
	}
	common.RemoveOwnerReference(secret, wandbOwner.GetUID())
	if err = cl.Update(ctx, secret); err != nil && !errors.IsNotFound(err) {
		log.Error("error detaching connection secret", logx.ErrAttr(err))
		return err
	}
	log.Info("detached connection secret", "name", nsnBuilder.ConnectionNsName().Name)
	return nil
}
