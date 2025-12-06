package altinity

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	transcommon "github.com/wandb/operator/internal/controller/translator/common"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "ClickHouseInstallation"
	AppConnTypeName  = "ClickHouseAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *chiv1.ClickHouseInstallation,
) error {
	var err error
	var actual = &chiv1.ClickHouseInstallation{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = common.GetResource(
		ctx, client, nsNameBldr.InstallationNsName(), ResourceTypeName, actual,
	); err != nil {
		return err
	}

	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}

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
	*transcommon.ClickHouseConnection, error,
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

	return &transcommon.ClickHouseConnection{
		URL: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: nsName.Name,
			},
			Key:      urlKey,
			Optional: ptr.To(false),
		},
	}, nil
}
