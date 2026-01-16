package manifest

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// Manifest defines the structure of the server manifest YAML (e.g. 0.76.1.yaml).
// It is intended to be a direct mapping of the YAML document for decoding via
// gopkg.in/yaml.v3 or sigs.k8s.io/yaml.
type Manifest struct {
	RequiredOperatorVersion string `yaml:"requiredOperatorVersion"`
	Features                map[string]bool `yaml:"features,omitempty"`
	// ServiceAccountName is the Kubernetes ServiceAccount used by all W&B application pods.
	ServiceAccountName string    `yaml:"serviceAccountName"`
	// Prefer plural, but accept singular key as found in some manifests.
	GeneratedSecrets []GeneratedSecret `yaml:"generatedSecrets,omitempty"`
	// CommonEnvvars defines reusable groups of env vars that can be referenced
	// by applications via the per-application `commonEnvs` list. This maps a
	// group name (e.g., "gorillaMysql") to a slice of EnvVar definitions.
	CommonEnvvars map[string][]EnvVar `yaml:"commonEnvvars,omitempty"`
	Bucket        SectionRef          `yaml:"bucket"`
	Clickhouse    SectionRef          `yaml:"clickhouse"`
	// Kafka is a list of topic declarations with optional feature gates in YAML.
	Kafka        []KafkaTopic  `yaml:"kafka"`
	Mysql        SectionRef    `yaml:"mysql"`
	Redis        SectionRef    `yaml:"redis"`
	Applications []Application `yaml:"applications"`
	// Migrations captures per-database migration jobs (e.g., default, runsdb, usagedb)
	// as found in 0.76.1.yaml under the top-level "migrations" key.
	Migrations map[string]MigrationJob `yaml:"migrations,omitempty"`
}

// GeneratedSecret represents the configuration for a dynamically generated secret.
type GeneratedSecret struct {
	Name          string `yaml:"name"`
	Length        int    `yaml:"length"`
	CharacterType string `yaml:"type"`
	// UseExactName when true, creates the secret with the exact name specified without prefixing it with the CR name.
	// This is useful for secrets that need to be referenced by external systems with a fixed name.
	UseExactName  bool   `yaml:"useExactName,omitempty"`
}

// SectionRef represents simple sections that commonly contain a single
// "default" key with an empty object (or future options).
type SectionRef struct {
	Default map[string]any `yaml:"default,omitempty"`
	// Extra captures any additional keys under the section (e.g., mysql.runsdb, redis.limiter)
	Extra map[string]any `yaml:",inline"`
}

// KafkaTopicDef models a topic configuration used both at the top-level
// kafka section and inside per-application kafka sections.
type KafkaTopicDef struct {
	Topic          string `yaml:"topic"`
	PartitionCount int    `yaml:"partitionCount,omitempty"`
	ConsumerGroup  string `yaml:"consumerGroup,omitempty"`
}

// KafkaTopic models one entry in the top-level kafka list in the YAML.
// Example:
//   - name: filestream
//     features: [filestreamQueue]
//     topic: filestream
//     partitionCount: 48
type KafkaTopic struct {
	Name           string   `yaml:"name"`
	Features       []string `yaml:"features,omitempty"`
	Topic          string   `yaml:"topic"`
	PartitionCount int      `yaml:"partitionCount,omitempty"`
}

// ImageRef represents an application container image reference.
type ImageRef struct {
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty"`
}

func (img ImageRef) GetImage() string {
	if img.Digest != "" {
		return img.Repository + "@" + img.Digest
	}
	if img.Tag != "" {
		return img.Repository + ":" + img.Tag
	}
	return img.Repository
}

// AppKafkaSection is the per-application kafka section; fields are optional
// and mirror the top-level topics.
type AppKafkaSection struct {
	Filestream               *KafkaTopicDef `yaml:"filestream,omitempty"`
	FlatRunFieldsUpdater     *KafkaTopicDef `yaml:"flatRunFieldsUpdater,omitempty"`
	WeaveWorker              *KafkaTopicDef `yaml:"weaveWorker,omitempty"`
	WeaveEvaluateModelWorker *KafkaTopicDef `yaml:"weaveEvaluateModelWorker,omitempty"`
}

