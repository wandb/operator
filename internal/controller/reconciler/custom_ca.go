package reconciler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	customCACertsChecksumAnnotation = "weightsandbiases.apps.wandb.com/ca-certs-checksum"

	customCACertsRootVolumeName      = "wandb-ca-certs-root"
	customCACertsInlineVolumeName    = "wandb-ca-certs"
	customCACertsConfigMapVolumeName = "wandb-ca-certs-user"

	customCACertsRootMountPath      = "/usr/local/share/ca-certificates/"
	customCACertsInlineMountPath    = "/usr/local/share/ca-certificates/inline"
	customCACertsConfigMapMountPath = "/usr/local/share/ca-certificates/configmap"

	mysqlCACertVolumeName = "mysql-ca"
	mysqlCACertPath       = "/etc/ssl/certs/mysql_ca.pem"
	mysqlCACertFileName   = "mysql_ca.pem"

	mysqlSSLCertVolumeName = "mysql-ssl-cert"
	mysqlSSLCertPath       = "/etc/ssl/certs/mysql_ssl_cert.pem"
	mysqlSSLCertFileName   = "mysql_ssl_cert.pem"

	mysqlSSLKeyVolumeName = "mysql-ssl-key"
	mysqlSSLKeyPath       = "/etc/ssl/certs/mysql_ssl_key.pem"
	mysqlSSLKeyFileName   = "mysql_ssl_key.pem"

	redisCACertVolumeName = "redis-ca"
	redisCACertPath       = "/etc/ssl/certs/redis_ca.pem"
	redisCACertFileName   = "redis_ca.pem"
)

var customCACertsEnvVars = []corev1.EnvVar{
	{Name: "SSL_CERT_FILE", Value: "/etc/ssl/certs/ca-certificates.crt"},
	{Name: "SSL_CERT_DIR", Value: "/etc/ssl/certs"},
	{Name: "REQUESTS_CA_BUNDLE", Value: "/etc/ssl/certs/ca-certificates.crt"},
}

func customCACertsConfigMapName(wandb *apiv2.WeightsAndBiases) string {
	return fmt.Sprintf("%s-ca-certs", wandb.Name)
}

func hasGlobalCustomCACertConfig(wandb *apiv2.WeightsAndBiases) bool {
	return len(wandb.Spec.Global.CustomCACerts) > 0 || wandb.Spec.Global.CACertsConfigMap != ""
}

// defaultMySQLConnection returns the default MySQL instance's connection. The
// app's TLS env vars (MYSQL_CA_CERT_PATH etc.) are singular, so only the
// default instance's certificate material is mounted.
func defaultMySQLConnection(wandb *apiv2.WeightsAndBiases) apiv2.MysqlConnection {
	status, _ := apiv2.ResolveInstance(wandb.Status.MySQLStatus, "")
	return status.Connection
}

// defaultRedisConnection returns the default Redis instance's connection; see
// defaultMySQLConnection.
func defaultRedisConnection(wandb *apiv2.WeightsAndBiases) apiv2.RedisConnection {
	status, _ := apiv2.ResolveInstance(wandb.Status.RedisStatus, "")
	return status.Connection
}

func secretSelectorConfigured(sel corev1.SecretKeySelector) bool {
	return sel.Name != "" && sel.Key != ""
}

func hasOwnerReference(obj ctrlClient.Object, owner ctrlClient.Object) bool {
	ownerUID := owner.GetUID()
	for _, ref := range obj.GetOwnerReferences() {
		if ownerUID != "" && ref.UID == ownerUID {
			return true
		}
	}
	return false
}

