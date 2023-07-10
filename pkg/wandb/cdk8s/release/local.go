package release

import (
	"context"

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
	return PnpmGenerate(r.Directory(), m)
}

func (r LocalRelease) Version() string {
	return r.codePath
}

func (r LocalRelease) Download() error {
	return nil
}

func (r LocalRelease) Install() error {
	return PnpmInstall(r.Directory())
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
	return KubectlApply(r.Directory(), owner.GetNamespace())
}
