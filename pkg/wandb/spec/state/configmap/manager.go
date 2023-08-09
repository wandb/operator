package configmap

import (
	"context"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	"github.com/wandb/operator/pkg/wandb/spec/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ManagerConfigName(owner metav1.Object) string {
	return fmt.Sprintf("%s-config-manager", owner.GetName())
}

func DesiredConfigName(owner metav1.Object) string {
	return fmt.Sprintf("%s-config-desired", owner.GetName())
}

func LatestConfigName(owner metav1.Object) string {
	return fmt.Sprintf("%s-config-latest", owner.GetName())
}

func VersionConfigName(owner metav1.Object, version int) string {
	return fmt.Sprintf("%s-config-v%d", owner.GetName(), version)
}

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