func reconcileCustomCACerts(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	nsName := types.NamespacedName{Name: customCACertsConfigMapName(wandb), Namespace: wandb.Namespace}
	actual := &corev1.ConfigMap{}
	err := c.Get(ctx, nsName, actual)
	if err != nil && !apiErrors.IsNotFound(err) {
		return err
	}

	if len(wandb.Spec.Global.CustomCACerts) == 0 {
		if apiErrors.IsNotFound(err) {
			return nil
		}
		if !hasOwnerReference(actual, wandb) {
			return nil
		}
		return c.Delete(ctx, actual)
	}

	data := make(map[string]string, len(wandb.Spec.Global.CustomCACerts))
	for i, pem := range wandb.Spec.Global.CustomCACerts {
		data[fmt.Sprintf("customCA%d.crt", i)] = pem
	}

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsName.Name,
			Namespace: nsName.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wandb-operator",
				"app.kubernetes.io/instance":   wandb.Name,
				"app.kubernetes.io/part-of":    "wandb",
			},
		},
		Data: data,
	}
	if err := controllerutil.SetOwnerReference(wandb, desired, c.Scheme()); err != nil {
		return err
	}

	if apiErrors.IsNotFound(err) {
		return c.Create(ctx, desired)
	}
	desired.ResourceVersion = actual.ResourceVersion
	return c.Update(ctx, desired)
}

func applyCustomCACertsToWorkload(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	envVars []corev1.EnvVar,
	volumes []corev1.Volume,
	volumeMounts []corev1.VolumeMount,
) ([]corev1.EnvVar, []corev1.Volume, []corev1.VolumeMount, string, error) {
	mysqlConn := defaultMySQLConnection(wandb)
	redisConn := defaultRedisConnection(wandb)

	if hasGlobalCustomCACertConfig(wandb) {
		envVars = appendMissingEnvVars(envVars, customCACertsEnvVars)
		volumes = upsertVolume(volumes, corev1.Volume{
			Name: customCACertsRootVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
			Name:      customCACertsRootVolumeName,
			MountPath: customCACertsRootMountPath,
			ReadOnly:  false,
		})

		if len(wandb.Spec.Global.CustomCACerts) > 0 {
			volumes = upsertVolume(volumes, corev1.Volume{
				Name: customCACertsInlineVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: customCACertsConfigMapName(wandb)},
					},
				},
			})
			volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
				Name:      customCACertsInlineVolumeName,
				MountPath: customCACertsInlineMountPath,
				ReadOnly:  true,
			})
		}

		if wandb.Spec.Global.CACertsConfigMap != "" {
			volumes = upsertVolume(volumes, corev1.Volume{
				Name: customCACertsConfigMapVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: wandb.Spec.Global.CACertsConfigMap},
						Optional:             boolPtr(true),
					},
				},
			})
			volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
				Name:      customCACertsConfigMapVolumeName,
				MountPath: customCACertsConfigMapMountPath,
				ReadOnly:  true,
			})
		}
	}

	if hasValue, err := secretSelectorHasValue(ctx, c, wandb.Namespace, mysqlConn.SslCa); err != nil {
		return nil, nil, nil, "", err
	} else if hasValue {
		envVars = appendMissingEnvVars(envVars, []corev1.EnvVar{{Name: "MYSQL_CA_CERT_PATH", Value: mysqlCACertPath}})
		volumes = upsertVolume(volumes, corev1.Volume{
			Name: mysqlCACertVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: secretCACertVolumeSource(mysqlConn.SslCa, mysqlCACertFileName),
			},
		})
		volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
			Name:      mysqlCACertVolumeName,
			MountPath: mysqlCACertPath,
			SubPath:   mysqlCACertFileName,
			ReadOnly:  true,
		})
	}
	if hasValue, err := secretSelectorHasValue(ctx, c, wandb.Namespace, mysqlConn.SslCert); err != nil {
		return nil, nil, nil, "", err
	} else if hasValue {
		volumes = upsertVolume(volumes, corev1.Volume{
			Name: mysqlSSLCertVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: secretCACertVolumeSource(mysqlConn.SslCert, mysqlSSLCertFileName),
			},
		})
		volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
			Name:      mysqlSSLCertVolumeName,
			MountPath: mysqlSSLCertPath,
			SubPath:   mysqlSSLCertFileName,
			ReadOnly:  true,
		})
	}
	if hasValue, err := secretSelectorHasValue(ctx, c, wandb.Namespace, mysqlConn.SslKey); err != nil {
		return nil, nil, nil, "", err
	} else if hasValue {
		volumes = upsertVolume(volumes, corev1.Volume{
			Name: mysqlSSLKeyVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: secretCACertVolumeSource(mysqlConn.SslKey, mysqlSSLKeyFileName),
			},
		})
		volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
			Name:      mysqlSSLKeyVolumeName,
			MountPath: mysqlSSLKeyPath,
			SubPath:   mysqlSSLKeyFileName,
			ReadOnly:  true,
		})
	}

	if hasValue, err := secretSelectorHasValue(ctx, c, wandb.Namespace, redisConn.SslCa); err != nil {
		return nil, nil, nil, "", err
	} else if hasValue {
		volumes = upsertVolume(volumes, corev1.Volume{
			Name: redisCACertVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: secretCACertVolumeSource(redisConn.SslCa, redisCACertFileName),
			},
		})
		volumeMounts = upsertVolumeMount(volumeMounts, corev1.VolumeMount{
			Name:      redisCACertVolumeName,
			MountPath: redisCACertPath,
			SubPath:   redisCACertFileName,
			ReadOnly:  true,
		})
	}

	checksum, err := customCACertsChecksum(ctx, c, wandb)
	if err != nil {
		return nil, nil, nil, "", err
	}
	return envVars, volumes, volumeMounts, checksum, nil
}

