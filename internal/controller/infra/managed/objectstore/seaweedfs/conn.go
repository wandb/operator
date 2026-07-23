package seaweedfs

import (
	"context"
	"fmt"
	"net/url"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
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

func s3ServiceHost(specName, namespace string) string {
	return fmt.Sprintf("%s-s3.%s.svc.cluster.local", SeaweedName(specName), namespace)
}

// s3ExternalURL is the endpoint the W&B server signs S3 requests against
// (it presigns with this host and rewrites the URL for external clients
// without re-signing). The s3 gateway must verify signatures against this
// host rather than the Host/X-Forwarded-Host of proxied requests.
func s3ExternalURL(specName, namespace string, tls bool) string {
	return fmt.Sprintf("%s://%s:%s", objectstore.SchemeForTLS(tls), s3ServiceHost(specName, namespace), S3Port)
}

func writeWandbConnInfo(
	ctx context.Context,
	cl client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *objectstore.ConnInfo,
) (
	*apiv2.ObjectStoreConnection, error,
) {
	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()

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
		Type:       corev1.SecretTypeOpaque,
		StringData: connInfo.ToSecretData(),
	}

	if _, err = common.CrudResource(ctx, cl, desired, actual); err != nil {
		return nil, err
	}

	// Managed SeaweedFS always writes the full key set, so every selector is required.
	return connInfo.ToObjectStoreConnection(nsName.Name, true), nil
}
