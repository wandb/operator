package altinity

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clickhouseConnInfo struct {
	Host        string
	TCPPort     string
	HTTPPort    string
	User        string
	Password    string
	Database    string
	Tls         bool
	Replicated  bool
	ClusterName string
}

func (c *clickhouseConnInfo) toURL() string {
	values := url.Values{
		"tls": []string{strconv.FormatBool(c.Tls)},
	}
	clickhouseUrl := url.URL{
		Scheme:   "clickhouse",
		Host:     fmt.Sprintf("%s:%s", c.Host, c.TCPPort),
		User:     url.UserPassword(c.User, c.Password),
		Path:     c.Database,
		RawQuery: values.Encode(),
	}
	return clickhouseUrl.String()
}

func writeClickHouseConnInfo(
	ctx context.Context,
	client client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *clickhouseConnInfo,
) (
	*apiv2.ClickHouseConnection, error,
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
			urlKey:        connInfo.toURL(),
			"Host":        connInfo.Host,
			"TCPPort":     connInfo.TCPPort,
			"HTTPPort":    connInfo.HTTPPort,
			"User":        connInfo.User,
			"Password":    connInfo.Password,
			"Database":    connInfo.Database,
			"Replicated":  strconv.FormatBool(connInfo.Replicated),
			"ClusterName": connInfo.ClusterName,
		},
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return nil, err
	}

	return &apiv2.ClickHouseConnection{
		URL:      apiv2.ValueFromSecret(nsName.Name, urlKey, false),
		Host:     apiv2.ValueFromSecret(nsName.Name, "Host", false),
		HTTPPort: apiv2.ValueFromSecret(nsName.Name, "HTTPPort", false),
		TCPPort:  apiv2.ValueFromSecret(nsName.Name, "TCPPort", false),
		Username: apiv2.ValueFromSecret(nsName.Name, "User", false),
		Password: apiv2.ValueFromSecret(nsName.Name, "Password", false),
		Database: apiv2.ValueFromSecret(nsName.Name, "Database", false),
		// The operator provisions the cluster, so these keys always exist.
		Replicated:  apiv2.ValueFromSecret(nsName.Name, "Replicated", false),
		ClusterName: apiv2.ValueFromSecret(nsName.Name, "ClusterName", false),
	}, nil
}
