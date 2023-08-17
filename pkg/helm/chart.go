package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
)

const (
	secretsStorageDriver = "secrets"
)

var (
	noopLogger = func(_ string, _ ...interface{}) {}
)

// GetConfig returns a helm action configuration. Namespace is used to determine
// where to store the versions
func GetConfig(namespace string) (*action.Configuration, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)
	config := new(action.Configuration)
	err := config.Init(
		settings.RESTClientGetter(),
		settings.Namespace(),
		secretsStorageDriver,
		noopLogger,
	)
	config.Releases.MaxHistory = 20
	return config, err
}

func NewActionableChart(chart *chart.Chart, releaseName string, namespace string) (*ActionableChart, error) {
	if err := chartutil.ValidateReleaseName(releaseName); err != nil {
		return nil, fmt.Errorf("release name %q", releaseName)
	}

	config, err := GetConfig(namespace)
	if err != nil {
		return nil, err
	}

	return &ActionableChart{
		releaseName: releaseName,
		chart:       chart,
		config:      config,
	}, nil
}

type ActionableChart struct {
	releaseName string
	chart       *chart.Chart
	config      *action.Configuration
	namespace   string
}

func (c *ActionableChart) isInstalled() bool {
	h, err := c.config.Releases.History(c.releaseName)
	if err != nil || len(h) < 1 {
		return false
	}

	releaseutil.Reverse(h, releaseutil.SortByRevision)
	rel := h[0]
	st := rel.Info.Status
	if st == release.StatusUninstalled || st == release.StatusFailed {
		return false
	}
	return true
}

func (c *ActionableChart) Apply(values map[string]interface{}) (*release.Release, error) {
	if c.isInstalled() {
		return c.Upgrade(values)
	}
	return c.Install(values)
}

func (c *ActionableChart) Install(values map[string]interface{}) (*release.Release, error) {
	client := action.NewInstall(c.config)
	client.ReleaseName = c.releaseName
	client.Namespace = c.namespace
	return client.Run(c.chart, values)
}

func (c *ActionableChart) History() ([]*release.Release, error) {
	return c.config.Releases.History(c.releaseName)
}

func (c *ActionableChart) Rollback(version int) error {
	client := action.NewRollback(c.config)
	return client.Run(c.releaseName)
}

func (c *ActionableChart) Upgrade(values map[string]interface{}) (*release.Release, error) {
	client := action.NewUpgrade(c.config)
	client.Namespace = c.namespace
	return client.Run(c.releaseName, c.chart, values)
}

func (c *ActionableChart) Uninstall() (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(c.config)
	return client.Run(c.releaseName)
}

func (c *ActionableChart) GetRelease(version int) (*release.Release, error) {
	return c.config.Releases.Get(c.releaseName, version)
}
