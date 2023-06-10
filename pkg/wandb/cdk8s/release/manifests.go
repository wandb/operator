package release

import (
	"io/fs"
	"os"
	"path"
	"strings"

	corev1 "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/yaml"
)

// readManifestResources reads the k8s manifests from the given directory
func readManifestResources(dir string) ([]corev1.Unstructured, error) {
	files, err := fs.ReadDir(os.DirFS(dir), ".")
	if err != nil {
		return nil, err
	}

	objs := []corev1.Unstructured{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		yamlFile, err := os.ReadFile(path.Join(dir, f.Name()))
		if err != nil {
			continue
		}

		manifests := strings.Split(string(yamlFile), "---")
		for _, manifest := range manifests {
			if len(strings.TrimSpace(manifest)) == 0 {
				continue
			}

			jsonContent, err := yaml.YAMLToJSON([]byte(manifest))
			if err != nil {
				continue
			}

			obj := &corev1.Unstructured{}
			if err := obj.UnmarshalJSON(jsonContent); err != nil {
				continue
			}

			objs = append(objs, *obj)
		}
	}

	return objs, nil
}
