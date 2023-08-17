package configmap

import (
	"context"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(
	ctx context.Context,
	c client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
) state.State {
	return &State{
		ctx:    ctx,
		owner:  owner,
		scheme: scheme,
		client: c,
	}
}

type State struct {
	ctx    context.Context
	owner  metav1.Object
	scheme *runtime.Scheme
	client client.Client
}

func (m State) Get(namespace string, name string) (*spec.Spec, error) {
	objKey := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	return read(m.ctx, m.client, m.owner, m.scheme, objKey)
}

func (m State) Set(namespace string, name string, s *spec.Spec) error {
	objKey := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}
	return write(m.ctx, m.client, m.owner, m.scheme, objKey, s)
}
