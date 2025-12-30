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
	var found bool
	var actual = &pxcv1.PerconaXtraDBCluster{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.ClusterNsName(), ResourceTypeName, actual,
	); err != nil {
		return err
	}
	if !found {
		actual = nil
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
