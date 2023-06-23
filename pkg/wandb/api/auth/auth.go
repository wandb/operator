package auth

import (
	"context"
	"net/http"

	"github.com/wandb/operator/pkg/utils/kubeclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const Namespace = "wandb"
const PasswordSecret = "wandb-password"
const PasswordKey = "password"

const CookieAuthName = "wandb_console_auth"

func GetPassword(ctx context.Context, c client.Client) (string, error) {
	kubeclient.UpsertNamespace(ctx, c, Namespace)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PasswordSecret,
			Namespace: Namespace,
		},
		Data: map[string][]byte{},
	}
	err := kubeclient.GetOrCreate(ctx, c, secret)
	if err != nil {
		return "", err
	}

	password := secret.Data[PasswordKey]
	return string(password), nil
}

func setPassword(ctx context.Context, c client.Client, password string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PasswordSecret,
			Namespace: Namespace,
		},
		Data: map[string][]byte{
			PasswordKey: []byte(password),
		},
	}
	return kubeclient.CreateOrUpdate(ctx, c, secret)
}

func isPasswordSet(ctx context.Context, c client.Client) bool {
	p, err := GetPassword(ctx, c)
	return err == nil && p != ""
}

func isPassword(ctx context.Context, c client.Client, password string) bool {
	p, _ := GetPassword(ctx, c)
	return p == password && p != ""
}

func getPassword(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieAuthName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}
