package charts

import (
	"context"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/helm"
	"github.com/wandb/operator/pkg/wandb/spec"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RepoRelease struct {
	URL  string `validate:"required,url" json:"url"`
	Name string `validate:"required" json:"name"`

	// If version is not set, download latest.
	Version string `json:"version"`

	Password string `json:"password"`
	Username string `json:"username"`
}

func (r RepoRelease) Chart() (*chart.Chart, error) {
	local, err := r.ToLocalRelease()
	if err != nil {
		return nil, err
	}
	return local.Chart()
}

func (c RepoRelease) Validate() error {
	return validator.New().Struct(c)
}

func (r RepoRelease) ToLocalRelease() (*LocalRelease, error) {
	chartPath, err := r.downloadChart()
	if err != nil {
		return nil, err
	}

	local := new(LocalRelease)
	local.Path = chartPath
	return local, nil
}

func (r RepoRelease) Apply(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config spec.Values,
) error {
	local, err := r.ToLocalRelease()
	if err != nil {
		return err
	}
	return local.Apply(ctx, c, wandb, scheme, config)
}

func (r RepoRelease) Prune(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config spec.Values,
) error {
	local, err := r.ToLocalRelease()
	if err != nil {
		return err
	}
	return local.Prune(ctx, c, wandb, scheme, config)
}

func (r RepoRelease) downloadChart() (string, error) {
	entry := new(repo.Entry)
	entry.URL = r.URL
	entry.Name = r.Name
	entry.Username = r.Username
	entry.Password = r.Password

	file := repo.NewFile()
	file.Update(entry)

	// Initialize Helm CLI settings respecting environment variables
	settings := cli.New()
	if helmCache := os.Getenv("HELM_CACHE_HOME"); helmCache != "" {
		settings.RepositoryCache = filepath.Join(helmCache, "repository")
	}
	if helmConfig := os.Getenv("HELM_CONFIG_HOME"); helmConfig != "" {
		settings.RepositoryConfig = filepath.Join(helmConfig, "repositories.yaml")
	}

	providers := getter.All(settings)
	chartRepo, err := repo.NewChartRepository(entry, providers)
	if err != nil {
		return "", err
	}
	_, err = chartRepo.DownloadIndexFile()
	if err != nil {
		return "", err
	}
	chartURL, err := repo.FindChartInRepoURL(
		entry.URL, entry.Name, r.Version,
		"", "", "",
		providers,
	)
	if err != nil {
		return "", err
	}

	_, cfg, err := helm.InitConfig("")
	if err != nil {
		return "", err
	}

	client := downloader.ChartDownloader{
		Verify:  downloader.VerifyNever,
		Getters: getter.All(settings),
		Options: []getter.Option{
			getter.WithBasicAuth(r.Username, r.Password),
			// TODO: Add support for other auth methods
			// getter.WithPassCredentialsAll(r.PassCredentialsAll),
			// getter.WithTLSClientConfig(r.CertFile, r.KeyFile, r.CaFile),
			// getter.WithInsecureSkipVerifyTLS(r.InsecureSkipTLSverify),
		},
		RegistryClient:   cfg.RegistryClient,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
	}

	// Use HELM_DATA_HOME as the destination directory if set, or fallback to a default location
	dest := filepath.Join(os.Getenv("HELM_DATA_HOME"), "charts")
	if dest == "" {
		dest = "./charts"
	}
	os.MkdirAll(dest, 0755)
	saved, _, err := client.DownloadTo(chartURL, r.Version, dest)
	if err != nil {
		return "", err
	}

	return saved, err
}
