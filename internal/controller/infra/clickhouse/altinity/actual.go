package altinity

import (
	"context"
	"fmt"

	chiv1 "github.com/wandb/operator/api/altinity-clickhouse-vendored/clickhouse.altinity.com/v1"
	"github.com/wandb/operator/internal/model"
	machErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type altinityClickHouse struct {
	chi    *chiv1.ClickHouseInstallation
	config model.ClickHouseConfig
	client client.Client
	owner  metav1.Object
	scheme *runtime.Scheme
}

// Initialize fetches existing ClickHouseInstallation CR
func Initialize(
	ctx context.Context,
	client client.Client,
	clickhouseConfig model.ClickHouseConfig,
	owner metav1.Object,
	scheme *runtime.Scheme,
) (*altinityClickHouse, error) {
	log := ctrl.LoggerFrom(ctx)

	var chi = &chiv1.ClickHouseInstallation{}
	var result = altinityClickHouse{
		config: clickhouseConfig,
		client: client,
		owner:  owner,
		scheme: scheme,
		chi:    nil,
	}

	// Try to get CHI CR
	chiKey := types.NamespacedName{
		Name:      CHIName,
		Namespace: clickhouseConfig.Namespace,
	}
	err := client.Get(ctx, chiKey, chi)
	if err != nil {
		if !machErrors.IsNotFound(err) {
			log.Error(err, "error getting actual CHI CR")
			return nil, err
		}
	} else {
		result.chi = chi
	}

	return &result, nil
}

// Upsert creates or updates ClickHouseInstallation CR based on whether it exists
func (a *altinityClickHouse) Upsert(ctx context.Context, clickhouseConfig model.ClickHouseConfig) *model.Results {
	results := model.InitResults()
	var nextResults *model.Results

	// Build desired CHI CR
	desiredCHI, nextResults := buildDesiredCHI(ctx, clickhouseConfig, a.owner, a.scheme)
	results.Merge(nextResults)
	if results.HasCriticalError() {
		return results
	}

	// Handle CHI CR
	if a.chi == nil {
		nextResults = a.createCHI(ctx, desiredCHI)
		results.Merge(nextResults)
	} else {
		nextResults = a.updateCHI(ctx, desiredCHI, clickhouseConfig)
		results.Merge(nextResults)
	}

	return results
}

// Delete removes ClickHouseInstallation CR
func (a *altinityClickHouse) Delete(ctx context.Context) *model.Results {
	log := ctrl.LoggerFrom(ctx)
	results := model.InitResults()

	// Delete CHI CR
	if a.chi != nil {
		if err := a.client.Delete(ctx, a.chi); err != nil {
			log.Error(err, "Failed to delete CHI CR")
			results.AddErrors(model.NewClickHouseError(
				model.ClickHouseErrFailedToDelete,
				fmt.Sprintf("failed to delete CHI: %v", err),
			))
			return results
		}
		results.AddStatuses(model.NewClickHouseStatus(model.ClickHouseDeleted, CHIName))
	}

	return results
}
