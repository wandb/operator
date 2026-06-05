package altinity

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	chtypes "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/common/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// StoragePolicyName is the ClickHouse storage policy backed by the object
	// store. It is set as the server-wide default so all MergeTree tables land in
	// the bucket without per-table DDL.
	StoragePolicyName = "s3_main"

	// DefaultObjectStoragePrefix is the key prefix used within the bucket when the
	// spec does not set one. The trailing slash is significant to ClickHouse.
	DefaultObjectStoragePrefix = "clickhouse/"

	s3DiskName      = "s3_disk"
	s3CacheDiskName = "s3_cache"
	s3MetadataPath  = "/var/lib/clickhouse/disks/s3_disk/"
	s3CachePath     = "/var/lib/clickhouse/s3_cache/"

	// storageConfigKey is the settings path prefix that renders to the
	// <storage_configuration> ClickHouse config section.
	storageConfigKey = "storage_configuration"
)

// ObjectStorageConn holds the resolved object-store connection details the CHI
// builder needs to configure the S3-backed disk. Non-secret fields are resolved
// to literals; credentials remain secret references that the Altinity operator
// injects into the pod as env vars and reads back via from_env.
type ObjectStorageConn struct {
	// Endpoint is the full http(s) URL including bucket and prefix, with a
	// trailing slash (ClickHouse treats it as a path prefix).
	Endpoint string
	Region   string
	// UseEnvCredentials is true when the bucket relies on ambient credentials
	// (pod IAM role / workload identity) rather than explicit access keys.
	UseEnvCredentials bool
	AccessKeyRef      corev1.SecretKeySelector
	SecretKeyRef      corev1.SecretKeySelector
}

// ResolveObjectStorage reads the object-store connection secret and derives the
// details needed to back ClickHouse with the bucket. It reads the secret from
// the managed ClickHouse namespace, which (with default namespace handling) is
// also where the ClickHouse pods resolve the credential env vars.
func ResolveObjectStorage(
	ctx context.Context,
	cl client.Client,
	wandb *apiv2.WeightsAndBiases,
	conn *apiv2.ObjectStoreConnection,
) (*ObjectStorageConn, error) {
	spec := wandb.Spec.ClickHouse.ManagedClickHouse
	if spec == nil {
		return nil, nil
	}
	if conn == nil {
		return nil, fmt.Errorf("object store connection is not available yet")
	}

	secretName := conn.Bucket.Name
	if secretName == "" {
		return nil, fmt.Errorf("object store connection has no secret reference")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: spec.Namespace, Name: secretName}
	if err := cl.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("read object store connection secret %q: %w", key, err)
	}

	get := func(k string) string { return strings.TrimSpace(string(secret.Data[k])) }

	bucket := get("Bucket")
	if bucket == "" {
		return nil, fmt.Errorf("object store connection secret %q is missing Bucket", secretName)
	}

	endpoint, err := buildEndpoint(get("Scheme"), get("Host"), get("Port"), bucket, get("Region"), objectStoragePrefix(spec))
	if err != nil {
		return nil, err
	}

	return &ObjectStorageConn{
		Endpoint:          endpoint,
		Region:            get("Region"),
		UseEnvCredentials: get("AccessKey") == "",
		AccessKeyRef:      conn.AccessKey,
		SecretKeyRef:      conn.SecretKey,
	}, nil
}

func objectStoragePrefix(spec *apiv2.ManagedClickHouseSpec) string {
	return normalizePrefix(spec.ObjectStorage.Prefix)
}

// normalizePrefix returns a bucket key prefix with no leading slash and exactly
// one trailing slash, defaulting when empty.
func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return DefaultObjectStoragePrefix
	}
	prefix = strings.Trim(prefix, "/")
	return prefix + "/"
}

// buildEndpoint constructs the ClickHouse S3 disk endpoint URL. When a custom
// host is present (managed SeaweedFS or an S3-compatible endpoint) it uses
// path-style addressing; otherwise it derives the AWS virtual-hosted endpoint
// from the region.
func buildEndpoint(scheme, host, port, bucket, region, prefix string) (string, error) {
	if host != "" {
		if scheme == "" {
			scheme = "https"
			if port == "80" {
				scheme = "http"
			}
		}
		hostport := host
		if port != "" {
			hostport = host + ":" + port
		}
		return fmt.Sprintf("%s://%s/%s/%s", scheme, hostport, bucket, prefix), nil
	}

	if region == "" {
		return "", fmt.Errorf("object store has no Host and no Region; cannot derive an S3 endpoint")
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, prefix), nil
}

// applyStorageConfiguration adds the ClickHouse <storage_configuration> (an S3
// disk, a local read-through cache wrapping it, and the storage policy) to the
// given settings via the typed Settings API. Credentials are expressed as
// secret references; the Altinity operator injects them as pod env vars and
// renders from_env in the generated config, so plaintext credentials never
// appear in the CHI or its ConfigMaps.
func applyStorageConfiguration(settings *v1.Settings, oc *ObjectStorageConn, cacheMaxSizeBytes int64) {
	disk := diskKey(s3DiskName)
	settings.Set(disk("type"), v1.NewSettingScalar("s3"))
	settings.Set(disk("endpoint"), v1.NewSettingScalar(oc.Endpoint))
	settings.Set(disk("metadata_path"), v1.NewSettingScalar(s3MetadataPath))
	if oc.Region != "" {
		settings.Set(disk("region"), v1.NewSettingScalar(oc.Region))
	}
	if oc.UseEnvCredentials {
		settings.Set(disk("use_environment_credentials"), v1.NewSettingScalar("true"))
	} else {
		settings.Set(disk("access_key_id"), secretSetting(oc.AccessKeyRef))
		settings.Set(disk("secret_access_key"), secretSetting(oc.SecretKeyRef))
	}

	cache := diskKey(s3CacheDiskName)
	settings.Set(cache("type"), v1.NewSettingScalar("cache"))
	settings.Set(cache("disk"), v1.NewSettingScalar(s3DiskName))
	settings.Set(cache("path"), v1.NewSettingScalar(s3CachePath))
	settings.Set(cache("max_size"), v1.NewSettingScalar(strconv.FormatInt(cacheMaxSizeBytes, 10)))

	settings.Set(
		storageConfigKey+"/policies/"+StoragePolicyName+"/volumes/main/disk",
		v1.NewSettingScalar(s3CacheDiskName),
	)
}

// diskKey returns a helper that builds settings paths for a named disk, e.g.
// diskKey("s3_disk")("type") -> "storage_configuration/disks/s3_disk/type".
func diskKey(name string) func(string) string {
	prefix := storageConfigKey + "/disks/" + name + "/"
	return func(field string) string { return prefix + field }
}

// secretSetting builds a ClickHouse setting whose value is sourced from a
// Kubernetes secret. The Altinity operator wires it as a pod env var + from_env.
func secretSetting(ref corev1.SecretKeySelector) *v1.Setting {
	r := ref
	return v1.NewSettingSource(&v1.SettingSource{
		ValueFrom: &chtypes.DataSource{SecretKeyRef: &r},
	})
}
