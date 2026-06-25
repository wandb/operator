package manifest

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"log/slog"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	v2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/logx"
	autoscalingv2 "k8s.io/api/autoscaling/v2"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote"
	//"oras.land/oras-go/v2/registry/remote/retry"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// Manifest defines the structure of the server manifest YAML (e.g. 0.76.1.yaml).
// It is intended to be a direct mapping of the YAML document for decoding via
// gopkg.in/yaml.v3 or sigs.k8s.io/yaml.
type Manifest struct {
	RequiredOperatorVersion string          `yaml:"requiredOperatorVersion"`
	Features                map[string]bool `yaml:"features,omitempty"`
	// Prefer plural, but accept singular key as found in some manifests.
	GeneratedSecrets []GeneratedSecret `yaml:"generatedSecrets,omitempty"`
	// CommonEnvvars defines reusable groups of env vars that can be referenced
	// by applications via the per-application `commonEnvs` list. This maps a
	// group name (e.g., "gorillaMysql") to a slice of EnvVar definitions.
	CommonEnvvars      map[string][]EnvVar      `yaml:"commonEnvvars,omitempty"`
	CommonVolumeMounts map[string][]VolumeMount `yaml:"commonVolumeMounts,omitempty"`
	Bucket             map[string]InfraConfig   `yaml:"bucket"`
	Clickhouse         map[string]InfraConfig   `yaml:"clickhouse"`
	Kafka              KafkaConfig              `yaml:"kafka"`
	Mysql              map[string]InfraConfig   `yaml:"mysql"`
	Redis              map[string]InfraConfig   `yaml:"redis"`
	Applications       map[string]Application   `yaml:"applications"`
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
	UseExactName bool `yaml:"useExactName,omitempty"`
}

type InfraConfig struct {
	Sizing  map[v2.Size]SizingConfig `yaml:"sizing"`
	Ingress *AppIngressSpec          `yaml:"ingress,omitempty"`
	Images  map[string]ImageRef      `yaml:"images,omitempty"`
}

// KafkaTopicDef models a topic configuration used both at the top-level
// kafka section and inside per-application kafka sections.
type KafkaTopicDef struct {
	Topic          string `yaml:"topic"`
	PartitionCount int    `yaml:"partitionCount,omitempty"`
	ConsumerGroup  string `yaml:"consumerGroup,omitempty"`
}

// KafkaConfig represents the top-level kafka section with sizing and topics.
type KafkaConfig struct {
	Sizing map[v2.Size]KafkaSizingConfig `yaml:"sizing"`
	Topics []KafkaTopic                  `yaml:"topics"`
	Image  ImageRef                      `yaml:"image,omitempty"`
}

// KafkaTopic models one entry in the kafka topics list in the YAML.
type KafkaTopic struct {
	Name           string   `yaml:"name"`
	Features       []string `yaml:"features,omitempty"`
	Topic          string   `yaml:"topic"`
	PartitionCount int      `yaml:"partitionCount,omitempty"`
}

// ImageRef represents an application container image reference.
type ImageRef struct {
	Registry   string `yaml:"registry"`
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag,omitempty"`
	Digest     string `yaml:"digest,omitempty"`
}

func (img ImageRef) GetImage(registry string) string {
	reg := img.Registry  // from manifest
	repository := img.Repository // from manifest, older verisons of manifest will contain both registry and repository in this field

	// manifest's ImageRef.Registry is blank, check if registry exists as part of repository string
	if reg == "" {
		if idx := strings.IndexByte(repository, '/'); idx != -1 && looksLikeRegistry(repository[:idx]) {
			reg = repository[:idx]
			repository = repository[idx+1:]
		}
	}
	// manifest's ImageRef.Registry exists, use that. already set at reg := above

	// user provided global image registry, replace provided registry
	if registry != "" {
		reg = registry

	}

	// repository can stand alone (e.g. "redis" or a Docker Hub namespace) when no
	// registry was supplied anywhere; only join with reg when we actually have one.
	image := repository
	if reg != "" {
		image = reg + "/" + repository
	}
	switch {
	case img.Digest != "":
		return image + "@" + img.Digest
	case img.Tag != "":
		return image + ":" + img.Tag
	default:
		return image
	}
}

