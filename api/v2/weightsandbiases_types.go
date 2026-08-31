/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=wandb
//+kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
//+kubebuilder:printcolumn:name="MySQL",type=string,JSONPath=`.status.mysqlStatus.default.state`
//+kubebuilder:printcolumn:name="Redis",type=string,JSONPath=`.status.redisStatus.default.state`
//+kubebuilder:printcolumn:name="Kafka",type=string,JSONPath=`.status.kafkaStatus.state`
//+kubebuilder:printcolumn:name="ObjectStore",type=string,JSONPath=`.status.objectStoreStatus.default.state`
//+kubebuilder:printcolumn:name="ClickHouse",type=string,JSONPath=`.status.clickhouseStatus.default.state`
//+kubebuilder:printcolumn:name="Migration",type=string,JSONPath=`.status.wandb.migration.phase`

// WeightsAndBiases is the Schema for the weightsandbiases API.
type WeightsAndBiases struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WeightsAndBiasesSpec   `json:"spec,omitempty"`
	Status WeightsAndBiasesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WeightsAndBiasesList contains a list of WeightsAndBiases.
type WeightsAndBiasesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WeightsAndBiases `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WeightsAndBiases{}, &WeightsAndBiasesList{})
}

// DefaultInstanceName is the reserved map key identifying the fallback instance
// for each multi-instance infrastructure type (MySQL, Redis, ObjectStore,
// ClickHouse). When an application requests an instance that is not provisioned,
// the operator resolves to this instance instead.
const DefaultInstanceName = "default"

// ResolveInstance returns the entry for key, falling back to the
// DefaultInstanceName entry when key is empty or absent. The boolean reports
// whether a value was found.
func ResolveInstance[T any](m map[string]T, key string) (T, bool) {
	if key == "" {
		key = DefaultInstanceName
	}
	if v, ok := m[key]; ok {
		return v, true
	}
	if v, ok := m[DefaultInstanceName]; ok {
		return v, true
	}
	var zero T
	return zero, false
}

type Size string

const (
	SizeDev     Size = "dev"
	SizeMicro   Size = "micro"
	SizeSmall   Size = "small"
	SizeMedium  Size = "medium"
	SizeLarge   Size = "large"
	SizeXLarge  Size = "xlarge"
	SizeXXLarge Size = "xxlarge"
)

type OnDeletePolicy string

const (
	// DetachOnDelete removes ownership of infrastructure CRs so they survive WandB CR deletion
	DetachOnDelete OnDeletePolicy = "detach"
	// PurgeOnDelete will delete all associated resources upon deletion
	PurgeOnDelete OnDeletePolicy = "purge"
)

type RetentionPolicy struct {
	// +kubebuilder:default="detach"
	OnDelete OnDeletePolicy `json:"onDelete" default:"detach"`
}

// WeightsAndBiasesSpec defines the desired state of WeightsAndBiases.
type WeightsAndBiasesSpec struct {
	// Size is akin to high-level environment info
	// +kubebuilder:validation:Enum=dev;micro;small;medium;large;xlarge;xxlarge
	Size Size `json:"size,omitempty"`
	// RequireLimits By default, only resource requests are set for deployments, set to true to also set resource limits
	RequireLimits bool `json:"requireLimits,omitempty"`

	RetentionPolicy RetentionPolicy `json:"retentionPolicy"`

	// Global holds settings shared across all managed components.
	// +optional
	Global GlobalSpec `json:"global,omitempty"`

	Wandb WandbAppSpec `json:"wandb,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`

	// MySQL, Redis, ObjectStore and ClickHouse are keyed by instance name. The
	// reserved DefaultInstanceName key identifies the fallback instance used when
	// an application requests an instance that is not provisioned.
	MySQL       map[string]MySQLSpec       `json:"mysql,omitempty"`
	Redis       map[string]RedisSpec       `json:"redis,omitempty"`
	Kafka       KafkaSpec                  `json:"kafka,omitempty"`
	ObjectStore map[string]ObjectStoreSpec `json:"objectStore,omitempty"`
	ClickHouse  map[string]ClickHouseSpec  `json:"clickhouse,omitempty"`

	// Networking configures how the W&B application is exposed externally.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`

	AdminConsoleEnabled *bool `json:"adminConsoleEnabled,omitempty"`
}

const (
	DefaultWatchtowerBasePath           = "/console"
	DefaultWatchtowerServiceAccountName = "wandb-watchtower"
)

// GlobalSpec holds settings shared across every managed component.
type GlobalSpec struct {
	// ImageRegistry, when set, retargets the container images to this registry.
	// Intended for air-gapped installs whose nodes cannot reach public registries; pair it
	// with a registry pre-populated by `wsm registry mirror`.
	ImageRegistry string `json:"imageRegistry,omitempty"`

	// ImagePullSecrets references kubernetes.io/dockerconfigjson Secrets in the
	// W&B namespace. They authenticate BOTH the operator's server-manifest pull
	// and, propagated onto the workload ServiceAccount, the workloads' image
	// pulls — a single credential for both.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// CustomCACerts contains PEM-encoded CA certificates that should be trusted
	// by W&B application workloads.
	// +optional
	CustomCACerts []string `json:"customCACerts,omitempty"`

	// CACertsConfigMap references a ConfigMap in the W&B namespace whose keys
	// contain CA certificates. Keys should use a .crt suffix so standard CA
	// update tooling can discover them.
	// +optional
	CACertsConfigMap string `json:"caCertsConfigMap,omitempty"`

	// Proxy configures the forward-proxy egress settings injected into the
	// application workloads (app Deployments, their init containers, and
	// migration Jobs). The operator emits HTTP_PROXY/HTTPS_PROXY/NO_PROXY and
	// their lowercase variants; NO_PROXY is always the operator-computed
	// in-cluster exclusions merged over the user-supplied noProxy entries, so
	// in-cluster datastore/service traffic never hairpins through the proxy.
	// Pair with CustomCACerts for a TLS-intercepting proxy.
	// +optional
	Proxy *ProxySpec `json:"proxy,omitempty"`
}

