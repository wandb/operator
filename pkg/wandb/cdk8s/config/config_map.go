package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wandb/operator/pkg/utils/kubeclient"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func releaseToAnnotation(release release.Release) string {
	v1 := strings.ReplaceAll(release.Version(), "/", "-")
	v2 := strings.Replace(v1, ".", "-", -1)
	v3 := strings.Trim(v2, "-")
	return v3
}

func CreateWithConfigMap(
	ctx context.Context,
	client client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,

	configMapName string,
	configMapNamespace string,
	release release.Release,
	config interface{},
) (*Config, error) {
	configJson, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
			Labels: map[string]string{
				"wandb.ai/release-version": releaseToAnnotation(release),
			},
		},
		Data: map[string]string{
			"release": release.Version(),
			"config":  string(configJson),
		},
	}
	if err := controllerutil.SetControllerReference(owner, configMap, scheme); err != nil {
		return nil, err
	}

	if err := kubeclient.CreateOrUpdate(ctx, client, configMap); err != nil {
		return nil, err
	}

	return &Config{Release: release, Config: config}, nil
}

func GetFromConfigMap(
	ctx context.Context,
	c client.Client,
	configMapName string,
	configMapNamespace string,
) (*Config, error) {
	configMap := &corev1.ConfigMap{}
	objKey := client.ObjectKey{Name: configMapName, Namespace: configMapNamespace}

	err := c.Get(ctx, objKey, configMap)
	if err != nil {
		return nil, err
	}

	relV, ok := configMap.Data["release"]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s/%s does not have a `release` key", configMapName, configMapNamespace)
	}
	rel := release.NewLocalRelease(relV)

	configString, ok := configMap.Data["config"]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s/%s does not have a `config` key", configMapName, configMapNamespace)
	}

	var config interface{}
	err = json.Unmarshal([]byte(configString), &config)
	if err != nil {
		return nil, err
	}
	return &Config{Release: rel, Config: config}, nil
}

// WroteToConfigMap is a helper function to write a config to a config map
func WriteToConfigMap(
	ctx context.Context,
	client client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
	configMapName string,
	configMapNamespace string,
	config *Config,
) error {
	_, err := CreateWithConfigMap(
		ctx,
		client,
		owner,
		scheme,
		configMapName,
		configMapNamespace,
		config.Release,
		config.Config,
	)
	if err != nil {
		_, err = UpdateWithConfigMap(
			ctx,
			client,
			scheme,
			configMapName,
			configMapNamespace,
			config.Release,
			config.Config,
		)
	}
	return err
}

func UpdateWithConfigMap(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	configMapName string,
	configMapNamespace string,
	release release.Release,
	config interface{},
) (*Config, error) {
	configMap := &corev1.ConfigMap{}
	objKey := client.ObjectKey{Name: configMapName, Namespace: configMapNamespace}
	err := c.Get(ctx, objKey, configMap)
	if err != nil {
		return nil, err
	}

	configJson, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	configMap.Data["release"] = release.Version()
	configMap.Data["config"] = string(configJson)
	if err := c.Update(ctx, configMap); err != nil {
		return nil, err
	}

	return &Config{Release: release, Config: config}, nil
}