func looksLikeRegistry(segment string) bool {
	return segment == "localhost" || strings.ContainsAny(segment, ".:")
}

// AppKafkaSection is the per-application kafka section; fields are optional
// and mirror the top-level topics.
type AppKafkaSection struct {
	Sizing map[v2.Size]SizingConfig `yaml:"sizing"`
	Topics map[string]KafkaTopicDef `yaml:"topics"`
}

// Application describes one entry in the applications list.
type Application struct {
	Name    string   `yaml:"name"`
	Image   ImageRef `yaml:"image"`
	Args    []string `yaml:"args,omitempty"`
	Command []string `yaml:"command,omitempty"`
	// CommonEnvs is a list of keys referencing top-level commonEnvvars groups
	// to be included for this application (e.g., ["gorillaMysql", "gorillaBucket"]).
	CommonEnvs         []string `yaml:"commonEnvs,omitempty"`
	CommonVolumeMounts []string `yaml:"commonVolumeMounts,omitempty"`
	// InitContainers allows specifying per-application init containers
	InitContainers []ContainerSpec `yaml:"initContainers,omitempty"`
	// Containers allows specifying per-application containers for multi-container apps
	Containers []ContainerSpec `yaml:"containers,omitempty"`
	// Features enables this application only when specific feature flags are set in the
	// top-level manifest features. In the YAML this appears as a list of strings.
	Features []string     `yaml:"features,omitempty"`
	Env      []EnvVar     `yaml:"env,omitempty"`
	Service  *ServiceSpec `yaml:"service,omitempty"`
	// Files allows injecting files into the application's container by mounting
	// data from ConfigMaps. Each entry may either inline file contents (stored
	// into an operator-managed ConfigMap) or reference an existing ConfigMap.
	// The file will be mounted at the provided mountPath/FileName using subPath.
	Files        []FileSpec               `yaml:"files,omitempty"`
	JWTTokens    []JWTToken               `yaml:"jwtTokens,omitempty"`
	VolumeMounts []VolumeMount            `yaml:"volumeMounts,omitempty"`
	Sizing       map[v2.Size]SizingConfig `yaml:"sizing,omitempty"`
	Ingress      *AppIngressSpec          `yaml:"ingress,omitempty"`
}

type AppIngressSpec struct {
	Paths       []string `yaml:"paths,omitempty"`
	ServicePort string   `yaml:"servicePort,omitempty"`
	PathType    string   `yaml:"pathType,omitempty"`
}

type SizingConfig struct {
	Replicas    int32                        `yaml:"replicas,omitempty"`
	Shards      int32                        `yaml:"shards,omitempty"`
	VolumeSize  string                       `yaml:"volumeSize,omitempty"`
	Resources   *corev1.ResourceRequirements `yaml:"resources,omitempty"`
	Autoscaling *AutoscalingConfig           `yaml:"autoscaling,omitempty"`
}

type KafkaSizingConfig struct {
	SizingConfig        `yaml:",inline"`
	ReplicationFactor   int32 `yaml:"replicationFactor,omitempty"`
	MinInSyncReplicas   int32 `yaml:"minInSyncReplicas,omitempty"`
	OffsetsTopicRF      int32 `yaml:"offsetsTopicRF,omitempty"`
	TransactionStateRF  int32 `yaml:"transactionStateRF,omitempty"`
	TransactionStateISR int32 `yaml:"transactionStateISR,omitempty"`
}

type AutoscalingConfig struct {
	Horizontal autoscalingv2.HorizontalPodAutoscalerSpec
}

// ContainerSpec represents a minimal container definition used by
// application-level initContainers entries in the manifest.
type ContainerSpec struct {
	Name           string          `yaml:"name"`
	Image          ImageRef        `yaml:"image"`
	Args           []string        `yaml:"args,omitempty"`
	Command        []string        `yaml:"command,omitempty"`
	Ports          []ContainerPort `yaml:"ports,omitempty"`
	LivenessProbe  *corev1.Probe   `yaml:"livenessProbe,omitempty"`
	ReadinessProbe *corev1.Probe   `yaml:"readinessProbe,omitempty"`
	StartupProbe   *corev1.Probe   `yaml:"startupProbe,omitempty"`
	Resources      *corev1.ResourceRequirements
}

