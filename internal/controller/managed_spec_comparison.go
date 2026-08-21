package controller

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/wandb/operator/pkg/wandb/spec"
)

const (
	orbUsageEventReporterEnvironment = "GORILLA_ORB_USAGE_EVENT_REPORTER_SECRET"
)

func managedSpecConfigurationMatches(managed *managedSpecSource, deployer *spec.Spec) (bool, error) {
	if managed == nil || deployer == nil {
		return false, nil
	}

	managedChart, err := normalizeJSONValue(managed.rawChart)
	if err != nil {
		return false, fmt.Errorf("normalize managed chart: %w", err)
	}
	deployerChart, err := normalizeJSONValue(deployer.Chart)
	if err != nil {
		return false, fmt.Errorf("normalize Deployer chart: %w", err)
	}
	managedValues, err := normalizeJSONValue(managed.rawValues)
	if err != nil {
		return false, fmt.Errorf("normalize managed values: %w", err)
	}
	deployerValues, err := normalizeJSONValue(deployer.Values)
	if err != nil {
		return false, fmt.Errorf("normalize Deployer values: %w", err)
	}

	managedValuesMap, ok := managedValues.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("normalized managed values are not an object")
	}
	deployerValuesMap, ok := deployerValues.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("normalized Deployer values are not an object")
	}
	if err := normalizeExpectedManagedSpecDifferences(managedValuesMap, deployerValuesMap); err != nil {
		return false, err
	}

	return managedJSONSubsetEqual(managedChart, deployerChart) &&
		managedJSONSubsetEqual(managedValuesMap, deployerValuesMap), nil
}

func normalizeExpectedManagedSpecDifferences(managed, deployer map[string]interface{}) error {
	value, ok := nestedValue(managed, "global", "cloudProvider")
	if !ok {
		return fmt.Errorf("managed global.cloudProvider is required")
	}
	cloudProvider, ok := value.(string)
	if !ok || !isSupportedManagedCloudProvider(cloudProvider) {
		return fmt.Errorf("managed global.cloudProvider must be aws, gcp, or azure")
	}
	deployerCloud, hasDeployerCloud := nestedValue(deployer, "global", "extraEnv", "TAG_CLOUD")
	if hasDeployerCloud {
		deployerCloudString, ok := deployerCloud.(string)
		if !ok || strings.ToLower(deployerCloudString) != cloudProvider {
			return fmt.Errorf("managed global.cloudProvider %q does not match Deployer TAG_CLOUD", cloudProvider)
		}
	}
	deleteNestedValue(managed, "global", "cloudProvider")

	value, ok = nestedValue(managed, "global", "extraEnv", "TAG_CUSTOMER_NS")
	if !ok {
		return fmt.Errorf("managed TAG_CUSTOMER_NS is required")
	}
	customerNamespace, ok := value.(string)
	if !ok || strings.TrimSpace(customerNamespace) == "" {
		return fmt.Errorf("managed TAG_CUSTOMER_NS must be a non-empty string")
	}
	deleteNestedValue(managed, "global", "extraEnv", "TAG_CUSTOMER_NS")

	deleteNestedValue(managed, "otel", "daemonset", "config", "exporters", "datadog", "api", "key")
	deleteNestedValue(managed, "otel", "daemonset", "extraEnvFrom", "DD_API_KEY")
	deleteNestedValue(managed, "app", "env", orbUsageEventReporterEnvironment)
	deleteNestedValue(managed, "glue", "env", orbUsageEventReporterEnvironment)
	return nil
}

func isSupportedManagedCloudProvider(cloudProvider string) bool {
	return cloudProvider == "aws" || cloudProvider == "gcp" || cloudProvider == "azure"
}

func nestedValue(root map[string]interface{}, path ...string) (interface{}, bool) {
	var current interface{} = root
	for _, key := range path {
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = object[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func deleteNestedValue(root map[string]interface{}, path ...string) {
	if len(path) == 0 {
		return
	}
	objects := []map[string]interface{}{root}
	current := root
	for _, key := range path[:len(path)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return
		}
		objects = append(objects, next)
		current = next
	}
	delete(current, path[len(path)-1])
	for index := len(objects) - 1; index > 0; index-- {
		if len(objects[index]) != 0 {
			break
		}
		delete(objects[index-1], path[index-1])
	}
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
