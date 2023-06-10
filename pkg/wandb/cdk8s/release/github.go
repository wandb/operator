package release

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetLatestGithubRelease(repo string) (*GithubRelease, error) {
	return &GithubRelease{repo, ""}, nil
}

type GithubRelease struct {
	repo string
	tag  string
}

func (*GithubRelease) Apply(ctx context.Context, client client.Client, owner v1.Object, scheme *runtime.Scheme) error {
	panic("unimplemented")
}

func (*GithubRelease) Directory() string {
	panic("unimplemented")
}

func (*GithubRelease) Download() error {
	panic("unimplemented")
}

func (*GithubRelease) Generate(m interface{}) error {
	panic("unimplemented")
}

func (*GithubRelease) Install() error {
	panic("unimplemented")
}

func (*GithubRelease) Version() string {
	panic("unimplemented")
}

func NewLocalRelease(path string) Release {
	return &LocalRelease{path, "dev"}
}