// EnvVar models an application environment variable sourced from manifest-defined services.
type EnvVar struct {
	Name         string               `yaml:"name"`
	Value        string               `yaml:"value,omitempty"`
	ValueFrom    *corev1.EnvVarSource `yaml:"valueFrom,omitempty"`
	Sources      []EnvSource          `yaml:"sources,omitempty"`
	DefaultValue string               `yaml:"defaultValue,omitempty"`
}

type VolumeMount struct {
	MountPath string    `yaml:"mountPath"`
	Name      string    `yaml:"name"`
	Source    EnvSource `yaml:"source"`
}

// EnvSource references a named source and its type (e.g., moco, redis, bucket).
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
	Image              ImageRef      `yaml:"image"`
	Args               []string      `yaml:"args,omitempty"`
	Command            []string      `yaml:"command,omitempty"`
	CommonEnvs         []string      `yaml:"commonEnvs,omitempty"`
	CommonVolumeMounts []string      `yaml:"commonVolumeMounts,omitempty"`
	Env                []EnvVar      `yaml:"env,omitempty"`
	VolumeMounts       []VolumeMount `yaml:"volumeMounts,omitempty"`
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

func GetServerManifest(ctx context.Context, repository string, version string) (Manifest, error) {
	versionURL, err := url.Parse(repository)
	if err != nil {
		return Manifest{}, err
	}

	switch versionURL.Scheme {
	case "http", "https":
		return Manifest{}, errors.New("http manifest not implemented")
	case "file":
		return LoadManifestFromFile(ctx, repository, version)
	default:
		return DownloadServerManifest(ctx, repository, version)
	}
}

func LoadManifestFromFile(ctx context.Context, repository string, version string) (Manifest, error) {
	repositoryURL, err := url.Parse(repository)
	if err != nil {
		return Manifest{}, err
	}
	basePath := repositoryURL.Path

	rootManifestFile := path.Join(basePath, fmt.Sprintf("%s.yaml", version))
	if rootManifestFileInfo, err := os.Stat(rootManifestFile); err == nil && !rootManifestFileInfo.IsDir() {
		return loadManifestFromFiles(ctx, []string{rootManifestFile})
	}

	versionDir := filepath.Join(basePath, version)
	versionDirInfo, err := os.Stat(versionDir)
	if err != nil || !versionDirInfo.IsDir() {
		if err == nil {
			return Manifest{}, fmt.Errorf("manifest path %q is not a directory", versionDir)
		}
		return Manifest{}, fmt.Errorf(
			"failed to locate manifest for version %q in %q: %w",
			version,
			basePath,
			err,
		)
	}

	manifestFiles, err := filepath.Glob(filepath.Join(versionDir, "*.yaml"))
	if err != nil {
		return Manifest{}, err
	}
	if len(manifestFiles) == 0 {
		return Manifest{}, fmt.Errorf("no manifest files found in %q", versionDir)
	}
	slices.Sort(manifestFiles)

	return loadManifestFromFiles(ctx, manifestFiles)
}

func loadManifestFromFiles(ctx context.Context, manifestFiles []string) (Manifest, error) {
	logger := logx.GetSlog(ctx)
	manifest := Manifest{}

	for _, manifestFile := range manifestFiles {
		manifestData, err := os.ReadFile(manifestFile)
		if err != nil {
			logger.Error("failed to read manifest file", "file", manifestFile, "error", err)
			return Manifest{}, err
		}
		var fileManifest Manifest
		if err = yaml.Unmarshal(manifestData, &fileManifest); err != nil {
			logger.Error("failed to unmarshal manifest file", "file", manifestFile, "error", err)
			return Manifest{}, fmt.Errorf("failed to unmarshal %q: %w", manifestFile, err)
		}

		// Simple merge: preserve existing data, add new data
		mergeSimple(&manifest, &fileManifest)
	}

	logger.Debug("loaded manifest files", "count", len(manifestFiles), "files", manifestFiles, "manifest", manifest)
	return manifest, nil
}

