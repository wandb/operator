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
)

func NewLocalRelease(path string) Release {
	return &LocalRelease{path}
}

type LocalRelease struct {
	codePath string
}

// Generate implements Release.
func (r LocalRelease) Generate(m map[string]interface{}) error {
	b, _ := json.Marshal(m)

	dist := path.Join(r.Directory(), "dist")
	rm := exec.Command("rm", "-rf", dist)
	rm.Run()

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
	cmd := exec.Command("kubectl", "apply", "-f", folder, "--prune", "-l", "app=wandb")
	cmd.Dir = r.Directory()
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}
