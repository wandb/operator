package reconciler

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/external"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	emailSinkSecretSuffix = "email-sink"
	emailSinkSecretKey    = "sink"
)

func emailSinkSecretName(wandb *apiv2.WeightsAndBiases) string {
	return fmt.Sprintf("%s-%s", wandb.Name, emailSinkSecretSuffix)
}

// reconcileEmailSink resolves the configured email mode into the single SecretKeySelector
func reconcileEmailSink(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
) error {
	var emailSpec *apiv2.EmailSpec
	if wandb.Spec.Wandb.Notifications != nil {
		emailSpec = wandb.Spec.Wandb.Notifications.Email
	}

	// No email configured, clean up the generated sink
	if emailSpec == nil {
		wandb.Status.EmailSink = nil
		return deleteGeneratedEmailSink(ctx, client, wandb)
	}

	// Use the sink given by the user
	if emailSpec.Sink != nil {
		wandb.Status.EmailSink = emailSpec.Sink.DeepCopy()
		// The sink is already present under the same name, no need for changes
		if emailSpec.Sink.Name == emailSinkSecretName(wandb) {
			return nil
		}
		return deleteGeneratedEmailSink(ctx, client, wandb)
	}

	// email spec exists, but neither sink nor smtp was supplied
	if emailSpec.SMTP == nil {
		wandb.Status.EmailSink = nil
		return deleteGeneratedEmailSink(ctx, client, wandb)
	}

	// Read the SMTP Secret values and build the sink URL
	sink, err := resolveSMTPURL(ctx, client, wandb.Namespace, emailSpec.SMTP)
	if err != nil {
		return fmt.Errorf("resolve SMTP configuration: %w", err)
	}

	secretName := emailSinkSecretName(wandb)
	// Creates a new desired secret under the same namespace as the wandb application and under the `sink` key
	// belonging to W&B
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: wandb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{emailSinkSecretKey: sink},
	}
	if err := controllerutil.SetControllerReference(wandb, newSecret, client.Scheme()); err != nil {
		return fmt.Errorf("set email sink Secret owner: %w", err)
	}

	actual := &corev1.Secret{}
	err = client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: wandb.Namespace}, actual)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("get email sink Secret: %w", err)
	}
	if apierrors.IsNotFound(err) {
		actual = nil
	} else if !hasOwnerReference(actual, wandb) {
		return fmt.Errorf("email sink Secret %s/%s already exists and is not owned by %s", wandb.Namespace, secretName, wandb.Name)
	}

	if _, err := common.CrudResource(ctx, client, newSecret, actual); err != nil {
		return fmt.Errorf("write email sink Secret: %w", err)
	}

	wandb.Status.EmailSink = &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
		Key:                  emailSinkSecretKey,
	}
	return nil
}

// resolveSMTPURL generates the URL from the EmailSMTPSpec
func resolveSMTPURL(
	ctx context.Context,
	client ctrlClient.Client,
	namespace string,
	smtp *apiv2.EmailSMTPSpec,
) (string, error) {

	host, err := external.ResolveSecretKey(ctx, client, namespace, smtp.Host)
	if err != nil {
		return "", fmt.Errorf("host: %w", err)
	}
	port, err := external.ResolveSecretKey(ctx, client, namespace, smtp.Port)
	if err != nil {
		return "", fmt.Errorf("port: %w", err)
	}

	username, err := external.ResolveSecretKey(ctx, client, namespace, smtp.Username)
	if err != nil {
		return "", fmt.Errorf("username: %w", err)
	}
	password, err := external.ResolveSecretKey(ctx, client, namespace, smtp.Password)
	if err != nil {
		return "", fmt.Errorf("password: %w", err)
	}

	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" || port == "" {
		return "", fmt.Errorf("host and port must not be empty")
	}
	// net.JoinHostPort expects an IPv6 host without surrounding brackets.
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	sink := &url.URL{Scheme: "smtp", Host: net.JoinHostPort(host, port)}
	sink.User = url.UserPassword(username, password)
	return sink.String(), nil
}

// Delete the generated email sink if it belongs to this W&B resource
func deleteGeneratedEmailSink(ctx context.Context, client ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	secret := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Name:      emailSinkSecretName(wandb),
		Namespace: wandb.Namespace,
	}, secret)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get generated email sink Secret: %w", err)
	}
	if !hasOwnerReference(secret, wandb) {
		return nil
	}
	if err := client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete generated email sink Secret: %w", err)
	}
	return nil
}
