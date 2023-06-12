package release

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	v1 "github.com/wandb/operator/api/v1"
)

const SupportedVersions = "~1"

func NewManager(
	ctx context.Context,
	wandb *v1.WeightsAndBiases,
) *Manager {
	c, _ := semver.NewConstraint(SupportedVersions)
	return &Manager{
		ctx:                ctx,
		wandb:              wandb,
		versionConstraints: c,
	}
}

type Manager struct {
	ctx                context.Context
	wandb              *v1.WeightsAndBiases
	versionConstraints *semver.Constraints
}

func (m Manager) GetLatestSupportedRelease() (Release, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m Manager) GetLatestDownloadedRelease() (Release, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m Manager) GetSpecRelease() Release {
	version := m.wandb.Spec.Cdk8sVersion
	if v, err := semver.NewVersion(version); err == nil {
		return &GithubRelease{
			Repo: "wandb/cdk8s",
			Tag:  v.String(),
		}
	}
	return NewLocalRelease(version)
}

func (m Manager) GetLatestRelease() (Release, error) {
	return GetLatestRelease(m.wandb)
}

func (m Manager) InstalledReleases() []Release {
	return []Release{}
}
