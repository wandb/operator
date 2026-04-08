package charts

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/helm"
	"github.com/wandb/operator/pkg/wandb/spec"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// OCIRelease pulls a Helm chart from an OCI-based registry.
// The URL must use the oci:// scheme (e.g., oci://ghcr.io/wandb/helm-charts/operator-wandb).
type OCIRelease struct {
	URL     string `validate:"required,ociurl" json:"url"`
	Version string `validate:"required" json:"version"`

	CredentialSecret *CredentialSecret `json:"credentialSecret,omitempty"`
	Password         string            `json:"password"`
	Username         string            `json:"username"`

	PlainHTTP bool `json:"plainHTTP,omitempty"`
	Debug     bool `json:"debug"`
}

func validateOCIURL(fl validator.FieldLevel) bool {
	return registry.IsOCI(fl.Field().String())
}

func (c OCIRelease) Validate() error {
	v := validator.New()
	v.RegisterValidation("ociurl", validateOCIURL)
	return v.Struct(c)
}

func (r OCIRelease) Chart() (*chart.Chart, error) {
	return r.pullChart()
}

func (r OCIRelease) pullChart() (*chart.Chart, error) {
	log := ctrllog.Log.WithName("chart-oci")

	opts := []registry.ClientOption{
		registry.ClientOptEnableCache(true),
	}
	if r.Debug {
		opts = append(opts, registry.ClientOptDebug(true))
	}
	if (r.Username == "") != (r.Password == "") {
		return nil, fmt.Errorf("both username and password must be set together for OCI basic auth")
	}
	if r.Username != "" && r.Password != "" {
		opts = append(opts, registry.ClientOptBasicAuth(r.Username, r.Password))
	}
	if r.PlainHTTP {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		log.Error(err, "Failed to create registry client")
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}

	// Build the OCI reference: strip oci:// prefix, append :version if set
	ref := strings.TrimPrefix(r.URL, fmt.Sprintf("%s://", registry.OCIScheme))
	if r.Version != "" {
		ref = fmt.Sprintf("%s:%s", ref, r.Version)
	}

	if r.Debug {
		log.Info("Pulling OCI chart", "ref", ref)
	}

	result, err := registryClient.Pull(ref)
	if err != nil {
		log.Error(err, "Failed to pull chart", "ref", ref)
		return nil, fmt.Errorf("failed to pull chart from %s: %w", r.URL, err)
	}

	if result.Chart == nil {
		return nil, fmt.Errorf("registry returned empty chart for %s", r.URL)
	}

	if r.Debug {
		log.Info("Chart pulled successfully",
			"name", result.Chart.Meta.Name,
			"version", result.Chart.Meta.Version,
			"size", result.Chart.Size)
	}

	// Load the chart directly from the pulled bytes
	chrt, err := loader.LoadArchive(bytes.NewReader(result.Chart.Data))
	if err != nil {
		log.Error(err, "Failed to load chart archive")
		return nil, fmt.Errorf("failed to load chart archive: %w", err)
	}

	return chrt, nil
}

func (r *OCIRelease) getActionableChart(wandb *v1.WeightsAndBiases) (*helm.ActionableChart, error) {
	namespace := wandb.GetNamespace()
	releaseName := wandb.GetName()
	return helm.NewActionableChart(releaseName, namespace)
}

func (r OCIRelease) Apply(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config spec.Values,
) error {
	log := ctrllog.Log.WithName("chart-oci")
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
		usernameBytes, ok := secret.Data[r.CredentialSecret.UsernameKey]
		if !ok || len(usernameBytes) == 0 {
			return fmt.Errorf("credential secret %s/%s missing key %q",
				wandb.Namespace, r.CredentialSecret.Name, r.CredentialSecret.UsernameKey)
		}
		passwordBytes, ok := secret.Data[r.CredentialSecret.PasswordKey]
		if !ok || len(passwordBytes) == 0 {
			return fmt.Errorf("credential secret %s/%s missing key %q",
				wandb.Namespace, r.CredentialSecret.Name, r.CredentialSecret.PasswordKey)
		}
		r.Username = string(usernameBytes)
		r.Password = string(passwordBytes)
	}

	chrt, err := r.pullChart()
	if err != nil {
		return err
	}

	actionableChart, err := r.getActionableChart(wandb)
	if err != nil {
		return err
	}

	_, err = actionableChart.Apply(chrt, config)
	return err
}

func (r OCIRelease) Prune(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	_ spec.Values,
) error {
	actionableChart, err := r.getActionableChart(wandb)
	if err != nil {
		return err
	}

	_, err = actionableChart.Uninstall()
	return err
}
