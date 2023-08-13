package cdk8s

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"github.com/wandb/operator/pkg/wandb/spec/release/k8s"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Git struct {
	URL string `json:"url" validate:"required,url"`
}

// A single application container that you want to run within a pod.
type ContainerRelease struct {
	// Container image name. More info:
	// https://kubernetes.io/docs/concepts/containers/images
	Image string `json:"image"`
	// Map of environment variables to set in the container.
	Envs map[string]string `json:"envs"`
	// Run pnpm install before running generate and build
	Git *Git `json:"git,omitempty"`
}

func (c ContainerRelease) Validate() error {
	return validator.New().Struct(c)
}

func (s ContainerRelease) Apply(
	ctx context.Context,
	c client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config map[string]interface{},
) error {
	if s.Image == "" {
		s.Image = "wandb/cdk8s:latest"
	}

	if s.Envs == nil {
		s.Envs = map[string]string{}
	}

	jsonConfig, _ := json.Marshal(config)
	s.Envs["CONFIG"] = string(jsonConfig)

	cmds := []string{}
	if s.Git != nil {
		cmds = append(cmds, "rm -rf /git")
		cmds = append(cmds, strings.Join(GitCloneCmd(s.Git.URL, "/git").Args, " "))
		cmds = append(cmds, "cd /git")
		cmds = append(cmds, strings.Join(PnpmInstallCmd().Args, " "))
		cmds = append(cmds, strings.Join(PnpmGenerateDevCmd(config).Args, " "))
	} else {
		cmds = append(cmds, strings.Join(PnpmGenerateBuildCmd(config).Args, " "))
	}
	cmds = append(cmds, strings.Join(KubectApplyCmd("/cdk8s/dist", wandb.GetNamespace()).Args, " "))

	container := k8s.ContainerRelease{
		Image: s.Image,
		Envs:  s.Envs,
		Command: []string{
			"/bin/bash",
			"-c",
			strings.Join(cmds, " && "),
		},
	}
	return container.Apply(ctx, c, wandb, scheme, config)
}
