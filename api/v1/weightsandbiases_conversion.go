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
	"errors"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv2 "github.com/wandb/operator/api/v2"
)

// ConvertTo converts this WeightsAndBiases (v1) to the Hub version (v2).
func (src *WeightsAndBiases) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv2.WeightsAndBiases)
	log.Printf("ConvertTo: Converting WeightsAndBiases from Spoke version v1 to Hub version v2;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	chart, err := src.Spec.Chart.MarshalJSON()
	if err != nil {
		return err
	}
	values, err := src.Spec.Values.MarshalJSON()
	if err != nil {
		return err
	}

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	dst.ObjectMeta = src.ObjectMeta

	if dst.Annotations == nil {
		dst.Annotations = make(map[string]string)
	}
	dst.Annotations["legacy.operator.wandb.com/version"] = "v1"
	dst.Annotations["legacy.operator.wandb.com/chart"] = string(chart)
	dst.Annotations["legacy.operator.wandb.com/values"] = string(values)

	return nil
}

// ConvertFrom converts the Hub version (v2) to this WeightsAndBiases (v1).
func (dst *WeightsAndBiases) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*appsv2.WeightsAndBiases)
	log.Printf("ConvertFrom: Converting WeightsAndBiases from Hub version v2 to Spoke version v1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	if src.Annotations["legacy.operator.wandb.com/version"] != "v1" {
		return errors.New("cannot convert from non-v1 version")
	}

	if chart, ok := src.Annotations["legacy.operator.wandb.com/chart"]; !ok || chart == "" {
		return errors.New("missing chart annotation")
	}
	err := dst.Spec.Chart.UnmarshalJSON([]byte(src.Annotations["legacy.operator.wandb.com/chart"]))
	if err != nil {
		return err
	}

	if values, ok := src.Annotations["legacy.operator.wandb.com/values"]; !ok || values == "" {
		return errors.New("missing values annotation")
	}
	err = dst.Spec.Values.UnmarshalJSON([]byte(src.Annotations["legacy.operator.wandb.com/values"]))
	if err != nil {
		return err
	}

	// Copy ObjectMeta to preserve name, namespace, labels, etc.
	dst.ObjectMeta = src.ObjectMeta
	delete(dst.Annotations, "legacy.operator.wandb.com/chart")
	delete(dst.Annotations, "legacy.operator.wandb.com/values")

	return nil
}