func (w *WeightsAndBiases) WatchtowerEnabled() bool {
	return w.Spec.AdminConsoleEnabled != nil && *w.Spec.AdminConsoleEnabled
}

// ProxySpec is the forward-proxy configuration under spec.global.proxy.
type ProxySpec struct {
	// HTTPProxy is the proxy URL for plain HTTP egress (HTTP_PROXY/http_proxy).
	// +optional
	HTTPProxy *ValueOrSecret `json:"httpProxy,omitempty"`

	// HTTPSProxy is the proxy URL for HTTPS egress (HTTPS_PROXY/https_proxy).
	// +optional
	HTTPSProxy *ValueOrSecret `json:"httpsProxy,omitempty"`

	// NoProxy holds EXTRA no-proxy entries appended to the operator-computed
	// in-cluster exclusions. Use it for external endpoints (e.g. a BYOB object
	// store) that must bypass the proxy. Entries must be comma-free; the
	// operator owns the join.
	// +optional
	NoProxy []string `json:"noProxy,omitempty"`
}

// ValueOrSecret supplies a configuration value either as a literal (Value) or
// from a Secret key (ValueFrom), mirroring corev1.EnvVar semantics: exactly one
// arm is set, enforced by the webhook. Sensitive values MUST use ValueFrom so
// they never land in the CR / etcd / kubectl output.
//
// The legacy Name/Key/Optional fields carry the historical bare-SecretKeySelector
// shape ({name, key}) so existing CRs keep validating; the defaulting webhook
// normalizes them into ValueFrom on admission. They are deprecated and will be
// removed at v2 GA.
type ValueOrSecret struct {
	// Value is a literal value.
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom sources the value from a Secret key.
	// +optional
	ValueFrom *SecretValueSource `json:"valueFrom,omitempty"`

	// Deprecated: use ValueFrom.secretKeyRef. Retained for backward compatibility
	// with the pre-envelope {name, key} shape; normalized into ValueFrom by the
	// defaulting webhook and removed at v2 GA.
	// +optional
	Name string `json:"name,omitempty"`
	// Deprecated: use ValueFrom.secretKeyRef.
	// +optional
	Key string `json:"key,omitempty"`
	// Deprecated: use ValueFrom.secretKeyRef.
	// +optional
	Optional *bool `json:"optional,omitempty"`
}

// SecretValueSource reads a value from a Secret key in the W&B namespace.
type SecretValueSource struct {
	// SecretKeyRef selects a key of a Secret in the W&B namespace.
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// IsZero reports whether neither a literal, an envelope secret ref, nor a legacy
// secret ref is set.
func (v *ValueOrSecret) IsZero() bool {
	return v == nil || (v.Value == "" && v.ValueFrom == nil && v.Name == "")
}

// SecretKeyRef returns the effective secret selector: the canonical
// ValueFrom.SecretKeyRef when set, otherwise one synthesized from the legacy
// Name/Key/Optional fields, otherwise nil (a literal or unset value).
func (v *ValueOrSecret) SecretKeyRef() *corev1.SecretKeySelector {
	if v == nil {
		return nil
	}
	if v.ValueFrom != nil && v.ValueFrom.SecretKeyRef != nil {
		return v.ValueFrom.SecretKeyRef
	}
	if v.Name != "" {
		return &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: v.Name},
			Key:                  v.Key,
			Optional:             v.Optional,
		}
	}
	return nil
}

// AsEnvVar renders the value as a container EnvVar: a literal keeps the value in
// the pod spec, while a secret ref stays a live SecretKeyRef so the secret is
// never materialized. Returns a zero EnvVar (name only) when unset.
func (v *ValueOrSecret) AsEnvVar(name string) corev1.EnvVar {
	if v != nil && v.Value != "" {
		return corev1.EnvVar{Name: name, Value: v.Value}
	}
	if ref := v.SecretKeyRef(); ref != nil {
		return corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: ref.DeepCopy()}}
	}
	return corev1.EnvVar{Name: name}
}

// LiteralValue wraps a literal string as a ValueOrSecret.
func LiteralValue(s string) ValueOrSecret {
	return ValueOrSecret{Value: s}
}

// Normalize rewrites the deprecated legacy {name, key} shape into the canonical
// ValueFrom.SecretKeyRef, so stored objects converge on the envelope. It is a
// no-op once the value is a literal or already an envelope secret ref. The
// defaulting webhook calls this on admission.
func (v *ValueOrSecret) Normalize() {
	if v == nil || v.Name == "" || v.ValueFrom != nil {
		return
	}
	v.ValueFrom = &SecretValueSource{SecretKeyRef: v.SecretKeyRef()}
	v.Name, v.Key, v.Optional = "", "", nil
}

// ValueFromSelector wraps an existing SecretKeySelector as the secret arm of a
// ValueOrSecret. Used by v1→v2 conversion, which classifies raw values into
// selectors before this envelope existed.
func ValueFromSelector(sel corev1.SecretKeySelector) ValueOrSecret {
	return ValueOrSecret{ValueFrom: &SecretValueSource{SecretKeyRef: &sel}}
}

