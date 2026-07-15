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

package reconciler

import (
	"context"
	"errors"
	"fmt"
	"sort"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	appsv1 "k8s.io/api/apps/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// legacyV1DeploymentSuffixes names the Deployments the v1 Helm chart
// leaves behind after a CR has been converted to v2. The v2 reconciler
// creates its own Application-owned Deployments under different names,
// so these are orphans that must be deleted explicitly — they have no
// owner reference pointing at the v2 CR, so garbage collection won't
// catch them.
var legacyV1DeploymentSuffixes = []string{
	"app-bc",
	"console-bc",
	"executor-bc",
	"parquet-bc",
	"weave-bc",
}

func buildDesiredAppNames(manifest serverManifest.Manifest) map[string]bool {
	out := make(map[string]bool)
	for _, app := range sortedManifestApplications(manifest) {
		if len(app.Features) > 0 && !manifest.FeaturesEnabled(app.Features) {
			continue
		}
		out[app.Name] = true
	}
	return out
}

// deploymentsHealthy reports whether every manifest-desired application's
// Deployment is fully rolled out, plus the sorted names still blocking. It
// reads live Deployments rather than status.wandb.applications so the legacy
// cleanup gate cannot act on a stale status snapshot.
func deploymentsHealthy(
	ctx context.Context,
	c ctrlClient.Client,
	namespace string,
	desiredAppNames map[string]bool,
) (bool, []string) {
	if len(desiredAppNames) == 0 {
		return false, nil
	}
	var notReady []string
	for name := range desiredAppNames {
		dep := &appsv1.Deployment{}
		if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep); err != nil {
			notReady = append(notReady, name)
			continue
		}
		if dep.Status.ObservedGeneration != dep.Generation ||
			dep.Status.ReadyReplicas != dep.Status.Replicas ||
			dep.Status.Replicas == 0 {
			notReady = append(notReady, name)
		}
	}
	sort.Strings(notReady)
	return len(notReady) == 0, notReady
}

func cleanupLegacyV1Deployments(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	logger := logx.GetSlog(ctx)
	var errs []error
	for _, suffix := range legacyV1DeploymentSuffixes {
		name := fmt.Sprintf("%s-%s", wandb.Name, suffix)
		dep := &appsv1.Deployment{}
		if err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: wandb.Namespace}, dep); err != nil {
			if apiErrors.IsNotFound(err) {
				continue
			}
			errs = append(errs, fmt.Errorf("get legacy deployment %s: %w", name, err))
			continue
		}
		logger.Info("Deleting legacy v1 deployment", "deployment", name)
		if err := c.Delete(ctx, dep, ctrlClient.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apiErrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete legacy deployment %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
