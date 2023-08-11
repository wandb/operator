package state

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/wandb/operator/pkg/wandb/spec"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func ManagerStateName(prefix string) string {
	return fmt.Sprintf("%s-spec-manager", prefix)
}

func UserSpecName(prefix string) string {
	return fmt.Sprintf("%s-spec-user", prefix)
}

func BackupSpecName(prefix string, version int) string {
	return fmt.Sprintf("%s-spec-backup-v%d", prefix, version)
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

func (m Manager) Backup(s *spec.Spec) error {
	latest, err := m.getLatestBackup()
	if err != nil {
		return err
	}

	if latest != nil {
		// For some reason reflect doesn't work, on map[string]interface{}
		cA, _ := json.Marshal(latest.Config)
		cB, _ := json.Marshal(s.Config)
		isConfigSame := string(cA) == string(cB)

		if isConfigSame && reflect.DeepEqual(s.Release, latest.Release) {
			return nil
		}
	}

	if err := m.increaseVersion(); err != nil {
		return err
	}

	namespace := m.owner.GetNamespace()
	_, name, err := m.getLatestBackupInfo()
	if err != nil {
		return err
	}

	return m.state.Set(namespace, name, s)
}

func (m Manager) SetActive(s *spec.Spec) error {
	namespace := m.owner.GetNamespace()
	name := ActiveSpecName(m.owner.GetName())
	return m.state.Set(namespace, name, s)
}

func (m Manager) getLatestBackup() (*spec.Spec, error) {
	namespace := m.owner.GetNamespace()
	_, name, err := m.getLatestBackupInfo()
	if errors.IsNotFound(err) {
		return nil, nil
	}

	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	configMap := &corev1.ConfigMap{}

	err = m.client.Get(m.ctx, objKey, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	fmt.Println("Loading from", namespace, name)
	return m.state.Get(namespace, name)
}

func (m Manager) getLatestBackupInfo() (int, string, error) {
	cfg, err := m.getManagerConfigMap()
	if err != nil {
		return 0, "", err
	}

	v := "0"
	if cfg != nil {
		v = cfg.Data["backup-count"]
	}
	if v == "" {
		v = "0"
	}

	vi, err := strconv.Atoi(v)
	if err != nil {
		return 0, "", err
	}

	name := BackupSpecName(m.owner.GetName(), vi)
	return vi, name, nil
}

func (m Manager) increaseVersion() error {
	currentCount, _, err := m.getLatestBackupInfo()
	if err != nil {
		return err
	}
	return m.updateManagerConfigMap(
		map[string]string{
			"backup-count": fmt.Sprint(currentCount + 1),
		},
	)
}

func (m Manager) updateManagerConfigMap(data map[string]string) error {
	cfg, err := m.getManagerConfigMap()
	if err != nil {
		return err
	}

	for k, v := range data {
		cfg.Data[k] = v
	}

	if err = m.client.Update(m.ctx, cfg); err != nil {
		return err
	}

	return nil
}

func (m Manager) getManagerConfigMap() (*corev1.ConfigMap, error) {
	namespace := m.owner.GetNamespace()
	name := ManagerStateName(m.owner.GetName())
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
			"backup-count": "0",
		},
	}
	controllerutil.SetControllerReference(m.owner, cfg, m.scheme)

	err = m.client.Create(m.ctx, cfg)
	return cfg, err
}
