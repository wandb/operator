package cdk8s

import (
	"fmt"
	"os/exec"
	"path"

	corev1 "k8s.io/api/core/v1"
)

func ComposeKubectApplyCmd(folder string, namespace string) ([]string, []corev1.EnvVar) {
	args := []string{
		"kubectl", "apply", "-f", folder, "--prune", "--applyset", "applyset", "-n", namespace,
	}
	env := []corev1.EnvVar{
		{Name: "KUBECTL_APPLYSET", Value: "true"},
	}
	return args, env
}

func KubectlApply(dir string, namespace string) error {
	folder := path.Join(dir, "dist")
	cmd := exec.Command("kubectl", "apply", "-f", folder, "--prune", "--applyset", "applyset", "-n", namespace)
	cmd.Env = append(cmd.Env, "KUBECTL_APPLYSET=true")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}
