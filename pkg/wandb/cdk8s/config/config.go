package config

import (
	"context"
	"encoding/json"

	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConfig(ctx context.Context, r client.Client, name string, namespace string) *Config {
	configMap := &corev1.ConfigMap{}
	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	r.Get(ctx, objKey, configMap)
	return &Config{}
}

func NewConfig(
	r client.Client,
	name string,
	version string,
	config interface{},
) *Config {
	configString, _ := json.Marshal(config)
	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Data: map[string]string{
			"version": version,
			"type":    "local",
			"config":  string(configString),
		},
	}
	return &Config{cfg}
}

type Config struct {
	configMap *corev1.ConfigMap
}

func (c Config) Values() interface{} {
	var obj interface{}
	val, ok := c.configMap.Data["config"]
	if !ok {
		val = "{}"
	}
	json.Unmarshal([]byte(val), &obj)
	return obj
}

func (c Config) Release() release.Release {
	version := c.configMap.Data["version"]
	typ := c.configMap.Data["type"]
	if typ == "local" {
		return release.NewLocalRelease(version)
	}
	return nil
}

func (c Config) Apply() {}
