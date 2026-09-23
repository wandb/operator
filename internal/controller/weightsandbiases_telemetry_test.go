package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// These go through YAML rather than the typed client on purpose. A Go struct always
// serialises `telemetry.enabled: false`, which hides the CRD default we are asserting.
var _ = Describe("Managed telemetry defaulting through admission", func() {
	const namespace = "default"

	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	applyYAML := func(name, infra string) *apiv2.WeightsAndBiases {
		doc := fmt.Sprintf(`
apiVersion: apps.wandb.com/v2
kind: WeightsAndBiases
metadata:
  name: %s
  namespace: %s
spec:
  wandb:
    hostname: http://localhost
    manifestRepository: %s
    version: 0.83.0-clickhouse-keeper.2
%s`, name, namespace, manifestsRepository, infra)

		obj := &unstructured.Unstructured{}
		Expect(yaml.Unmarshal([]byte(doc), &obj.Object)).To(Succeed())
		Expect(k8sClient.Create(ctx, obj)).Should(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, obj)).Should(Succeed())
		})

		admitted := &apiv2.WeightsAndBiases{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, admitted)).Should(Succeed())
		return admitted
	}

	expectAllManaged := func(admitted *apiv2.WeightsAndBiases, enabled bool) {
		Expect(admitted.Spec.MySQL[apiv2.DefaultInstanceName].ManagedMysql.Telemetry.Enabled).To(Equal(enabled))
		Expect(admitted.Spec.Redis[apiv2.DefaultInstanceName].ManagedRedis.Telemetry.Enabled).To(Equal(enabled))
		Expect(admitted.Spec.Kafka.ManagedKafka.Telemetry.Enabled).To(Equal(enabled))
		Expect(admitted.Spec.ObjectStore[apiv2.DefaultInstanceName].ManagedObjectStore.Telemetry.Enabled).To(Equal(enabled))
		Expect(admitted.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Telemetry.Enabled).To(Equal(enabled))
	}

	It("enables telemetry when no infra components are declared", func() {
		expectAllManaged(applyYAML("telemetry-absent-blocks", ""), true)
	})

	// The regression: a declared managed block with no telemetry key used to land as false.
	It("enables telemetry when a managed block omits the telemetry key", func() {
		admitted := applyYAML("telemetry-absent-key", `  clickhouse:
    default:
      managedClickhouse:
        storageSize: 10Gi
`)
		expectAllManaged(admitted, true)
	})

	It("keeps telemetry disabled when the user sets it to false", func() {
		admitted := applyYAML("telemetry-explicit-false", `  clickhouse:
    default:
      managedClickhouse:
        telemetry:
          enabled: false
`)
		Expect(admitted.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse.Telemetry.Enabled).To(BeFalse())
		Expect(admitted.Spec.MySQL[apiv2.DefaultInstanceName].ManagedMysql.Telemetry.Enabled).To(BeTrue())
	})

	It("does not create a managed block for external infra", func() {
		admitted := applyYAML("telemetry-external", `  clickhouse:
    default:
      externalClickhouse:
        host:
          value: clickhouse.example.com
`)
		Expect(admitted.Spec.ClickHouse[apiv2.DefaultInstanceName].ManagedClickHouse).To(BeNil())
	})
})
