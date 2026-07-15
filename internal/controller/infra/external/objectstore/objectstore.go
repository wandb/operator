package objectstore

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/external"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ConnectionSecretName = "wandb-objectstore-connection"

func connectionSecretName(key string) string {
	if key == "" || key == apiv2.DefaultInstanceName {
		return ConnectionSecretName
	}
	return fmt.Sprintf("%s-%s", ConnectionSecretName, key)
}

func WriteState(
	ctx context.Context,
	c client.Client,
	wandb *apiv2.WeightsAndBiases,
	key string,
	spec *apiv2.ObjectStoreConnection,
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	logger := ctrl.LoggerFrom(ctx)

	fields := map[string]corev1.SecretKeySelector{
		"Host":           spec.Endpoint,
		"Port":           spec.Port,
		"AccessKey":      spec.AccessKey,
		"SecretKey":      spec.SecretKey,
		"Bucket":         spec.Bucket,
		"Region":         spec.Region,
		"Provider":       spec.Provider,
		"TlsEnabled":     spec.TlsEnabled,
		"ForcePathStyle": spec.ForcePathStyle,
	}

	data, err := external.ResolveFields(ctx, c, wandb.Namespace, fields)
	if err != nil {
		logger.Error(err, "failed to resolve external object store fields")
		return []metav1.Condition{{
			Type:   "Reconciled",
			Status: metav1.ConditionFalse,
			Reason: "ApiError",
		}}, nil
	}

	provider := apiv2.ObjectStoreProvider(data["Provider"])
	if provider == "" {
		provider = apiv2.ObjectStoreProviderS3
	}
	data["Provider"] = string(provider)

	switch provider {
	case apiv2.ObjectStoreProviderGCS:
		data["url"] = buildGCSURL(data)
	case apiv2.ObjectStoreProviderAzure:
		data["url"] = buildAzureURL(data)
	default:
		data["url"] = buildS3URL(data)
	}

	nsName := types.NamespacedName{Namespace: wandb.Namespace, Name: connectionSecretName(key)}
	if conditions := external.WriteConnectionSecret(ctx, c, wandb, nsName, data); conditions != nil {
		return conditions, nil
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	// ResolveFields only writes non-empty values, so any field that is
	// legitimately absent for some deployment must be optional: Host (plain
	// AWS S3 with no custom endpoint), AccessKey/SecretKey (IAM-role /
	// workload-identity auth), Region (MinIO or region supplied out-of-band).
	// url and Bucket are always written.
	return nil, &apiv2.ObjectStoreConnection{
		Provider:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Provider", Optional: ptr.To(false)},
		URL:            corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "url", Optional: ptr.To(false)},
		Endpoint:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(true)},
		Port:           corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(true)},
		AccessKey:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "AccessKey", Optional: ptr.To(true)},
		SecretKey:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SecretKey", Optional: ptr.To(true)},
		Bucket:         corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Bucket", Optional: ptr.To(false)},
		Region:         corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Region", Optional: ptr.To(true)},
		TlsEnabled:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "TlsEnabled", Optional: ptr.To(true)},
		ForcePathStyle: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "ForcePathStyle", Optional: ptr.To(true)},
	}
}

// buildS3URL assembles s3://[accessKey:secretKey@][host[:port]]/bucket; host and creds are omitted for native AWS S3 / IAM-role auth.
func buildS3URL(data map[string]string) string {
	bucketURL := url.URL{
		Scheme: "s3",
		Path:   data["Bucket"],
	}
	if _, ok := data["Host"]; ok {
		if _, ok := data["Port"]; ok {
			bucketURL.Host = fmt.Sprintf("%s:%s", data["Host"], data["Port"])
		} else {
			bucketURL.Host = data["Host"]
		}
	}
	if _, ok := data["AccessKey"]; ok {
		bucketURL.User = url.UserPassword(data["AccessKey"], data["SecretKey"])
	}
	return bucketURL.String()
}

// buildGCSURL assembles gs://<bucket>[/path]; creds default to workload identity, or accessKey (SA email) + secretKey (PEM key) as userinfo.
func buildGCSURL(data map[string]string) string {
	bucket, path := splitBucketPath(data["Bucket"])
	bucketURL := url.URL{Scheme: "gs", Host: bucket}
	if path != "" {
		bucketURL.Path = "/" + path
	}
	if ak := data["AccessKey"]; ak != "" {
		bucketURL.User = url.UserPassword(ak, data["SecretKey"])
	}
	return bucketURL.String()
}

// buildAzureURL assembles az://<account>/<container>[/path] from accessKey (account), bucket (container), and secretKey (account key, when set).
func buildAzureURL(data map[string]string) string {
	account := data["AccessKey"]
	container, path := splitBucketPath(data["Bucket"])
	bucketURL := url.URL{Scheme: "az", Host: account, Path: "/" + container}
	if path != "" {
		bucketURL.Path += "/" + path
	}
	if key := data["SecretKey"]; key != "" {
		bucketURL.User = url.UserPassword("", key)
	}
	return bucketURL.String()
}

// splitBucketPath splits "bucket/optional/prefix" into the leading bucket (or container) segment and the remaining object prefix.
func splitBucketPath(raw string) (bucket, path string) {
	trimmed := strings.TrimPrefix(raw, "/")
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		return trimmed[:slash], trimmed[slash+1:]
	}
	return trimmed, ""
}

func ReadState(
	_ context.Context,
	_ client.Client,
	_ *apiv2.WeightsAndBiases,
	_ string,
	newConditions []metav1.Condition,
) []metav1.Condition {
	return newConditions
}

func DeleteConnectionSecret(ctx context.Context, c client.Client, wandb *apiv2.WeightsAndBiases, key string) error {
	return external.DeleteConnectionSecret(ctx, c, types.NamespacedName{
		Namespace: wandb.Namespace,
		Name:      connectionSecretName(key),
	})
}
