package reconciler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysqlconnection"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const mysqlBundlesChecksumAnnotation = "weightsandbiases.apps.wandb.com/mysql-bundles-checksum"

func applyMySQLBundlesToWorkload(
	ctx context.Context,
	c ctrlclient.Client,
	wandb *apiv2.WeightsAndBiases,
	instances map[string]struct{},
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
) ([]corev1.Volume, []corev1.VolumeMount, string, error) {
	if len(instances) == 0 {
		return volumes, volumeMounts, "", nil
	}

	keys := make([]string, 0, len(instances))
	for key := range instances {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hash := sha256.New()
	hasBundles := false
	for _, key := range keys {
		status, ok := wandb.Status.MySQLStatus[key]
		if !ok || status.Connection.URL.Name == "" {
			return nil, nil, "", fmt.Errorf("mysql instance %q has no connection bundle", key)
		}

		secret := &corev1.Secret{}
		nsName := types.NamespacedName{
			Namespace: wandb.Namespace,
			Name:      status.Connection.URL.Name,
		}
		if err := c.Get(ctx, nsName, secret); apierrors.IsNotFound(err) {
			// Status can briefly refer to a legacy connection Secret during an
			// operator upgrade. The provider reconcile replaces it before new
			// bundle mounts are required.
			continue
		} else if err != nil {
			return nil, nil, "", fmt.Errorf("read mysql bundle %s: %w", nsName, err)
		}
		if secret.Annotations[mysqlconnection.BundleVersionAnnotation] == "" {
			continue
		}
		hasBundles = true

		_, _ = fmt.Fprintf(hash, "instance:%s\nsecret:%s\n", key, secret.Name)
		dataKeys := make([]string, 0, len(secret.Data))
		for dataKey := range secret.Data {
			dataKeys = append(dataKeys, dataKey)
		}
		sort.Strings(dataKeys)
		for _, dataKey := range dataKeys {
			_, _ = fmt.Fprintf(hash, "%s=", dataKey)
			_, _ = hash.Write(secret.Data[dataKey])
			_, _ = hash.Write([]byte("\n"))
		}

		items := make([]corev1.KeyToPath, 0, 3)
		if len(secret.Data[mysqlconnection.SSLCAKey]) > 0 {
			items = append(items, corev1.KeyToPath{Key: mysqlconnection.SSLCAKey, Path: mysqlconnection.CACertFile})
		}
		if len(secret.Data[mysqlconnection.SSLCertKey]) > 0 {
			items = append(items,
				corev1.KeyToPath{Key: mysqlconnection.SSLCertKey, Path: mysqlconnection.ClientCertFile},
				corev1.KeyToPath{Key: mysqlconnection.SSLKeyKey, Path: mysqlconnection.ClientKeyFile},
			)
		}
		if len(items) == 0 {
			continue
		}

		volumeName := mysqlconnection.VolumeName(key)
		volumes = upsertVolume(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
					Items:      items,
				},
			},
		})
		volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mysqlconnection.MountPath(key),
			ReadOnly:  true,
		})
	}
	if !hasBundles {
		return volumes, volumeMounts, "", nil
	}

	return volumes, volumeMounts, hex.EncodeToString(hash.Sum(nil)), nil
}

func setMySQLBundlesChecksumAnnotation(podTemplate *corev1.PodTemplateSpec, checksum string) {
	annotations := podTemplate.GetAnnotations()
	if checksum == "" {
		if annotations == nil {
			return
		}
		delete(annotations, mysqlBundlesChecksumAnnotation)
		if len(annotations) == 0 {
			annotations = nil
		}
		podTemplate.SetAnnotations(annotations)
		return
	}
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[mysqlBundlesChecksumAnnotation] = checksum
	podTemplate.SetAnnotations(annotations)
}
