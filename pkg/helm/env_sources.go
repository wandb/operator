package helm

import (
	"bytes"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

type envSourcePostRenderer struct{}

func (envSourcePostRenderer) Run(manifests *bytes.Buffer) (*bytes.Buffer, error) {
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifests.Bytes()), 4096)
	result := new(bytes.Buffer)
	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				return result, nil
			}
			return nil, fmt.Errorf("decode rendered resource: %w", err)
		}
		if len(obj.Object) == 0 {
			continue
		}
		if err := clearAlternativeEnvSources(&obj); err != nil {
			return nil, err
		}
		data, err := yaml.Marshal(obj.Object)
		if err != nil {
			return nil, fmt.Errorf("encode rendered resource: %w", err)
		}
		result.WriteString("---\n")
		result.Write(data)
	}
}

func clearAlternativeEnvSources(obj *unstructured.Unstructured) error {
	var path []string
	switch obj.GetAPIVersion() + "/" + obj.GetKind() {
	case "apps/v1/Deployment", "apps/v1/DaemonSet", "apps/v1/StatefulSet":
		path = []string{"spec", "template", "spec"}
	case "batch/v1/CronJob":
		path = []string{"spec", "jobTemplate", "spec", "template", "spec"}
	default:
		return nil
	}
	podSpec, found, err := unstructured.NestedMap(obj.Object, path...)
	if err != nil || !found {
		return err
	}
	for _, field := range []string{"containers", "initContainers"} {
		containers, _ := podSpec[field].([]interface{})
		for _, item := range containers {
			container, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			envs, _ := container["env"].([]interface{})
			for _, item := range envs {
				env, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				// Explicit nulls remove fields left by a partially applied Helm upgrade.
				for _, source := range []string{"value", "valueFrom"} {
					if _, exists := env[source]; !exists {
						env[source] = nil
					}
				}
			}
		}
	}
	return unstructured.SetNestedMap(obj.Object, podSpec, path...)
}
