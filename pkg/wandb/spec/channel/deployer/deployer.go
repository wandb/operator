package deployer

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
)

const (
	DeployerAPI            = "https://deploy.wandb.ai"
	DeployerChannelPath    = "/api/v1/operator/channel"
	DeployerReleaseAPIPath = "/api/v1/operator/channel/release/:versionId"
)

type GetSpecOptions struct {
	License     string
	ActiveState *spec.Spec
	ReleaseId   string
	Debug       bool
	RetryDelay  time.Duration
}

//counterfeiter:generate . DeployerInterface
type DeployerInterface interface {
	GetSpec(opts GetSpecOptions) (*spec.Spec, error)
}

type DeployerClient struct {
	DeployerAPI string
}

type SpecUnknownChart struct {
	Metadata *spec.Metadata `json:"metadata"`
	Values   *spec.Values   `json:"values"`
	Chart    interface{}    `json:"chart"`
}

func (c *DeployerClient) getDeployerURL(opts GetSpecOptions) string {
	var url string
	if c.DeployerAPI != "" {
		url = c.DeployerAPI
	} else {
		url = DeployerAPI
	}

	if opts.ReleaseId != "" {
		return url + strings.Replace(DeployerReleaseAPIPath, ":versionId", opts.ReleaseId, 1)
	}
	return url + DeployerChannelPath
}

// GetSpec returns the spec for the given license. If the license or an empty
// string it will pull down the latest stable version.
func (c *DeployerClient) GetSpec(opts GetSpecOptions) (*spec.Spec, error) {
	url := c.getDeployerURL(opts)

	client := &http.Client{}

	retryDelay := opts.RetryDelay
	if retryDelay == 0 {
		retryDelay = 2 * time.Second
	}

	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		if opts.License != "" {
			req.SetBasicAuth("license", opts.License)
		}

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			time.Sleep(retryDelay)
			continue
		}
		defer resp.Body.Close()

		resBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var specUnknown SpecUnknownChart
		err = json.Unmarshal(resBody, &specUnknown)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		spec := new(spec.Spec)
		spec.Metadata = specUnknown.Metadata
		spec.Chart = charts.Get(specUnknown.Chart)
		if specUnknown.Values != nil {
			spec.SetValues(*specUnknown.Values)
		}

		return spec, nil
	}

	return nil, errors.New("all retries failed")
}
