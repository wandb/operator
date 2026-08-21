package controller

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	managedSpecConfigMapName      = "wandb-spec-managed"
	managedSpecStateConfigMapName = "wandb-managed-spec-state"
	managedSpecStateKey           = "managed"
)

type managedSpecSource struct {
	spec      *spec.Spec
	rawChart  interface{}
	rawValues map[string]interface{}
}

type baseSpecSelection struct {
	selectedSpec          *spec.Spec
	shouldCompleteCutover bool
}

func (r *WeightsAndBiasesReconciler) selectBaseSpec(
	ctx context.Context,
	namespace string,
	getDeployerSpec func() (*spec.Spec, error),
) (baseSpecSelection, error) {
	if !r.ManagedSpecCutoverEnabled {
		deployerSpec, err := getDeployerSpec()
		return baseSpecSelection{selectedSpec: deployerSpec}, err
	}

	log := ctrllog.FromContext(ctx)
	cutoverComplete, err := r.isManagedSpecCutoverComplete(ctx, namespace)
	if err != nil {
		return baseSpecSelection{}, err
	}
	if cutoverComplete {
		log.Info("Managed spec cutover is active; skipping Deployer")
		managedSpec, err := r.getManagedSpec(ctx, namespace)
		if err != nil {
			return baseSpecSelection{}, err
		}
		return baseSpecSelection{selectedSpec: managedSpec.spec}, nil
	}

	deployerSpec, err := getDeployerSpec()
	if err != nil {
		return baseSpecSelection{}, err
	}

	managedSpec, err := r.getManagedSpec(ctx, namespace)
	if apierrors.IsNotFound(err) {
		return baseSpecSelection{selectedSpec: deployerSpec}, nil
	}
	if err != nil {
		log.Info("Managed spec is invalid; continuing with Deployer", "error", err)
		return baseSpecSelection{selectedSpec: deployerSpec}, nil
	}
	matches, err := managedSpecConfigurationMatches(managedSpec, deployerSpec)
	if err != nil {
		log.Info("Managed spec could not be compared; continuing with Deployer", "error", err)
		return baseSpecSelection{selectedSpec: deployerSpec}, nil
	}
	if matches {
		log.Info("Managed spec matches Deployer; cutover is pending successful apply")
		return baseSpecSelection{
			selectedSpec:          managedSpec.spec,
			shouldCompleteCutover: true,
		}, nil
	}

	log.Info("Managed spec does not match Deployer; continuing with Deployer")
	return baseSpecSelection{selectedSpec: deployerSpec}, nil
}

func (r *WeightsAndBiasesReconciler) isManagedSpecCutoverComplete(ctx context.Context, namespace string) (bool, error) {
	state := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: managedSpecStateConfigMapName, Namespace: namespace}, state)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	value, ok := state.Data[managedSpecStateKey]
	if !ok {
		return false, nil
	}
	if value == "true" {
		return true, nil
	}
	return false, fmt.Errorf("invalid %s value %q in ConfigMap %s", managedSpecStateKey, value, managedSpecStateConfigMapName)
}

func (r *WeightsAndBiasesReconciler) markManagedSpecCutoverComplete(ctx context.Context, namespace string) error {
	key := client.ObjectKey{Name: managedSpecStateConfigMapName, Namespace: namespace}
	state := &corev1.ConfigMap{}
	err := r.Get(ctx, key, state)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			Data:       map[string]string{managedSpecStateKey: "true"},
		})
	}
	if err != nil {
		return err
	}
	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	if state.Data[managedSpecStateKey] == "true" {
		return nil
	}
	state.Data[managedSpecStateKey] = "true"
	return r.Update(ctx, state)
}

func (r *WeightsAndBiasesReconciler) completeManagedSpecCutoverIfNeeded(
	ctx context.Context,
	namespace string,
	shouldComplete bool,
) error {
	if !shouldComplete {
		return nil
	}
	if err := r.markManagedSpecCutoverComplete(ctx, namespace); err != nil {
		return err
	}
	ctrllog.FromContext(ctx).Info("Managed spec cutover completed")
	return nil
}

func (r *WeightsAndBiasesReconciler) getManagedSpec(ctx context.Context, namespace string) (*managedSpecSource, error) {
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

	return &managedSpecSource{
		spec:      &spec.Spec{Chart: chart, Values: spec.Values(rawValues)},
		rawChart:  rawChart,
		rawValues: rawValues,
	}, nil
}
