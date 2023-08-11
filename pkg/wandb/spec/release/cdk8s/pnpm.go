package cdk8s

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
)

func PnpmGenerateDevCmd(m map[string]interface{}) *exec.Cmd {
	b, _ := json.Marshal(m)
	return exec.Command("pnpm", "run", "gen", "--json="+string(b))
}

func PnpmGenerateBuildCmd(m map[string]interface{}) *exec.Cmd {
	b, _ := json.Marshal(m)
	return exec.Command("pnpm", "run", "start", "--json="+string(b))
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

func PnpmInstall(dir string) error {
	cmd := exec.Command("pnpm", "install", "--frozen-lockfile")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}
