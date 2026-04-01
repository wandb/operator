package altinity

import (
	"context"
	"errors"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clickhouseConnInfo struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func (c *clickhouseConnInfo) toURL() string {
	return fmt.Sprintf("clickhouse://%s:%s@%s:%s/%s", c.User, c.Password, c.Host, c.Port, c.Database)
}

func writeClickHouseConnInfo(
	ctx context.Context,
	client client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *clickhouseConnInfo,
) (
	*translator.ClickHouseConnection, error,
) {
	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

	if connInfo == nil {
		return nil, errors.New("missing connection info")
	}

	nsName := nsnBuilder.ConnectionNsName()
	urlKey := "url"

	if found, err = common.GetResource(
		ctx, client, nsName, AppConnTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	if gvk, err = client.GroupVersionKindFor(owner); err != nil {
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
			urlKey:     connInfo.toURL(),
			"Host":     connInfo.Host,
			"Port":     connInfo.Port,
			"User":     connInfo.User,
			"Password": connInfo.Password,
			"Database": connInfo.Database,
		},
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return nil, err
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return &translator.ClickHouseConnection{
		URL:      corev1.SecretKeySelector{LocalObjectReference: localRef, Key: urlKey, Optional: ptr.To(false)},
		Host:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:     corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		Username: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "User", Optional: ptr.To(false)},
		Password: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Password", Optional: ptr.To(false)},
		Database: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Database", Optional: ptr.To(false)},
	}, nil
}
