/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv2 "github.com/wandb/operator/api/v2"
)

var logger = ctrl.Log.WithName("weightsandbiases-conversion")

// Round-trip annotations stashed on v2 so ConvertFrom can reproduce the
// original v1 chart/values across apiserver-internal v2 → v1 → v2 bounces.
const (
	v1ChartAnnotation  = "legacy.operator.wandb.com/v1-chart"
	v1ValuesAnnotation = "legacy.operator.wandb.com/v1-values"
)

const conversionLookupTimeout = 5 * time.Second

var conversionReader ctrlclient.Reader

// SetConversionReader wires a Reader (typically mgr.GetAPIReader()) into the
// conversion webhook for auxiliary lookups. Call before the manager starts.
func SetConversionReader(r ctrlclient.Reader) {
	conversionReader = r
}

// lookupSecret returns (nil, nil) when the reader isn't wired, name is
// empty, or the Secret doesn't exist; other errors propagate.
func lookupSecret(namespace, name string) (*corev1.Secret, error) {
	if conversionReader == nil || name == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), conversionLookupTimeout)
	defer cancel()

	var secret corev1.Secret
	if err := conversionReader.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: name}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("conversion read secret %s/%s: %w", namespace, name, err)
	}
	return &secret, nil
}

// resolveValues prefers `<cr-name>-spec-active`'s data.values (the coalesced
// values written by the v1 reconciler) and falls back to src.Spec.Values
// when the Secret is absent, has no `values` key, or the reader isn't wired.
func resolveValues(src *WeightsAndBiases) (map[string]interface{}, error) {
	secretName := fmt.Sprintf("%s-spec-active", src.Name)
	secret, err := lookupSecret(src.Namespace, secretName)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return src.Spec.Values.Object, nil
	}
	raw, ok := secret.Data["values"]
	if !ok || len(raw) == 0 {
		return src.Spec.Values.Object, nil
	}
	var values map[string]interface{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("decode values from secret %s/%s: %w", src.Namespace, secretName, err)
	}
	return values, nil
}

// ConvertTo converts this WeightsAndBiases (v1) to the Hub version (v2).
func (src *WeightsAndBiases) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv2.WeightsAndBiases)
	logger.Info("ConvertTo: Converting WeightsAndBiases from Spoke version v1 to Hub version v2",
		"source", src.Namespace+"/"+src.Name,
	)

	dst.ObjectMeta = src.ObjectMeta

	if err := applyValueMappings(src, dst); err != nil {
		return err
	}

	// Stash raw v1 chart/values for round-trip recovery in ConvertFrom.
	return stashV1Source(src, dst)
}

// ConvertFrom restores the raw v1 chart/values from the annotations stashed
// by ConvertTo so apiserver's v2 → v1 → v2 admission bounces are lossless.
func (dst *WeightsAndBiases) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv2.WeightsAndBiases)
	logger.Info("ConvertFrom: Converting WeightsAndBiases from Hub version v2 to Spoke version v1",
		"source", src.Namespace+"/"+src.Name,
	)

	dst.ObjectMeta = src.ObjectMeta

	if err := loadV1Source(src, dst); err != nil {
		return err
	}
	return nil
}

func stashV1Source(src *WeightsAndBiases, dst *appsv2.WeightsAndBiases) error {
	chartJSON, err := json.Marshal(src.Spec.Chart.Object)
	if err != nil {
		return fmt.Errorf("marshal v1 chart: %w", err)
	}
	valuesJSON, err := json.Marshal(src.Spec.Values.Object)
	if err != nil {
		return fmt.Errorf("marshal v1 values: %w", err)
	}
	if dst.Annotations == nil {
		dst.Annotations = make(map[string]string)
	}
	dst.Annotations[v1ChartAnnotation] = string(chartJSON)
	dst.Annotations[v1ValuesAnnotation] = string(valuesJSON)
	return nil
}

func loadV1Source(src *appsv2.WeightsAndBiases, dst *WeightsAndBiases) error {
	dst.Spec.Chart = Object{Object: map[string]interface{}{}}
	dst.Spec.Values = Object{Object: map[string]interface{}{}}

	if raw, ok := src.Annotations[v1ChartAnnotation]; ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &dst.Spec.Chart.Object); err != nil {
			return fmt.Errorf("unmarshal %s: %w", v1ChartAnnotation, err)
		}
	}
	if raw, ok := src.Annotations[v1ValuesAnnotation]; ok && raw != "" {
		if err := json.Unmarshal([]byte(raw), &dst.Spec.Values.Object); err != nil {
			return fmt.Errorf("unmarshal %s: %w", v1ValuesAnnotation, err)
		}
	}
	return nil
}
