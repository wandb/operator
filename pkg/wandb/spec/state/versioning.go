package state

import (
	"context"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/spec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UserSpecName(prefix string) string {
	return fmt.Sprintf("%s-spec-user", prefix)
}

func ActiveSpecName(prefix string) string {
	return fmt.Sprintf("%s-spec-active", prefix)
}

func New(
	ctx context.Context,
	client client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
	state State,
) *Manager {
	return &Manager{
		ctx,
		state,
		owner,
		client,
		scheme,
	}
}

type Manager struct {
	ctx    context.Context
	state  State
	owner  metav1.Object
	client client.Client
	scheme *runtime.Scheme
}

func (m Manager) SetUserInput(s *spec.Spec) error {
	namespace := m.owner.GetNamespace()
	name := UserSpecName(m.owner.GetName())
	return m.state.Set(namespace, name, s)
}

func (m Manager) GetUserInput() (*spec.Spec, error) {
	namespace := m.owner.GetNamespace()
	name := UserSpecName(m.owner.GetName())
	return m.state.Get(namespace, name)
}

func (m Manager) GetActive() (*spec.Spec, error) {
	namespace := m.owner.GetNamespace()
	name := ActiveSpecName(m.owner.GetName())
	return m.state.Get(namespace, name)
}

func (m Manager) SetActive(s *spec.Spec) error {
	namespace := m.owner.GetNamespace()
	name := ActiveSpecName(m.owner.GetName())
	return m.state.Set(namespace, name, s)
}

func (m Manager) Get(name string) (*spec.Spec, error) {
	namespace := m.owner.GetNamespace()
	ownerName := m.owner.GetName()
	finalName := fmt.Sprintf("%s-%s", ownerName, name)
	return m.state.Get(namespace, finalName)
}

func (m Manager) Set(name string, s *spec.Spec) error {
	namespace := m.owner.GetNamespace()
	ownerName := m.owner.GetName()
	finalName := fmt.Sprintf("%s-%s", ownerName, name)
	return m.state.Set(namespace, finalName, s)
}
