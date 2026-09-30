package manifest

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

// LegacyManifestVersion is the permanent default for unversioned artifacts.
const LegacyManifestVersion = 1

// Registration promises both decoding and reconciliation support for a contract.
var manifestDecoders = map[int]func(map[string][]byte) (Manifest, error){
	LegacyManifestVersion: decodeVersion1,
}

// SupportedVersions returns a sorted copy of the contracts this build implements.
func SupportedVersions() []int {
	return slices.Sorted(maps.Keys(manifestDecoders))
}

// InvalidManifestVersionError means an explicit declaration is not a valid
// positive decimal integer, or declarations in an artifact disagree.
type InvalidManifestVersionError struct {
	File   string
	Detail string
}

func (e *InvalidManifestVersionError) Error() string {
	return fmt.Sprintf("invalid manifestVersion in %q: %s", e.File, e.Detail)
}

// UnsupportedManifestVersionError means the version is valid but unimplemented.
type UnsupportedManifestVersionError struct {
	Version   int
	Supported []int
}

func (e *UnsupportedManifestVersionError) Error() string {
	return fmt.Sprintf("manifestVersion %d is unsupported; this operator supports %v", e.Version, e.Supported)
}

// ManifestDecodeError distinguishes malformed YAML or payloads from fetch errors.
type ManifestDecodeError struct {
	File string
	Err  error
}

func (e *ManifestDecodeError) Error() string {
	return fmt.Sprintf("decode manifest file %q: %v", e.File, e.Err)
}

func (e *ManifestDecodeError) Unwrap() error { return e.Err }

// ValidateVersion also protects callers that construct a Manifest directly.
// Only the wire loader defaults an absent declaration; zero is not an alias.
func ValidateVersion(version int) error {
	if version <= 0 {
		return &InvalidManifestVersionError{Detail: "expected a positive version on a decoded manifest"}
	}
	if _, ok := manifestDecoders[version]; !ok {
		return &UnsupportedManifestVersionError{Version: version, Supported: SupportedVersions()}
	}
	return nil
}

// VersionExplicit reports whether the loaded artifact declared its version.
func (m Manifest) VersionExplicit() bool { return m.versionExplicit }

// decodeManifestFiles resolves metadata across the whole artifact before
// selecting a decoder. A fragment without metadata inherits the artifact version.
func decodeManifestFiles(files map[string][]byte) (Manifest, error) {
	version, source := 0, ""
	for _, name := range slices.Sorted(maps.Keys(files)) {
		declared, err := readManifestVersion(name, files[name])
		if err != nil {
			return Manifest{}, err
		}
		if declared == 0 {
			continue
		}
		if version != 0 && version != declared {
			return Manifest{}, &InvalidManifestVersionError{File: name, Detail: fmt.Sprintf("version %d conflicts with version %d in %q", declared, version, source)}
		}
		version, source = declared, name
	}
	explicit := version != 0
	if !explicit {
		version = LegacyManifestVersion
	}
	if err := ValidateVersion(version); err != nil {
		return Manifest{}, err
	}
	m, err := manifestDecoders[version](files)
	if err != nil {
		return Manifest{}, err
	}
	m.ManifestVersion, m.versionExplicit = version, explicit
	return m, nil
}

func decodeVersion1(files map[string][]byte) (Manifest, error) {
	var result Manifest
	for _, name := range slices.Sorted(maps.Keys(files)) {
		var fragment Manifest
		if err := yaml.Unmarshal(files[name], &fragment); err != nil {
			return Manifest{}, &ManifestDecodeError{File: name, Err: err}
		}
		mergeSimple(&result, &fragment)
	}
	return result, nil
}

func readManifestVersion(file string, data []byte) (int, error) {
	decoder := yamlv3.NewDecoder(bytes.NewReader(data))
	var document yamlv3.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return 0, nil
		}
		return 0, &ManifestDecodeError{File: file, Err: err}
	}
	// A file has one payload document. Never inspect metadata from a different
	// document than the one the version-1 payload decoder consumes.
	var extra yamlv3.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple YAML documents are not supported")
		}
		return 0, &ManifestDecodeError{File: file, Err: err}
	}
	if len(document.Content) == 0 {
		return 0, nil
	}
	return versionFromMapping(file, document.Content[0], map[*yamlv3.Node]bool{})
}

func versionFromMapping(file string, root *yamlv3.Node, visiting map[*yamlv3.Node]bool) (int, error) {
	invalid := func(detail string) (int, error) {
		return 0, &InvalidManifestVersionError{File: file, Detail: detail}
	}
	if visiting[root] {
		return invalid("cyclic YAML merge")
	}
	visiting[root] = true
	defer delete(visiting, root)
	if root.Kind == yamlv3.AliasNode {
		return versionFromMapping(file, root.Alias, visiting)
	}
	if root.Kind != yamlv3.MappingNode {
		return 0, nil // Let the existing payload decoder diagnose its shape.
	}
	version, found := 0, false
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Kind == yamlv3.AliasNode && strings.EqualFold(key.Alias.Value, "manifestVersion") {
			return invalid("manifestVersion must not use an aliased key")
		}
		if strings.EqualFold(key.Value, "manifestVersion") && key.Value != "manifestVersion" {
			return invalid("the version key must be spelled manifestVersion")
		}
		if key.Value == "manifestVersion" {
			if found {
				return invalid("duplicate manifestVersion key")
			}
			found = true
			if value.Kind != yamlv3.ScalarNode || value.Tag != "!!int" || value.Value == "" || value.Value[0] < '1' || value.Value[0] > '9' {
				return invalid("expected a positive decimal integer scalar")
			}
			for _, ch := range value.Value {
				if ch < '0' || ch > '9' {
					return invalid("expected a positive decimal integer scalar")
				}
			}
			parsed, err := strconv.ParseInt(value.Value, 10, 32)
			if err != nil {
				return invalid("version exceeds the supported integer representation")
			}
			version = int(parsed)
		}
		if key.Tag == "!!merge" {
			// Existing payload aliases remain supported, but version metadata
			// must be explicit so YAML merging cannot hide a declaration.
			merged := []*yamlv3.Node{value}
			if value.Kind == yamlv3.SequenceNode {
				merged = value.Content
			}
			for _, node := range merged {
				v, err := versionFromMapping(file, node, visiting)
				if err != nil {
					return 0, err
				}
				if v != 0 {
					return invalid("manifestVersion must not be supplied by a YAML merge")
				}
			}
		}
	}
	return version, nil
}
