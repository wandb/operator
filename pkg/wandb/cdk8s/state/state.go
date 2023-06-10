package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewState(
	ctx context.Context,
	c client.Client,
	namespace string,
	name string,
	release release.Release,
	config interface{},
) (*State, error) {

	configJson, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"version": release.Version(),
			"config":  string(configJson),
		},
	}

	if err := c.Create(ctx, cfg); err != nil {
		return nil, err
	}

	state := &State{
		name,
		namespace,
		ctx,
		c,
		release,
		config,
	}
	return state, nil
}

func DesiredStateFromConfig(
	ctx context.Context,
	c client.Client,
	namespace string,
	name string,
) (*State, error) {
	configMap := &corev1.ConfigMap{}
	objKey := client.ObjectKey{Name: name, Namespace: namespace}

	err := c.Get(ctx, objKey, configMap)
	if err != nil {
		return nil, err
	}

	version, ok := configMap.Data["version"]
	if !ok {
		return nil, fmt.Errorf("config map %s/%s does not have a version key", namespace, name)
	}

	configString, ok := configMap.Data["config"]
	if !ok {
		return nil, fmt.Errorf("config map %s/%s does not have a config key", namespace, name)
	}
	var config interface{}
	json.Unmarshal([]byte(configString), &config)

	state := &State{
		name,
		namespace,
		ctx,
		c,
		release.NewLocalRelease(version),
		config,
	}
	return state, nil
}

type State struct {
	name      string
	namespace string
	ctx       context.Context
	client    client.Client
	Release   release.Release
	Config    interface{}
}

func (s *State) SetRelease(release release.Release) {
	s.Release = release
}

func (s *State) SetConfig(config interface{}) {
	s.Config = config
}

func (s *State) Reconcile(
	owner metav1.Object,
	scheme *runtime.Scheme,
) error {
	s.Release.Download()
	s.Release.Install()
	s.Release.Generate(s.Config)
	s.Release.Apply(s.ctx, s.client, owner, scheme)
	return nil
}

func (s *State) Save(namespace string, name string) error {
	configJson, err := json.Marshal(s.Config)
	if err != nil {
		return err
	}

	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"version": s.Release.Version(),
			"config":  string(configJson),
		},
	}

	if err := s.client.Create(s.ctx, cfg); err != nil {
		return err
	}
	return nil
}
