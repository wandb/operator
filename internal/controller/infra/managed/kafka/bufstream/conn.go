package bufstream

import (
	"context"
	"fmt"
	"strconv"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type kafkaConnInfo struct {
	Host string
	Port string
}

func (c *kafkaConnInfo) toURL() string {
	return fmt.Sprintf("kafka://%s:%s", c.Host, c.Port)
}

func readConnectionDetails(nsnBuilder *NsNameBuilder) *kafkaConnInfo {
	return &kafkaConnInfo{
		Host: nsnBuilder.BufstreamHost(),
		Port: strconv.Itoa(KafkaListenerPort),
	}
}

func writeKafkaConnInfo(
	ctx context.Context,
	cl client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *kafkaConnInfo,
) (*apiv2.KafkaConnection, error) {
	log := logx.GetSlog(ctx)

	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

	nsName := nsnBuilder.ConnectionNsName()
	urlKey := "url"

	if found, err = common.GetResource(ctx, cl, nsName, AppConnTypeName, actual); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	var ownerRefs []metav1.OwnerReference
	if owner.GetNamespace() == nsName.Namespace {
		if gvk, err = cl.GroupVersionKindFor(owner); err != nil {
			log.Error(fmt.Sprintf("Error getting GVK for %s", owner.GetName()), logx.ErrAttr(err))
			return nil, fmt.Errorf("could not get GVK for owner: %w", err)
		}
		ownerRefs = []metav1.OwnerReference{{
			APIVersion:         gvk.GroupVersion().String(),
			Kind:               gvk.Kind,
			Name:               owner.GetName(),
			UID:                owner.GetUID(),
			Controller:         ptr.To(false),
			BlockOwnerDeletion: ptr.To(false),
		}}
	}

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            nsName.Name,
			Namespace:       nsName.Namespace,
			OwnerReferences: ownerRefs,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			urlKey: connInfo.toURL(),
			"Host": connInfo.Host,
			"Port": connInfo.Port,
		},
	}

	if _, err = common.CrudResource(ctx, cl, desired, actual); err != nil {
		return nil, err
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return &apiv2.KafkaConnection{
		URL:            corev1.SecretKeySelector{LocalObjectReference: localRef, Key: urlKey, Optional: ptr.To(false)},
		Host:           corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port:           corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
		BrokerEndpoint: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
	}, nil
}
