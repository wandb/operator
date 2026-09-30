package v2

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/randfill"
)

// These flat types preserve the API layout before the base-type extraction.
// Do not embed the new bases here: the independent layout catches accidental
// nesting, shadowed fields, and changes to JSON tags or optionality.
type flatDeploymentSpec struct {
	Size                Size                       `json:"size,omitempty"`
	RequireLimits       bool                       `json:"requireLimits,omitempty"`
	RetentionPolicy     RetentionPolicy            `json:"retentionPolicy"`
	Global              GlobalSpec                 `json:"global,omitempty"`
	Wandb               flatWorkloadSpec           `json:"wandb,omitempty"`
	Affinity            *corev1.Affinity           `json:"affinity,omitempty"`
	Tolerations         *[]corev1.Toleration       `json:"tolerations,omitempty"`
	MySQL               map[string]MySQLSpec       `json:"mysql,omitempty"`
	Redis               map[string]RedisSpec       `json:"redis,omitempty"`
	Kafka               KafkaSpec                  `json:"kafka,omitempty"`
	ObjectStore         map[string]ObjectStoreSpec `json:"objectStore,omitempty"`
	ClickHouse          map[string]ClickHouseSpec  `json:"clickhouse,omitempty"`
	Networking          NetworkingSpec             `json:"networking,omitempty"`
	AdminConsoleEnabled *bool                      `json:"adminConsoleEnabled,omitempty"`
}

type flatDeploymentStatus struct {
	Ready              bool                                `json:"ready"`
	Conditions         []metav1.Condition                  `json:"conditions,omitempty"`
	Wandb              flatWorkloadStatus                  `json:"wandb,omitempty"`
	MySQLStatus        map[string]MysqlInfraStatus         `json:"mysqlStatus,omitempty"`
	RedisStatus        map[string]RedisInfraStatus         `json:"redisStatus,omitempty"`
	KafkaStatus        KafkaInfraStatus                    `json:"kafkaStatus,omitempty"`
	ObjectStoreStatus  map[string]ObjectStoreInfraStatus   `json:"objectStoreStatus,omitempty"`
	ClickHouseStatus   map[string]ClickHouseInfraStatus    `json:"clickhouseStatus,omitempty"`
	TelemetryStatus    TelemetryInfraStatus                `json:"telemetryStatus,omitempty"`
	EmailSink          *corev1.SecretKeySelector           `json:"emailSink,omitempty"`
	GeneratedSecrets   map[string]corev1.SecretKeySelector `json:"generatedSecrets,omitempty"`
	ObservedGeneration int64                               `json:"observedGeneration"`
	GatewayStatus      *GatewayStatusSummary               `json:"gatewayStatus,omitempty"`
	IngressStatus      *IngressStatusSummary               `json:"ingressStatus,omitempty"`
	WatchtowerStatus   *WatchtowerStatusSummary            `json:"watchtowerStatus,omitempty"`
}

type flatWorkloadSpec struct {
	Hostname            string                              `json:"hostname"`
	License             string                              `json:"license,omitempty"`
	ManifestRepository  string                              `json:"manifestRepository,omitempty"`
	Version             string                              `json:"version"`
	Features            map[string]bool                     `json:"features"`
	InternalServiceAuth InternalServiceAuth                 `json:"internalServiceAuth,omitempty"`
	BucketProxy         bool                                `json:"bucketProxy"`
	ServiceAccount      ServiceAccountSpec                  `json:"serviceAccount,omitempty"`
	Probes              WandbProbeDefaults                  `json:"probes,omitempty"`
	AdditionalHostnames []string                            `json:"additionalHostnames,omitempty"`
	OIDC                OidcSpec                            `json:"oidc,omitempty"`
	Notifications       *NotificationsSpec                  `json:"notifications,omitempty"`
	Security            SecuritySpec                        `json:"security,omitempty"`
	Retention           *RetentionSpec                      `json:"retention,omitempty"`
	LegacyOverrides     map[string]LegacyOverrides          `json:"legacyOverrides,omitempty"`
	Applications        map[string]WandbApplicationOverride `json:"applications,omitempty"`
}

type flatWorkloadStatus struct {
	Hostname     string                        `json:"hostname"`
	Applications map[string]ApplicationStatus  `json:"applications,omitempty"`
	Migration    WandbMigrationStatus          `json:"migration,omitempty"`
	MySQLInit    map[string]MigrationJobStatus `json:"mysqlInit,omitempty"`
}

