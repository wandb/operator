package cdk8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-playground/validator/v10"
	v1 "github.com/wandb/operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ClonePath = "/tmp/git"

func GetCdk8sGitSpec(s interface{}) *Cdk8sGit {
	spec := &Cdk8sGit{}
	specBytes, _ := json.Marshal(s)

	if err := json.Unmarshal(specBytes, spec); err != nil {
		return nil
	}

	if err := spec.Validate(); err != nil {
		return nil
	}

	return spec
}

type Cdk8sGit struct {
	URL    string `validate:"required" json:"url"`
	Branch string `json:"branch"`
}

func (c *Cdk8sGit) Validate() error {
	return validator.New().Struct(c)
}

func (c *Cdk8sGit) Path() string {
	path := strings.ReplaceAll(c.URL, "https://", "")
	path = strings.ReplaceAll(path, "http://", "")
	path = strings.ReplaceAll(path, ".git", "")
	path = strings.ReplaceAll(path, "github.com", "")
	return "/tmp/git/" + strings.Trim(path, "/")
}

func (c Cdk8sGit) Apply(
	ctx context.Context,
	client client.Client,
	wandb *v1.WeightsAndBiases,
	scheme *runtime.Scheme,
	config map[string]interface{},
) error {
	os.MkdirAll(c.Path(), 0755)

	rm := exec.Command("rm", "-rf", c.Path())
	rm.Run()

	fmt.Println(c.Path())

	if err := clone(c.URL, "", c.Path()); err != nil {
		return err
	}

	return Cdk8sLocal{Directory: c.Path()}.
		Apply(ctx, client, wandb, scheme, config)
}

func clone(url string, branch string, to string) error {
	g, err := git.PlainClone(to, false, &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
		Depth:    1,
	})

	if branch != "" {
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
