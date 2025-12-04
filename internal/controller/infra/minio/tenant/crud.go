package tenant

import (
	"context"
	"fmt"

	"github.com/Masterminds/goutils"
	"github.com/wandb/operator/internal/controller/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceTypeName = "Tenant"
	ConfigTypeName   = "MinioConfig"
)

func CrudResourceAndConfig(
	ctx context.Context,
	client client.Client,
	specNamespacedName types.NamespacedName,
	desiredCr *miniov2.Tenant,
	envConfig MinioEnvConfig,
) error {
	var err error
	var actual = &miniov2.Tenant{}

	if err = common.GetResource(
		ctx, client, TenantNamespacedName(specNamespacedName), ResourceTypeName, actual,
	); err != nil {
		return err
	}
	if err = common.CrudResource(ctx, client, desiredCr, actual); err != nil {
		return err
	}

	configNamespacedName := ConfigNamespacedName(specNamespacedName)
	if err = crudMinioConfig(ctx, client, desiredCr, client.Scheme(), configNamespacedName, envConfig); err != nil {
		return err
	}

	return nil
}

// CrudMinioConfig builds the Minio config, that includes the root password. To keep the
// password generated only if not present, we have to read the actual config
func crudMinioConfig(
	ctx context.Context,
	client client.Client,
	owner *miniov2.Tenant,
	scheme *runtime.Scheme,
	configNamespacedName types.NamespacedName,
	envConfig MinioEnvConfig,
) error {
	var err error
	var rootPassword string
	var actual = &corev1.Secret{}

	log := ctrl.LoggerFrom(ctx)

	if err = common.GetResource(
		ctx, client, configNamespacedName, ConfigTypeName, actual,
	); err != nil {
		return err
	}

	if actual != nil {
		rootPassword = actual.StringData[TenantMinioRootPasswordKey]
	}
	if rootPassword == "" {
		if rootPassword, err = goutils.RandomAlphabetic(20); err != nil {
			return err
		}
	}
	rootUser := envConfig.RootUser
	minioBrowser := envConfig.MinioBrowserSetting

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configNamespacedName.Name,
			Namespace: configNamespacedName.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"config.env": fmt.Sprintf(`export MINIO_ROOT_USER="%s"
export MINIO_ROOT_PASSWORD="%s"
export MINIO_BROWSER="%s"`, rootUser, rootPassword, minioBrowser),
			TenantMinioRootUserKey:     rootUser,
			TenantMinioRootPasswordKey: rootPassword,
			TenantMinioBrowserKey:      minioBrowser,
		},
	}

	// Set owner reference
	if err := ctrl.SetControllerReference(owner, desired, scheme); err != nil {
		log.Error(err, "failed to set owner reference on Minio config secret")
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	if err = common.CrudResource(ctx, client, desired, actual); err != nil {
		return err
	}

	return nil
}
