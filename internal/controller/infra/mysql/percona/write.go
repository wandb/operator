package percona

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	transcommon "github.com/wandb/operator/internal/controller/translator/common"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "PerconaXtraDBCluster"
	AppConnTypeName  = "MySQLAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desired *pxcv1.PerconaXtraDBCluster,
) error {
	var err error
	var actual = &pxcv1.PerconaXtraDBCluster{}

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = common.GetResource(
		ctx, client, nsNameBldr.ClusterNsName(), ResourceTypeName, actual,
	); err != nil {
		return err
	}

	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}

type mysqlConnInfo struct {
	Host string
	Port string
	User string
}

func (c *mysqlConnInfo) toURL() string {
	return fmt.Sprintf("mysql://%s@%s:%s", c.User, c.Host, c.Port)
}

func writeMySQLConnInfo(
	ctx context.Context,
	client client.Client,
	owner client.Object,
	nsNameBldr *NsNameBuilder,
	connInfo *mysqlConnInfo,
) (
	*transcommon.MySQLConnection, error,
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

	return &transcommon.MySQLConnection{
		URL: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: nsName.Name,
			},
			Key:      urlKey,
			Optional: ptr.To(false),
		},
	}, nil
}
