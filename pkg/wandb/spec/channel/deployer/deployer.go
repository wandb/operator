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
	DeployerAPI        = "https://deploy.wandb.ai/api/v1/operator/channel"
	DeployerReleaseAPI = "https://deploy.wandb.ai/api/v1/operator/channel/release/:versionId"
)

type GetSpecOptions struct {
	License     string
	ActiveState *spec.Spec
	ReleaseId   string
}

//counterfeiter:generate . DeployerInterface
type DeployerInterface interface {
	GetSpec(opts GetSpecOptions) (*spec.Spec, error)
}

type DeployerClient struct {
	DeployerChannelUrl string
	DeployerReleaseURL string
}

type SpecUnknownChart struct {
	Metadata *spec.Metadata `json:"metadata"`
	Values   *spec.Values   `json:"values"`
	Chart    interface{}    `json:"chart"`
}

func (c *DeployerClient) getDeployerURL(releaseId string) string {
	if releaseId == "" {
		if c.DeployerChannelUrl == "" {
			return DeployerAPI
		}
		return c.DeployerChannelUrl
	}

	if c.DeployerReleaseURL == "" {
		c.DeployerReleaseURL = DeployerReleaseAPI
	}
	return strings.Replace(c.DeployerReleaseURL, ":versionId", releaseId, 1)
}

// GetSpec returns the spec for the given license. If the license or an empty
// string it will pull down the latest stable version.
func (c *DeployerClient) GetSpec(opts GetSpecOptions) (*spec.Spec, error) {
	url := c.getDeployerURL(opts.ReleaseId)

	client := &http.Client{}

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
			time.Sleep(time.Second * 2)
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
