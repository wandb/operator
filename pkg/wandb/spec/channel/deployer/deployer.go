package deployer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/charts"
)

const (
	DeployerAPI = "https://deploy.wandb.ai/api/v1/operator/channel"
)

func GetURL() string {
	return utils.Getenv("DEPLOYER_CHANNEL_URL", DeployerAPI)
}

type SpecUnknownChart struct {
	Metadata *spec.Metadata `json:"metadata"`
	Values   *spec.Values   `json:"values"`
	Chart    interface{}    `json:"chart"`
}

// GetSpec returns the spec for the given license. If the license or an empty
// string it will pull down the latest stable version.
func GetSpec(license string, activeState *spec.Spec) (*spec.Spec, error) {
	url := GetURL()
	client := &http.Client{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if license != "" {
		req.SetBasicAuth("license", license)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println("resBody: ", string(resBody))

	var specUnknown SpecUnknownChart
	err = json.Unmarshal(resBody, &specUnknown)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	spec := new(spec.Spec)
	spec.Metadata = specUnknown.Metadata
	spec.Chart = charts.Get(specUnknown.Chart)
	if spec.Values != nil {
		spec.Values = *specUnknown.Values
	}

	return spec, nil
}
