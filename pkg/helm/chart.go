package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
)

const (
	secretsStorageDriver = "secrets"
)

var (
	noopLogger = func(_ string, _ ...interface{}) {}
)

// GetConfig returns a helm action configuration. Namespace is used to determine
// where to store the versions
func InitConfig(namespace string) (*cli.EnvSettings, *action.Configuration, error) {
	settings := cli.New()
	config := new(action.Configuration)
	err := config.Init(
		settings.RESTClientGetter(),
		settings.Namespace(),
		secretsStorageDriver,
		noopLogger,
	)
	return settings, config, err
}

func DownloadChart(repoURL string, name string) (string, error) {
	settings := cli.New()
	providers := getter.All(settings)

	entry := new(repo.Entry)
	entry.URL = repoURL
	entry.Name = name

	file := repo.NewFile()
	file.Update(entry)

	chartRepo, err := repo.NewChartRepository(entry, providers)
	if err != nil {
		return "", err
	}

	_, err = chartRepo.DownloadIndexFile()
	if err != nil {
		return "", err
	}

	chartURL, err := repo.FindChartInRepoURL(
		repoURL, name,
		"", "", "", "",
		providers,
	)
	if err != nil {
		return "", err
	}

	client := action.NewPull()
	client.Settings = settings
	return client.Run(chartURL)
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

	releaseutil.Reverse(h, releaseutil.SortByRevision)
	rel := h[0]
	st := rel.Info.Status
	if st == release.StatusUninstalled || st == release.StatusFailed {
		return false
	}
	return true
}

func (c *ActionableChart) Apply(chart *chart.Chart, values map[string]interface{}) (*release.Release, error) {
	if c.isInstalled() {
		return c.Upgrade(chart, values)
	}
	return c.Install(chart, values)
}

func (c *ActionableChart) Install(chart *chart.Chart, values map[string]interface{}) (*release.Release, error) {
	client := action.NewInstall(c.config)
	client.ReleaseName = c.releaseName
	client.Namespace = c.namespace
	return client.Run(chart, values)
}

func (c *ActionableChart) History() ([]*release.Release, error) {
	return c.config.Releases.History(c.releaseName)
}

func (c *ActionableChart) Rollback(version int) error {
	client := action.NewRollback(c.config)
	return client.Run(c.releaseName)
}

func (c *ActionableChart) Upgrade(chart *chart.Chart, values map[string]interface{}) (*release.Release, error) {
	client := action.NewUpgrade(c.config)
	client.Namespace = c.namespace
	return client.Run(c.releaseName, chart, values)
}

func (c *ActionableChart) Uninstall() (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(c.config)
	return client.Run(c.releaseName)
}

func (c *ActionableChart) GetRelease(version int) (*release.Release, error) {
	return c.config.Releases.Get(c.releaseName, version)
}
