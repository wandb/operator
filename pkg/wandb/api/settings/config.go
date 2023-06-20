package settings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const name = "wandb-console"
const namespace = "wandb-system"

func GetOrCreateSecret(ctx context.Context, c client.Client) (*corev1.Secret, error) {
	configMap := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	err := c.Get(ctx, objKey, configMap)

	if err != nil {
		if errors.IsNotFound(err) {
			err := c.Create(ctx, configMap)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return configMap, nil
}

func GetOrCreateConfig(ctx context.Context, c client.Client) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	objKey := client.ObjectKey{Name: name, Namespace: namespace}
	err := c.Get(ctx, objKey, configMap)

	if err != nil {
		if errors.IsNotFound(err) {
			err := c.Create(ctx, configMap)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return configMap, nil
}

func SetConfigValue(ctx context.Context, c client.Client, key, value string) error {
	configMap, err := GetOrCreateConfig(ctx, c)
	if err != nil {
		return err
	}
	configMap.Data[key] = value
	return c.Update(ctx, configMap)
}

func GetConfigValue(ctx context.Context, c client.Client, key string) (string, error) {
	configMap, err := GetOrCreateConfig(ctx, c)
	if err != nil {
		return "", err
	}
	return configMap.Data[key], nil
}

func sha256Hash(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	hashed := hasher.Sum(nil)
	return hex.EncodeToString(hashed)
}

func New(c client.Client) *Settings {
	return &Settings{
		ctx:    context.Background(),
		client: c,
	}
}

type Settings struct {
	ctx    context.Context
	client client.Client
}

func (c Settings) IsPassword(pwd string) bool {
	passwordHash := sha256Hash(pwd)
	storedPasswordHash, err := GetConfigValue(c.ctx, c.client, "password")
	return storedPasswordHash == passwordHash && err == nil
}

func (c Settings) SetPassword(pwd string) error {
	passwordHash := sha256Hash(pwd)
	return SetConfigValue(
		context.Background(), c.client,
		"password", passwordHash,
	)
}
