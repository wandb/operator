package release

import (
	"context"
	"os"
	"path"

	apiv1 "github.com/wandb/operator/api/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetLatestRelease(wandb *apiv1.WeightsAndBiases) (Release, error) {
	// Use a local directory for the release. Useful for development
	if wandb.Spec.ReleasePath != "" {
		return &LocalRelease{wandb.Spec.ReleasePath, "dev"}, nil
	}
	return GetLatestGithubRelease("wandb/cdk8s")
}

// InstallDirectory returns the directory where cdk8s versions are installed
func InstallDirectory() string {
	dirname, _ := os.UserHomeDir()
	path := path.Join(dirname, "operator", "cdk8s")
	os.MkdirAll(path, 0755)
	return path
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
	Generate(m interface{}) error
	// Version returns the version of the release
	Version() string

	Apply(
		ctx context.Context,
		client client.Client,
		owner v1.Object,
		scheme *runtime.Scheme,
	) error
}
