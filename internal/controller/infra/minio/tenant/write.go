package tenant

import (
	"context"
	"fmt"

	"github.com/Masterminds/goutils"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "Tenant"
	ConfigTypeName   = "MinioConfig"
	AppConnTypeName  = "MinioAppConn"
)

func WriteState(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredCr *miniov2.Tenant,
	envConfig MinioEnvConfig,
	wandbOwner client.Object,
) (*translator.InfraConnection, error) {
	var err error
	var found bool
	var actual = &miniov2.Tenant{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.SpecNsName(), ResourceTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	if _, err = common.CrudResource(ctx, client, desiredCr, actual); err != nil {
		return nil, err
	}

	var connInfo *minioConnInfo
	if connInfo, err = writeMinioConfig(
		ctx, client, desiredCr, nsnBuilder, envConfig,
	); err != nil {
		return nil, err
	}

	if connInfo != nil {
		var connection *translator.InfraConnection
		if connection, err = writeWandbConnInfo(
			ctx, client, wandbOwner, nsnBuilder, connInfo,
		); err != nil {
			return nil, err
		}
		return connection, nil
	}

	return nil, nil
}

// writeMinioConfig builds the Minio Config with credentials.
// This generates a password if one does not exist.
// Note: the owner of the minio-config is the Minio CR
func writeMinioConfig(
	ctx context.Context,
	client client.Client,
	owner *miniov2.Tenant,
	nsnBuilder *NsNameBuilder,
	envConfig MinioEnvConfig,
) (*minioConnInfo, error) {
	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var configFile minioConfigFile
	var rootPassword string
	var actual = &corev1.Secret{}

	configFileName := "config.env"

	//log := ctrl.LoggerFrom(ctx)

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.ConfigNsName(), ConfigTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	if actual != nil {
		rootPassword = parseMinioConfigFile(string(actual.Data[configFileName])).rootPassword
	}
	if rootPassword == "" {
		if rootPassword, err = goutils.RandomAlphabetic(20); err != nil {
			return nil, err
		}
	}
	configFile = buildMinioConfigFile(envConfig.RootUser, rootPassword, envConfig.MinioBrowserSetting)

	// Compute owner reference
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
			Name:            nsnBuilder.ConfigName(),
			Namespace:       nsnBuilder.Namespace(),
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			configFileName: configFile.toFileContents(),
		},
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return nil, err
	}

	return buildMinioConnInfo(configFile.rootUser, configFile.rootPassword, nsnBuilder), nil
}
