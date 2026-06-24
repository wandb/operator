package bufstream

import (
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const defaultEtcdStorageSize = "10Gi"

func BuildWandbKafkaLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	return common.BuildWandbLabels(wandb, KafkaModuleName)
}

func ToKafkaOnDeleteRule(wandb *apiv2.WeightsAndBiases, retentionPolicy apiv2.RetentionPolicy) common.OnDeleteRule {
	return common.ToOnDeleteRule(wandb, retentionPolicy, KafkaModuleName)
}

// sameNamespace reports whether the managed Kafka resources live in the same
// namespace as the owning WeightsAndBiases CR. Owner references are only valid
// within a single namespace.
func sameNamespace(wandb *apiv2.WeightsAndBiases, nsnBuilder *NsNameBuilder) bool {
	return wandb.Namespace == nsnBuilder.Namespace()
}

func setOwner(wandb *apiv2.WeightsAndBiases, obj metav1.Object, nsnBuilder *NsNameBuilder, scheme *runtime.Scheme) error {
	if !sameNamespace(wandb, nsnBuilder) {
		return nil
	}
	return ctrl.SetControllerReference(wandb, obj, scheme)
}

func intstrFromInt(port int) intstr.IntOrString {
	return intstr.FromInt32(int32(port))
}

// tolerations safely dereferences the wandb tolerations pointer, which may be
// nil when neither the component nor the CR specify any.
func tolerations(wandb *apiv2.WeightsAndBiases, spec apiv2.ManagedInfraSpec) []corev1.Toleration {
	if t := wandb.GetTolerations(spec); t != nil {
		return *t
	}
	return nil
}

// ToCredentialsSecret builds the secret that holds the object-store credentials
// referenced by the broker config's env_var data sources.
func ToCredentialsSecret(
	wandb *apiv2.WeightsAndBiases,
	nsnBuilder *NsNameBuilder,
	storage storageConnInfo,
	scheme *runtime.Scheme,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.CredentialsName(),
			Namespace: nsnBuilder.Namespace(),
			Labels:    BuildWandbKafkaLabels(wandb),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			EnvStorageAccessKeyID:     storage.AccessKey,
			EnvStorageSecretAccessKey: storage.SecretKey,
		},
	}
	if err := setOwner(wandb, secret, nsnBuilder, scheme); err != nil {
		return nil, err
	}
	return secret, nil
}

// ToConfigMap renders the bufstream.yaml into a ConfigMap.
func ToConfigMap(
	wandb *apiv2.WeightsAndBiases,
	nsnBuilder *NsNameBuilder,
	storage storageConnInfo,
	scheme *runtime.Scheme,
) (*corev1.ConfigMap, error) {
	rendered, err := renderBufstreamConfig(
		nsnBuilder.BufstreamName(),
		nsnBuilder.BufstreamHost(),
		nsnBuilder.EtcdClientEndpoints(EtcdReplicas, EtcdClientPort),
		storage,
	)
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.ConfigMapName(),
			Namespace: nsnBuilder.Namespace(),
			Labels:    BuildWandbKafkaLabels(wandb),
		},
		Data: map[string]string{ConfigFileName: rendered},
	}
	if err := setOwner(wandb, cm, nsnBuilder, scheme); err != nil {
		return nil, err
	}
	return cm, nil
}

