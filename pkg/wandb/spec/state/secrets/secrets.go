package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
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
	secret := &corev1.Secret{}

	err := c.Get(ctx, objKey, secret)
	if err != nil {
		return nil, err
	}

	valuesJson, ok := secret.Data["values"]
	if !ok {
		return nil, fmt.Errorf(
			"secret %s/%s does not have a `values` key", objKey.Namespace, objKey.Name)
	}

	spec := &spec.Spec{}

	err = json.Unmarshal(valuesJson, &spec.Values)
	if err != nil {
		return nil, err
	}

	metadataJson, ok := secret.Data["metadata"]
	if ok {
		err = json.Unmarshal(metadataJson, &spec.Metadata)
		if err != nil {
			return nil, err
		}
	}

	chartJson, ok := secret.Data["chart"]
	if !ok {
		return nil, fmt.Errorf(
			"secret %s/%s does not have a `release` key", objKey.Namespace, objKey.Name)
	}

	var maybeChart interface{}
	err = json.Unmarshal(chartJson, &maybeChart)
	if err != nil {
		return nil, err
	}

	chart := charts.Get(maybeChart)
	spec.SetChart(chart)
	if chart == nil {
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
	chartJson, err := json.Marshal(spec.Chart)
	if err != nil {
		return nil
	}

	valuesJson, err := json.Marshal(spec.Values)
	if err != nil {
		return nil
	}

	metadataJson, err := json.Marshal(spec.Metadata)
	fmt.Println("metadataJson", string(metadataJson))
	if err != nil {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
		Data: map[string][]byte{
			"chart":    chartJson,
			"values":   valuesJson,
			"metadata": metadataJson,
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
