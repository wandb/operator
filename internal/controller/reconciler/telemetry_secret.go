package reconciler

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func reconcileTelemetryConnectionSecret(
	ctx context.Context,
	client ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	telemetryConfig TelemetryRuntimeConfig,
) error {
	if !telemetryConfig.Enabled {
		return nil
	}

	resolvedEndpoints := telemetryConfig.ResolveEndpoints()
	gorillaTracer := resolveGorillaTracerConnection(telemetryConfig.OTel.Protocol, resolvedEndpoints.TracesEndpoint)
	desiredData := map[string][]byte{
		"OTEL_EXPORTER_OTLP_PROTOCOL":         []byte(telemetryConfig.OTel.Protocol),
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": []byte(resolvedEndpoints.MetricsEndpoint),
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT":    []byte(resolvedEndpoints.LogsEndpoint),
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  []byte(resolvedEndpoints.TracesEndpoint),
		"OTEL_METRICS_EXPORTER":               []byte("otlp"),
		"OTEL_LOGS_EXPORTER":                  []byte("otlp"),
		"OTEL_TRACES_EXPORTER":                []byte("otlp"),
		"OTEL_SERVICE_NAME":                   []byte(telemetryConfig.OTel.ServiceName),
		"OTEL_RESOURCE_ATTRIBUTES":            []byte(telemetryConfig.OTel.ResourceAttributes),
		"GORILLA_TRACER":                      []byte(gorillaTracer),
	}

	secretLookup := types.NamespacedName{
		Name:      telemetryConfig.OTel.SecretName,
		Namespace: wandb.Namespace,
	}
	secret := &corev1.Secret{}
	err := client.Get(ctx, secretLookup, secret)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretLookup.Name,
				Namespace: secretLookup.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "wandb-operator",
					"app.kubernetes.io/instance":   wandb.Name,
					"app.kubernetes.io/component":  "telemetry",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: desiredData,
		}
		if err := controllerutil.SetOwnerReference(wandb, secret, client.Scheme()); err != nil {
			return err
		}
		return client.Create(ctx, secret)
	}

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels["app.kubernetes.io/managed-by"] = "wandb-operator"
	secret.Labels["app.kubernetes.io/instance"] = wandb.Name
	secret.Labels["app.kubernetes.io/component"] = "telemetry"
	secret.Type = corev1.SecretTypeOpaque

	updated := false
	if !reflect.DeepEqual(secret.Data, desiredData) {
		secret.Data = desiredData
		updated = true
	}

	if !metav1.IsControlledBy(secret, wandb) {
		if err := controllerutil.SetOwnerReference(wandb, secret, client.Scheme()); err != nil {
			return err
		}
		updated = true
	}

	if !updated {
		return nil
	}
	return client.Update(ctx, secret)
}

func resolveGorillaTracerConnection(protocol, tracesEndpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(tracesEndpoint))
	if err != nil || parsed.Host == "" {
		return "noop://"
	}

	protocol = strings.ToLower(strings.TrimSpace(protocol))
	connectionType := "otlp+http"
	switch {
	case strings.Contains(protocol, "grpc"):
		connectionType = "otlp+grpc"
	case parsed.Scheme == "https":
		connectionType = "otlp+https"
	}

	return fmt.Sprintf("%s://%s", connectionType, parsed.Host)
}
