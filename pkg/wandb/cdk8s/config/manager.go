package config

import (
	"context"
	"fmt"
	"strconv"

	"github.com/wandb/operator/pkg/wandb/cdk8s/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func NewManager(
	ctx context.Context,
	c client.Client,
	owner metav1.Object,
	scheme *runtime.Scheme,
) *Manager {
	return &Manager{
		ctx:    ctx,
		owner:  owner,
		scheme: scheme,
		client: c,
	}
}

// Manager manages the state of the config for the cdk8s. It holds information
// such as the version and config to apply. It creates an empty config map call
// `<spac.name>-config` (The API will write to this config map).
//
// Anytime the manager notices a change for this config, it will create a backup of it
// `<space.name>-config-v<version>`. This can be used to rollback to a previous version.
type Manager struct {
	ctx    context.Context
	owner  metav1.Object
	scheme *runtime.Scheme
	client client.Client
}

func (m Manager) getOrCreateConfigMap() (*corev1.ConfigMap, error) {
	name := fmt.Sprintf("%s-config-manager", m.owner.GetName())
	namespace := m.owner.GetNamespace()
	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	configMap := &corev1.ConfigMap{}

	err := m.client.Get(m.ctx, objKey, configMap)
	if err == nil {
		return configMap, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	cfg := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"count": "0",
		},
	}
	controllerutil.SetControllerReference(m.owner, cfg, m.scheme)

	err = m.client.Create(m.ctx, cfg)
	return cfg, err
}

func (m Manager) updateConfigCount(count int) error {
	cfg, err := m.getOrCreateConfigMap()
	if err != nil {
		return err
	}

	if m.getCount() == count {
		return nil
	}

	cfg.Data["count"] = fmt.Sprint(count)
	err = m.client.Update(m.ctx, cfg)
	return err
}

func (m Manager) getCount() int {
	cfg, err := m.getOrCreateConfigMap()
	if err != nil {
		return 0
	}
	v, ok := cfg.Data["count"]
	if !ok {
		return 0
	}
	vi, _ := strconv.Atoi(v)
	return vi
}

func (m Manager) LatestConfigName() string {
	return fmt.Sprintf("%s-config-latest", m.owner.GetName())
}

func (m Manager) CreateLatest(release release.Release, config interface{}) (*Config, error) {
	name := m.LatestConfigName()
	namespace := m.owner.GetNamespace()
	return CreateWithConfigMap(
		m.ctx, m.client, m.owner, m.scheme,
		name, namespace,
		release, config,
	)
}

func (m Manager) SetDesiredState(cfg *Config) error {
	name := m.LatestConfigName()
	namespace := m.owner.GetNamespace()

	err := WriteToConfigMap(
		m.ctx, m.client, m.owner, m.scheme,
		name, namespace,
		cfg,
	)
	if err != nil {
		return err
	}

	if _, err = m.backupLatest(); err != nil {
		return err
	}

	return nil
}

func (m Manager) GetDesiredState() (*Config, error) {
	name := m.LatestConfigName()
	namespace := m.owner.GetNamespace()
	return GetFromConfigMap(m.ctx, m.client, name, namespace)
}

func (m Manager) GetBackup(version int) (*Config, error) {
	name := fmt.Sprintf("%s-config-v%d", m.owner.GetName(), version)
	namespace := m.owner.GetNamespace()
	return GetFromConfigMap(m.ctx, m.client, name, namespace)
}

func (m Manager) getLastBackup() (*Config, error) {
	current := m.getCount()
	return m.GetBackup(current)
}

func (m Manager) backupLatest() (string, error) {
	config, err := m.GetDesiredState()
	if err != nil {
		return "", err
	}

	lastBackup, _ := m.getLastBackup()
	if lastBackup != nil {
		if config.Equals(lastBackup) {
			name := fmt.Sprintf("%s-config-v%d", m.owner.GetName(), m.getCount())
			return name, nil
		}
	}

	current := m.getCount()
	next := current + 1
	if err := m.updateConfigCount(next); err != nil {
		return "", err
	}

	namespace := m.owner.GetNamespace()
	name := fmt.Sprintf("%s-config-v%d", m.owner.GetName(), next)

	err = WriteToConfigMap(
		m.ctx, m.client,
		m.owner, m.scheme,
		name, namespace,
		config,
	)
	return name, err
}

func (m Manager) Reconcile(cfg *Config) error {
	if err := cfg.Release.Download(); err != nil {
		return err
	}
	if err := cfg.Release.Install(); err != nil {
		return err
	}
	if err := cfg.Release.Generate(cfg.Config); err != nil {
		return err
	}
	if err := cfg.Release.Apply(m.ctx, m.client, m.owner, m.scheme); err != nil {
		return err
	}
	return nil
}