// ValueFromSecret builds the canonical secret arm pointing at name/key. Used by
// status writers referencing the operator-owned connection secret.
func ValueFromSecret(name, key string, optional bool) ValueOrSecret {
	return ValueOrSecret{
		ValueFrom: &SecretValueSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Key:                  key,
				Optional:             &optional,
			},
		},
	}
}

type NetworkingMode string

const (
	NetworkingModeNone       NetworkingMode = ""
	NetworkingModeIngress    NetworkingMode = "ingress"
	NetworkingModeGatewayAPI NetworkingMode = "gateway"
)

type NetworkingSpec struct {
	// Mode selects the networking strategy: "Ingress" or "GatewayAPI".
	// Empty/unset means no operator-managed ingress (preserves current NodePort behavior).
	// +kubebuilder:validation:Enum="";ingress;gateway
	Mode NetworkingMode `json:"mode,omitempty"`

	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

	// +optional
	GatewayAPI *GatewayAPIConfig `json:"gatewayAPI,omitempty"`

	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`

	// Annotations applied to all generated Ingress or Gateway resources.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type IngressConfig struct {
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Name overrides the generated Ingress resource name. When empty the
	// operator defaults to the CR name.
	// +optional
	Name string `json:"name,omitempty"`
}

type GatewayAPIConfig struct {
	Gateway GatewayConfig `json:"gateway"`

	// ListenerName selects which listener on the Gateway to attach HTTPRoutes to.
	// +optional
	ListenerName *string `json:"listenerName,omitempty"`
}

type GatewayConfig struct {
	// Managed controls whether the operator creates and manages the Gateway resource.
	// When false (default), gatewayRef must reference an existing Gateway.
	// +kubebuilder:default=false
	Managed bool `json:"managed,omitempty"`

	// +optional
	GatewayRef *GatewayReference `json:"gatewayRef,omitempty"`

	// GatewayClassName is required when managed=true.
	// +optional
	GatewayClassName *string `json:"gatewayClassName,omitempty"`

	// Listeners defines the listeners on a managed Gateway.
	// If empty and managed=true, a default HTTPS listener is created from
	// spec.wandb.hostname and spec.networking.tls.
	// +optional
	Listeners []GatewayListener `json:"listeners,omitempty"`

	// Annotations passed to the managed Gateway resource.
	// +optional
	InfrastructureAnnotations map[string]string `json:"infrastructureAnnotations,omitempty"`
}

type GatewayReference struct {
	Name string `json:"name"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type GatewayListener struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
	// +optional
	Hostname *string `json:"hostname,omitempty"`
	// +optional
	TLS *ListenerTLSConfig `json:"tls,omitempty"`
}

type ListenerTLSConfig struct {
	// +optional
	Mode *string `json:"mode,omitempty"`
	// +optional
	CertificateRef *SecretRef `json:"certificateRef,omitempty"`
	// Options contains implementation-specific TLS settings. Keys must use
	// domain-prefixed names, such as networking.gke.io/pre-shared-certs.
	// +optional
	// +kubebuilder:validation:MaxProperties=16
	Options map[string]TLSOptionValue `json:"options,omitempty"`
}

// TLSOptionValue mirrors the Gateway API AnnotationValue contract so options
// rejected by the Gateway CRD are rejected on the WeightsAndBiases CR instead.
// +kubebuilder:validation:MaxLength=4096
type TLSOptionValue string

