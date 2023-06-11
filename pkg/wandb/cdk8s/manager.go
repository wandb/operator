package cdk8s

import (
	"context"

	"github.com/Masterminds/semver"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
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

func (m Manager) GetLatestSupportedRelease() (release.Release, error) {
	return nil, nil
}

func (m Manager) GetDownloadedRelease() (release.Release, error) {
	return nil, nil
}

func (m Manager) GetSpecRelease() release.Release {
	version := m.wandb.Spec.Cdk8sVersion
	if v, err := semver.NewVersion(version); err == nil {
		return &release.GithubRelease{
			Repo: "wandb/cdk8s",
			Tag:  v.String(),
		}
	}
	return release.NewLocalRelease(version)
}

func (m Manager) GetLatestRelease() (release.Release, error) {
	return release.GetLatestRelease(m.wandb)
}

func (m Manager) InstalledReleases() []release.Release {
	return []release.Release{}
}
