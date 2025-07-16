package charts

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const CredentialUsernameKey = "HELM_USERNAME"
const CredentialPasswordKey = "HELM_PASSWORD"

type RepoRelease struct {
	URL  string `validate:"required,url" json:"url"`
	Name string `validate:"required" json:"name"`

	// If version is not set, download latest.
	Version string `json:"version"`

	// Optional repository name override. If not set, will be derived from URL.
	RepoName string `json:"repoName,omitempty"`

	CredentialSecret *CredentialSecret `json:"credentialSecret,omitempty"`
	Password         string            `json:"password"`
	Username         string            `json:"username"`

	Debug bool `json:"debug"`
}

type CredentialSecret struct {
	Name        string `json:"name"`
	UsernameKey string `json:"usernameKey"`
	PasswordKey string `json:"passwordKey"`
}

// deriveRepoName generates a repository name from the URL if one isn't explicitly set
func (r RepoRelease) deriveRepoName() (string, error) {
	if r.RepoName != "" {
		return r.RepoName, nil
	}

	parsedURL, err := url.Parse(r.URL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Use hostname without dots as the repo name
	repoName := strings.ReplaceAll(parsedURL.Hostname(), ".", "-")
	if repoName == "" {
		return "", fmt.Errorf("could not derive repository name from URL: %s", r.URL)
	}

	return repoName, nil
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
	log := ctrllog.Log.WithName("chart-repo")
	if r.CredentialSecret != nil {
		if r.CredentialSecret.UsernameKey == "" {
			r.CredentialSecret.UsernameKey = CredentialUsernameKey
		}
		if r.CredentialSecret.PasswordKey == "" {
			r.CredentialSecret.PasswordKey = CredentialPasswordKey
		}
		log.Info("Retrieving credentials from secret",
			"name", r.CredentialSecret.Name,
			"usernameKey", r.CredentialSecret.UsernameKey,
			"passwordKey", r.CredentialSecret.PasswordKey)

		secret := &corev1.Secret{}
		err := c.Get(ctx, client.ObjectKey{Name: r.CredentialSecret.Name, Namespace: wandb.Namespace}, secret)
		if err != nil {
			log.Error(err, "Failed to get credentials from secret")
			return err
		}
		r.Username = string(secret.Data[r.CredentialSecret.UsernameKey])
		r.Password = string(secret.Data[r.CredentialSecret.PasswordKey])
	}

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
	log := ctrllog.Log.WithName("chart-repo")

	repoName, err := r.deriveRepoName()
	if err != nil {
		log.Error(err, "Failed to derive repository name")
		return "", err
	}

	entry := new(repo.Entry)
	entry.URL = r.URL
	entry.Name = repoName
	entry.Username = r.Username
	entry.Password = r.Password

	parsedURL, err := url.Parse(r.URL)
	if err != nil {
		log.Error(err, "Failed to parse URL for TLS verification check")
		return "", err
	}

	entry.InsecureSkipTLSverify = parsedURL.Scheme == "http"
	if entry.InsecureSkipTLSverify && r.Debug {
		log.Info("TLS verification disabled for HTTP URL", "url", r.URL)
	}

	if r.Debug {
		log.Info("Setting up chart repository",
			"url", entry.URL,
			"name", r.Name,
			"repoName", entry.Name,
			"username", entry.Username)
	}

	file := repo.NewFile()
	file.Update(entry)

	settings := cli.New()
	if helmCache := os.Getenv("HELM_CACHE_HOME"); helmCache != "" {
		settings.RepositoryCache = filepath.Join(helmCache, "repository")
		if r.Debug {
			log.Info("Using HELM_CACHE_HOME", "path", helmCache)
		}
	}
	if helmConfig := os.Getenv("HELM_CONFIG_HOME"); helmConfig != "" {
		settings.RepositoryConfig = filepath.Join(helmConfig, "repositories.yaml")
		if r.Debug {
			log.Info("Using HELM_CONFIG_HOME", "path", helmConfig)
		}
	}

	getterOpts := []getter.Option{
		getter.WithBasicAuth(r.Username, r.Password),
		getter.WithInsecureSkipVerifyTLS(true),
	}

	providers := getter.All(settings)
	if r.Debug {
		log.Info("Created providers")
	}

	chartRepo, err := repo.NewChartRepository(entry, providers)
	if err != nil {
		log.Error(err, "Failed to create chart repository")
		return "", err
	}

	if r.Debug {
		log.Info("Attempting to download index file",
			"url", entry.URL,
			"username", entry.Username)
	}
	_, err = chartRepo.DownloadIndexFile()
	if err != nil {
		log.Error(err, "Failed to download index file")
		return "", fmt.Errorf("failed to download index file from %s: %w", entry.URL, err)
	}

	if r.Debug {
		log.Info("Successfully downloaded index file",
			"chart", r.Name,
			"version", r.Version)
	}

	if chartRepo.IndexFile == nil {
		log.Error(nil, "Index file is nil")
		return "", fmt.Errorf("index file is nil")
	}

	indexPath := filepath.Join(settings.RepositoryCache, fmt.Sprintf("%s-index.yaml", entry.Name))
	indexFile, err := repo.LoadIndexFile(indexPath)
	if err != nil {
		log.Error(err, "Failed to load index file")
		return "", err
	}

	cv, err := indexFile.Get(r.Name, r.Version)
	if err != nil {
		log.Error(err, "Failed to find chart version")
		return "", err
	}

	if len(cv.URLs) == 0 {
		return "", fmt.Errorf("chart %s version %s has no downloadable URLs", r.Name, r.Version)
	}

	chartURL := cv.URLs[0]
	if !strings.HasPrefix(chartURL, "http://") && !strings.HasPrefix(chartURL, "https://") {
		chartURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(r.URL, "/"), chartURL)
	}
	if r.Debug {
		log.Info("Found chart URL", "url", chartURL)
	}

	_, cfg, err := helm.InitConfig("")
	if err != nil {
		log.Error(err, "Failed to init helm config")
		return "", err
	}

	if r.Debug {
		log.Info("Setting up chart downloader with auth", "username", r.Username)
	}
	client := downloader.ChartDownloader{
		Verify:           downloader.VerifyNever,
		Getters:          providers,
		Options:          getterOpts,
		RegistryClient:   cfg.RegistryClient,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
	}

	dest := filepath.Join(os.Getenv("HELM_DATA_HOME"), "charts")
	if dest == "" {
		dest = "./charts"
	}
	os.MkdirAll(dest, 0755)

	if r.Debug {
		log.Info("Attempting to download chart", "destination", dest)
	}
	saved, _, err := client.DownloadTo(chartURL, r.Version, dest)
	if err != nil {
		log.Error(err, "Failed to download chart")
		return "", err
	}

	return saved, nil
}
