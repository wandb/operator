package deployer

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
)

const (
	DeployerAPI = "https://deploy.wandb.ai/api/v1/operator/channel"
)

//counterfeiter:generate . DeployerInterface
type DeployerInterface interface {
	GetSpec(license string, activeState *spec.Spec) (*spec.Spec, error)
}

type DeployerClient struct {
	DeployerChannelUrl string
}

func (c *DeployerClient) getURL() string {
	if c.DeployerChannelUrl == "" {
		c.DeployerChannelUrl = DeployerAPI
	}
	return c.DeployerChannelUrl
}

type SpecUnknownChart struct {
	Metadata *spec.Metadata `json:"metadata"`
	Values   *spec.Values   `json:"values"`
	Chart    interface{}    `json:"chart"`
}

// GetSpec returns the spec for the given license. If the license or an empty
// string it will pull down the latest stable version.
func (c *DeployerClient) GetSpec(license string, activeState *spec.Spec) (*spec.Spec, error) {
	url := c.getURL()
	client := &http.Client{}

	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", "application/json")
		if license != "" {
			req.SetBasicAuth("license", license)
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
