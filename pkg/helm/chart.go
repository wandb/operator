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

	// Find the release with the highest revision using the Accessor interface,
	// which handles any concrete release type (not just v1).
	var latest release.Accessor
	for _, r := range h {
		acc, err := release.NewAccessor(r)
		if err != nil {
			continue
		}
		if latest == nil || acc.Version() > latest.Version() {
			latest = acc
		}
	}
	if latest == nil {
		return false
	}

	return latest.Status() != releasecommon.StatusUninstalled.String()
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
	client.WaitStrategy = kube.HookOnlyStrategy
	client.ServerSideApply = false
	return client.Run(chart, values)
}

func (c *ActionableChart) History() ([]release.Releaser, error) {
	return c.config.Releases.History(c.releaseName)
}

func (c *ActionableChart) Rollback(version int) error {
	client := action.NewRollback(c.config)
	client.WaitStrategy = kube.HookOnlyStrategy
	client.ServerSideApply = "false"
	return client.Run(c.releaseName)
}

func (c *ActionableChart) Upgrade(chart *chart.Chart, values map[string]interface{}) (release.Releaser, error) {
	client := action.NewUpgrade(c.config)
	client.Namespace = c.namespace
	client.MaxHistory = maxReleasesToKeep
	client.WaitStrategy = kube.HookOnlyStrategy
	client.ServerSideApply = "false"
	return client.Run(c.releaseName, chart, values)
}

func (c *ActionableChart) Uninstall() (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(c.config)
	client.WaitStrategy = kube.HookOnlyStrategy
	client.Timeout = 600 * time.Second
	return client.Run(c.releaseName)
}

func (c *ActionableChart) GetRelease(version int) (release.Releaser, error) {
	return c.config.Releases.Get(c.releaseName, version)
}
