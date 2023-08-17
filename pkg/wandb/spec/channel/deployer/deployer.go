package deployer

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/channel"
)

const (
	DeployerAPI = "https://deploy.wandb.ai/api/v1/operator/channel"
)

func New(license string) channel.Channel {
	return &DeployerChannel{
		license: license,
	}
}

type DeployerChannel struct {
	license string
}

func (c DeployerChannel) Get() (*spec.Spec, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", DeployerAPI, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("license", c.license)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var spec spec.Spec
	err = json.Unmarshal(body, &spec)
	if err != nil {
		return nil, err
	}
	return &spec, nil
}