type flatWeightsAndBiases struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              flatDeploymentSpec   `json:"spec,omitempty"`
	Status            flatDeploymentStatus `json:"status,omitempty"`
}

func canonicalJSON(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	var result any
	require.NoError(t, json.Unmarshal(raw, &result))
	return result
}

func TestBaseTypesPreserveWireContract(t *testing.T) {
	filler := randfill.New().RandSource(rand.NewSource(42)).NilChance(.3).NumElements(0, 2).MaxDepth(14).Funcs(
		func(x *metav1.FieldsV1, _ randfill.Continue) { *x = metav1.FieldsV1{} },
		func(x *runtime.RawExtension, _ randfill.Continue) { *x = runtime.RawExtension{Raw: []byte(`{}`)} },
		func(x *json.RawMessage, _ randfill.Continue) { *x = json.RawMessage(`{}`) },
		func(x *metav1.Time, c randfill.Continue) {
			*x = metav1.NewTime(time.Unix(int64(c.Intn(2000000000)), 0).UTC())
		},
		func(x *metav1.MicroTime, c randfill.Continue) {
			*x = metav1.NewMicroTime(time.Unix(int64(c.Intn(2000000000)), 0).UTC())
		},
		func(x *resource.Quantity, c randfill.Continue) { *x = resource.MustParse(fmt.Sprint(c.Intn(1000))) },
		func(x *intstr.IntOrString, c randfill.Continue) { *x = intstr.FromInt(c.Intn(1000)) },
	)
	// Include the zero value, then exercise maps, slices, pointers and nested
	// workload statuses with deterministic generated values.
	for i := 0; i <= 200; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var original flatWeightsAndBiases
			if i > 0 {
				filler.Fill(&original.Spec)
				filler.Fill(&original.Status)
			}
			raw, err := json.Marshal(&original)
			require.NoError(t, err)

			// Decode both layouts identically. A pointer to a nil slice already
			// normalizes from a JSON null to a nil pointer in the original API.
			var expected flatWeightsAndBiases
			var actual WeightsAndBiases
			require.NoError(t, json.Unmarshal(raw, &expected))
			require.NoError(t, json.Unmarshal(raw, &actual))
			require.True(t, reflect.DeepEqual(canonicalJSON(t, &expected), canonicalJSON(t, &actual)), "JSON layout differs")

			expectedMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&expected)
			require.NoError(t, err)
			actualMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&actual)
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(expectedMap, actualMap), "Kubernetes unstructured layout differs")
			var restored WeightsAndBiases
			require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(actualMap, &restored))
			require.True(t, reflect.DeepEqual(canonicalJSON(t, &actual), canonicalJSON(t, &restored)))
			require.Equal(t, &actual, actual.DeepCopy())
		})
	}
}

func TestBaseTypesDeepCopyIsolation(t *testing.T) {
	var original WeightsAndBiases
	require.NoError(t, json.Unmarshal([]byte(`{
        "spec": {
            "global": {"customCACerts": ["original"]},
            "tolerations": [{"key": "original"}],
            "mysql": {"default": {"managedMysql": {"storageSize": "10Gi"}}},
            "wandb": {
                "features": {"original": true},
                "probes": {"readinessProbe": {"periodSeconds": 10}},
                "applications": {"api": {"autoscaling": {"minReplicas": 1}}}
            }
        },
        "status": {
            "conditions": [{"type": "Ready", "message": "original"}],
            "generatedSecrets": {"auth": {"name": "original", "key": "secret"}},
            "wandb": {"applications": {"api": {"deploymentStatus": {"replicas": 1}}}}
        }
    }`), &original))
	expected := canonicalJSON(t, &original)
	clone := original.DeepCopy()
	clone.Spec.Global.CustomCACerts[0] = "changed"
	(*clone.Spec.Tolerations)[0].Key = "changed"
	clone.Spec.MySQL["default"].ManagedMysql.StorageSize = "20Gi"
	clone.Spec.Wandb.Features["original"] = false
	clone.Spec.Wandb.Probes.ReadinessProbe.PeriodSeconds = 20
	*clone.Spec.Wandb.Applications["api"].Autoscaling.MinReplicas = 2
	clone.Status.Conditions[0].Message = "changed"
	delete(clone.Status.GeneratedSecrets, "auth")
	clone.Status.Wandb.Applications["api"].DeploymentStatus.Replicas = 2
	require.Equal(t, expected, canonicalJSON(t, &original))
}