// mergeSimple performs a simple merge that preserves existing data and adds new data
func mergeSimple(dst, src *Manifest) {
	// Preserve existing data, add new data from src

	// Basic fields - only set if dst is zero
	if dst.RequiredOperatorVersion == "" {
		dst.RequiredOperatorVersion = src.RequiredOperatorVersion
	}
	if dst.Features == nil {
		dst.Features = src.Features
	} else if src.Features != nil {
		for k, v := range src.Features {
			dst.Features[k] = v
		}
	}

	// Infrastructure configs - merge maps
	if src.Bucket != nil {
		if dst.Bucket == nil {
			dst.Bucket = make(map[string]InfraConfig)
		}
		mergeInfraConfigs(dst.Bucket, src.Bucket)
	}
	if src.Mysql != nil {
		if dst.Mysql == nil {
			dst.Mysql = make(map[string]InfraConfig)
		}
		mergeInfraConfigs(dst.Mysql, src.Mysql)
	}
	if src.Redis != nil {
		if dst.Redis == nil {
			dst.Redis = make(map[string]InfraConfig)
		}
		mergeInfraConfigs(dst.Redis, src.Redis)
	}
	if src.Clickhouse != nil {
		if dst.Clickhouse == nil {
			dst.Clickhouse = make(map[string]InfraConfig)
		}
		mergeInfraConfigs(dst.Clickhouse, src.Clickhouse)
	}

	// Kafka sizing
	if src.Kafka.Sizing != nil {
		if dst.Kafka.Sizing == nil {
			dst.Kafka.Sizing = make(map[v2.Size]KafkaSizingConfig)
		}
		for k, v := range src.Kafka.Sizing {
			dst.Kafka.Sizing[k] = v
		}
	}

	// Applications - merge maps
	if src.Applications != nil {
		if dst.Applications == nil {
			dst.Applications = make(map[string]Application)
		}
		mergeApplications(dst.Applications, src.Applications)
	}

	// Migrations
	if src.Migrations != nil {
		if dst.Migrations == nil {
			dst.Migrations = make(map[string]MigrationJob)
		}
		for k, v := range src.Migrations {
			dst.Migrations[k] = v
		}
	}

	// GeneratedSecrets - only from first file
	if len(dst.GeneratedSecrets) == 0 {
		dst.GeneratedSecrets = src.GeneratedSecrets
	}

	// CommonEnvvars - merge maps
	if src.CommonEnvvars != nil {
		if dst.CommonEnvvars == nil {
			dst.CommonEnvvars = make(map[string][]EnvVar)
		}
		for k, v := range src.CommonEnvvars {
			dst.CommonEnvvars[k] = v
		}
	}

	// CommonVolumeMounts - merge maps
	if src.CommonVolumeMounts != nil {
		if dst.CommonVolumeMounts == nil {
			dst.CommonVolumeMounts = make(map[string][]VolumeMount)
		}
		for k, v := range src.CommonVolumeMounts {
			dst.CommonVolumeMounts[k] = v
		}
	}

	// Kafka topics - only from first file (they don't exist in sizing.yaml anyway)
	if len(dst.Kafka.Topics) == 0 {
		dst.Kafka.Topics = src.Kafka.Topics
	}
}

// mergeInfraConfigs merges two map[string]InfraConfig maps
func mergeInfraConfigs(dst, src map[string]InfraConfig) {
	if src == nil {
		return
	}
	if dst == nil {
		return
	}

	for name, srcConfig := range src {
		if dstConfig, exists := dst[name]; exists {
			// Merge existing config
			mergedConfig := dstConfig

			// Preserve ingress from dst if src doesn't have it
			if srcConfig.Ingress == nil && dstConfig.Ingress != nil {
				mergedConfig.Ingress = dstConfig.Ingress
			} else if srcConfig.Ingress != nil {
				mergedConfig.Ingress = srcConfig.Ingress
			}

			// Merge sizing maps
			if srcConfig.Sizing != nil {
				if mergedConfig.Sizing == nil {
					mergedConfig.Sizing = make(map[v2.Size]SizingConfig)
				}
				for k, v := range srcConfig.Sizing {
					mergedConfig.Sizing[k] = v
				}
			}

			dst[name] = mergedConfig
		} else {
			// New config
			dst[name] = srcConfig
		}
	}
}

