package release

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
)

func PnpmGenerate(dir string, m map[string]interface{}) error {
	b, _ := json.Marshal(m)

	dist := path.Join(dir, "dist")
	rm := exec.Command("rm", "-rf", dist)
	rmOutput, _ := rm.CombinedOutput()
	fmt.Println(string(rmOutput))

	cmd := exec.Command("pnpm", "run", "gen", "--json="+string(b))
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))

	return err
}

func PnpmInstall(dir string) error {
	cmd := exec.Command("pnpm", "install", "--prod")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}
