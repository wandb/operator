package release

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func NewLocalRelease(path string) Release {
	return &LocalRelease{path}
}

type LocalRelease struct {
	codePath string
}

// Generate implements Release.
func (r LocalRelease) Generate(m interface{}) error {
	b, _ := json.Marshal(m)
	cmd := exec.Command("pnpm", "run", "gen", "--json="+string(b))
	cmd.Dir = r.Directory()
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}

func (r LocalRelease) Version() string {
	return r.codePath
}

func (r LocalRelease) Download() error {
	return nil
}

func (r LocalRelease) Install() error {
	cmd := exec.Command("pnpm", "install", "--frozen-lockfile")
	cmd.Dir = r.Directory()
	_, err := cmd.CombinedOutput()
	// fmt.Println(string(output))
	return err
}

func (r LocalRelease) Directory() string {
	return r.codePath
}

func (r LocalRelease) Apply(
	ctx context.Context,
	client client.Client,
	owner v1.Object,
	scheme *runtime.Scheme,
) error {
	folder := path.Join(r.Directory(), "dist")
	resources, err := readManifestResources(folder)
	if err != nil {
		return err
	}

	for _, resource := range resources {
		controllerutil.SetControllerReference(owner, &resource, scheme)

		if resource.GetNamespace() == "" {
			resource.SetNamespace(owner.GetNamespace())
		}

		if err = client.Create(ctx, &resource); err != nil {
			if err = client.Update(ctx, &resource); err != nil {
				return fmt.Errorf("failed to update resource %s: %w", resource.GetName(), err)
			}
		}
	}
	return nil
}