// mergeApplications merges two map[string]Application maps
func mergeApplications(dst, src map[string]Application) {
	if src == nil {
		return
	}
	if dst == nil {
		return
	}

	for name, srcApp := range src {
		if dstApp, exists := dst[name]; exists {
			// Merge existing application
			mergedApp := dstApp

			// Preserve ingress from dst if src doesn't have it
			if srcApp.Ingress == nil && dstApp.Ingress != nil {
				mergedApp.Ingress = dstApp.Ingress
			} else if srcApp.Ingress != nil {
				mergedApp.Ingress = srcApp.Ingress
			}

			// Merge sizing maps
			if srcApp.Sizing != nil {
				if mergedApp.Sizing == nil {
					mergedApp.Sizing = make(map[v2.Size]SizingConfig)
				}
				for k, v := range srcApp.Sizing {
					mergedApp.Sizing[k] = v
				}
			}

			// Merge common envs (avoid duplicates)
			if len(srcApp.CommonEnvs) > 0 {
				envSet := make(map[string]bool)
				for _, env := range mergedApp.CommonEnvs {
					envSet[env] = true
				}
				for _, env := range srcApp.CommonEnvs {
					if !envSet[env] {
						mergedApp.CommonEnvs = append(mergedApp.CommonEnvs, env)
					}
				}
			}

			dst[name] = mergedApp
		} else {
			// New application
			dst[name] = srcApp
		}
	}
}

func DownloadServerManifest(ctx context.Context, repository string, version string) (Manifest, error) {
	logger := logx.GetSlog(ctx)
	var manifest Manifest

	repository = strings.TrimPrefix(repository, "oci://")

	ociDir := "/tmp/server-manifest"
	localRepo, err := oci.New(ociDir)
	if err != nil {
		return manifest, err
	}

	var descriptor ocispec.Descriptor
	descriptor, err = localRepo.Resolve(ctx, version)
	if err != nil {
		logger.Info("image not found in local", "repository", repository, "version", version, "error", err)
		remoteRepo, err := remote.NewRepository(repository)
		if err != nil {
			logger.Error("failed to create repository", "repository", repository, "version", version, "error", err)
			return manifest, err
		}
		descriptor, err = oras.Copy(ctx, remoteRepo, version, localRepo, version, oras.DefaultCopyOptions)
		if err != nil {
			logger.Error("failed to fetch image from remote", "repository", repository, "version", version, "error", err)
			return manifest, err
		}
		logger.Info("successfully fetched image from remote", "repository", repository, "version", version, "desc", descriptor)
	} else {
		logger.Debug("successfully fetched image from local", "repository", repository, "version", version)
	}

	return processManifest(ctx, localRepo, descriptor, logger)
}

