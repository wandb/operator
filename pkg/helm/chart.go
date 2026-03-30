package helm

import (
	"fmt"
	"time"

	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

const (
	secretsStorageDriver = "secrets"
	maxReleasesToKeep    = 10
)

type ActionableChartInterface interface {
	Apply(
		chart *chart.Chart,
		values map[string]interface{},
	) (release.Releaser, error)
	Install(
		chart *chart.Chart,
		values map[string]interface{},
	) (release.Releaser, error)
	History() ([]release.Releaser, error)
	Rollback(version int) error
	Upgrade(chart *chart.Chart, values map[string]interface{}) (release.Releaser, error)
	Uninstall() (*release.UninstallReleaseResponse, error)
	GetRelease(version int) (release.Releaser, error)
}

// InitConfig returns a helm action configuration. Namespace is used to determine
// where to store the versions
func InitConfig(namespace string) (*cli.EnvSettings, *action.Configuration, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)
	config := new(action.Configuration)
	err := config.Init(
		settings.RESTClientGetter(),
		settings.Namespace(),
		secretsStorageDriver,
	)
	config.Releases.MaxHistory = maxReleasesToKeep
	return settings, config, err
}

func NewActionableChart(releaseName string, namespace string) (*ActionableChart, error) {
	if err := chartutil.ValidateReleaseName(releaseName); err != nil {
		return nil, fmt.Errorf("release name %q", releaseName)
	}

	_, config, err := InitConfig(namespace)
	if err != nil {
		return nil, err
	}

	return &ActionableChart{
		releaseName: releaseName,
		config:      config,
		namespace:   namespace,
	}, nil
}

type ActionableChart struct {
	releaseName string
	config      *action.Configuration
	namespace   string
}

func (c *ActionableChart) isInstalled() bool {
	h, err := c.config.Releases.History(c.releaseName)
	if err != nil || len(h) < 1 {
		return false
	}

	// Convert Releaser slice to concrete releases for sorting
	concrete := make([]*releasev1.Release, 0, len(h))
	for _, r := range h {
		if rel, ok := r.(*releasev1.Release); ok {
			concrete = append(concrete, rel)
		}
	}
	if len(concrete) == 0 {
		return false
	}

	releaseutil.Reverse(concrete, releaseutil.SortByRevision)
	st := concrete[0].Info.Status

	return st != releasecommon.StatusUninstalled
}

func (c *ActionableChart) Apply(
	chart *chart.Chart,
	values map[string]interface{},
) (release.Releaser, error) {
	if c.isInstalled() {
		return c.Upgrade(chart, values)
	}
	return c.Install(chart, values)
}

func (c *ActionableChart) Install(
	chart *chart.Chart,
	values map[string]interface{},
) (release.Releaser, error) {
	client := action.NewInstall(c.config)
	client.ReleaseName = c.releaseName
	client.Namespace = c.namespace
	return client.Run(chart, values)
}

func (c *ActionableChart) History() ([]release.Releaser, error) {
	return c.config.Releases.History(c.releaseName)
}

func (c *ActionableChart) Rollback(version int) error {
	client := action.NewRollback(c.config)
	return client.Run(c.releaseName)
}

func (c *ActionableChart) Upgrade(chart *chart.Chart, values map[string]interface{}) (release.Releaser, error) {
	client := action.NewUpgrade(c.config)
	client.Namespace = c.namespace
	client.MaxHistory = maxReleasesToKeep
	return client.Run(c.releaseName, chart, values)
}

func (c *ActionableChart) Uninstall() (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(c.config)
	client.WaitStrategy = kube.LegacyStrategy
	client.Timeout = 600 * time.Second
	return client.Run(c.releaseName)
}

func (c *ActionableChart) GetRelease(version int) (release.Releaser, error) {
	return c.config.Releases.Get(c.releaseName, version)
}
