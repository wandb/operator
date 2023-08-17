package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func read(
	ctx context.Context,
	c client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
	objKey client.ObjectKey,
) (*spec.Spec, error) {
	configMap := &corev1.Secret{}

	err := c.Get(ctx, objKey, configMap)
	if err != nil {
		return nil, err
	}

	configJson, ok := configMap.Data["config"]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s/%s does not have a `config` key", objKey.Namespace, objKey.Name)
	}

	var config map[string]interface{}
	err = json.Unmarshal([]byte(configJson), &config)
	if err != nil {
		return nil, err
	}

	spec := &spec.Spec{
		Config: config,
	}

	releaseJson, ok := configMap.Data["release"]
	if !ok {
		return nil, fmt.Errorf(
			"config map %s/%s does not have a `release` key", objKey.Namespace, objKey.Name)
	}

	var maybeRelease interface{}
	err = json.Unmarshal([]byte(releaseJson), &maybeRelease)
	if err != nil {
		return nil, err
	}

	release := release.Get(maybeRelease)
	spec.SetRelease(release)
	if release == nil {
		return spec, fmt.Errorf("could not find a matching release in config map %s/%s", objKey.Namespace, objKey.Name)
	}
	return spec, nil
}

func write(
	ctx context.Context,
	c client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
	objKey client.ObjectKey,
	spec *spec.Spec,
) error {
	releaseJson, err := json.Marshal(spec.Release)
	if err != nil {
		return nil
	}
	configJson, err := json.Marshal(spec.Config)
	if err != nil {
		return nil
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
		Data: map[string][]byte{
			"release": releaseJson,
			"config":  configJson,
		},
	}
	if err := controllerutil.SetControllerReference(owner, secret, scheme); err != nil {
		return err
	}

	if err := c.Create(ctx, secret); err != nil {
		if errors.IsAlreadyExists(err) {
			return c.Update(ctx, secret)
		}
		return err
	}
	return nil
}
