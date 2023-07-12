package deployer

import "github.com/wandb/operator/pkg/utils"

var (
	DeployerAPIUrl = utils.Getenv("DEPLOYER_API_URL", "https://localhost:3000/api")
)
