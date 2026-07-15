package altinity

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	"github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	chtypes "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/common/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// StoragePolicyName is the object-store-backed policy, set server-wide so all
	// MergeTree tables land in the bucket without per-table DDL.
	StoragePolicyName = "s3_main"

	// DefaultObjectStoragePrefix is the in-bucket prefix when unset (trailing slash matters).
	DefaultObjectStoragePrefix = "clickhouse/"

	s3DiskName = "s3_disk"
	// Must sort after s3DiskName ('/' < '_'): the renderer emits <disks> in sorted
	// order and ClickHouse requires the wrapped disk before its cache disk.
	s3CacheDiskName = "s3_disk_cache"
	s3MetadataPath  = "/var/lib/clickhouse/disks/s3_disk/"
	s3CachePath     = "/var/lib/clickhouse/disks/s3_disk_cache/"

	// storageConfigKey renders to the <storage_configuration> config section.
	storageConfigKey = "storage_configuration"
)

// ObjectStorageConn holds the resolved bucket connection used to configure the
// S3-backed disk: endpoint/region as literals, credentials as secret references.
type ObjectStorageConn struct {
	// Endpoint is the full http(s) URL incl. bucket + prefix, trailing slash.
	Endpoint string
	Region   string
	// UseEnvCredentials uses ambient creds (IAM role) instead of access keys.
	UseEnvCredentials bool
	AccessKeyRef      corev1.SecretKeySelector
	SecretKeyRef      corev1.SecretKeySelector
}

// ResolveObjectStorage reads the object-store connection's secret key references
// (which may span multiple secrets) and derives the S3 disk details.
func ResolveObjectStorage(
	ctx context.Context,
	cl client.Client,
	spec *apiv2.ManagedClickHouseSpec,
	conn *apiv2.ObjectStoreConnection,
) (*ObjectStorageConn, error) {
	if spec == nil {
		return nil, nil
	}
	if conn == nil {
		return nil, fmt.Errorf("object store connection is not available yet")
	}

	r := &utils.ConnSecretResolver{Client: cl, Namespace: spec.Namespace, Cache: map[string]*corev1.Secret{}}

	bucket, err := r.Value(ctx, conn.Bucket)
	if err != nil {
		return nil, err
	}
	if bucket == "" {
		return nil, fmt.Errorf("object store connection has no bucket reference")
	}

	host, err := r.Value(ctx, conn.Endpoint)
	if err != nil {
		return nil, err
	}
	port, err := r.Value(ctx, conn.Port)
	if err != nil {
		return nil, err
	}
	region, err := r.Value(ctx, conn.Region)
	if err != nil {
		return nil, err
	}
	accessKey, err := r.Value(ctx, conn.AccessKey)
	if err != nil {
		return nil, err
	}

	tlsEnabledString, err := r.Value(ctx, conn.TlsEnabled)
	if err != nil {
		return nil, err
	}

	tlsEnabled, err := strconv.ParseBool(tlsEnabledString)
	if err != nil {
		tlsEnabled = false
	}

	// The connection has no scheme reference; honor one advertised in the
	// endpoint value, otherwise let buildEndpoint infer from spec.Insecure.
	scheme := ""
	if i := strings.Index(host, "://"); i >= 0 {
		scheme, host = host[:i], host[i+len("://"):]
	}

	endpoint, err := buildEndpoint(scheme, host, port, bucket, region, objectStoragePrefix(spec), tlsEnabled)
	if err != nil {
		return nil, err
	}

	return &ObjectStorageConn{
		Endpoint:          endpoint,
		Region:            region,
		UseEnvCredentials: accessKey == "",
		AccessKeyRef:      conn.AccessKey,
		SecretKeyRef:      conn.SecretKey,
	}, nil
}

func objectStoragePrefix(spec *apiv2.ManagedClickHouseSpec) string {
	return normalizePrefix(spec.ObjectStorage.Prefix)
}

// normalizePrefix strips leading slashes and ensures one trailing slash, defaulting when empty.
func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return DefaultObjectStoragePrefix
	}
	prefix = strings.Trim(prefix, "/")
	return prefix + "/"
}

// buildEndpoint builds the S3 disk endpoint: path-style for a custom host, else
// the AWS virtual-hosted URL derived from the region.
func buildEndpoint(scheme, host, port, bucket, region, prefix string, secure bool) (string, error) {
	if host != "" {
		if scheme == "" {
			scheme = "http"
			if secure {
				scheme = "https"
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

// applyStorageConfiguration adds the <storage_configuration> (S3 disk + cache +
// policy) to settings. Credentials are secret references so the Altinity
// operator injects them via from_env rather than as plaintext.
// TODO(dpanzella): Currently only supports S3 compatible storage, add support for Azure and GCS
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

	// Server-wide default so W&B tables use the bucket without per-table DDL.
	// system_*_log tables ship a predefined <engine> (which can't take a separate
	// storage_policy), so they inherit this and live in the bucket too.
	settings.Set("merge_tree/storage_policy", v1.NewSettingScalar(StoragePolicyName))
}

// diskKey builds settings paths for a named disk, e.g.
// diskKey("s3_disk")("type") -> "storage_configuration/disks/s3_disk/type".
func diskKey(name string) func(string) string {
	prefix := storageConfigKey + "/disks/" + name + "/"
	return func(field string) string { return prefix + field }
}

// secretSetting builds a setting sourced from a Kubernetes secret; the Altinity
// operator wires it as a pod env var + from_env.
func secretSetting(ref corev1.SecretKeySelector) *v1.Setting {
	r := ref
	return v1.NewSettingSource(&v1.SettingSource{
		ValueFrom: &chtypes.DataSource{SecretKeyRef: &r},
	})
}
