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

import "embed"

// Each group's CRDs are committed under crds/<group>/ and embedded here.
// The source-of-truth lives outside this package (config/crd/bases for the
// operator's own CRDs; pkg/vendored/<vendor>/crds for upstream) — the
// `sync-crd-embed` Makefile target copies updates into these directories.

//go:embed crds/operator/*.yaml
var operatorCRDs embed.FS

//go:embed crds/redis/*.yaml
var redisCRDs embed.FS

// optionalGroups maps the value used on the --groups CLI flag to the
// matching embedded filesystem. The operator's own CRDs are NOT in here
// because they're always installed regardless of which optional groups
// are requested.
var optionalGroups = map[string]embed.FS{
	"redis": redisCRDs,
}