// ToEtcdApplication builds the Application CR that deploys etcd as a highly
// available StatefulSet: an odd-sized cluster (EtcdReplicas) fronted by a
// headless Service that gives each member a stable peer DNS identity.
func ToEtcdApplication(
	wandb *apiv2.WeightsAndBiases,
	nsnBuilder *NsNameBuilder,
	scheme *runtime.Scheme,
) (*apiv2.Application, error) {
	infraSpec := wandb.Spec.Kafka.ManagedKafka
	labels := BuildWandbKafkaLabels(wandb)

	storageSize := infraSpec.StorageSize
	if storageSize == "" {
		storageSize = defaultEtcdStorageSize
	}
	quantity, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return nil, fmt.Errorf("invalid etcd storage size %q: %w", storageSize, err)
	}

	// Each pod derives its own member identity from its stable StatefulSet name
	// via the downward API. Kubernetes expands $(POD_NAME) in later env values,
	// so peer/client advertise URLs resolve to the per-pod headless DNS record.
	// The static ETCD_INITIAL_CLUSTER lists every member up front so the cluster
	// bootstraps without external discovery.
	etcdEnv := []corev1.EnvVar{
		{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		{Name: "ETCD_NAME", Value: "$(POD_NAME)"},
		{Name: "ETCD_DATA_DIR", Value: EtcdDataDir},
		{Name: "ETCD_LISTEN_CLIENT_URLS", Value: fmt.Sprintf("http://0.0.0.0:%d", EtcdClientPort)},
		{Name: "ETCD_ADVERTISE_CLIENT_URLS", Value: fmt.Sprintf("http://$(POD_NAME).%s:%d", nsnBuilder.EtcdHost(), EtcdClientPort)},
		{Name: "ETCD_LISTEN_PEER_URLS", Value: fmt.Sprintf("http://0.0.0.0:%d", EtcdPeerPort)},
		{Name: "ETCD_INITIAL_ADVERTISE_PEER_URLS", Value: fmt.Sprintf("http://$(POD_NAME).%s:%d", nsnBuilder.EtcdHost(), EtcdPeerPort)},
		{Name: "ETCD_INITIAL_CLUSTER", Value: nsnBuilder.EtcdInitialCluster(EtcdReplicas, EtcdPeerPort)},
		{Name: "ETCD_INITIAL_CLUSTER_STATE", Value: "new"},
		{Name: "ETCD_INITIAL_CLUSTER_TOKEN", Value: fmt.Sprintf("%s-%s", EtcdClusterToken, nsnBuilder.SpecName())},
		{Name: "ETCD_AUTO_COMPACTION_MODE", Value: "periodic"},
		{Name: "ETCD_AUTO_COMPACTION_RETENTION", Value: "30s"},
	}

	app := &apiv2.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.EtcdName(),
			Namespace: nsnBuilder.Namespace(),
			Labels:    labels,
		},
		Spec: apiv2.ApplicationSpec{
			Kind:        "StatefulSet",
			Replicas:    ptr.To(int32(EtcdReplicas)),
			ServiceName: nsnBuilder.EtcdName(),
			MetaTemplate: metav1.ObjectMeta{
				Labels: labels,
			},
			PodTemplate: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Affinity:    spreadAffinity(wandb, infraSpec.ManagedInfraSpec, labels),
					Tolerations: tolerations(wandb, infraSpec.ManagedInfraSpec),
					Containers: []corev1.Container{
						{
							Name:  "etcd",
							Image: EtcdImage,
							Env:   etcdEnv,
							Ports: []corev1.ContainerPort{
								{Name: "client", ContainerPort: EtcdClientPort},
								{Name: "peer", ContainerPort: EtcdPeerPort},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: EtcdDataVolumeName, MountPath: EtcdDataDir},
							},
							ReadinessProbe: etcdProbe(),
							LivenessProbe:  etcdProbe(),
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: EtcdDataVolumeName},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceStorage: quantity},
						},
					},
				},
			},
			// Headless Service (ClusterIP None) backs the StatefulSet's per-pod
			// DNS. PublishNotReadyAddresses lets members resolve each other
			// during bootstrap, before quorum makes them Ready.
			ServiceTemplate: &corev1.ServiceSpec{
				Type:                     corev1.ServiceTypeClusterIP,
				ClusterIP:                corev1.ClusterIPNone,
				PublishNotReadyAddresses: true,
				Selector:                 map[string]string{},
				Ports: []corev1.ServicePort{
					{Name: "client", Port: EtcdClientPort, TargetPort: intstrFromInt(EtcdClientPort)},
					{Name: "peer", Port: EtcdPeerPort, TargetPort: intstrFromInt(EtcdPeerPort)},
				},
			},
		},
	}

	if err := setOwner(wandb, app, nsnBuilder, scheme); err != nil {
		return nil, err
	}
	return app, nil
}

// etcdProbe checks the client port. A member only accepts client connections
// once it has joined the cluster, so this doubles as a readiness and liveness
// signal without requiring etcdctl in the image.
func etcdProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstrFromInt(EtcdClientPort)},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    6,
	}
}

// spreadAffinity preserves any operator/CR-provided affinity, otherwise it
// spreads the component's pods across nodes so a single node failure cannot take
// down quorum (etcd) or all brokers (Bufstream). GetAffinity may return a
// non-nil but empty Affinity (from the CR's global affinity field), so an empty
// value is treated the same as unset and gets the default spread.
func spreadAffinity(wandb *apiv2.WeightsAndBiases, spec apiv2.ManagedInfraSpec, labels map[string]string) *corev1.Affinity {
	if a := wandb.GetAffinity(spec); a != nil && (a.NodeAffinity != nil || a.PodAffinity != nil || a.PodAntiAffinity != nil) {
		return a
	}
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey:   "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
					},
				},
			},
		},
	}
}

// bucketEnsureContainer returns an init container that idempotently creates the
// object-store bucket Bufstream reads from on startup. Bufstream itself never
// creates the bucket, and it can come up before the W&B applications that would
// otherwise create it, so it must be ensured here.
func bucketEnsureContainer(nsnBuilder *NsNameBuilder, storage storageConnInfo) corev1.Container {
	region := storage.Region
	if region == "" {
		region = "us-east-1"
	}
	credsName := nsnBuilder.CredentialsName()
	// head-bucket succeeds when the bucket already exists; otherwise create it.
	// Both paths tolerate concurrent creation by other brokers.
	script := fmt.Sprintf(
		"aws --endpoint-url %q s3api head-bucket --bucket %q || "+
			"aws --endpoint-url %q s3api create-bucket --bucket %q",
		storage.Endpoint, storage.Bucket, storage.Endpoint, storage.Bucket,
	)
	return corev1.Container{
		Name:    "ensure-bucket",
		Image:   BucketEnsureImage,
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{script},
		Env: []corev1.EnvVar{
			{Name: "AWS_REGION", Value: region},
			{
				Name: "AWS_ACCESS_KEY_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: credsName},
						Key:                  EnvStorageAccessKeyID,
					},
				},
			},
			{
				Name: "AWS_SECRET_ACCESS_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: credsName},
						Key:                  EnvStorageSecretAccessKey,
					},
				},
			},
		},
	}
}

