package deployer

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/wandb/spec"
)

const (
	DeployerAPI = "https://deploy.wandb.ai/api/v1/operator/channel"
)

func GetURL() string {
	return utils.Getenv("DEPLOYER_CHANNEL_URL", DeployerAPI)
}

type Payload struct {
	Release  spec.Release      `json:"release"`
	Metadata map[string]string `json:"metadata"`
}

// GetSpec returns the spec for the given license. If the license or an empty
// string it will pull down the latest stable version.
func GetSpec(license string, activeState *spec.Spec) (*spec.Spec, error) {
	url := GetURL()
	client := &http.Client{}

	// Config can hold secrets. We shouldn't submit it.
	payload := &Payload{
		Metadata: activeState.Metadata,
		Release:  activeState.Release,
	}

	payloadBytes, _ := json.Marshal(payload)
	body := bytes.NewReader(payloadBytes)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("license", license)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var spec spec.Spec
	err = json.Unmarshal(resBody, &spec)
	if err != nil {
		return nil, err
	}

	return &spec, nil
}
