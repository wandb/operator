package cdk8s

import "os/exec"

func GitCloneCmd(url string, folder string) *exec.Cmd {
	return exec.Command("git", "clone", url, folder, "--depth", "1")
}