func processManifest(ctx context.Context, repo oras.ReadOnlyTarget, descriptor ocispec.Descriptor, logger *slog.Logger) (Manifest, error) {
	var manifest Manifest

	switch descriptor.MediaType {
	case ocispec.MediaTypeImageIndex, "application/vnd.docker.distribution.manifest.list.v2+json":
		indexData, err := content.FetchAll(ctx, repo, descriptor)
		if err != nil {
			return manifest, fmt.Errorf("failed to fetch index: %w", err)
		}
		var index ocispec.Index
		if err := json.Unmarshal(indexData, &index); err != nil {
			return manifest, fmt.Errorf("failed to unmarshal index: %w", err)
		}
		if len(index.Manifests) == 0 {
			return manifest, errors.New("index has no manifests")
		}
		// For now, we just pick the first manifest. In the future we might want to match platform.
		return processManifest(ctx, repo, index.Manifests[0], logger)

	case ocispec.MediaTypeImageManifest, "application/vnd.docker.distribution.manifest.v2+json":
		manifestData, err := content.FetchAll(ctx, repo, descriptor)
		if err != nil {
			return manifest, fmt.Errorf("failed to fetch manifest: %w", err)
		}

		var ociManifest ocispec.Manifest
		if err := json.Unmarshal(manifestData, &ociManifest); err != nil {
			return manifest, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}

		manifestFileContents := map[string][]byte{}

		for _, layer := range ociManifest.Layers {
			layerReader, err := repo.Fetch(ctx, layer)
			if err != nil {
				logger.Error("failed to fetch layer", "digest", layer.Digest, "err", err)
				continue
			}

			var tr *tar.Reader
			if layer.MediaType == ocispec.MediaTypeImageLayerGzip || layer.MediaType == "application/vnd.docker.image.rootfs.diff.tar.gzip" {
				gzr, err := gzip.NewReader(layerReader)
				if err != nil {
					layerReader.Close()
					logger.Error("failed to create gzip reader for layer", "digest", layer.Digest, "err", err)
					continue
				}
				tr = tar.NewReader(gzr)
				defer gzr.Close()
			} else {
				tr = tar.NewReader(layerReader)
			}
			defer layerReader.Close()

			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					logger.Error("failed to read tar header", "err", err)
					break
				}

				if filepath.Ext(header.Name) != ".yaml" {
					continue
				}

				manifestBytes, err := io.ReadAll(tr)
				if err != nil {
					logger.Error("failed to read manifest file", "file", header.Name, "err", err)
					continue
				}

				manifestFileContents[header.Name] = manifestBytes
			}
		}

		if len(manifestFileContents) == 0 {
			return manifest, errors.New("no manifest yaml files found in image layers")
		}

		manifestFiles := slices.Sorted(maps.Keys(manifestFileContents))
		for _, manifestFile := range manifestFiles {
			var fileManifest Manifest
			if err := yaml.Unmarshal(manifestFileContents[manifestFile], &fileManifest); err != nil {
				logger.Error("failed to unmarshal manifest file", "file", manifestFile, "err", err)
				return Manifest{}, fmt.Errorf("failed to unmarshal %q: %w", manifestFile, err)
			}

			mergeSimple(&manifest, &fileManifest)
		}

		logger.Debug("successfully unmarshaled manifest files", "files", manifestFiles, "manifest", manifest)
		return manifest, nil
	default:
		return manifest, fmt.Errorf("unsupported media type: %s", descriptor.MediaType)
	}
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

// FeaturesEnabled returns true if any of the supplied feature flags are
// enabled in the manifest's top-level Features section.
func (m *Manifest) FeaturesEnabled(topicFeatures []string) bool {
	if len(topicFeatures) == 0 || m.Features == nil {
		return false
	}
	for _, f := range topicFeatures {
		if enabled, ok := m.Features[f]; ok && enabled {
			return true
		}
	}
	return false
}

func (m *Manifest) ResolveServiceURL(src EnvSource) (string, bool) {
	if src.Name == "" {
		return "", false
	}

	app, ok := m.Applications[src.Name]
	if !ok {
		return "", false
	}

	port, ok := app.ResolveServicePortFromManifest(src.Port)
	if !ok {
		return "", false
	}

	protoPrefix := ""
	if src.Proto != "" {
		protoPrefix = fmt.Sprintf("%s://", src.Proto)
	}
	return fmt.Sprintf("%s%s:%d%s", protoPrefix, src.Name, port, src.Path), true
}

func (a *Application) ResolveServicePortFromManifest(requestedPort string) (int32, bool) {
	if requestedPort != "" {
		if n, err := strconv.ParseInt(requestedPort, 10, 32); err == nil {
			return int32(n), true
		}
	}

	if a.Service != nil {
		if requestedPort == "" && len(a.Service.Ports) > 0 {
			return a.Service.Ports[0].Port, true
		}
		for _, p := range a.Service.Ports {
			if p.Name == requestedPort {
				return p.Port, true
			}
		}
	}

	for _, container := range a.Containers {
		if requestedPort == "" && len(container.Ports) > 0 {
			return container.Ports[0].ContainerPort, true
		}
		for _, p := range container.Ports {
			if p.Name == requestedPort {
				return p.ContainerPort, true
			}
		}
	}
	for _, container := range a.InitContainers {
		if requestedPort == "" && len(container.Ports) > 0 {
			return container.Ports[0].ContainerPort, true
		}
		for _, p := range container.Ports {
			if p.Name == requestedPort {
				return p.ContainerPort, true
			}
		}
	}

	return 0, false
}
