package release

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetLatestGithubRelease(repo string) (Release, error) {
	return &GithubRelease{repo, ""}, nil
}

type GithubRelease struct {
	Repo string
	Tag  string
}

func (*GithubRelease) Directory() string {
	panic("unimplemented")
}

func (*GithubRelease) Download() error {
	panic("unimplemented")
}

func (*GithubRelease) Generate(m map[string]interface{}) error {
	panic("unimplemented")
}

func (*GithubRelease) Install() error {
	panic("unimplemented")
}

func (*GithubRelease) Version() string {
	panic("unimplemented")
}

func (*GithubRelease) Apply(
	ctx context.Context,
	client client.Client,
	owner v1.Object,
	scheme *runtime.Scheme,
) error {
	panic("unimplemented")
}
