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
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Options carries the chart-computed values that get plugged into the
// operator-owned CRDs at install time. Upstream CRDs (redis, clickhouse) are
// applied as-is and ignore these fields.
type Options struct {
	// CertInjectReference is the verbatim value for the
	// cert-manager.io/inject-ca-from annotation on operator-owned CRDs,
	// e.g. "wandb/test-serving-cert". Required for operator CRDs.
	CertInjectReference string

	// WebhookServiceName is the service name written into
	// spec.conversion.webhook.clientConfig.service.name on operator-owned CRDs.
	WebhookServiceName string

	// WebhookServiceNamespace is the namespace written into
	// spec.conversion.webhook.clientConfig.service.namespace on operator-owned CRDs.
	WebhookServiceNamespace string

	// Groups is the set of optional CRD groups to install in addition to
	// the operator's own CRDs (e.g. {"redis", "clickhouse"}). Unknown values are
	// rejected up-front by ParseGroups.
	Groups []string
}

// Validate ensures every option needed to install operator-owned CRDs is set.
// Optional-group CRDs don't depend on these values.
func (o Options) Validate() error {
	var missing []string
	if o.CertInjectReference == "" {
		missing = append(missing, "cert-inject-reference")
	}
	if o.WebhookServiceName == "" {
		missing = append(missing, "webhook-service-name")
	}
	if o.WebhookServiceNamespace == "" {
		missing = append(missing, "webhook-service-namespace")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required option(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// ParseGroups splits a comma-separated --groups flag value, rejecting unknown
// names so misconfiguration fails fast instead of silently skipping CRDs.
func ParseGroups(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		g := strings.TrimSpace(part)
		if g == "" {
			continue
		}
		if _, ok := optionalGroups[g]; !ok {
			known := make([]string, 0, len(optionalGroups))
			for k := range optionalGroups {
				known = append(known, k)
			}
			sort.Strings(known)
			return nil, fmt.Errorf("unknown CRD group %q (known: %s)", g, strings.Join(known, ", "))
		}
		out = append(out, g)
	}
	return out, nil
}

// compose walks every selected CRD source in deterministic order, parses each
// CRD, applies operator-owned mutations, and returns the typed objects. The
// order matches what Render emits to stdout and what Apply uses for client
// calls — invariant so debug-time `render | kubectl diff` is meaningful.
func compose(opts Options) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	type source struct {
		group string
		fsys  embed.FS
	}
	sources := []source{{group: "operator", fsys: operatorCRDs}}
	for _, g := range opts.Groups {
		fsys, ok := optionalGroups[g]
		if !ok {
			// Defense in depth — ParseGroups already rejects unknowns.
			return nil, fmt.Errorf("unknown CRD group %q", g)
		}
		sources = append(sources, source{group: g, fsys: fsys})
	}

	var crds []*apiextensionsv1.CustomResourceDefinition
	for _, src := range sources {
		paths, err := listYAMLs(src.fsys)
		if err != nil {
			return nil, fmt.Errorf("listing %s CRDs: %w", src.group, err)
		}
		for _, path := range paths {
			raw, err := src.fsys.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", path, err)
			}
			docs, err := splitYAMLDocs(raw)
			if err != nil {
				return nil, fmt.Errorf("splitting %s: %w", path, err)
			}
			for _, doc := range docs {
				crd := &apiextensionsv1.CustomResourceDefinition{}
				if err := yaml.Unmarshal(doc, crd); err != nil {
					return nil, fmt.Errorf("unmarshalling CRD from %s: %w", path, err)
				}
				if crd.Name == "" {
					continue // empty doc
				}
				if src.group == "operator" {
					if err := mutateOperatorCRD(crd, opts); err != nil {
						return nil, fmt.Errorf("mutating %s: %w", crd.Name, err)
					}
				}
				crds = append(crds, crd)
			}
		}
	}
	return crds, nil
}

// listYAMLs walks fsys and returns every .yaml file path, sorted, so two
// invocations on the same binary produce byte-identical output.
func listYAMLs(fsys embed.FS) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".yaml") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// splitYAMLDocs handles multi-document YAML files (separated by `---`).
// CRDs with a leading `---` line produce an empty first document, which we
// filter out by checking for a missing kind in the caller.
func splitYAMLDocs(raw []byte) ([][]byte, error) {
	const sep = "\n---"
	chunks := bytes.SplitAfter(raw, []byte(sep))
	// SplitAfter keeps the separator in each chunk; trim them and prepend
	// `---` removal at the head of each.
	var out [][]byte
	for _, chunk := range chunks {
		c := bytes.TrimSpace(chunk)
		// Strip a leading "---" left over from the previous separator.
		c = bytes.TrimPrefix(c, []byte("---"))
		c = bytes.TrimSpace(c)
		if len(c) > 0 {
			out = append(out, c)
		}
	}
	return out, nil
}

// crdsNeedingConversionWebhook lists operator CRDs that require a webhook
// conversion config injected at install time. Add new entries here whenever
// a CRD grows multiple stored versions that need server-side conversion.
var crdsNeedingConversionWebhook = map[string]bool{
	"weightsandbiases.apps.wandb.com": true,
}

// mutateOperatorCRD plugs the chart-computed values into the operator's own
// CRDs: the cert-manager CA-injection annotation, and (for CRDs that need
// conversion) the webhook conversion service identity. Upstream CRDs are not
// mutated.
func mutateOperatorCRD(crd *apiextensionsv1.CustomResourceDefinition, opts Options) error {
	if crd.Annotations == nil {
		crd.Annotations = make(map[string]string, 1)
	}
	crd.Annotations["cert-manager.io/inject-ca-from"] = opts.CertInjectReference

	if crdsNeedingConversionWebhook[crd.Name] {
		crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
			Strategy: apiextensionsv1.WebhookConverter,
			Webhook: &apiextensionsv1.WebhookConversion{
				ConversionReviewVersions: []string{"v1"},
				ClientConfig: &apiextensionsv1.WebhookClientConfig{
					Service: &apiextensionsv1.ServiceReference{
						Name:      opts.WebhookServiceName,
						Namespace: opts.WebhookServiceNamespace,
						Path:      stringPtr("/convert"),
					},
				},
			},
		}
	}
	return nil
}

func stringPtr(s string) *string { return &s }
