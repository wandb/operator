package reconciler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func customCATestClient(t *testing.T, objects ...ctrlClient.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(objects) > 0 {
		builder.WithObjects(objects...)
	}
	return builder
}

func TestReconcileCustomCACertsCreatesInlineConfigMap(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
			UID:       "wandb-uid",
		},
		Spec: apiv2.WeightsAndBiasesSpec{
			Global: apiv2.GlobalSpec{
				CustomCACerts: []string{"---cert-one---", "---cert-two---"},
			},
		},
	}
	builder := customCATestClient(t, wandb)
	client := builder.Build()

	require.NoError(t, reconcileCustomCACerts(context.Background(), client, wandb))

	var cm corev1.ConfigMap
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "wandb-ca-certs", Namespace: "default"}, &cm))
	require.Equal(t, "---cert-one---", cm.Data["customCA0.crt"])
	require.Equal(t, "---cert-two---", cm.Data["customCA1.crt"])
	require.Len(t, cm.OwnerReferences, 1)
	require.Equal(t, "wandb", cm.OwnerReferences[0].Name)
}

func TestReconcileCustomCACertsDoesNotDeleteUnownedGeneratedConfigMap(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
			UID:       "wandb-uid",
		},
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-ca-certs", Namespace: "default"},
		Data:       map[string]string{"user.crt": "---user---"},
	}
	builder := customCATestClient(t, wandb, configMap)
	client := builder.Build()

	require.NoError(t, reconcileCustomCACerts(context.Background(), client, wandb))

	var cm corev1.ConfigMap
	require.NoError(t, client.Get(context.Background(), types.NamespacedName{Name: "wandb-ca-certs", Namespace: "default"}, &cm))
	require.Equal(t, "---user---", cm.Data["user.crt"])
}

func TestApplyCustomCACertsToWorkloadAddsGlobalAndInfraMounts(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps.wandb.com/v2", Kind: "WeightsAndBiases"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
		},
		Spec: apiv2.WeightsAndBiasesSpec{
			Global: apiv2.GlobalSpec{
				CustomCACerts:    []string{"---inline---"},
				CACertsConfigMap: "user-ca-certs",
			},
		},
		Status: apiv2.WeightsAndBiasesStatus{
			MySQLStatus: apiv2.MysqlInfraStatus{
				Connection: apiv2.MysqlConnection{
					SslCa: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-mysql-connection"},
						Key:                  "SslCa",
					},
					SslCert: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-mysql-connection"},
						Key:                  "SslCert",
					},
					SslKey: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-mysql-connection"},
						Key:                  "SslKey",
					},
				},
			},
			RedisStatus: apiv2.RedisInfraStatus{
				Connection: apiv2.RedisConnection{
					SslCa: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-redis-connection"},
						Key:                  "SslCa",
						Optional:             ptr.To(true),
					},
				},
			},
		},
	}
	mysqlSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-mysql-connection", Namespace: "default"},
		Data: map[string][]byte{
			"SslCa":   []byte("---mysql-ca---"),
			"SslCert": []byte("---mysql-cert---"),
			"SslKey":  []byte("---mysql-key---"),
		},
	}
	redisSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-redis-connection", Namespace: "default"},
		Data:       map[string][]byte{"SslCa": []byte("---redis-ca---")},
	}
	userCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "user-ca-certs", Namespace: "default"},
		Data:       map[string]string{"corp.crt": "---corp---"},
	}
	builder := customCATestClient(t, wandb, mysqlSecret, redisSecret, userCM)
	client := builder.Build()

	envs, volumes, mounts, checksum, err := applyCustomCACertsToWorkload(context.Background(), client, wandb, nil, nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, checksum)

	requireContainsEnv(t, envs, "SSL_CERT_FILE", "/etc/ssl/certs/ca-certificates.crt")
	requireContainsEnv(t, envs, "SSL_CERT_DIR", "/etc/ssl/certs")
	requireContainsEnv(t, envs, "REQUESTS_CA_BUNDLE", "/etc/ssl/certs/ca-certificates.crt")
	requireContainsEnv(t, envs, "MYSQL_CA_CERT_PATH", mysqlCACertPath)

	requireVolume(t, volumes, customCACertsRootVolumeName)
	requireVolume(t, volumes, customCACertsInlineVolumeName)
	requireVolume(t, volumes, customCACertsConfigMapVolumeName)
	requireVolume(t, volumes, mysqlCACertVolumeName)
	requireVolume(t, volumes, mysqlSSLCertVolumeName)
	requireVolume(t, volumes, mysqlSSLKeyVolumeName)
	requireVolume(t, volumes, redisCACertVolumeName)

	requireMount(t, mounts, customCACertsRootVolumeName, customCACertsRootMountPath)
	requireMount(t, mounts, customCACertsInlineVolumeName, customCACertsInlineMountPath)
	requireMount(t, mounts, customCACertsConfigMapVolumeName, customCACertsConfigMapMountPath)
	requireMount(t, mounts, mysqlCACertVolumeName, mysqlCACertPath)
	requireMount(t, mounts, mysqlSSLCertVolumeName, mysqlSSLCertPath)
	requireMount(t, mounts, mysqlSSLKeyVolumeName, mysqlSSLKeyPath)
	requireMount(t, mounts, redisCACertVolumeName, redisCACertPath)

	podTemplate := &corev1.PodTemplateSpec{}
	setCustomCACertsChecksumAnnotation(podTemplate, checksum)
	require.Equal(t, checksum, podTemplate.Annotations[customCACertsChecksumAnnotation])
}

