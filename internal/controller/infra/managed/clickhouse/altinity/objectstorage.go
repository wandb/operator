package altinity

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
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

// ResolveObjectStorage resolves the connection and builds the S3 endpoint.
func ResolveObjectStorage(
	ctx context.Context,
	cl client.Client,
	spec *apiv2.ManagedClickHouseSpec,
	conn *apiv2.ObjectStoreConnection,
) (*objectstore.ConnInfo, string, error) {
	if spec == nil {
		return nil, "", nil
	}

	ci, err := objectstore.Resolve(ctx, cl, spec.Namespace, conn)
	if err != nil {
		return nil, "", err
	}
	if ci.Bucket == "" {
		return nil, "", fmt.Errorf("object store connection has no bucket reference")
	}

	endpoint, err := buildEndpoint(ci, objectStoragePrefix(spec))
	if err != nil {
		return nil, "", err
	}

	return &ci, endpoint, nil
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

// buildEndpoint builds the S3 disk endpoint: path-style for a custom endpoint,
// else the AWS virtual-hosted URL derived from the region.
func buildEndpoint(ci objectstore.ConnInfo, prefix string) (string, error) {
	if base := ci.EndpointURL(); base != "" {
		return fmt.Sprintf("%s/%s/%s", base, ci.Bucket, prefix), nil
	}

	if ci.Region == "" {
		return "", fmt.Errorf("object store has no Host and no Region; cannot derive an S3 endpoint")
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", ci.Bucket, ci.Region, prefix), nil
}

// applyStorageConfiguration sets the S3 disk, cache, and storage policy.
// TODO(dpanzella): only S3 supported; add Azure and GCS.
func applyStorageConfiguration(settings *v1.Settings, ci *objectstore.ConnInfo, endpoint string, cacheMaxSizeBytes int64) {
	disk := diskKey(s3DiskName)
	settings.Set(disk("type"), v1.NewSettingScalar("s3"))
	settings.Set(disk("endpoint"), v1.NewSettingScalar(endpoint))
	settings.Set(disk("metadata_path"), v1.NewSettingScalar(s3MetadataPath))
	if ci.Region != "" {
		settings.Set(disk("region"), v1.NewSettingScalar(ci.Region))
	}
	if ci.AccessKey == "" {
		settings.Set(disk("use_environment_credentials"), v1.NewSettingScalar("true"))
	} else {
		settings.Set(disk("access_key_id"), secretSetting(ci.AccessKeyRef))
		settings.Set(disk("secret_access_key"), secretSetting(ci.SecretKeyRef))
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
