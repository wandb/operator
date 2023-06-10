package cdk8s

import (
	"context"

	"github.com/Masterminds/semver"
	apiv1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const SupportedVersions = "~1"

func NewManager(
	ctx context.Context,
	wandb *apiv1.WeightsAndBiases,
	client client.Client,
	scheme *runtime.Scheme,
) *Manager {
	c, _ := semver.NewConstraint(SupportedVersions)
	return &Manager{
		versionConstraints: c,
		ctx:                ctx,
		wandb:              wandb,
		Client:             client,
		Scheme:             scheme,
		Config:             config.NewManager(ctx, wandb, client, scheme),
	}
}

type Manager struct {
	versionConstraints *semver.Constraints
	ctx                context.Context
	wandb              *apiv1.WeightsAndBiases
	Client             client.Client
	Scheme             *runtime.Scheme
	Config             *config.Manager
}

func (m Manager) IsUpgrade(release release.Release) bool {
	if m.CurrentVersion().Equal(release.Version()) {
		return false
	}
	withinConstraints, _ := m.versionConstraints.Validate(release.Version())
	if !withinConstraints {
		return false
	}
	return release.Version().GreaterThan(m.CurrentVersion())
}

func (m Manager) IsDowngrade(release release.Release) bool {
	if m.CurrentVersion().Equal(release.Version()) {
		return false
	}
	withinConstraints, _ := m.versionConstraints.Validate(release.Version())
	if !withinConstraints {
		return false
	}
	return release.Version().LessThan(m.CurrentVersion())
}

func (m Manager) HasConfigChanaged() bool {
	return false
}

func (m Manager) CurrentConfig() interface{} {
	return nil
}

func (m Manager) CurrentVersion() *semver.Version {
	return semver.MustParse("1.0.0")
}

func (m Manager) ApplyWithVersion(release release.Release) error {
	if !m.IsUpgrade(release) {
		return nil
	}
	return m.Apply(release, config)
}

func (m Manager) Apply(release release.Release, config interface{}) error {
	err := release.Apply(
		m.ctx,
		m.Client,
		m.wandb,
		m.Scheme,
	)
}
