package tenant

import (
	"context"
	"fmt"

	"github.com/Masterminds/goutils"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/translator"
	"github.com/wandb/operator/internal/logx"
	miniov2 "github.com/wandb/operator/pkg/vendored/minio-operator/minio.min.io/v2"
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
) ([]metav1.Condition, *translator.InfraConnection) {
	ctx, _ = logx.IntoContext(ctx, logx.Minio)
	var actual = &miniov2.Tenant{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := common.GetResource(
		ctx, client, nsnBuilder.SpecNsName(), ResourceTypeName, actual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   MinioCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		actual = nil
	}

	result := make([]metav1.Condition, 0)

	action, err := common.CrudResource(ctx, client, desiredCr, actual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   MinioCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   MinioCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction:
		result = append(result, metav1.Condition{
			Type:   MinioCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   MinioCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	connInfo, err := writeMinioConfig(
		ctx, client, desiredCr, nsnBuilder, envConfig,
	)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   MinioConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
		return result, nil
	}

	if connInfo != nil {
		connection, err := writeWandbConnInfo(
			ctx, client, wandbOwner, nsnBuilder, connInfo,
		)
		if err != nil {
			result = append(result, metav1.Condition{
				Type:   MinioConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
			return result, nil
		}
		if connection != nil {
			result = append(result, metav1.Condition{
				Type:   MinioConnectionInfoType,
				Status: metav1.ConditionTrue,
				Reason: common.ResourceExistsReason,
			})
			return result, connection
		}
	}

	result = append(result, metav1.Condition{
		Type:   MinioConnectionInfoType,
		Status: metav1.ConditionFalse,
		Reason: common.NoResourceReason,
	})
	return result, nil
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
