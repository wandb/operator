package cdk8s

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/wandb/operator/pkg/wandb/cdk8s/config"
	"github.com/wandb/operator/pkg/wandb/deployer"
)

// Deployer returns the config suggested by deployer
func Deployer(license string) config.Modifier {
	url := deployer.DeployerAPIUrl + "/api/channel/license"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}

	req.Header.Add(deployer.DeployerLicenseHeader, license)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var responseStruct deployerResponse
	err = json.Unmarshal(body, &responseStruct)
	if err != nil {
		return nil
	}

	return &deployerChannel{
		response: &responseStruct,
	}
}

type state struct {
	Recommend *config.Config `json:"recommend,omitempty"`
	Override  *config.Config `json:"override,omitempty"`
}

type deployerResponse struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	State state  `json:"state"`
}

type deployerChannel struct {
	response *deployerResponse
}

func (c deployerChannel) Recommend(_ *config.Config) *config.Config {
	return c.response.State.Recommend
}

func (c deployerChannel) Override(_ *config.Config) *config.Config {
	return c.response.State.Override
}
