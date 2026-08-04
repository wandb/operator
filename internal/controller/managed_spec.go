package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

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

func (r *WeightsAndBiasesReconciler) selectBaseSpec(
	ctx context.Context,
	namespace string,
	getDeployerSpec func() (*spec.Spec, error),
) (*spec.Spec, bool, error) {
	log := ctrllog.FromContext(ctx)
	managed, err := r.managedSpecEnabled(ctx, namespace)
	if err != nil {
		return nil, false, err
	}
	if managed {
		log.Info("Managed spec cutover is active; skipping Deployer")
		managedSpec, err := r.getManagedSpec(ctx, namespace)
		if err != nil {
			return nil, false, err
		}
		return managedSpec.spec, false, nil
	}

	deployerSpec, err := getDeployerSpec()
	if err != nil {
		return nil, false, err
	}

	managedSpec, err := r.getManagedSpec(ctx, namespace)
	if apierrors.IsNotFound(err) {
		return deployerSpec, false, nil
	}
	if err != nil {
		log.Info("Managed spec is invalid; continuing with Deployer", "error", err)
		return deployerSpec, false, nil
	}
	matches, err := managedSpecConfigurationMatches(managedSpec, deployerSpec)
	if err != nil {
		log.Info("Managed spec could not be compared; continuing with Deployer", "error", err)
		return deployerSpec, false, nil
	}
	if matches {
		log.Info("Managed spec matches Deployer; cutover is pending successful apply")
		return managedSpec.spec, true, nil
	}

	log.Info("Managed spec does not match Deployer; continuing with Deployer")
	return deployerSpec, false, nil
}

func (r *WeightsAndBiasesReconciler) managedSpecEnabled(ctx context.Context, namespace string) (bool, error) {
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

func (r *WeightsAndBiasesReconciler) setManagedSpecEnabled(ctx context.Context, namespace string) error {
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

func managedSpecConfigurationMatches(managed *managedSpecSource, deployer *spec.Spec) (bool, error) {
	if managed == nil || deployer == nil {
		return false, nil
	}

	deployerChart, err := normalizeJSONValue(deployer.Chart)
	if err != nil {
		return false, fmt.Errorf("normalize Deployer chart: %w", err)
	}
	deployerValues, err := normalizeJSONValue(deployer.Values)
	if err != nil {
		return false, fmt.Errorf("normalize Deployer values: %w", err)
	}

	return managedJSONSubsetEqual(managed.rawChart, deployerChart) &&
		managedJSONSubsetEqual(managed.rawValues, deployerValues), nil
}

func normalizeJSONValue(value interface{}) (interface{}, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var normalized interface{}
	if err := json.Unmarshal(data, &normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func managedJSONSubsetEqual(managed, deployer interface{}) bool {
	switch managedValue := managed.(type) {
	case map[string]interface{}:
		deployerValue, ok := deployer.(map[string]interface{})
		if !ok {
			return false
		}
		for key, managedChild := range managedValue {
			deployerChild, ok := deployerValue[key]
			if !ok || !managedJSONSubsetEqual(managedChild, deployerChild) {
				return false
			}
		}
		return true
	case []interface{}:
		deployerValue, ok := deployer.([]interface{})
		if !ok || len(managedValue) != len(deployerValue) {
			return false
		}
		for index, managedChild := range managedValue {
			if !managedJSONSubsetEqual(managedChild, deployerValue[index]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(managed, deployer)
	}
}
