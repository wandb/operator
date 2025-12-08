package percona

import (
	"context"

	"github.com/wandb/operator/internal/controller/common"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "PerconaXtraDBCluster"
	AppConnTypeName  = "MySQLAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *pxcv1.PerconaXtraDBCluster,
) error {
	var err error
	var actual = &pxcv1.PerconaXtraDBCluster{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = common.GetResource(
		ctx, client, nsNameBldr.ClusterNsName(), ResourceTypeName, actual,
	); err != nil {
		return err
	}

	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
