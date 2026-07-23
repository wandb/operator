package objectstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// connectionRequiredKeys are the secret keys every consumer must be able to read;
// their selectors are never marked optional regardless of the optionality policy.
var connectionRequiredKeys = map[string]bool{"url": true, "Provider": true, "Bucket": true}

// ToSecretData encodes the connection into the Secret's StringData: the canonical
// "url", the discrete keys consumers read, and the bool flags. Empty discrete
// values are omitted so absent fields stay absent (matching the resolver's
// treatment of missing keys). ForcePathStyle is S3-only.
func (c ConnInfo) ToSecretData() map[string]string {
	data := map[string]string{"url": c.URL}

	put := func(key, value string) {
		if value != "" {
			data[key] = value
		}
	}
	put("Provider", string(c.Provider))
	put("Bucket", c.Bucket)
	put("Host", c.Endpoint)
	put("Port", c.Port)
	put("AccessKey", c.AccessKey)
	put("SecretKey", c.SecretKey)
	put("Region", c.Region)
	put("Path", c.Path)
	put("Scheme", c.Scheme)

	data["TlsEnabled"] = strconv.FormatBool(c.TlsEnabled)
	if c.Provider == apiv2.ObjectStoreProviderS3 {
		data["ForcePathStyle"] = strconv.FormatBool(c.ForcePathStyle)
	}
	return data
}

// ToObjectStoreConnection builds the selector view of the connection secret named
// secretName. It emits a selector only for keys ToSecretData actually writes.
// When requireAll is true every selector is required (managed SeaweedFS always
// writes the full key set); otherwise only url/Provider/Bucket are required and
// the rest are optional (external configs omit provider-dependent keys).
func (c ConnInfo) ToObjectStoreConnection(secretName string, requireAll bool) *apiv2.ObjectStoreConnection {
	data := c.ToSecretData()
	localRef := corev1.LocalObjectReference{Name: secretName}

	sel := func(key string) corev1.SecretKeySelector {
		optional := !requireAll && !connectionRequiredKeys[key]
		return corev1.SecretKeySelector{LocalObjectReference: localRef, Key: key, Optional: ptr.To(optional)}
	}
	has := func(key string) bool { _, ok := data[key]; return ok }

	conn := &apiv2.ObjectStoreConnection{}
	if has("url") {
		conn.URL = sel("url")
	}
	if has("Provider") {
		conn.Provider = sel("Provider")
	}
	if has("Host") {
		conn.Endpoint = sel("Host")
	}
	if has("Port") {
		conn.Port = sel("Port")
	}
	if has("AccessKey") {
		conn.AccessKey = sel("AccessKey")
	}
	if has("SecretKey") {
		conn.SecretKey = sel("SecretKey")
	}
	if has("Bucket") {
		conn.Bucket = sel("Bucket")
	}
	if has("Path") {
		conn.Path = sel("Path")
	}
	if has("Region") {
		conn.Region = sel("Region")
	}
	if has("TlsEnabled") {
		conn.TlsEnabled = sel("TlsEnabled")
	}
	if has("ForcePathStyle") {
		conn.ForcePathStyle = sel("ForcePathStyle")
	}
	return conn
}

// Resolve reads the connection's secret selectors into a ConnInfo.
func Resolve(
	ctx context.Context,
	cl client.Client,
	namespace string,
	conn *apiv2.ObjectStoreConnection,
) (ConnInfo, error) {
	if conn == nil {
		return ConnInfo{}, fmt.Errorf("object store connection is not available yet")
	}

	resolver := &utils.ConnSecretResolver{Client: cl, Namespace: namespace, Cache: map[string]*corev1.Secret{}}

	info := ConnInfo{
		AccessKeyRef: conn.AccessKey,
		SecretKeyRef: conn.SecretKey,
	}

	provider, err := resolver.Value(ctx, conn.Provider)
	if err != nil {
		return ConnInfo{}, err
	}
	// Legacy-migrated connections never set Provider; match WriteState's default so
	// downstream provider switches (ProviderURI, ToSecretData) stay correct.
	if provider == "" {
		provider = string(apiv2.ObjectStoreProviderS3)
	}
	info.Provider = apiv2.ObjectStoreProvider(provider)

	if info.Bucket, err = resolver.Value(ctx, conn.Bucket); err != nil {
		return ConnInfo{}, err
	}
	if info.Endpoint, err = resolver.Value(ctx, conn.Endpoint); err != nil {
		return ConnInfo{}, err
	}
	if info.Port, err = resolver.Value(ctx, conn.Port); err != nil {
		return ConnInfo{}, err
	}
	if info.Region, err = resolver.Value(ctx, conn.Region); err != nil {
		return ConnInfo{}, err
	}
	if info.AccessKey, err = resolver.Value(ctx, conn.AccessKey); err != nil {
		return ConnInfo{}, err
	}
	if info.SecretKey, err = resolver.Value(ctx, conn.SecretKey); err != nil {
		return ConnInfo{}, err
	}
	// A half-configured pair silently picks the wrong credential mode downstream.
	if (info.AccessKey == "") != (info.SecretKey == "") {
		return ConnInfo{}, fmt.Errorf("object store access key and secret key must be configured together")
	}
	if info.Path, err = resolver.Value(ctx, conn.Path); err != nil {
		return ConnInfo{}, err
	}

	forcePathStyleString, err := resolver.Value(ctx, conn.ForcePathStyle)
	if err != nil {
		return ConnInfo{}, err
	}
	if fps, parseErr := strconv.ParseBool(forcePathStyleString); parseErr == nil {
		info.ForcePathStyle = fps
	} else {
		// Connection secrets written before the operator derived this key lack it.
		info.ForcePathStyle = RequiresPathStyle(info.Endpoint)
	}

	tlsEnabledString, err := resolver.Value(ctx, conn.TlsEnabled)
	if err != nil {
		return ConnInfo{}, err
	}
	if tls, parseErr := strconv.ParseBool(tlsEnabledString); parseErr == nil {
		info.TlsEnabled = tls
	}

	return info, nil
}

// SplitBucketPath splits "bucket/optional/prefix" into the leading bucket (or container) segment and the remaining object prefix.
func SplitBucketPath(raw string) (bucket, path string) {
	trimmed := strings.TrimPrefix(raw, "/")
	if slash := strings.IndexByte(trimmed, '/'); slash >= 0 {
		return trimmed[:slash], trimmed[slash+1:]
	}
	return trimmed, ""
}
