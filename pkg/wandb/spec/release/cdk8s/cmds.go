package cdk8s

import (
	"encoding/json"
	"os/exec"
)

func GitCloneCmd(url string, folder string) *exec.Cmd {
	return exec.Command("git", "clone", url, folder, "--depth", "1")
}

func KubectApplyCmd(folder string, namespace string) *exec.Cmd {
	return exec.Command("kubectl", "apply", "-f", folder, "--prune", "-l", "app=wandb", "-n", namespace)
}

func PnpmGenerateDevCmd(m map[string]interface{}) *exec.Cmd {
	b, _ := json.Marshal(m)
	cmd := exec.Command("pnpm", "run", "gen")
	cmd.Env = append(cmd.Env, "CONFIG="+string(b))
	return cmd
}

func PnpmGenerateBuildCmd(m map[string]interface{}) *exec.Cmd {
	b, _ := json.Marshal(m)
	cmd := exec.Command("pnpm", "run", "start")
	cmd.Env = append(cmd.Env, "CONFIG="+string(b))
	return cmd
}

func PnpmInstallCmd() *exec.Cmd {
	return exec.Command("pnpm", "install", "--frozen-lockfile")
}
