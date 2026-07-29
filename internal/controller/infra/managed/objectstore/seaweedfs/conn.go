package seaweedfs

import (
	"context"
	"fmt"
	"net/url"
<<<<<<< HEAD
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
=======

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
>>>>>>> main
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	S3UrlScheme = "s3"
	S3Port      = "8333"
)

<<<<<<< HEAD
type s3ConnInfo struct {
	AccessKey      string
	SecretKey      string
	Host           string
	Port           string
	Bucket         string
	TLS            bool
	ForcePathStyle bool
}

func buildS3ConnInfo(
	accessKey, secretKey string, nsnBuilder *NsNameBuilder, tls bool,
) *s3ConnInfo {
	return &s3ConnInfo{
		AccessKey:      accessKey,
		TLS:            tls,
		SecretKey:      secretKey,
		Host:           s3ServiceHost(nsnBuilder.SpecName(), nsnBuilder.Namespace()),
		Port:           S3Port,
		Bucket:         "bucket",
		ForcePathStyle: true,
	}
}

=======
// buildS3ConnInfo assembles the ConnInfo for the managed SeaweedFS S3 gateway:
// the in-cluster service host/port, path-style addressing, and default region.
func buildS3ConnInfo(
	accessKey, secretKey string, nsnBuilder *NsNameBuilder, tls bool,
) *objectstore.ConnInfo {
	connInfo := &objectstore.ConnInfo{
		Provider:       apiv2.ObjectStoreProviderS3,
		AccessKey:      accessKey,
		SecretKey:      secretKey,
		Endpoint:       s3ServiceHost(nsnBuilder.SpecName(), nsnBuilder.Namespace()),
		Port:           S3Port,
		Region:         objectstore.DefaultRegion,
		Bucket:         "bucket",
		Scheme:         objectstore.SchemeForTLS(tls),
		TlsEnabled:     tls,
		ForcePathStyle: true,
	}
	connInfo.URL = managedS3URL(connInfo)
	return connInfo
}

// managedS3URL builds the canonical connection URL the W&B server signs against:
// s3://<accessKey>:<secretKey>@<host>:<port>/<bucket>?tls=<bool>.
func managedS3URL(connInfo *objectstore.ConnInfo) string {
	s3URL := &url.URL{
		Scheme: S3UrlScheme,
		Host:   fmt.Sprintf("%s:%s", connInfo.Endpoint, connInfo.Port),
		User:   url.UserPassword(connInfo.AccessKey, connInfo.SecretKey),
		Path:   connInfo.Bucket,
	}
	return fmt.Sprintf("%s?tls=%t", s3URL.String(), connInfo.TlsEnabled)
}

// s3ServiceHost returns the in-cluster FQDN of the SeaweedFS S3 service.
>>>>>>> main
func s3ServiceHost(specName, namespace string) string {
	return fmt.Sprintf("%s-s3.%s.svc.cluster.local", SeaweedName(specName), namespace)
}

// s3ExternalURL is the endpoint the W&B server signs S3 requests against
// (it presigns with this host and rewrites the URL for external clients
// without re-signing). The s3 gateway must verify signatures against this
// host rather than the Host/X-Forwarded-Host of proxied requests.
func s3ExternalURL(specName, namespace string, tls bool) string {
<<<<<<< HEAD
	return fmt.Sprintf("%s://%s:%s", s3Scheme(tls), s3ServiceHost(specName, namespace), S3Port)
}

func s3Scheme(tls bool) string {
	if tls {
		return "https"
	}
	return "http"
}

func (s *s3ConnInfo) toUrl() *url.URL {
	return &url.URL{
		Scheme: S3UrlScheme,
		Host:   fmt.Sprintf("%s:%s", s.Host, s.Port),
		User:   url.UserPassword(s.AccessKey, s.SecretKey),
		Path:   s.Bucket,
	}
}

func (s *s3ConnInfo) scheme() string {
	return s3Scheme(s.TLS)
}

=======
	return fmt.Sprintf("%s://%s:%s", objectstore.SchemeForTLS(tls), s3ServiceHost(specName, namespace), S3Port)
}

// writeWandbConnInfo writes the connection secret consumed by W&B and returns
// the ObjectStoreConnection with every selector required, since managed
// SeaweedFS always persists the full key set.
>>>>>>> main
func writeWandbConnInfo(
	ctx context.Context,
	cl client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
<<<<<<< HEAD
	connInfo *s3ConnInfo,
=======
	connInfo *objectstore.ConnInfo,
>>>>>>> main
) (
	*apiv2.ObjectStoreConnection, error,
) {
	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()
<<<<<<< HEAD
	urlKey := "url"
=======
>>>>>>> main

	if found, err = common.GetResource(
		ctx, cl, nsName, AppConnTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	if gvk, err = cl.GroupVersionKindFor(owner); err != nil {
		return nil, fmt.Errorf("could not get GVK for owner: %w", err)
	}
	ref := metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         ptr.To(false),
		BlockOwnerDeletion: ptr.To(false),
	}

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            nsName.Name,
			Namespace:       nsName.Namespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
<<<<<<< HEAD
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			urlKey:           fmt.Sprintf("%s?tls=%t", connInfo.toUrl().String(), connInfo.TLS),
			"Host":           connInfo.Host,
			"Port":           connInfo.Port,
			"AccessKey":      connInfo.AccessKey,
			"SecretKey":      connInfo.SecretKey,
			"Region":         "us-east-1",
			"Bucket":         connInfo.Bucket,
			"Scheme":         connInfo.scheme(),
			"TlsEnabled":     strconv.FormatBool(connInfo.TLS),
			"Provider":       "s3",
			"ForcePathStyle": strconv.FormatBool(connInfo.ForcePathStyle),
		},
=======
		Type:       corev1.SecretTypeOpaque,
		StringData: connInfo.ToSecretData(),
>>>>>>> main
	}

	if _, err = common.CrudResource(ctx, cl, desired, actual); err != nil {
		return nil, err
	}

<<<<<<< HEAD
	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return &apiv2.ObjectStoreConnection{
		URL:            corev1.SecretKeySelector{LocalObjectReference: localRef, Key: urlKey, Optional: ptr.To(false)},
		Endpoint:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:           corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		AccessKey:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "AccessKey", Optional: ptr.To(false)},
		SecretKey:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SecretKey", Optional: ptr.To(false)},
		Region:         corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Region", Optional: ptr.To(false)},
		Bucket:         corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Bucket", Optional: ptr.To(false)},
		TlsEnabled:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "TlsEnabled", Optional: ptr.To(false)},
		Provider:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Provider", Optional: ptr.To(false)},
		ForcePathStyle: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "ForcePathStyle", Optional: ptr.To(false)},
	}, nil
=======
	// Managed SeaweedFS always writes the full key set, so every selector is required.
	return connInfo.ToObjectStoreConnection(nsName.Name, true), nil
>>>>>>> main
}
