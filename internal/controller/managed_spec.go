package controller

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const managedSpecConfigMapName = "wandb-spec-managed"

func (r *WeightsAndBiasesReconciler) selectBaseSpec(
	ctx context.Context,
	namespace string,
	getDeployerSpec func() (*spec.Spec, error),
) (*spec.Spec, error) {
	// Refresh the Deployer cache even when managed spec is selected.
	deployerSpec, err := getDeployerSpec()
	if err != nil {
		return nil, err
	}
	if !r.ManagedSpecEnabled {
		return deployerSpec, nil
	}

	managedSpec, err := r.getManagedSpec(ctx, namespace)
	if apierrors.IsNotFound(err) {
		return deployerSpec, nil
	}
	if err != nil {
		ctrllog.FromContext(ctx).Error(err, "Unable to use managed spec; falling back to Deployer")
		return deployerSpec, nil
	}
	return managedSpec, nil
}

func (r *WeightsAndBiasesReconciler) getManagedSpec(ctx context.Context, namespace string) (*spec.Spec, error) {
	configMap := &corev1.ConfigMap{}
	key := client.ObjectKey{Name: managedSpecConfigMapName, Namespace: namespace}
	if err := r.Get(ctx, key, configMap); err != nil {
		return nil, err
	}

	valuesJSON, ok := configMap.Data["values"]
	if !ok {
		return nil, fmt.Errorf("ConfigMap %s/%s does not have a values key", namespace, managedSpecConfigMapName)
	}
	rawValues := map[string]interface{}{}
	if err := json.Unmarshal([]byte(valuesJSON), &rawValues); err != nil {
		return nil, fmt.Errorf("decode values from ConfigMap %s/%s: %w", namespace, managedSpecConfigMapName, err)
	}

	if rawValues == nil {
		return nil, fmt.Errorf("ConfigMap %s/%s values must be an object", namespace, managedSpecConfigMapName)
	}

	chartJSON, ok := configMap.Data["chart"]
	if !ok {
		return nil, fmt.Errorf("ConfigMap %s/%s does not have a chart key", namespace, managedSpecConfigMapName)
	}
	var rawChart interface{}
	if err := json.Unmarshal([]byte(chartJSON), &rawChart); err != nil {
		return nil, fmt.Errorf("decode chart from ConfigMap %s/%s: %w", namespace, managedSpecConfigMapName, err)
	}
	chart := charts.Get(rawChart)
	if chart == nil {
		return nil, fmt.Errorf("ConfigMap %s/%s contains an unsupported chart", namespace, managedSpecConfigMapName)
	}

	return &spec.Spec{Chart: chart, Values: spec.Values(rawValues)}, nil
}
