package reconciler

import (
	"context"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func emailTestSelector(name, key string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}
}

// emailTestValue is the ValueOrSecret counterpart for the notification spec
// fields (Sink/SMTP/Slack); emailTestSelector remains for status.emailSink.
func emailTestValue(name, key string) apiv2.ValueOrSecret {
	return apiv2.ValueFromSecret(name, key, false)
}

func emailTestWandb(t *testing.T) (*runtime.Scheme, *apiv2.WeightsAndBiases) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := apiv2.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
			UID:       types.UID("wandb-uid"),
		},
	}
	wandb.Spec.Wandb.Notifications = &apiv2.NotificationsSpec{}
	return scheme, wandb
}

func TestReconcileEmailSinkUsesConfiguredSink(t *testing.T) {
	scheme, wandb := emailTestWandb(t)
	selector := emailTestValue("existing-email", "url")
	wandb.Spec.Wandb.Notifications.Email = &apiv2.EmailSpec{Sink: &selector}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb).Build()

	if err := reconcileEmailSink(context.Background(), client, wandb); err != nil {
		t.Fatal(err)
	}
	if wandb.Status.EmailSink == nil || wandb.Status.EmailSink.Name != "existing-email" || wandb.Status.EmailSink.Key != "url" {
		t.Fatalf("unexpected effective email sink: %+v", wandb.Status.EmailSink)
	}
}

func TestReconcileEmailSinkDoesNotDeleteConfiguredSink(t *testing.T) {
	scheme, wandb := emailTestWandb(t)
	selector := emailTestValue("wandb-email-sink", "sink")
	wandb.Spec.Wandb.Notifications.Email = &apiv2.EmailSpec{Sink: &selector}
	configured := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      selector.SecretKeyRef().Name,
			Namespace: wandb.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: apiv2.GroupVersion.String(),
				Kind:       "WeightsAndBiases",
				Name:       wandb.Name,
				UID:        wandb.UID,
			}},
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, configured).Build()

	if err := reconcileEmailSink(context.Background(), client, wandb); err != nil {
		t.Fatal(err)
	}
	if err := client.Get(context.Background(), types.NamespacedName{Name: configured.Name, Namespace: configured.Namespace}, &corev1.Secret{}); err != nil {
		t.Fatalf("configured sink was deleted: %v", err)
	}
}

func TestResolveEnvvarsCustomResourceEmailSink(t *testing.T) {
	_, wandb := emailTestWandb(t)
	selector := emailTestSelector("effective-email", "sink")
	wandb.Status.EmailSink = &selector
	envs := []serverManifest.EnvVar{{
		Name: "GORILLA_EMAIL_SINK",
		Sources: []serverManifest.EnvSource{{
			Type: "custom-resource", Field: "status.emailSink",
		}},
	}}

	resolved, err := resolveEnvvars(context.Background(), fake.NewClientBuilder().Build(), wandb, serverManifest.Manifest{}, nil, envs)
	if err != nil {
		t.Fatal(err)
	}
	env := mustFindEnvVar(t, resolved, "GORILLA_EMAIL_SINK")
	if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("email sink did not resolve to a SecretKeyRef: %+v", env)
	}
	if env.ValueFrom.SecretKeyRef.Name != "effective-email" || env.ValueFrom.SecretKeyRef.Key != "sink" {
		t.Fatalf("unexpected email sink selector: %+v", env.ValueFrom.SecretKeyRef)
	}
}

func TestReconcileEmailSinkGeneratesAuthenticatedSMTPURL(t *testing.T) {
	scheme, wandb := emailTestWandb(t)
	username := emailTestValue("smtp", "username")
	password := emailTestValue("smtp", "password")
	wandb.Spec.Wandb.Notifications.Email = &apiv2.EmailSpec{SMTP: &apiv2.EmailSMTPSpec{
		Host:     emailTestValue("smtp", "host"),
		Port:     emailTestValue("smtp", "port"),
		Username: username,
		Password: password,
	}}
	smtpSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "smtp", Namespace: "default"},
		Data: map[string][]byte{
			"host":     []byte("smtp.example.com"),
			"port":     []byte("587"),
			"username": []byte("user@example.com"),
			"password": []byte("p@ss/word"),
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, smtpSecret).Build()

	if err := reconcileEmailSink(context.Background(), client, wandb); err != nil {
		t.Fatal(err)
	}

	generated := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: "wandb-email-sink", Namespace: "default"}, generated); err != nil {
		t.Fatal(err)
	}
	value := generated.StringData[emailSinkSecretKey]
	if value == "" {
		value = string(generated.Data[emailSinkSecretKey])
	}
	if want := "smtp://user%40example.com:p%40ss%2Fword@smtp.example.com:587"; value != want {
		t.Fatalf("unexpected SMTP URL: got %q want %q", value, want)
	}
	if wandb.Status.EmailSink == nil || wandb.Status.EmailSink.Name != "wandb-email-sink" || wandb.Status.EmailSink.Key != emailSinkSecretKey {
		t.Fatalf("unexpected effective email sink: %+v", wandb.Status.EmailSink)
	}
}

func TestReconcileEmailSinkRemovesGeneratedSecretWhenDisabled(t *testing.T) {
	scheme, wandb := emailTestWandb(t)
	generated := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb-email-sink",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: apiv2.GroupVersion.String(),
				Kind:       "WeightsAndBiases",
				Name:       wandb.Name,
				UID:        wandb.UID,
			}},
		},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(wandb, generated).Build()

	if err := reconcileEmailSink(context.Background(), client, wandb); err != nil {
		t.Fatal(err)
	}
	err := client.Get(context.Background(), types.NamespacedName{Name: generated.Name, Namespace: generated.Namespace}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected generated Secret to be deleted, got %v", err)
	}
}
