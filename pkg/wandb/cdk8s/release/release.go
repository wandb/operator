package release

import (
	"context"
	"fmt"
	"net/url"

	"github.com/Masterminds/semver"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Cdk8sSupportedVersions = "~1"

func ReleaseFromString(version string) (Release, error) {
	if v, err := semver.NewVersion(version); err == nil {
		constraint, _ := semver.NewConstraint(Cdk8sSupportedVersions)
		if !constraint.Check(v) {
			return nil, fmt.Errorf("cdk8s version %s is not supported. Supported versions: %s", v, Cdk8sSupportedVersions)
		}
		return &GithubRelease{
			Repo: "wandb/cdk8s",
			Tag:  v.String(),
		}, nil
	}

	if url, err := url.ParseRequestURI(version); err == nil {
		return NewGitRelease(url, version), nil
	}

	return NewLocalRelease(version), nil
}

// Release represents a cdk8s release
type Release interface {
	// Directory returns the directory where the release is installed
	Directory() string
	// Download downloads the release into the directory
	Download() error
	// Install installs the release into the directory
	Install() error
	// Generate generates the k8s manifests give the provided configuration
	Generate(m map[string]interface{}) error
	// Version returns the version of the release
	Version() string

	Apply(
		ctx context.Context,
		client client.Client,
		owner v1.Object,
		scheme *runtime.Scheme,
	) error
}