func setCustomCACertsChecksumAnnotation(podTemplate *corev1.PodTemplateSpec, checksum string) {
	annotations := podTemplate.GetAnnotations()
	if checksum == "" {
		if annotations == nil {
			return
		}
		delete(annotations, customCACertsChecksumAnnotation)
		if len(annotations) == 0 {
			annotations = nil
		}
		podTemplate.SetAnnotations(annotations)
		return
	}
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[customCACertsChecksumAnnotation] = checksum
	podTemplate.SetAnnotations(annotations)
}

func secretCACertVolumeSource(sel corev1.SecretKeySelector, fileName string) *corev1.SecretVolumeSource {
	return &corev1.SecretVolumeSource{
		SecretName: sel.Name,
		Items: []corev1.KeyToPath{{
			Key:  sel.Key,
			Path: fileName,
		}},
		Optional: sel.Optional,
	}
}

func upsertVolume(volumes []corev1.Volume, volume corev1.Volume) []corev1.Volume {
	for i := range volumes {
		if volumes[i].Name == volume.Name {
			volumes[i] = volume
			return volumes
		}
	}
	return append(volumes, volume)
}

func upsertVolumeMount(volumeMounts []corev1.VolumeMount, mount corev1.VolumeMount) []corev1.VolumeMount {
	for i := range volumeMounts {
		if volumeMounts[i].Name == mount.Name {
			volumeMounts[i] = mount
			return volumeMounts
		}
	}
	return append(volumeMounts, mount)
}

func secretSelectorHasValue(ctx context.Context, c ctrlClient.Client, namespace string, sel corev1.SecretKeySelector) (bool, error) {
	if !secretSelectorConfigured(sel) {
		return false, nil
	}

	secret := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: sel.Name, Namespace: namespace}, secret)
	if apiErrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if _, ok := secret.Data[sel.Key]; ok {
		return true, nil
	}
	if _, ok := secret.StringData[sel.Key]; ok {
		return true, nil
	}
	return false, nil
}

