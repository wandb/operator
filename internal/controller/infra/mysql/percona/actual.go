package percona

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/model"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type perconaPXC struct {
	pxc    *pxcv1.PerconaXtraDBCluster
	config model.MySQLConfig
	client client.Client
	owner  metav1.Object
	scheme *runtime.Scheme
}

// Initialize fetches existing PerconaXtraDBCluster CR
func Initialize(
	ctx context.Context,
	client client.Client,
	mysqlConfig model.MySQLConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*perconaPXC, error) {
	log := ctrl.LoggerFrom(ctx)

	var pxc = &pxcv1.PerconaXtraDBCluster{}
	var result = perconaPXC{
		config: mysqlConfig,
		client: client,
		owner:  owner,
		scheme: scheme,
		pxc:    nil,
	}

	// Try to get PXC CR
	pxcKey := types.NamespacedName{
		Name:      PXCName,
		Namespace: mysqlConfig.Namespace,
	}
	err := client.Get(ctx, pxcKey, pxc)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error getting actual PXC CR")
			return nil, err
		}
	} else {
		result.pxc = pxc
	}

	return &result, nil
}

// Upsert creates or updates PerconaXtraDBCluster CR based on whether it exists
func (a *perconaPXC) Upsert(ctx context.Context, mysqlConfig model.MySQLConfig) *model.Results {
	results := model.InitResults()
	var nextResults *model.Results

	// Build desired PXC CR
	desiredPXC, nextResults := buildDesiredPXC(ctx, mysqlConfig, a.owner, a.scheme)
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	// Handle PXC CR
	if a.pxc == nil {
		nextResults = a.createPXC(ctx, desiredPXC)
		results.Merge(nextResults)
	} else {
		nextResults = a.updatePXC(ctx, desiredPXC, mysqlConfig)
		results.Merge(nextResults)
	}

	return results
}

// Delete removes PerconaXtraDBCluster CR
func (a *perconaPXC) Delete(ctx context.Context) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	// Delete PXC CR
	if a.pxc != nil {
		if err := a.client.Delete(ctx, a.pxc); err != nil {
			log.Error(err, "Failed to delete PXC CR")
			results.AddErrors(model.NewMySQLError(
				model.MySQLErrFailedToDeleteCode,
				fmt.Sprintf("failed to delete PXC: %v", err),
			))
			return results
		}
		results.AddStatuses(model.NewMySQLStatusDetail(model.MySQLDeletedCode, PXCName))
	}

	return results
}
