package opstree

import (
	"context"
	"fmt"

	"github.com/wandb/operator/internal/controller/common"
	transcommon "github.com/wandb/operator/internal/controller/translator/common"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	StandaloneType  = "RedisStandalone"
	SentinelType    = "RedisSentinel"
	ReplicationType = "RedisReplication"
	AppConnTypeName = "RedisAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	standaloneDesired *redisv1beta2.Redis,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) error {
	var err error

	nsNameBldr := createNsNameBuilder(specNamespacedName)

	if err = writeStandaloneState(ctx, client, nsNameBldr, standaloneDesired); err != nil {
		return err
	}

	if err = writeSentinelState(ctx, client, nsNameBldr, sentinelDesired); err != nil {
		return err
	}

	if err = writeReplicationState(ctx, client, nsNameBldr, replicationDesired); err != nil {
		return err
	}

	return nil
}

func writeStandaloneState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	standaloneDesired *redisv1beta2.Redis,
) error {
	var standaloneActual = &redisv1beta2.Redis{}
	var err error

	if err = common.GetResource(
		ctx, client, nsNameBldr.StandaloneNsName(), StandaloneType, standaloneActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, standaloneDesired, standaloneActual)
}

func writeSentinelState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	sentinelDesired *redissentinelv1beta2.RedisSentinel,
) error {
	var sentinelActual = &redissentinelv1beta2.RedisSentinel{}
	var err error

	if err = common.GetResource(
		ctx, client, nsNameBldr.SentinelNsName(), SentinelType, sentinelActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, sentinelDesired, sentinelActual)
}

func writeReplicationState(
	ctx context.Context,
	client client.Client,
	nsNameBldr *NsNameBuilder,
	replicationDesired *redisreplicationv1beta2.RedisReplication,
) error {
	var replicationActual = &redisreplicationv1beta2.RedisReplication{}
	var err error

	if err = common.GetResource(
		ctx, client, nsNameBldr.ReplicationNsName(), ReplicationType, replicationActual,
	); err != nil {
		return err
	}

	return common.CrudResource(ctx, client, replicationDesired, replicationActual)
}

type redisConnInfo struct {
	Host           string
	Port           string
	SentinelHost   string
	SentinelPort   string
	SentinelMaster string
}

func (c *redisConnInfo) toURL() string {
	if c.SentinelHost != "" {
		return fmt.Sprintf("redis://%s:%s?master=%s", c.SentinelHost, c.SentinelPort, c.SentinelMaster)
	}
	return fmt.Sprintf("redis://%s:%s", c.Host, c.Port)
}

func writeRedisConnInfo(
	ctx context.Context,
	client client.Client,
	owner client.Object,
	nsNameBldr *NsNameBuilder,
	connInfo *redisConnInfo,
) (
	*transcommon.RedisConnection, error,
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

	return &transcommon.RedisConnection{
		URL: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: nsName.Name,
			},
			Key:      urlKey,
			Optional: ptr.To(false),
		},
	}, nil
}
