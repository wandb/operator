package strimzi

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/wandb/operator/internal/controller/translator/common"
	kafkav1beta2 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKafkaConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kafka Strimzi Conditions Suite")
}

var _ = Describe("Kafka Strimzi Conditions", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(kafkav1beta2.AddToScheme(scheme)).To(Succeed())
	})

	Describe("GetConditions", func() {
		It("should create connection condition when both Kafka and NodePool exist", func() {
			specName := "test-kafka"
			specNamespace := "default"
			namespacedName := types.NamespacedName{
				Name:      specName,
				Namespace: specNamespace,
			}

			kafka := &kafkav1beta2.Kafka{
				ObjectMeta: metav1.ObjectMeta{
					Name:      KafkaName(specName),
					Namespace: specNamespace,
				},
			}

			nodePool := &kafkav1beta2.KafkaNodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodePoolName(specName),
					Namespace: specNamespace,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(kafka, nodePool).
				Build()

			result, err := GetConditions(ctx, fakeClient, namespacedName)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Code()).To(Equal(string(common.KafkaConnectionCode)))

			status := common.ExtractKafkaStatus(ctx, result)
			Expect(status.Connection.Host).To(Equal(fmt.Sprintf("%s.%s.svc.cluster.local", KafkaName(specName), specNamespace)))
			Expect(status.Connection.Port).To(Equal(strconv.Itoa(PlainListenerPort)))
			Expect(status.Ready).To(BeTrue())
		})

		It("should still create condition when Kafka doesn't exist", func() {
			specName := "missing-kafka"
			specNamespace := "default"
			namespacedName := types.NamespacedName{
				Name:      specName,
				Namespace: specNamespace,
			}

			nodePool := &kafkav1beta2.KafkaNodePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      NodePoolName(specName),
					Namespace: specNamespace,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(nodePool).
				Build()

			result, err := GetConditions(ctx, fakeClient, namespacedName)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
		})

		It("should still create condition when NodePool doesn't exist", func() {
			specName := "test-kafka"
			specNamespace := "default"
			namespacedName := types.NamespacedName{
				Name:      specName,
				Namespace: specNamespace,
			}

			kafka := &kafkav1beta2.Kafka{
				ObjectMeta: metav1.ObjectMeta{
					Name:      KafkaName(specName),
					Namespace: specNamespace,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(kafka).
				Build()

			result, err := GetConditions(ctx, fakeClient, namespacedName)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
		})

		Context("connection string formatting", func() {
			It("should format host using KafkaName and namespace", func() {
				namespacedName := types.NamespacedName{
					Name:      "my-kafka",
					Namespace: "my-namespace",
				}

				kafka := &kafkav1beta2.Kafka{
					ObjectMeta: metav1.ObjectMeta{
						Name:      KafkaName("my-kafka"),
						Namespace: "my-namespace",
					},
				}

				nodePool := &kafkav1beta2.KafkaNodePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      NodePoolName("my-kafka"),
						Namespace: "my-namespace",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(kafka, nodePool).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractKafkaStatus(ctx, result)
				Expect(status.Connection.Host).To(Equal("my-kafka.my-namespace.svc.cluster.local"))
			})

			It("should use PlainListenerPort for port", func() {
				namespacedName := types.NamespacedName{
					Name:      "test-kafka",
					Namespace: "test-ns",
				}

				kafka := &kafkav1beta2.Kafka{
					ObjectMeta: metav1.ObjectMeta{
						Name:      KafkaName("test-kafka"),
						Namespace: "test-ns",
					},
				}

				nodePool := &kafkav1beta2.KafkaNodePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      NodePoolName("test-kafka"),
						Namespace: "test-ns",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(kafka, nodePool).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractKafkaStatus(ctx, result)
				Expect(status.Connection.Port).To(Equal(strconv.Itoa(PlainListenerPort)))
				Expect(status.Connection.Port).To(Equal("9092"))
			})
		})
	})
})