// Application describes one entry in the applications list.
type Application struct {
	Name    string   `yaml:"name"`
	Image   ImageRef `yaml:"image"`
	Args    []string `yaml:"args,omitempty"`
	Command []string `yaml:"command,omitempty"`
	// CommonEnvs is a list of keys referencing top-level commonEnvvars groups
	// to be included for this application (e.g., ["gorillaMysql", "gorillaBucket"]).
	CommonEnvs []string `yaml:"commonEnvs,omitempty"`
	// InitContainers allows specifying per-application init containers
	// (e.g., the api app defines a "migrate" init container in 0.76.1.yaml).
	InitContainers []ContainerSpec `yaml:"initContainers,omitempty"`
	// Features enables this application only when specific feature flags are set in the
	// top-level manifest features. In the YAML this appears as a list of strings.
	Features       []string         `yaml:"features,omitempty"`
	Env            []EnvVar         `yaml:"env,omitempty"`
	Mysql          *SectionRef      `yaml:"mysql,omitempty"`
	Redis          *SectionRef      `yaml:"redis,omitempty"`
	Bucket         *SectionRef      `yaml:"bucket,omitempty"`
	Clickhouse     *SectionRef      `yaml:"clickhouse,omitempty"`
	Kafka          *AppKafkaSection `yaml:"kafka,omitempty"`
	Service        *ServiceSpec     `yaml:"service,omitempty"`
	Ports          []ContainerPort  `yaml:"ports,omitempty"`
	LivenessProbe  *corev1.Probe               `yaml:"livenessProbe,omitempty"`
	ReadinessProbe *corev1.Probe               `yaml:"readinessProbe,omitempty"`
	StartupProbe   *corev1.Probe               `yaml:"startupProbe,omitempty"`
	Resources      corev1.ResourceRequirements `yaml:"resources,omitempty"`
	// Files allows injecting files into the application's container by mounting
	// data from ConfigMaps. Each entry may either inline file contents (stored
	// into an operator-managed ConfigMap) or reference an existing ConfigMap.
	// The file will be mounted at the provided mountPath/FileName using subPath.
	Files     []FileSpec `yaml:"files,omitempty"`
	JWTTokens []JWTToken `yaml:"jwtTokens,omitempty"`
}

// ContainerSpec represents a minimal container definition used by
// application-level initContainers entries in the manifest.
type ContainerSpec struct {
	Name    string   `yaml:"name"`
	Image   ImageRef `yaml:"image"`
	Args    []string `yaml:"args,omitempty"`
	Command []string `yaml:"command,omitempty"`
}

// EnvVar models an application environment variable sourced from manifest-defined services.
type EnvVar struct {
	Name         string      `yaml:"name"`
	Value        string      `yaml:"value,omitempty"`
	Sources      []EnvSource `yaml:"sources,omitempty"`
	DefaultValue string      `yaml:"defaultValue,omitempty"`
}

// EnvSource references a named source and its type (e.g., mysql, redis, bucket).
type EnvSource struct {
	Name  string `yaml:"name"`
	Type  string `yaml:"type"`
	Field string `yaml:"field,omitempty"`
	Proto string `yaml:"proto,omitempty"`
	Path  string `yaml:"path,omitempty"`
	Port  string `yaml:"port,omitempty"`
}

// ServiceSpec represents an optional Service definition for an application
// (currently only ports are modeled as per 0.76.1.yaml needs).
type ServiceSpec struct {
	Type  corev1.ServiceType   `yaml:"type,omitempty"`
	Ports []corev1.ServicePort `yaml:"ports,omitempty"`
}

// ContainerPort models a single port entry in an application's ports section.
type ContainerPort struct {
	ContainerPort int32           `yaml:"containerPort"`
	Protocol      corev1.Protocol `yaml:"protocol,omitempty"`
	Name          string          `yaml:"name,omitempty"`
}

// MigrationJob represents a migration invocation with an image and args, used
// by the top-level "migrations" section (e.g., default, runsdb, usagedb).
type MigrationJob struct {
	Image   ImageRef `yaml:"image"`
	Args    []string `yaml:"args,omitempty"`
	Command []string `yaml:"command,omitempty"`
}

