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
	"fmt"
	"io"

	"sigs.k8s.io/yaml"
)

// Render emits every selected CRD as YAML to out, separated by `---`, in a
// deterministic order so two invocations on the same binary produce byte-
// identical output. This is the inspection path: pipe through `kubectl apply`,
// `kubectl diff`, `yq`, etc. for debugging.
func Render(_ context.Context, opts Options, out io.Writer) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	crds, err := compose(opts)
	if err != nil {
		return err
	}
	for i, crd := range crds {
		// Clear ManagedFields and ResourceVersion so render output stays
		// reproducible and doesn't carry junk from typed defaulting.
		crd.ManagedFields = nil
		crd.ResourceVersion = ""

		data, err := yaml.Marshal(crd)
		if err != nil {
			return fmt.Errorf("marshalling %s: %w", crd.Name, err)
		}
		if i > 0 {
			if _, err := io.WriteString(out, "---\n"); err != nil {
				return err
			}
		}
		if _, err := out.Write(data); err != nil {
			return err
		}
	}
	return nil
}
