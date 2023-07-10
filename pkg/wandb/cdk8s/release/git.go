package release

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewGitRelease(repo string) Release {
	path := strings.ReplaceAll(repo, "https://", "")
	path = strings.ReplaceAll(path, "http://", "")
	path = strings.ReplaceAll(path, ".git", "")
	path = strings.ReplaceAll(path, "github.com", "")
	return &GitRelease{
		url:  repo,
		path: "/tmp/git/" + path,
	}
}

type GitRelease struct {
	url  string
	path string
}

func (r GitRelease) Download() error {
	fmt.Println("cloning", r.url, "into", r.path)

	rm := exec.Command("rm", "-rf", r.path)
	rm.Run()

	os.MkdirAll(r.path, 0755)
	_, err := git.PlainClone(r.path, false, &git.CloneOptions{
		URL:      r.Version(),
		Progress: os.Stdout,
	})
	return err
}

func (r GitRelease) Version() string {
	return r.url
}

func (r GitRelease) Directory() string {
	return r.path
}

func (r GitRelease) Generate(m map[string]interface{}) error {
	return PnpmGenerate(r.Directory(), m)
}

func (r GitRelease) Install() error {
	return PnpmInstall(r.Directory())
}

func (r GitRelease) Apply(
	ctx context.Context,
	client client.Client,
	owner v1.Object,
	scheme *runtime.Scheme,
) error {
	return KubectlApply(r.Directory(), owner.GetNamespace())
}