// storageCredentialEnv injects the object-store credentials into the broker when
// static keys exist. Providers that authenticate via ambient identity (AWS IAM
// roles, GCS workload identity) get no env vars and no secret reference.
func storageCredentialEnv(nsnBuilder *NsNameBuilder, storage storageConnInfo) []corev1.EnvVar {
	if !storage.hasStaticCredentials() {
		return nil
	}
	credsName := nsnBuilder.CredentialsName()
	return []corev1.EnvVar{
		{
			Name: EnvStorageAccessKeyID,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: credsName},
					Key:                  EnvStorageAccessKeyID,
				},
			},
		},
		{
			Name: EnvStorageSecretAccessKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: credsName},
					Key:                  EnvStorageSecretAccessKey,
				},
			},
		},
	}
}

// needsBucketEnsure reports whether to run the S3 bucket-creation init
// container. It only applies to S3-compatible endpoints (SeaweedFS, MinIO),
// which is where the operator provisions the bucket; AWS S3, GCS, and Azure
// buckets are expected to already exist.
func needsBucketEnsure(storage storageConnInfo) bool {
	return storage.Provider == providerS3 && storage.Endpoint != ""
}

// effectiveBufstreamReplicas applies the HA floor to a requested broker count.
// BufstreamReplicas is an HA floor, not just a zero-value default: manifest infra
// sizing (ApplyInfraSizing) may set Replicas to 1 for small/dev sizes, but
// Bufstream brokers are stateless and must stay HA, so never run fewer than the
// floor. Larger sizes that request more brokers are still honored. This is the
// single source of truth for the desired broker count, so detach re-adoption
// (CheckDetached) compares against the same value the Deployment is built with.
func effectiveBufstreamReplicas(requested int32) int32 {
	if requested < BufstreamReplicas {
		return BufstreamReplicas
	}
	return requested
}

// ToBufstreamApplication builds the Application CR that deploys the stateless
// Bufstream brokers as a Deployment.
func ToBufstreamApplication(
	wandb *apiv2.WeightsAndBiases,
	nsnBuilder *NsNameBuilder,
	storage storageConnInfo,
	scheme *runtime.Scheme,
) (*apiv2.Application, error) {
	infraSpec := wandb.Spec.Kafka.ManagedKafka
	labels := BuildWandbKafkaLabels(wandb)

	replicas := effectiveBufstreamReplicas(infraSpec.Replicas)

	container := corev1.Container{
		Name:  "bufstream",
		Image: BufstreamImage,
		Args:  []string{"serve", "--config", fmt.Sprintf("%s/%s", ConfigMountPath, ConfigFileName)},
		Ports: []corev1.ContainerPort{
			{Name: "kafka", ContainerPort: KafkaListenerPort},
			{Name: "metrics", ContainerPort: DebugPort},
			{Name: "admin", ContainerPort: AdminPort},
		},
		Env: storageCredentialEnv(nsnBuilder, storage),
		VolumeMounts: []corev1.VolumeMount{
			{Name: "config", MountPath: ConfigMountPath, ReadOnly: true},
		},
	}
	if len(infraSpec.Config.Resources.Requests) > 0 || len(infraSpec.Config.Resources.Limits) > 0 {
		container.Resources = infraSpec.Config.Resources
	}

	var initContainers []corev1.Container
	if needsBucketEnsure(storage) {
		initContainers = append(initContainers, bucketEnsureContainer(nsnBuilder, storage))
	}

	app := &apiv2.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsnBuilder.BufstreamName(),
			Namespace: nsnBuilder.Namespace(),
			Labels:    labels,
		},
		Spec: apiv2.ApplicationSpec{
			Kind:     "Deployment",
			Replicas: ptr.To(replicas),
			MetaTemplate: metav1.ObjectMeta{
				Labels: labels,
			},
			PodTemplate: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Affinity:       spreadAffinity(wandb, infraSpec.ManagedInfraSpec, labels),
					Tolerations:    tolerations(wandb, infraSpec.ManagedInfraSpec),
					InitContainers: initContainers,
					Containers:     []corev1.Container{container},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: nsnBuilder.ConfigMapName()},
								},
							},
						},
					},
				},
			},
			ServiceTemplate: &corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: map[string]string{},
				Ports: []corev1.ServicePort{
					{Name: "kafka", Port: KafkaListenerPort, TargetPort: intstrFromInt(KafkaListenerPort)},
					{Name: "metrics", Port: DebugPort, TargetPort: intstrFromInt(DebugPort)},
				},
			},
		},
	}

	if err := setOwner(wandb, app, nsnBuilder, scheme); err != nil {
		return nil, err
	}
	return app, nil
}
