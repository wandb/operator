package opstree

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	redisv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redis/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/pkg/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readStandaloneConnectionDetails(standaloneActual *redisv1beta2.Redis) *redisConnInfo {
	redisHost := fmt.Sprintf("%s.%s.svc.cluster.local", standaloneActual.Name, standaloneActual.GetNamespace())
	redisPort := "6379"

	return &redisConnInfo{
		Host: redisHost,
		Port: redisPort,
	}
}

func readSentinelConnectionDetails(sentinelActual *redissentinelv1beta2.RedisSentinel) *redisConnInfo {
	sentinelHost := fmt.Sprintf("%s-sentinel.%s.svc.cluster.local", sentinelActual.Name, sentinelActual.GetNamespace())
	sentinelPort := "26379"
	masterName := "gorilla"

	return &redisConnInfo{
		SentinelHost:   sentinelHost,
		SentinelPort:   sentinelPort,
		SentinelMaster: masterName,
	}
}

func writeRedisConnInfo(
	ctx context.Context,
	client client.Client,
	owner client.Object,
	nsnBuilder *NsNameBuilder,
	connInfo *redisConnInfo,
) (
	*apiv2.RedisConnection, error,
) {
	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var actual = &corev1.Secret{}

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
			urlKey: connInfo.toURL(),
			"Host": connInfo.Host,
			"Port": connInfo.Port,
		},
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return nil, err
	}

	localRef := corev1.LocalObjectReference{Name: nsName.Name}
	return &apiv2.RedisConnection{
		URL:  corev1.SecretKeySelector{LocalObjectReference: localRef, Key: urlKey, Optional: ptr.To(false)},
		Host: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Host", Optional: ptr.To(false)},
		Port: corev1.SecretKeySelector{LocalObjectReference: localRef, Key: "Port", Optional: ptr.To(false)},
	}, nil
}