// FileSpec defines a single file to project into the application's container.
// Exactly one of Inline or ConfigMapRef should be provided. The file is mounted
// as a single file using subPath. MountPath should be a directory that already
// exists in the container image (e.g., /etc/nginx/conf.d). If FileName is not
// provided, Name will be used as the target filename. Name is also the key name
// stored inside the ConfigMap data.
type FileSpec struct {
	// Name is the key used in the ConfigMap data and defaults to the filename
	// if FileName is not provided.
	Name string `yaml:"name"`
	// MountPath is the directory inside the container where the file should be placed.
	MountPath string `yaml:"mountPath"`
	// FileName is the filename to write within MountPath. Optional; defaults to Name.
	FileName string `yaml:"fileName,omitempty"`
	// Inline is the file contents to embed directly into an operator-managed ConfigMap.
	Inline string `yaml:"inline,omitempty"`
	// ConfigMapRef references an existing ConfigMap (in the same namespace) to source the file from.
	// When set, Inline should be empty.
	ConfigMapRef string `yaml:"configMapRef,omitempty"`
}

// This will eventually be loaded externally outside of testing
var Path = "0.76.1.yaml"

func GetServerManifest(version string) (Manifest, error) {
	manifest := Manifest{}
	manifestData, err := os.ReadFile(Path)
	if err != nil {
		return Manifest{}, err
	}
	if err = yaml.Unmarshal(manifestData, &manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}
// JWTToken defines a JWT token to be mounted into the application's container.
// This abstraction supports multiple token sources: Kubernetes service account tokens,
// pre-created secrets, or cloud provider token stores (CSI).
type JWTToken struct {
	// Name is a unique identifier for this JWT token mount.
	Name string `yaml:"name"`
	// MountPath is the directory path where the token file will be mounted.
	MountPath string `yaml:"mountPath"`
	// Source specifies where the JWT token comes from. Exactly one source type should be set.
	Source JWTTokenSource `yaml:"source"`
}

// JWTTokenSource is a union type representing the different ways to source a JWT token.
// Exactly one field should be set.
type JWTTokenSource struct {
	// KubernetesServiceAccount requests a token from the Kubernetes API server
	// for the pod's service account with custom audience and expiration.
	KubernetesServiceAccount *K8sServiceAccountToken `yaml:"kubernetesServiceAccount,omitempty"`
	// SecretRef references an existing Kubernetes Secret containing the JWT token.
	SecretRef *SecretReference `yaml:"secretRef,omitempty"`
	// CSIProvider configures a CSI driver to fetch the token (e.g., AWS Secrets Manager,
	// Azure Key Vault, GCP Secret Manager).
	CSIProvider *CSIProviderConfig `yaml:"csiProvider,omitempty"`
}

// K8sServiceAccountToken configures a Kubernetes service account token projection.
type K8sServiceAccountToken struct {
	// Audience is the intended audience of the token (e.g., "internal-service").
	Audience string `yaml:"audience"`
	// ExpirationSeconds is the token's lifetime. Kubernetes will auto-rotate before expiration.
	// Optional; defaults to 3607 seconds (1 hour).
	ExpirationSeconds int64 `yaml:"expirationSeconds,omitempty"`
}

// SecretReference points to a Kubernetes Secret containing a JWT token.
type SecretReference struct {
	// Name is the name of the Secret in the same namespace.
	Name string `yaml:"name"`
	// Key is the data key within the Secret that contains the token.
	// Optional; defaults to "token" if not specified.
	Key string `yaml:"key,omitempty"`
}

// CSIProviderConfig configures a Container Storage Interface (CSI) driver
// for fetching JWT tokens from cloud provider secret stores.
type CSIProviderConfig struct {
	// Driver is the CSI driver name (e.g., "secrets-store.csi.k8s.io").
	Driver string `yaml:"driver"`
	// Parameters are driver-specific configuration key-value pairs.
	Parameters map[string]string `yaml:"parameters,omitempty"`
}
