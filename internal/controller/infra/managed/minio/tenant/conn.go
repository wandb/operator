package tenant

import (
	"context"
	"fmt"
	"net/url"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MinioUrlScheme = "s3"
	MinioPort      = "80"
)

type minioConnInfo struct {
	RootUser     string
	RootPassword string
	Host         string
	Port         string
	Bucket       string
}

func buildMinioConnInfo(
	rootUser, rootPassword string, nsnBuilder *NsNameBuilder,
) *minioConnInfo {
	namespace := nsnBuilder.Namespace()
	serviceName := nsnBuilder.ServiceName()
	return &minioConnInfo{
		RootUser:     rootUser,
		RootPassword: rootPassword,
		Host:         fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		Port:         MinioPort,
		Bucket:       "bucket",
	}
}

func (m *minioConnInfo) toUrl() *url.URL {
	return &url.URL{
		Scheme: MinioUrlScheme,
		Host:   fmt.Sprintf("%s:%s", m.Host, m.Port),
		User:   url.UserPassword(m.RootUser, m.RootPassword),
		Path:   m.Bucket,
	}
}

func writeWandbConnInfo(
	ctx context.Context,
	cl client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *minioConnInfo,
) (
	*translator.ObjectStoreConnection, error,
) {
	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()
	urlKey := "url"

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
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			urlKey:      connInfo.toUrl().String(),
			"Host":      connInfo.Host,
			"Port":      connInfo.Port,
			"AccessKey": connInfo.RootUser,
			"SecretKey": connInfo.RootPassword,
			"Region":    "us-east-1",
			"Bucket":    connInfo.Bucket,
		},
	}

	if _, err = common.CrudResource(ctx, cl, desired, actual); err != nil {
		return nil, err
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return &translator.ObjectStoreConnection{
		URL:       corev1.SecretKeySelector{LocalObjectReference: localRef, Key: urlKey, Optional: ptr.To(false)},
		Endpoint:  corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		AccessKey: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "AccessKey", Optional: ptr.To(false)},
		SecretKey: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "SecretKey", Optional: ptr.To(false)},
		Region:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Region", Optional: ptr.To(false)},
		Bucket:    corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Bucket", Optional: ptr.To(false)},
	}, nil
}
