package release

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewGitRelease(url *url.URL, repo string) Release {
	path := strings.ReplaceAll(repo, "https://", "")
	path = strings.ReplaceAll(path, "http://", "")
	path = strings.ReplaceAll(path, ".git", "")
	path = strings.ReplaceAll(path, "github.com", "")

	gr := &GitRelease{
		url:  url,
		repo: repo,
		path: "/tmp/git/" + strings.Trim(path, "/"),
	}

	urlRef := url.Query().Get("ref")
	if urlRef != "" {
		gr.path += "/" + urlRef
		gr.ref = urlRef
	} else {
		gr.path += "/main"
	}

	return gr
}

type GitRelease struct {
	url  *url.URL
	repo string
	path string
	ref  string
}

func (r GitRelease) Download() error {
	fmt.Println("cloning", r.repo, "into", r.path)

	rm := exec.Command("rm", "-rf", r.path)
	rm.Run()

	os.MkdirAll(r.path, 0755)
	g, err := git.PlainClone(r.path, false, &git.CloneOptions{
		URL:      r.Version(),
		Progress: os.Stdout,
		Depth:    1,
	})

	if r.ref != "" {
		hash, err := g.ResolveRevision(plumbing.Revision("HEAD"))
		if err != nil {
			return err
		}

		workTree, err := g.Worktree()
		if err != nil {
			return nil
		}

		return workTree.Checkout(&git.CheckoutOptions{
			Hash: *hash,
		})
	}

	return err
}

func (r GitRelease) Version() string {
	return r.repo
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
