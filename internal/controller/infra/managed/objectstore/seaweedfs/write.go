package seaweedfs

import (
	"context"
	"fmt"

	"github.com/Masterminds/goutils"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
	"github.com/wandb/operator/internal/logx"
	seaweedv1 "github.com/wandb/operator/pkg/vendored/seaweedfs-operator/seaweed.seaweedfs.com/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "Seaweed"
	ConfigTypeName   = "SeaweedS3Config"
	AppConnTypeName  = "SeaweedAppConn"
)

// WriteState reconciles the managed SeaweedFS CR, its S3 identity config, and
// the W&B connection secret, returning the reconcile conditions plus the
// resulting ObjectStoreConnection (nil until the connection is available).
func WriteState(
	ctx context.Context,
	kubeClient client.Client,
	specNamespacedName types.NamespacedName,
	desiredCr *seaweedv1.Seaweed,
	envConfig SeaweedS3Config,
	wandbOwner client.Object,
) ([]metav1.Condition, *apiv2.ObjectStoreConnection) {
	ctx, _ = logx.WithSlog(ctx, logx.ObjectStore)
	var actual = &seaweedv1.Seaweed{}

	nsnBuilder := createNsNameBuilder(specNamespacedName)

	found, err := common.GetResource(
		ctx, kubeClient, nsnBuilder.SpecNsName(), ResourceTypeName, actual,
	)
	if err != nil {
		return []metav1.Condition{
			{
				Type:   common.ReconciledType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			},
			{
				Type:   SeaweedCustomResourceType,
				Status: metav1.ConditionUnknown,
				Reason: common.ApiErrorReason,
			},
		}, nil
	}
	if !found {
		actual = nil
	}

	result := make([]metav1.Condition, 0)
	topologyConditions, err := reconcileTopology(ctx, kubeClient, desiredCr, actual, wandbOwner)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
		return result, nil
	}
	result = append(result, topologyConditions...)

	if err := preserveClaimTemplateStorage(ctx, kubeClient, desiredCr); err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
		return result, nil
	}

	action, err := common.CrudResource(ctx, kubeClient, desiredCr, actual)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   common.ReconciledType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
		return result, nil
	}

	switch action {
	case common.CreateAction:
		result = append(result, metav1.Condition{
			Type:   SeaweedCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingCreateReason,
		})
	case common.DeleteAction:
		result = append(result, metav1.Condition{
			Type:   SeaweedCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.PendingDeleteReason,
		})
	case common.UpdateAction, common.UnchangedAction:
		result = append(result, metav1.Condition{
			Type:   SeaweedCustomResourceType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
	case common.NoAction:
		result = append(result, metav1.Condition{
			Type:   SeaweedCustomResourceType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
	}

	connInfo, err := writeSeaweedS3Config(
		ctx, kubeClient, desiredCr, nsnBuilder, envConfig,
	)
	if err != nil {
		result = append(result, metav1.Condition{
			Type:   SeaweedConnectionInfoType,
			Status: metav1.ConditionFalse,
			Reason: common.ApiErrorReason,
		})
		return result, nil
	}

	if connInfo != nil {
		connection, err := writeWandbConnInfo(
			ctx, kubeClient, wandbOwner, nsnBuilder, connInfo,
		)
		if err != nil {
			result = append(result, metav1.Condition{
				Type:   SeaweedConnectionInfoType,
				Status: metav1.ConditionFalse,
				Reason: common.ApiErrorReason,
			})
			return result, nil
		}
		if connection != nil {
			result = append(result, metav1.Condition{
				Type:   SeaweedConnectionInfoType,
				Status: metav1.ConditionTrue,
				Reason: common.ResourceExistsReason,
			})
			return result, connection
		}
	}

	result = append(result, metav1.Condition{
		Type:   SeaweedConnectionInfoType,
		Status: metav1.ConditionFalse,
		Reason: common.NoResourceReason,
	})
	return result, nil
}

func preserveClaimTemplateStorage(
	ctx context.Context,
	kubeClient client.Client,
	desired *seaweedv1.Seaweed,
) error {
	if desired == nil {
		return nil
	}

	volumeStorage, found, err := statefulSetClaimTemplateStorage(
		ctx,
		kubeClient,
		types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name + "-volume"},
	)
	if err != nil {
		return err
	}
	if found && desired.Spec.Volume != nil {
		if desired.Spec.Volume.Requests == nil {
			desired.Spec.Volume.Requests = corev1.ResourceList{}
		}
		desired.Spec.Volume.Requests[corev1.ResourceStorage] = volumeStorage
	}

	filerStorage, found, err := statefulSetClaimTemplateStorage(
		ctx,
		kubeClient,
		types.NamespacedName{Namespace: desired.Namespace, Name: desired.Name + "-filer"},
	)
	if err != nil {
		return err
	}
	if found && desired.Spec.Filer != nil && desired.Spec.Filer.Persistence != nil {
		if desired.Spec.Filer.Persistence.Resources.Requests == nil {
			desired.Spec.Filer.Persistence.Resources.Requests = corev1.ResourceList{}
		}
		desired.Spec.Filer.Persistence.Resources.Requests[corev1.ResourceStorage] = filerStorage
	}

	return nil
}

func statefulSetClaimTemplateStorage(
	ctx context.Context,
	kubeClient client.Client,
	nsn types.NamespacedName,
) (resource.Quantity, bool, error) {
	statefulSet := &appsv1.StatefulSet{}
	if err := kubeClient.Get(ctx, nsn, statefulSet); err != nil {
		if apierrors.IsNotFound(err) {
			return resource.Quantity{}, false, nil
		}
		return resource.Quantity{}, false, fmt.Errorf("get StatefulSet %s/%s: %w", nsn.Namespace, nsn.Name, err)
	}
	for _, claimTemplate := range statefulSet.Spec.VolumeClaimTemplates {
		if storage, ok := claimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			return storage, true, nil
		}
	}
	return resource.Quantity{}, false, nil
}

// writeSeaweedS3Config persists the SeaweedFS S3 identity config secret,
// preserving the existing secret key when one is already present so credentials
// stay stable across reconciles, and returns the resolved ConnInfo.
func writeSeaweedS3Config(
	ctx context.Context,
	client client.Client,
	owner *seaweedv1.Seaweed,
	nsnBuilder *NsNameBuilder,
	envConfig SeaweedS3Config,
) (*objectstore.ConnInfo, error) {
	var err error
	var found bool
	var gvk schema.GroupVersionKind
	var secretKey string
	var actual = &corev1.Secret{}

	configFileName := "config.json"

	if owner == nil {
		return nil, nil
	}

	if found, err = common.GetResource(
		ctx, client, nsnBuilder.ConfigNsName(), ConfigTypeName, actual,
	); err != nil {
		return nil, err
	}
	if !found {
		actual = nil
	}

	if actual != nil {
		if raw, ok := actual.Data[configFileName]; ok {
			existingConfig, parseError := parseS3IdentityConfig(string(raw))
			if parseError != nil {
				return nil, fmt.Errorf("parse existing %s: %w", configFileName, parseError)
			}
			secretKey, err = extractSecretKey(existingConfig, envConfig.AccessKey)
			if err != nil {
				return nil, err
			}
		}
	}
	if secretKey == "" {
		if secretKey, err = goutils.RandomAlphabetic(20); err != nil {
			return nil, err
		}
	}

	identityConfig := buildS3IdentityConfig(envConfig.AccessKey, secretKey)
	configJSON, err := identityConfig.toJSON()
	if err != nil {
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
			Name:            nsnBuilder.ConfigName(),
			Namespace:       nsnBuilder.Namespace(),
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			configFileName: configJSON,
		},
	}

	if _, err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return nil, err
	}

	tls := owner.Spec.TLS != nil && owner.Spec.TLS.Enabled
	return buildS3ConnInfo(envConfig.AccessKey, secretKey, nsnBuilder, tls), nil
}
