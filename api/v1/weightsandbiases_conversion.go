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
	"encoding/json"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv2 "github.com/wandb/operator/api/v2"
)

var logger = ctrl.Log.WithName("weightsandbiases-conversion")

// Round-trip annotations: kube-apiserver bounces objects through v2 → v1 → v2
// during admission (e.g. mutating-webhook chains), so ConvertFrom must be able
// to reproduce the original v1 chart/values it would otherwise have to invent
// from the typed v2 spec. We stash the original JSON on ConvertTo and read it
// back on ConvertFrom.
const (
	v1ChartAnnotation  = "legacy.operator.wandb.com/v1-chart"
	v1ValuesAnnotation = "legacy.operator.wandb.com/v1-values"
)

// ConvertTo converts this WeightsAndBiases (v1) to the Hub version (v2).
func (src *WeightsAndBiases) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv2.WeightsAndBiases)
	logger.Info("ConvertTo: Converting WeightsAndBiases from Spoke version v1 to Hub version v2",
		"source", src.Namespace+"/"+src.Name,
	)

	dst.ObjectMeta = src.ObjectMeta

	if err := applyGlobalMappings(src, dst); err != nil {
		return err
	}

	// Preserve the original v1 chart/values JSON so a later v2 → v1 → v2
	// round-trip can recover them. Without this, ConvertFrom would have to
	// produce an empty v1 and the subsequent ConvertTo would erase the typed
	// v2 fields we just populated.
	return stashV1Source(src, dst)
}

// ConvertFrom converts the Hub version (v2) to this WeightsAndBiases (v1).
//
// v1 is no longer a reconciliation target, but apiserver still bounces
// objects through this direction during admission round-trips. We recover
// the original v1 chart/values from the annotations stashed by ConvertTo so
// the round-trip is lossless.
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
