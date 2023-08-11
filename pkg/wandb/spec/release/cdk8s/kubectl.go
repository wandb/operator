package cdk8s

import (
	"fmt"
	"os/exec"
	"path"
)

func KubectApplyCmd(folder string, namespace string) *exec.Cmd {
	return exec.Command("kubectl", "apply", "-f", folder, "--prune", "-l", "app=wandb", "-n", namespace)
}

func KubectlApply(dir string, namespace string) error {
	folder := path.Join(dir, "dist")
	cmd := exec.Command("kubectl", "apply", "-f", folder, "--prune", "-l", "app=wandb", "-n", namespace)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}
