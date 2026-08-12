/*
Copyright 2026.

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

package controller

import (
	"context"
	stderrors "errors"
	"fmt"

	wandbcomv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	helmReleaseNameAnnotation = "meta.helm.sh/release-name"
	standaloneConsoleRelease  = "console"
)

var errStandaloneConsoleOwnsService = stderrors.New("standalone Console release owns the bundled Console service")

// validateConsoleServiceOwnership prevents the main W&B release from enabling
// its bundled Console when the standalone Console Helm release already owns the
// Service that the bundled chart would render.
func validateConsoleServiceOwnership(
	ctx context.Context,
	k8sClient client.Client,
	wandb *wandbcomv1.WeightsAndBiases,
	desiredSpec *spec.Spec,
) error {
	if desiredSpec == nil || !desiredSpec.Values.GetBool("console.install") {
		return nil
	}

	serviceKey := types.NamespacedName{
		Name:      wandb.Name + "-console",
		Namespace: wandb.Namespace,
	}
	service := &corev1.Service{}
	if err := k8sClient.Get(ctx, serviceKey, service); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("check Console Service ownership: %w", err)
	}

	ownerRelease := service.Annotations[helmReleaseNameAnnotation]
	if ownerRelease != standaloneConsoleRelease || ownerRelease == wandb.Name {
		return nil
	}

	return fmt.Errorf(
		"%w: release %q owns Service %q in namespace %q",
		errStandaloneConsoleOwnsService,
		standaloneConsoleRelease,
		service.Name,
		service.Namespace,
	)
}

func isConsoleServiceOwnershipConflict(err error) bool {
	return stderrors.Is(err, errStandaloneConsoleOwnsService)
}
