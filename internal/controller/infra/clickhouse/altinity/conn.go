package altinity

import (
	"context"
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
	Host string
	Port string
	User string
}

func (c *clickhouseConnInfo) toURL() string {
	return fmt.Sprintf("clickhouse://%s@%s:%s", c.User, c.Host, c.Port)
}

func writeClickHouseConnInfo(
	ctx context.Context,
	client client.Client,
	owner client.Object,
	nsNameBldr *NsNameBuilder,
	connInfo *clickhouseConnInfo,
) (
	*translator.InfraConnection, error,
) {
	var err error
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

	nsName := nsNameBldr.ConnectionNsName()
	urlKey := "url"

	if err = common.GetResource(
		ctx, client, nsName, AppConnTypeName, actual,
	); err != nil {
		return nil, err
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
			urlKey: connInfo.toURL(),
		},
	}

	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return nil, err
	}

	return &translator.InfraConnection{
		URL: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: nsName.Name,
			},
			Key:      urlKey,
			Optional: ptr.To(false),
		},
	}, nil
}
