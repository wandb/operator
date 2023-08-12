package cdk8s

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
)

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

func PnpmInstallCmd() *exec.Cmd {
	return exec.Command("pnpm", "install", "--frozen-lockfile")
}

func PnpmInstall(dir string) error {
	cmd := PnpmInstallCmd()
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}
