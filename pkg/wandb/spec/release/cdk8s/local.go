package cdk8s

import (
	"context"
	"fmt"
	"os/exec"
	"path"

	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalRelease struct {
	Directory string `validate:"required,dir" json:"directory"`
}

func (c *LocalRelease) Validate() error {
	return validator.New().Struct(c)
}

func (c LocalRelease) Apply(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config map[string]interface{},
) error {
	if err := PnpmInstall(c.Directory); err != nil {
		return err
	}

	if err := PnpmGenerate(c.Directory, config); err != nil {
		return err
	}

	if err := KubectlApply(c.Directory, wandb.GetNamespace()); err != nil {
		return err
	}

	return nil
}

func PnpmInstall(dir string) error {
	cmd := PnpmInstallCmd()
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}


func PnpmGenerate(dir string, m map[string]interface{}) error {
	dist := path.Join(dir, "dist")
	rm := exec.Command("rm", "-rf", dist)
	rmOutput, _ := rm.CombinedOutput()
	fmt.Println(string(rmOutput))

	cmd := PnpmGenerateDevCmd(m)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))

	return err
}

func KubectlApply(dir string, namespace string) error {
	folder := path.Join(dir, "dist")
	cmd := KubectApplyCmd(folder, namespace)
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}