func customCACertsChecksum(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) (string, error) {
	mysqlConn := defaultMySQLConnection(wandb)
	redisConn := defaultRedisConnection(wandb)

	hasMySQLCA, err := secretSelectorHasValue(ctx, c, wandb.Namespace, mysqlConn.SslCa)
	if err != nil {
		return "", err
	}
	hasMySQLCert, err := secretSelectorHasValue(ctx, c, wandb.Namespace, mysqlConn.SslCert)
	if err != nil {
		return "", err
	}
	hasMySQLKey, err := secretSelectorHasValue(ctx, c, wandb.Namespace, mysqlConn.SslKey)
	if err != nil {
		return "", err
	}
	hasRedisCA, err := secretSelectorHasValue(ctx, c, wandb.Namespace, redisConn.SslCa)
	if err != nil {
		return "", err
	}

	if !hasGlobalCustomCACertConfig(wandb) && !hasMySQLCA && !hasMySQLCert && !hasMySQLKey && !hasRedisCA {
		return "", nil
	}

	hash := sha256.New()
	for i, pem := range wandb.Spec.Global.CustomCACerts {
		_, _ = fmt.Fprintf(hash, "inline:%d:%s\n", i, pem)
	}

	if wandb.Spec.Global.CACertsConfigMap != "" {
		_, _ = fmt.Fprintf(hash, "configmap:%s\n", wandb.Spec.Global.CACertsConfigMap)
		if err := hashConfigMapData(ctx, c, wandb.Namespace, wandb.Spec.Global.CACertsConfigMap, hashWriteString(hash)); err != nil {
			return "", err
		}
	}

	if sel := mysqlConn.SslCa; hasMySQLCA {
		_, _ = fmt.Fprintf(hash, "mysql:%s/%s\n", sel.Name, sel.Key)
		if err := hashSecretKeyData(ctx, c, wandb.Namespace, sel, hashWriteString(hash)); err != nil {
			return "", err
		}
	}
	if sel := mysqlConn.SslCert; hasMySQLCert {
		_, _ = fmt.Fprintf(hash, "mysql-cert:%s/%s\n", sel.Name, sel.Key)
		if err := hashSecretKeyData(ctx, c, wandb.Namespace, sel, hashWriteString(hash)); err != nil {
			return "", err
		}
	}
	if sel := mysqlConn.SslKey; hasMySQLKey {
		_, _ = fmt.Fprintf(hash, "mysql-key:%s/%s\n", sel.Name, sel.Key)
		if err := hashSecretKeyData(ctx, c, wandb.Namespace, sel, hashWriteString(hash)); err != nil {
			return "", err
		}
	}

	if sel := redisConn.SslCa; hasRedisCA {
		_, _ = fmt.Fprintf(hash, "redis:%s/%s\n", sel.Name, sel.Key)
		if err := hashSecretKeyData(ctx, c, wandb.Namespace, sel, hashWriteString(hash)); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

type writeStringFunc func(string)

func hashWriteString(hash interface{ Write([]byte) (int, error) }) writeStringFunc {
	return func(s string) {
		_, _ = hash.Write([]byte(s))
	}
}

func hashConfigMapData(ctx context.Context, c ctrlClient.Client, namespace, name string, write writeStringFunc) error {
	configMap := &corev1.ConfigMap{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, configMap)
	if apiErrors.IsNotFound(err) {
		write("missing-configmap\n")
		return nil
	}
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(configMap.Data)+len(configMap.BinaryData))
	for k := range configMap.Data {
		keys = append(keys, "data:"+k)
	}
	for k := range configMap.BinaryData {
		keys = append(keys, "binary:"+k)
	}
	sort.Strings(keys)
	for _, typedKey := range keys {
		write(typedKey)
		write("=")
		switch {
		case len(typedKey) > len("data:") && typedKey[:len("data:")] == "data:":
			write(configMap.Data[typedKey[len("data:"):]])
		case len(typedKey) > len("binary:") && typedKey[:len("binary:")] == "binary:":
			write(string(configMap.BinaryData[typedKey[len("binary:"):]]))
		}
		write("\n")
	}
	return nil
}

func hashSecretKeyData(ctx context.Context, c ctrlClient.Client, namespace string, sel corev1.SecretKeySelector, write writeStringFunc) error {
	secret := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: sel.Name, Namespace: namespace}, secret)
	if apiErrors.IsNotFound(err) {
		write("missing-secret\n")
		return nil
	}
	if err != nil {
		return err
	}

	if data, ok := secret.Data[sel.Key]; ok {
		write(string(data))
		write("\n")
		return nil
	}
	if stringData, ok := secret.StringData[sel.Key]; ok {
		write(stringData)
		write("\n")
		return nil
	}
	write("missing-key\n")
	return nil
}

func boolPtr(v bool) *bool {
	return &v
}
