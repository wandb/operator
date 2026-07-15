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

package crdinstaller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// FieldManager is the SSA owner string used for every CRD this binary
// applies. cert-manager's CA injector uses its own field manager
// ("cainjector") and owns the conversion webhook caBundle field, so SSA
// here leaves caBundle alone on re-apply.
const FieldManager = "wandb-crd-installer"

// Apply installs (or updates) every selected CRD via server-side apply. It is
// idempotent — re-running with the same options is a no-op for CRDs that
// already match.
func Apply(ctx context.Context, opts Options, client apiextensionsclient.Interface, logger *slog.Logger) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	if logger == nil {
		logger = slog.Default()
	}
	crds, err := compose(opts)
	if err != nil {
		return err
	}

	for _, crd := range crds {
		if err := applyOne(ctx, client, crd, logger); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, client apiextensionsclient.Interface, crd *apiextensionsv1.CustomResourceDefinition, logger *slog.Logger) error {
	// SSA expects the patch body to declare apiVersion + kind even though
	// the typed object already knows them.
	crd.APIVersion = apiextensionsv1.SchemeGroupVersion.String()
	crd.Kind = "CustomResourceDefinition"
	// Strip fields that are populated by the server. Including them in an
	// SSA patch either gets rejected or forces ownership of immutable bits.
	crd.ResourceVersion = ""
	crd.UID = ""
	crd.ManagedFields = nil
	crd.Generation = 0
	crd.CreationTimestamp = metav1.Time{}

	data, err := json.Marshal(crd)
	if err != nil {
		return fmt.Errorf("marshalling CRD %s: %w", crd.Name, err)
	}

	force := true
	applied, err := client.ApiextensionsV1().CustomResourceDefinitions().Patch(
		ctx,
		crd.Name,
		types.ApplyPatchType,
		data,
		metav1.PatchOptions{
			FieldManager: FieldManager,
			Force:        &force,
		},
	)
	if err != nil {
		return fmt.Errorf("applying CRD %s: %w", crd.Name, err)
	}
	logger.Info("applied CRD", "name", applied.Name, "resourceVersion", applied.ResourceVersion)
	return nil
}
