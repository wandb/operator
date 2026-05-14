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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	appsv2 "github.com/wandb/operator/api/v2"
)

var logger = ctrl.Log.WithName("weightsandbiases-conversion")

// ConvertTo converts this WeightsAndBiases (v1) to the Hub version (v2).
func (src *WeightsAndBiases) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*appsv2.WeightsAndBiases)
	logger.Info("ConvertTo: Converting WeightsAndBiases from Spoke version v1 to Hub version v2",
		"source", src.Namespace+"/"+src.Name,
		"target", dst.Namespace+"/"+dst.Name,
	)

	dst.ObjectMeta = src.ObjectMeta

	return applyGlobalMappings(src, dst)
}

// ConvertFrom converts the Hub version (v2) to this WeightsAndBiases (v1).
// v1 reconciliation has been removed, so this direction is no longer supported.
func (dst *WeightsAndBiases) ConvertFrom(_ conversion.Hub) error {
	return errors.New("conversion from v2 to v1 is not supported")
}