type SecretRef struct {
	Name string `json:"name"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type TLSConfig struct {
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// +optional
	CertManager *CertManagerConfig `json:"certManager,omitempty"`
}

type CertManagerConfig struct {
	// +optional
	ClusterIssuer string `json:"clusterIssuer,omitempty"`
	// +optional
	Issuer string `json:"issuer,omitempty"`
}

func (w *WeightsAndBiases) GetRetentionPolicy(spec ManagedInfraSpec) RetentionPolicy {
	if spec.RetentionPolicy != nil {
		return *spec.RetentionPolicy
	}
	return w.Spec.RetentionPolicy
}

func (w *WeightsAndBiases) GetAffinity(spec ManagedInfraSpec) *corev1.Affinity {
	if spec.Affinity != nil {
		return spec.Affinity
	}
	return w.Spec.Affinity
}

func (w *WeightsAndBiases) GetTolerations(spec ManagedInfraSpec) *[]corev1.Toleration {
	if spec.Tolerations != nil {
		return spec.Tolerations
	}
	return w.Spec.Tolerations
}

// ValidMysqlReplicaCount reports whether r is a count Moco accepts: a positive odd number.
func ValidMysqlReplicaCount(r int32) bool {
	return r > 0 && r%2 == 1
}

// WandbAppSpec defines the configuration for the Wandb application deployment.
type WandbAppSpec struct {
	Hostname            string              `json:"hostname"`
	License             string              `json:"license,omitempty"`
	ManifestRepository  string              `json:"manifestRepository,omitempty"`
	Version             string              `json:"version"`
	Features            map[string]bool     `json:"features"`
	InternalServiceAuth InternalServiceAuth `json:"internalServiceAuth,omitempty"`
	BucketProxy         bool                `json:"bucketProxy"`

	ServiceAccount ServiceAccountSpec `json:"serviceAccount,omitempty"`

	// Probes configures default health probes for W&B application workload
	// containers generated by the operator. The full Kubernetes Probe shape is
	// exposed so operators can tune timings and handlers for their environment.
	// Explicit probes on generated containers remain authoritative; these values
	// only fill missing probes or missing probe fields.
	// +optional
	Probes WandbProbeDefaults `json:"probes,omitempty"`

	// +optional
	AdditionalHostnames []string `json:"additionalHostnames,omitempty"`

	// +optional
	OIDC OidcSpec `json:"oidc,omitempty"`

	// Notification Configurations
	// +optional
	Notifications *NotificationsSpec `json:"notifications,omitempty"`

	// Security Flag Configurations
	// +optional
	Security SecuritySpec `json:"security,omitempty"`

	// Retention Spec for the WandB application
	// +optional
	Retention *RetentionSpec `json:"retention,omitempty"`

	// LegacyOverrides holds env/resource overrides extracted from v1
	// spec.values, keyed by manifest application name plus the reserved
	// "global" key (env only, applied to every application). Unknown keys are
	// logged and ignored. Conversion-owned; prefer first-class fields over
	// hand-editing.
	// +optional
	LegacyOverrides map[string]LegacyOverrides `json:"legacyOverrides,omitempty"`

	// Applications overlays sizing-derived per-application config, keyed by
	// manifest application name. Unknown keys are logged and ignored.
	// +optional
	Applications map[string]WandbApplicationOverride `json:"applications,omitempty"`
}

type WandbApplicationOverride struct {
	// +optional
	Autoscaling *ApplicationAutoscalingOverride `json:"autoscaling,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!has(self.minReplicas) || !has(self.maxReplicas) || self.minReplicas <= self.maxReplicas",message="minReplicas must be <= maxReplicas"
type ApplicationAutoscalingOverride struct {
	// +optional
	// +kubebuilder:validation:Minimum=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

// LegacyOverridesGlobalKey is the reserved LegacyOverrides key whose env
// applies to every application and migration job.
const LegacyOverridesGlobalKey = "global"

const (
	// DefaultImageRegistry hosts the public W&B images and the server manifest.
	// wsm mirrors both into one registry, so the manifest sits next to the images.
	DefaultImageRegistry = "us-docker.pkg.dev/wandb-production/public"
	// ManifestRepositorySuffix is the path under a registry where the server
	// manifest lives; manifestRepository defaults to <imageRegistry>/<suffix>.
	ManifestRepositorySuffix = "wandb/server-manifest"
	// DefaultManifestRepository is used when spec.wandb.manifestRepository is
	// unset and no spec.global.imageRegistry override is set.
	DefaultManifestRepository = "oci://" + DefaultImageRegistry + "/" + ManifestRepositorySuffix
)

// ManifestRepositoryFor derives the server-manifest repository from an image
// registry (spec.global.imageRegistry). An empty registry yields the public
// default. This mirrors the wsm flow: the manifest is mirrored alongside the
// images at <imageRegistry>/wandb/server-manifest.
func ManifestRepositoryFor(imageRegistry string) string {
	if imageRegistry == "" {
		return DefaultManifestRepository
	}
	return "oci://" + imageRegistry + "/" + ManifestRepositorySuffix
}

// LegacyOverrides holds v1-derived overrides for one application (or "global").
type LegacyOverrides struct {
	// Env is applied last, replacing same-named manifest or injected vars.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources overlays sizing-derived resources per field; limits are still
	// gated by spec.requireLimits.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type WandbProbeDefaults struct {
	// StartupProbe defines default startup probe values for generated W&B
	// application containers. If the handler is omitted, the operator derives
	// one from the container readiness probe, then liveness probe.
	// +optional
	StartupProbe *corev1.Probe `json:"startupProbe,omitempty"`

	// LivenessProbe defines default liveness probe values for generated W&B
	// application containers.
	// +optional
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`

	// ReadinessProbe defines default readiness probe values for generated W&B
	// application containers.
	// +optional
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
}

type ServiceAccountSpec struct {
	// +kubebuilder:default=true
	Create *bool `json:"create"`
	// +kubebuilder:default="wandb"
	ServiceAccountName string            `json:"serviceAccountName,omitempty"`
	Annotations        map[string]string `json:"annotations,omitempty"`
}

// ManagedServiceAccountSpec configures the Kubernetes identity used by a
// managed infrastructure workload.
type ManagedServiceAccountSpec struct {
	// Create controls whether the operator reconciles the ServiceAccount. It
	// defaults to true; set it to false to reference an existing identity.
	Create *bool `json:"create,omitempty"`
	// ServiceAccountName defaults to the managed infrastructure resource name.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Annotations supports cloud workload identity integrations such as IRSA.
	Annotations map[string]string `json:"annotations,omitempty"`
}

type InternalServiceAuth struct {
	Enabled    *bool  `json:"enabled,omitempty"`
	OIDCIssuer string `json:"oidcIssuer,omitempty"`
}

// OidcSpec defines the structure for OpenID Connect (OIDC) configuration used in Wandb application deployments.
type OidcSpec struct {
	ClientId     ValueOrSecret `json:"clientId,omitempty"`
	ClientSecret ValueOrSecret `json:"clientSecret,omitempty" masq:"secret"`
	IssuerUrl    ValueOrSecret `json:"issuerUrl,omitempty"`
	AuthMethod   ValueOrSecret `json:"authMethod,omitempty"`

	SessionLength string `json:"sessionLength,omitempty"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (o *OidcSpec) Normalize() {
	if o == nil {
		return
	}
	o.ClientId.Normalize()
	o.ClientSecret.Normalize()
	o.IssuerUrl.Normalize()
	o.AuthMethod.Normalize()
}

type SecuritySpec struct {
	// +kubebuilder:default=false
	AllowUserTeamCreation bool `json:"allowUserTeamCreation,omitempty"`

	// +kubebuilder:default=false
	DisableCodeSaving bool `json:"disableCodeSaving,omitempty"`

	// +kubebuilder:default=false
	AllowAnonymousPublicProjects bool `json:"allowAnonymousPublicProjects,omitempty"`

	// +kubebuilder:default=false
	DisableSSOProvisioning bool `json:"disableSSOProvisioning,omitempty"`

	// +kubebuilder:default=false
	InsecureAllowAPIKeyAdminAccess bool `json:"insecureAllowAPIKeyAdminAccess,omitempty"`

	// +kubebuilder:default=false
	HideUpgradeBanner bool `json:"hideUpgradeBanner,omitempty"`
}

type NotificationsSpec struct {
	Email *EmailSpec `json:"email,omitempty"`
	Slack *SlackSpec `json:"slack,omitempty"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (n *NotificationsSpec) Normalize() {
	if n == nil {
		return
	}
	n.Email.Normalize()
	n.Slack.Normalize()
}

type EmailSMTPSpec struct {
	Host     ValueOrSecret `json:"host"`
	Port     ValueOrSecret `json:"port"`
	Username ValueOrSecret `json:"username"`
	Password ValueOrSecret `json:"password" masq:"secret"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (s *EmailSMTPSpec) Normalize() {
	if s == nil {
		return
	}
	s.Host.Normalize()
	s.Port.Normalize()
	s.Username.Normalize()
	s.Password.Normalize()
}

type EmailSpec struct {
	// Sink is a full notification sink URL; it may embed credentials.
	Sink *ValueOrSecret `json:"sink,omitempty" masq:"secret"`
	SMTP *EmailSMTPSpec `json:"smtp,omitempty"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (e *EmailSpec) Normalize() {
	if e == nil {
		return
	}
	e.Sink.Normalize()
	e.SMTP.Normalize()
}

type SlackSpec struct {
	ClientID     ValueOrSecret `json:"clientId,omitempty"`
	ClientSecret ValueOrSecret `json:"clientSecret,omitempty" masq:"secret"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (s *SlackSpec) Normalize() {
	if s == nil {
		return
	}
	s.ClientID.Normalize()
	s.ClientSecret.Normalize()
}

type RetentionSpec struct {
	ArtifactGarbageCollection bool             `json:"artifactGarbageCollection,omitempty"`
	DataRetentionPeriod       *metav1.Duration `json:"dataRetentionPeriod,omitempty"`
}

type ManagedInfraSpec struct {
	RetentionPolicy *RetentionPolicy `json:"retentionPolicy,omitempty"`

	Affinity    *corev1.Affinity     `json:"affinity,omitempty"`
	Tolerations *[]corev1.Toleration `json:"tolerations,omitempty"`
}

// MySQLSpec fields have many default values that, if unspecified,
// will be applied by a defaulting webook
type MySQLSpec struct {
	ManagedMysql  *ManagedMysqlSpec `json:"managedMysql,omitempty"`
	ExternalMysql *MysqlConnection  `json:"externalMysql,omitempty"`
}

type ManagedMysqlSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize string      `json:"storageSize,omitempty"`
	Replicas    int32       `json:"replicas,omitempty"`
	Config      MySQLConfig `json:"config,omitempty"`
	Namespace   string      `json:"namespace,omitempty"`
	Name        string      `json:"name,omitempty"`
	Telemetry   Telemetry   `json:"telemetry,omitempty"`
}

type MysqlConnection struct {
	// required
	Host     ValueOrSecret `json:"host,omitempty"`
	Port     ValueOrSecret `json:"port,omitempty"`
	Database ValueOrSecret `json:"database,omitempty"`
	Username ValueOrSecret `json:"username,omitempty"`
	Password ValueOrSecret `json:"password,omitempty" masq:"secret"`

	// optional
	Tls     ValueOrSecret `json:"tls,omitempty"`
	SslCa   ValueOrSecret `json:"sslCa,omitempty"`
	SslCert ValueOrSecret `json:"sslCert,omitempty"`
	SslKey  ValueOrSecret `json:"sslKey,omitempty" masq:"secret"`

	// URL is the operator-assembled DSN; it embeds the password.
	URL ValueOrSecret `json:"url,omitempty" masq:"secret"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (c *MysqlConnection) Normalize() {
	if c == nil {
		return
	}
	c.Host.Normalize()
	c.Port.Normalize()
	c.Database.Normalize()
	c.Username.Normalize()
	c.Password.Normalize()
	c.Tls.Normalize()
	c.SslCa.Normalize()
	c.SslCert.Normalize()
	c.SslKey.Normalize()
	c.URL.Normalize()
}

type MySQLConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// Telemetry defines telemetry configuration for infrastructure components
type Telemetry struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled" default:"true"`
}

// RedisSpec defines the desired state of the Redis infrastructure component.
type RedisSpec struct {
	ManagedRedis  *ManagedRedisSpec `json:"managedRedis,omitempty"`
	ExternalRedis *RedisConnection  `json:"externalRedis,omitempty"`
}

type ManagedRedisSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize string            `json:"storageSize,omitempty"`
	Config      RedisConfig       `json:"config,omitempty"`
	Sentinel    RedisSentinelSpec `json:"sentinel,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Name        string            `json:"name,omitempty"`
	Telemetry   Telemetry         `json:"telemetry,omitempty"`
}

type RedisConnection struct {
	Host     ValueOrSecret `json:"host,omitempty"`
	Port     ValueOrSecret `json:"port,omitempty"`
	Password ValueOrSecret `json:"password,omitempty" masq:"secret"`
	Tls      ValueOrSecret `json:"tls,omitempty"`
	SslCa    ValueOrSecret `json:"sslCa,omitempty"`

	// URL is the operator-assembled URL; it may embed the password.
	URL ValueOrSecret `json:"url,omitempty" masq:"secret"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (c *RedisConnection) Normalize() {
	if c == nil {
		return
	}
	c.Host.Normalize()
	c.Port.Normalize()
	c.Password.Normalize()
	c.Tls.Normalize()
	c.SslCa.Normalize()
	c.URL.Normalize()
}

type RedisConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type RedisSentinelSpec struct {
	Enabled bool                `json:"enabled"`
	Config  RedisSentinelConfig `json:"config,omitempty"`
}

type RedisSentinelConfig struct {
	MasterName string                      `json:"masterName,omitempty"`
	Resources  corev1.ResourceRequirements `json:"resources,omitempty"`
}

// KafkaSpec defines the desired state of the Kafka infrastructure component.
// Kafka is managed-only (backed by Bufstream); there is no external Kafka option.
type KafkaSpec struct {
	ManagedKafka *ManagedKafkaSpec `json:"managedKafka,omitempty"`
}

type ManagedKafkaSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize string      `json:"storageSize,omitempty"`
	Replicas    int32       `json:"replicas,omitempty"`
	Config      KafkaConfig `json:"config,omitempty"`
	Namespace   string      `json:"namespace,omitempty"`
	Name        string      `json:"name,omitempty"`
	Telemetry   Telemetry   `json:"telemetry,omitempty"`
	// ServiceAccount configures the identity used by the Bufstream broker.
	ServiceAccount   ManagedServiceAccountSpec `json:"serviceAccount,omitempty"`
	SkipDataRecovery bool                      `json:"skipDataRecovery,omitempty"`
}

type KafkaConnection struct {
	Host           ValueOrSecret `json:"host,omitempty"`
	Port           ValueOrSecret `json:"port,omitempty"`
	BrokerEndpoint ValueOrSecret `json:"brokerEndpoint,omitempty"`
	ClusterID      ValueOrSecret `json:"clusterID,omitempty"`

	// URL is the operator-assembled connection URL.
	URL ValueOrSecret `json:"url,omitempty" masq:"secret"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (c *KafkaConnection) Normalize() {
	if c == nil {
		return
	}
	c.Host.Normalize()
	c.Port.Normalize()
	c.BrokerEndpoint.Normalize()
	c.ClusterID.Normalize()
	c.URL.Normalize()
}

type KafkaConfig struct {
	Resources         corev1.ResourceRequirements `json:"resources,omitempty"`
	ReplicationConfig KafkaReplicationConfig      `json:"replicationConfig,omitempty"`
}

type KafkaReplicationConfig struct {
	DefaultReplicationFactor int32 `json:"defaultReplicationFactor,omitempty"`
	MinInSyncReplicas        int32 `json:"minInSyncReplicas,omitempty"`
	OffsetsTopicRF           int32 `json:"offsetsTopicRF,omitempty"`
	TransactionStateRF       int32 `json:"transactionStateISR,omitempty"`
	TransactionStateISR      int32 `json:"transactionStateRF,omitempty"`
}

// ObjectStoreSpec defines the desired state of the object store infrastructure component.
type ObjectStoreSpec struct {
	// +kubebuilder:default=false
	BucketAttributionDisabled bool                    `json:"bucketAttributionDisabled,omitempty"`
	ManagedObjectStore        *ManagedObjectStoreSpec `json:"managedObjectStore,omitempty"`
	ExternalObjectStore       *ObjectStoreConnection  `json:"externalObjectStore,omitempty"`
}

type ManagedObjectStoreSpec struct {
	ManagedInfraSpec       `json:",inline"`
	SeaweedObjectStoreSpec SeaweedObjectStoreSpec `json:"SeaweedObjectStoreSpec,omitempty"`
	StorageSize            string                 `json:"storageSize,omitempty"`
	Replicas               int32                  `json:"replicas,omitempty"`
	Copies                 int32                  `json:"copies,omitempty"`
	Config                 ObjectStoreConfig      `json:"config,omitempty"`
	Namespace              string                 `json:"namespace,omitempty"`
	Name                   string                 `json:"name,omitempty"`
	Telemetry              Telemetry              `json:"telemetry,omitempty"`
}

type SeaweedObjectStoreSpec struct {
	TlsEnabled bool `json:"tlsEnabled,omitempty"`
	// FilerStorageSize sizes the filer's metadata index disk. It grows with the
	// number of objects, not their total size, so bump it for large object counts.
	// Defaults to 20Gi when unset.
	FilerStorageSize string `json:"filerStorageSize,omitempty"`
}

// ObjectStoreProvider selects the object store backend for an external object store.
type ObjectStoreProvider string

const (
	ObjectStoreProviderS3    ObjectStoreProvider = "s3"
	ObjectStoreProviderGCS   ObjectStoreProvider = "gcs"
	ObjectStoreProviderAzure ObjectStoreProvider = "azure"
)

type ObjectStoreConnection struct {
	// Provider selects the externalObjectStore backend (s3, gcs, or azure); defaults to s3 when absent.
	Provider ValueOrSecret `json:"provider,omitempty"`

	Endpoint  ValueOrSecret `json:"endpoint,omitempty"`
	Port      ValueOrSecret `json:"port,omitempty"`
	AccessKey ValueOrSecret `json:"accessKey,omitempty" masq:"secret"`
	SecretKey ValueOrSecret `json:"secretKey,omitempty" masq:"secret"`
	Bucket    ValueOrSecret `json:"bucket,omitempty"`
	// Path is an optional key prefix within the bucket under which W&B stores its data.
	Path           ValueOrSecret `json:"path,omitempty"`
	Region         ValueOrSecret `json:"region,omitempty"`
	TlsEnabled     ValueOrSecret `json:"tlsEnabled,omitempty"`
	ForcePathStyle ValueOrSecret `json:"forcePathStyle,omitempty"`
	// URL is the operator-assembled connection URL; it embeds credentials as userinfo.
	URL ValueOrSecret `json:"url,omitempty" masq:"secret"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (c *ObjectStoreConnection) Normalize() {
	if c == nil {
		return
	}
	c.Provider.Normalize()
	c.Endpoint.Normalize()
	c.Port.Normalize()
	c.AccessKey.Normalize()
	c.SecretKey.Normalize()
	c.Bucket.Normalize()
	c.Path.Normalize()
	c.Region.Normalize()
	c.TlsEnabled.Normalize()
	c.ForcePathStyle.Normalize()
	c.URL.Normalize()
}

type ObjectStoreConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	AccessKey string                      `json:"accessKey,omitempty"`

	// Deprecated: Use AccessKey instead. Kept for backward compatibility during migration.
	RootUser string `json:"rootUser,omitempty"`
	// Deprecated: No longer used. Kept to avoid schema validation failures on upgrade.
	MinioBrowserSetting string `json:"minioBrowserSetting,omitempty"`
}

// ClickHouseSpec defines the desired state of the ClickHouse infrastructure component.
type ClickHouseSpec struct {
	ManagedClickHouse  *ManagedClickHouseSpec `json:"managedClickhouse,omitempty"`
	ExternalClickHouse *ClickHouseConnection  `json:"externalClickhouse,omitempty"`
}

type ManagedClickHouseSpec struct {
	ManagedInfraSpec `json:",inline"`

	StorageSize string           `json:"storageSize,omitempty"`
	Replicas    int32            `json:"replicas,omitempty"`
	Version     string           `json:"version,omitempty"`
	Config      ClickHouseConfig `json:"config,omitempty"`
	Namespace   string           `json:"namespace,omitempty"`
	Name        string           `json:"name,omitempty"`
	Telemetry   Telemetry        `json:"telemetry,omitempty"`
	// ServiceAccount configures the identity used by ClickHouse server pods.
	ServiceAccount ManagedServiceAccountSpec `json:"serviceAccount,omitempty"`

	// ObjectStorage configures the S3-backed disk that holds ClickHouse table
	// data in the configured W&B object store (managed SeaweedFS or external
	// bucket). Managed ClickHouse always stores table data in object storage;
	// StorageSize sizes the local PV used only for metadata, system tables, and
	// the S3 read cache.
	ObjectStorage ClickHouseObjectStorageSpec `json:"objectStorage,omitempty"`

	// Keeper configures the ClickHouse Keeper ensemble that coordinates
	// ReplicatedMergeTree replication across ClickHouse replicas.
	Keeper ClickHouseKeeperSpec `json:"keeper,omitempty"`
}

// ClickHouseObjectStorageSpec configures object-store-backed storage for managed
// ClickHouse.
type ClickHouseObjectStorageSpec struct {
	// Prefix is the key prefix within the bucket under which ClickHouse stores
	// its data. Lets multiple consumers share a single bucket. Defaults to
	// "clickhouse/".
	Prefix string `json:"prefix,omitempty"`

	// Insecure connects to the object store over HTTP instead of HTTPS. It only
	// applies to external object stores that do not advertise a scheme; the
	// managed object store's scheme is taken from its connection. Defaults to
	// false (HTTPS).
	Insecure bool `json:"insecure,omitempty"`
}

// ClickHouseKeeperSpec configures the managed ClickHouse Keeper ensemble.
type ClickHouseKeeperSpec struct {
	// Replicas is the number of Keeper nodes. Use an odd number (1, 3, 5) so the
	// ensemble can form a quorum. Defaults to 3.
	Replicas int32 `json:"replicas,omitempty"`

	// StorageSize is the persistent volume size for each Keeper node's raft log
	// and snapshots. Keeper state is small; defaults to a modest value.
	StorageSize string `json:"storageSize,omitempty"`

	// Config holds resource requirements for the Keeper pods.
	Config ClickHouseConfig `json:"config,omitempty"`
}

type ClickHouseConnection struct {
	Host     ValueOrSecret `json:"host,omitempty"`
	TCPPort  ValueOrSecret `json:"tcpPort,omitempty"`
	HTTPPort ValueOrSecret `json:"httpPort,omitempty"`
	Database ValueOrSecret `json:"database,omitempty"`
	Username ValueOrSecret `json:"username,omitempty"`
	Password ValueOrSecret `json:"password,omitempty" masq:"secret"`

	// URL is the operator-assembled URL; it may embed the password.
	URL ValueOrSecret `json:"url,omitempty" masq:"secret"`

	// Replicated tells applications whether to create ReplicatedMergeTree tables.
	Replicated ValueOrSecret `json:"replicated,omitempty"`

	// CLUSTER. Only meaningful when Replicated is true.
	ClusterName ValueOrSecret `json:"clusterName,omitempty"`
}

// Normalize rewrites any legacy {name, key} field into the ValueFrom envelope.
func (c *ClickHouseConnection) Normalize() {
	if c == nil {
		return
	}
	c.Host.Normalize()
	c.TCPPort.Normalize()
	c.HTTPPort.Normalize()
	c.Database.Normalize()
	c.Username.Normalize()
	c.Password.Normalize()
	c.URL.Normalize()
	c.Replicated.Normalize()
	c.ClusterName.Normalize()
}

type ClickHouseConfig struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WeightsAndBiasesStatus defines the observed state of WeightsAndBiases.
type WeightsAndBiasesStatus struct {
	Ready bool `json:"ready"`
	// Conditions includes the standard Ready condition for the current generation.
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Wandb      WandbStatus        `json:"wandb,omitempty"`
	// MySQLStatus, RedisStatus, ObjectStoreStatus and ClickHouseStatus are keyed
	// by instance name, mirroring the corresponding spec maps.
	MySQLStatus       map[string]MysqlInfraStatus       `json:"mysqlStatus,omitempty"`
	RedisStatus       map[string]RedisInfraStatus       `json:"redisStatus,omitempty"`
	KafkaStatus       KafkaInfraStatus                  `json:"kafkaStatus,omitempty"`
	ObjectStoreStatus map[string]ObjectStoreInfraStatus `json:"objectStoreStatus,omitempty"`
	ClickHouseStatus  map[string]ClickHouseInfraStatus  `json:"clickhouseStatus,omitempty"`
	TelemetryStatus   TelemetryInfraStatus              `json:"telemetryStatus,omitempty"`
	EmailSink         *corev1.SecretKeySelector         `json:"emailSink,omitempty"`
	// GeneratedSecrets stores references to secrets generated by the operator
	// from the server manifest's generatedSecrets section. The key is the
	// logical secret name from the manifest, and the value is a SecretKeySelector
	// referencing the concrete Secret and key that holds the generated value.
	GeneratedSecrets   map[string]corev1.SecretKeySelector `json:"generatedSecrets,omitempty"`
	ObservedGeneration int64                               `json:"observedGeneration"`

	// +optional
	GatewayStatus *GatewayStatusSummary `json:"gatewayStatus,omitempty"`
	// +optional
	IngressStatus    *IngressStatusSummary    `json:"ingressStatus,omitempty"`
	WatchtowerStatus *WatchtowerStatusSummary `json:"watchtowerStatus,omitempty"`
}

type WatchtowerStatusSummary struct {
	Ready       bool   `json:"ready"`
	URL         string `json:"url,omitempty"`
	Image       string `json:"image,omitempty"`
	AuthService string `json:"authService,omitempty"`
}

type GatewayStatusSummary struct {
	Name       string            `json:"name,omitempty"`
	Ready      bool              `json:"ready,omitempty"`
	Addresses  []string          `json:"addresses,omitempty"`
	GatewayRef *GatewayReference `json:"gatewayRef,omitempty"`
}

type IngressStatusSummary struct {
	Name                string                       `json:"name,omitempty"`
	LoadBalancerIngress []corev1.LoadBalancerIngress `json:"loadBalancerIngress,omitempty"`
	Ready               bool                         `json:"ready"`
}

type WandbStatus struct {
	Hostname string `json:"hostname"`

	// +kubebuilder:default:={}
	Applications map[string]ApplicationStatus `json:"applications,omitempty"`

	Migration WandbMigrationStatus `json:"migration,omitempty"`

	// MySQLInit tracks the per-instance database-initialization job, keyed by
	// managed MySQL instance name.
	// +kubebuilder:default:={}
	MySQLInit map[string]MigrationJobStatus `json:"mysqlInit,omitempty"`
}

type WandbMigrationStatus struct {
	Version            string `json:"version,omitempty"`
	LastSuccessVersion string `json:"lastSuccessVersion,omitempty"`
	Ready              bool   `json:"ready,omitempty"`
	// Phase is Running, Failed, Succeeded, or Unknown.
	Phase  string                        `json:"phase,omitempty"`
	Reason string                        `json:"reason,omitempty"`
	Jobs   map[string]MigrationJobStatus `json:"jobs,omitempty"`
}

type MigrationJobStatus struct {
	Name      string `json:"name,omitempty"`
	Succeeded bool   `json:"succeeded,omitempty"`
	Failed    bool   `json:"failed,omitempty"`
	// Phase is Running, Failed, Succeeded, or Unknown.
	Phase string `json:"phase,omitempty"`
	// Reason is copied from the terminal Kubernetes Job condition when present.
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type WBInfraStatus struct {
	Ready      bool               `json:"ready"`
	State      string             `json:"state,omitempty" default:"Unknown"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type MysqlInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    MysqlConnection `json:"connection,omitempty"`
}

type RedisInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    RedisConnection `json:"connection,omitempty"`
}

type KafkaInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    KafkaConnection `json:"connection,omitempty"`
}

type ObjectStoreInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    ObjectStoreConnection `json:"connection,omitempty"`
}

type ClickHouseInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Connection    ClickHouseConnection `json:"connection,omitempty"`
}

type TelemetryInfraStatus struct {
	WBInfraStatus `json:",inline"`
	Mode          string                    `json:"mode,omitempty"`
	Connection    TelemetryConnectionStatus `json:"connection,omitempty"`
}

type TelemetryConnectionStatus struct {
	ManagedNamespace      string `json:"managedNamespace,omitempty"`
	ConnectionSecret      string `json:"connectionSecret,omitempty"`
	Protocol              string `json:"protocol,omitempty"`
	MetricsExporter       string `json:"metricsExporter,omitempty"`
	LogsExporter          string `json:"logsExporter,omitempty"`
	TracesExporter        string `json:"tracesExporter,omitempty"`
	MetricsEndpoint       string `json:"metricsEndpoint,omitempty"`
	LogsEndpoint          string `json:"logsEndpoint,omitempty"`
	TracesEndpoint        string `json:"tracesEndpoint,omitempty"`
	ServiceName           string `json:"serviceName,omitempty"`
	ResourceAttributes    string `json:"resourceAttributes,omitempty"`
	GorillaTracer         string `json:"gorillaTracer,omitempty"`
	StatsdAddress         string `json:"statsdAddress,omitempty"`
	DatadogTraceAgentURL  string `json:"datadogTraceAgentURL,omitempty"`
	DatadogTraceAgentHost string `json:"datadogTraceAgentHost,omitempty"`
	DatadogTraceAgentPort string `json:"datadogTraceAgentPort,omitempty"`
}
