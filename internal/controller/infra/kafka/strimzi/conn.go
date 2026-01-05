package strimzi

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kafkaConnInfo struct {
	Host      string
	Port      string
	Username  string
	Password  string
	ClusterId string
}

const (
	connInfoUrl       = "url"
	connInfoHost      = "Host"
	connInfoPort      = "Port"
	connInfoClusterId = "ClusterId"
)

func (c *kafkaConnInfo) toURL() string {
	return fmt.Sprintf("kafka://%s:%s", c.Host, c.Port)
}

// toKafkaConnInfo will translate a K8S secret and into a kafkaConnInfo, if
// the expected fields are present.
func toKafkaConnInfo(
	ctx context.Context, secret *corev1.Secret,
) *kafkaConnInfo {
	if secret == nil {
		return nil
	}
	result := &kafkaConnInfo{}
	var ok bool
	var value []byte
	if value, ok = secret.Data[connInfoHost]; !ok {
		return nil
	}
	result.Host = string(value)

	if value, ok = secret.Data[connInfoPort]; !ok {
		return nil
	}
	result.Port = string(value)

	if value, ok = secret.Data[connInfoClusterId]; !ok {
		return nil
	}
	result.ClusterId = string(value)

	return result
}

func readKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
) (*kafkaConnInfo, error) {
	var err error
	var found bool
	var actual = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()

	if found, err = common.GetResource(
		ctx, cl, nsName, AppConnTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("kafka connection secret %s not found", nsName)
	}

	return &kafkaConnInfo{
		Host:      string(actual.Data[connInfoHost]),
		Port:      string(actual.Data[connInfoPort]),
		ClusterId: string(actual.Data[connInfoClusterId]),
	}, nil
}

func writeKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *kafkaConnInfo,
) (
	*translator.InfraConnection, error,
) {
	log := ctrl.LoggerFrom(ctx)

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

	// prefer connInfo ClusterId but fall back to AppConn Secret's value
	wandbSecretClusterId := ""
	if actual != nil {
		wandbSecretClusterId = string(actual.Data[connInfoClusterId])
	}
	nextClusterId := utils.Coalesce(connInfo.ClusterId, wandbSecretClusterId)

	newConnInfo := &kafkaConnInfo{
		Host:      connInfo.Host,
		Port:      connInfo.Port,
		ClusterId: nextClusterId,
	}

	if gvk, err = cl.GroupVersionKindFor(owner); err != nil {
		log.Error(err, fmt.Sprintf("Error getting GVK for %s", owner.GetName()))
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

	desired := buildConnInfoSecret(nsName, newConnInfo, &ref)

	if _, err = common.CrudResource(ctx, cl, desired, actual); err != nil {
		return nil, err
	}

	return &translator.InfraConnection{
		URL: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: nsName.Name,
			},
			Key:      connInfoUrl,
			Optional: ptr.To(false),
		},
	}, nil
}

func deleteKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	nsnBuilder *NsNameBuilder,
) error {
	var secret = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()

	found, err := common.GetResource(
		ctx, cl, nsName, AppConnTypeName, secret,
	)
	if err != nil {
		return err
	}

	if found {
		err = cl.Delete(ctx, secret)
		if err != nil {
			return err
		}
	}

	return nil
}

func buildConnInfoSecret(nsName types.NamespacedName, connInfo *kafkaConnInfo, ref *metav1.OwnerReference) *corev1.Secret {
	objMeta := metav1.ObjectMeta{
		Name:      nsName.Name,
		Namespace: nsName.Namespace,
	}
	if ref != nil {
		objMeta.OwnerReferences = []metav1.OwnerReference{*ref}
	}
	return &corev1.Secret{
		ObjectMeta: objMeta,
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{
			connInfoUrl:       connInfo.toURL(),
			connInfoHost:      connInfo.Host,
			connInfoPort:      connInfo.Port,
			connInfoClusterId: connInfo.ClusterId,
		},
	}
}