func TestApplyCustomCACertsToWorkloadSkipsMissingOptionalInfraKeys(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "default",
		},
		Status: apiv2.WeightsAndBiasesStatus{
			MySQLStatus: apiv2.MysqlInfraStatus{
				Connection: apiv2.MysqlConnection{
					SslCa: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-mysql-connection"},
						Key:                  "SslCa",
						Optional:             ptr.To(true),
					},
				},
			},
			RedisStatus: apiv2.RedisInfraStatus{
				Connection: apiv2.RedisConnection{
					SslCa: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "wandb-redis-connection"},
						Key:                  "SslCa",
						Optional:             ptr.To(true),
					},
				},
			},
		},
	}
	mysqlSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-mysql-connection", Namespace: "default"},
		Data:       map[string][]byte{"url": []byte("mysql://user:pass@db:3306/wandb")},
	}
	redisSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb-redis-connection", Namespace: "default"},
		Data:       map[string][]byte{"url": []byte("redis://redis:6379")},
	}
	builder := customCATestClient(t, wandb, mysqlSecret, redisSecret)
	client := builder.Build()

	envs, volumes, mounts, checksum, err := applyCustomCACertsToWorkload(context.Background(), client, wandb, nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, checksum)
	requireNoEnv(t, envs, "MYSQL_CA_CERT_PATH")
	requireNoVolume(t, volumes, mysqlCACertVolumeName)
	requireNoVolume(t, volumes, redisCACertVolumeName)
	require.Empty(t, mounts)
}

func requireContainsEnv(t *testing.T, envs []corev1.EnvVar, name, value string) {
	t.Helper()
	for _, env := range envs {
		if env.Name == name {
			require.Equal(t, value, env.Value)
			return
		}
	}
	t.Fatalf("env var %q not found in %+v", name, envs)
}

func requireNoEnv(t *testing.T, envs []corev1.EnvVar, name string) {
	t.Helper()
	for _, env := range envs {
		require.NotEqual(t, name, env.Name)
	}
}

func requireVolume(t *testing.T, volumes []corev1.Volume, name string) {
	t.Helper()
	for _, volume := range volumes {
		if volume.Name == name {
			return
		}
	}
	t.Fatalf("volume %q not found in %+v", name, volumes)
}

func requireNoVolume(t *testing.T, volumes []corev1.Volume, name string) {
	t.Helper()
	for _, volume := range volumes {
		require.NotEqual(t, name, volume.Name)
	}
}

func requireMount(t *testing.T, mounts []corev1.VolumeMount, name, mountPath string) {
	t.Helper()
	for _, mount := range mounts {
		if mount.Name == name {
			require.Equal(t, mountPath, mount.MountPath)
			return
		}
	}
	t.Fatalf("volume mount %q not found in %+v", name, mounts)
}
