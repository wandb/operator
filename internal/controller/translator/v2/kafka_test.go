package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/kafka/strimzi"
	"github.com/wandb/operator/internal/controller/translator/common"
	kafkav1beta2 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Kafka Translator", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("TranslateKafkaStatus", func() {
		type statusFixture struct {
			name               string
			input              common.KafkaStatus
			expectedReady      bool
			expectedState      apiv2.WBStateType
			expectedHost       string
			expectedPort       string
			expectedConditions int
		}

		DescribeTable("status translation scenarios",
			func(fixture statusFixture) {
				result := TranslateKafkaStatus(ctx, fixture.input)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.State).To(Equal(fixture.expectedState))
				Expect(result.Connection.KafkaHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.KafkaPort).To(Equal(fixture.expectedPort))
				Expect(result.Conditions).To(HaveLen(fixture.expectedConditions))
				Expect(result.LastReconciled).ToNot(BeZero())
			},
			Entry("ready status with connection", statusFixture{
				name: "ready with connection",
				input: common.KafkaStatus{
					Ready: true,
					Connection: common.KafkaConnection{
						Host: "kafka.example.com",
						Port: "9092",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaConnectionCode, "Connected"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateReady,
				expectedHost:       "kafka.example.com",
				expectedPort:       "9092",
				expectedConditions: 1,
			}),
			Entry("not ready status - creating", statusFixture{
				name: "creating",
				input: common.KafkaStatus{
					Ready: false,
					Connection: common.KafkaConnection{
						Host: "",
						Port: "",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaCreatedCode, "Creating Kafka cluster"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "",
				expectedPort:       "",
				expectedConditions: 1,
			}),
			Entry("updating status", statusFixture{
				name: "updating",
				input: common.KafkaStatus{
					Ready: true,
					Connection: common.KafkaConnection{
						Host: "kafka.example.com",
						Port: "9092",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaUpdatedCode, "Updating configuration"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "kafka.example.com",
				expectedPort:       "9092",
				expectedConditions: 1,
			}),
			Entry("deleting status", statusFixture{
				name: "deleting",
				input: common.KafkaStatus{
					Ready: false,
					Connection: common.KafkaConnection{
						Host: "kafka.example.com",
						Port: "9092",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaDeletedCode, "Deleting resources"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateDeleting,
				expectedHost:       "kafka.example.com",
				expectedPort:       "9092",
				expectedConditions: 1,
			}),
			Entry("node pool created status", statusFixture{
				name: "node pool created",
				input: common.KafkaStatus{
					Ready: false,
					Connection: common.KafkaConnection{
						Host: "",
						Port: "",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaNodePoolCreatedCode, "Node pool created"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "",
				expectedPort:       "",
				expectedConditions: 1,
			}),
			Entry("node pool updated status", statusFixture{
				name: "node pool updated",
				input: common.KafkaStatus{
					Ready: true,
					Connection: common.KafkaConnection{
						Host: "kafka.example.com",
						Port: "9092",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaNodePoolUpdatedCode, "Node pool updated"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "kafka.example.com",
				expectedPort:       "9092",
				expectedConditions: 1,
			}),
			Entry("node pool deleted status", statusFixture{
				name: "node pool deleted",
				input: common.KafkaStatus{
					Ready: false,
					Connection: common.KafkaConnection{
						Host: "",
						Port: "",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaNodePoolDeletedCode, "Node pool deleted"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateDeleting,
				expectedHost:       "",
				expectedPort:       "",
				expectedConditions: 1,
			}),
			Entry("multiple conditions with worst state taking precedence", statusFixture{
				name: "multiple conditions",
				input: common.KafkaStatus{
					Ready: true,
					Connection: common.KafkaConnection{
						Host: "kafka.example.com",
						Port: "9092",
					},
					Conditions: []common.KafkaCondition{
						common.NewKafkaCondition(common.KafkaCreatedCode, "Created"),
						common.NewKafkaCondition(common.KafkaConnectionCode, "Connected"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "kafka.example.com",
				expectedPort:       "9092",
				expectedConditions: 2,
			}),
			Entry("empty status", statusFixture{
				name: "empty status",
				input: common.KafkaStatus{
					Ready:      false,
					Connection: common.KafkaConnection{},
					Conditions: []common.KafkaCondition{},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateUnknown,
				expectedHost:       "",
				expectedPort:       "",
				expectedConditions: 0,
			}),
		)
	})

	Describe("ExtractKafkaStatus", func() {
		type extractFixture struct {
			name          string
			conditions    []common.KafkaCondition
			expectedReady bool
			expectedHost  string
			expectedPort  string
		}

		DescribeTable("condition extraction scenarios",
			func(fixture extractFixture) {
				result := ExtractKafkaStatus(ctx, fixture.conditions)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.Connection.KafkaHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.KafkaPort).To(Equal(fixture.expectedPort))
			},
			Entry("connection info available", extractFixture{
				name: "with connection info",
				conditions: []common.KafkaCondition{
					common.NewKafkaConnCondition(common.KafkaConnInfo{
						Host: "kafka-svc.default.svc.cluster.local",
						Port: "9092",
					}),
				},
				expectedReady: true,
				expectedHost:  "kafka-svc.default.svc.cluster.local",
				expectedPort:  "9092",
			}),
			Entry("no connection info", extractFixture{
				name: "no connection info",
				conditions: []common.KafkaCondition{
					common.NewKafkaCondition(common.KafkaCreatedCode, "Created"),
				},
				expectedReady: false,
				expectedHost:  "",
				expectedPort:  "",
			}),
			Entry("mixed conditions", extractFixture{
				name: "mixed conditions with connection",
				conditions: []common.KafkaCondition{
					common.NewKafkaCondition(common.KafkaCreatedCode, "Created"),
					common.NewKafkaConnCondition(common.KafkaConnInfo{
						Host: "kafka-host",
						Port: "9093",
					}),
					common.NewKafkaCondition(common.KafkaUpdatedCode, "Updated"),
				},
				expectedReady: true,
				expectedHost:  "kafka-host",
				expectedPort:  "9093",
			}),
			Entry("empty conditions", extractFixture{
				name:          "empty conditions",
				conditions:    []common.KafkaCondition{},
				expectedReady: false,
				expectedHost:  "",
				expectedPort:  "",
			}),
		)
	})

	Describe("ToKafkaVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(kafkav1beta2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		type kafkaSpecFixture struct {
			name                             string
			spec                             apiv2.WBKafkaSpec
			expectNil                        bool
			expectedName                     string
			expectedNamespace                string
			expectedReplicas                 int32
			expectedVersion                  string
			expectedMetadataVersion          string
			expectedListeners                int
			expectedOffsetsTopicRF           string
			expectedTransactionStateRF       string
			expectedTransactionStateISR      string
			expectedDefaultReplicationFactor string
			expectedMinInSyncReplicas        string
			expectedNodePoolAnnotation       bool
			expectedOwnerReferences          int
		}

		DescribeTable("kafka spec translation scenarios",
			func(fixture kafkaSpecFixture) {
				result, err := ToKafkaVendorSpec(ctx, fixture.spec, owner, testScheme)

				if fixture.expectNil {
					Expect(result).To(BeNil())
					Expect(err).ToNot(HaveOccurred())
					return
				}

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				Expect(result.Name).To(Equal(fixture.expectedName))
				Expect(result.Namespace).To(Equal(fixture.expectedNamespace))
				Expect(result.Labels).To(HaveKeyWithValue("app", strimzi.KafkaName(fixture.spec.Name)))

				if fixture.expectedNodePoolAnnotation {
					Expect(result.Annotations).To(HaveKeyWithValue("strimzi.io/node-pools", "enabled"))
				}

				Expect(result.Spec.Kafka.Version).To(Equal(fixture.expectedVersion))
				Expect(result.Spec.Kafka.MetadataVersion).To(Equal(fixture.expectedMetadataVersion))
				Expect(result.Spec.Kafka.Replicas).To(Equal(fixture.expectedReplicas))
				Expect(result.Spec.Kafka.Listeners).To(HaveLen(fixture.expectedListeners))

				if fixture.expectedListeners > 0 {
					Expect(result.Spec.Kafka.Listeners[0].Name).To(Equal(strimzi.PlainListenerName))
					Expect(result.Spec.Kafka.Listeners[0].Port).To(Equal(int32(strimzi.PlainListenerPort)))
					Expect(result.Spec.Kafka.Listeners[0].Type).To(Equal(strimzi.ListenerType))
					Expect(result.Spec.Kafka.Listeners[0].Tls).To(BeFalse())
				}

				if fixture.expectedListeners > 1 {
					Expect(result.Spec.Kafka.Listeners[1].Name).To(Equal(strimzi.TLSListenerName))
					Expect(result.Spec.Kafka.Listeners[1].Port).To(Equal(int32(strimzi.TLSListenerPort)))
					Expect(result.Spec.Kafka.Listeners[1].Type).To(Equal(strimzi.ListenerType))
					Expect(result.Spec.Kafka.Listeners[1].Tls).To(BeTrue())
				}

				Expect(result.Spec.Kafka.Config).ToNot(BeNil())
				Expect(result.Spec.Kafka.Config["offsets.topic.replication.factor"]).To(Equal(fixture.expectedOffsetsTopicRF))
				Expect(result.Spec.Kafka.Config["transaction.state.log.replication.factor"]).To(Equal(fixture.expectedTransactionStateRF))
				Expect(result.Spec.Kafka.Config["transaction.state.log.min.isr"]).To(Equal(fixture.expectedTransactionStateISR))
				Expect(result.Spec.Kafka.Config["default.replication.factor"]).To(Equal(fixture.expectedDefaultReplicationFactor))
				Expect(result.Spec.Kafka.Config["min.insync.replicas"]).To(Equal(fixture.expectedMinInSyncReplicas))

				Expect(result.OwnerReferences).To(HaveLen(fixture.expectedOwnerReferences))
				if fixture.expectedOwnerReferences > 0 {
					Expect(result.OwnerReferences[0].UID).To(Equal(owner.UID))
					Expect(result.OwnerReferences[0].Name).To(Equal(owner.Name))
				}
			},
			Entry("disabled kafka returns nil", kafkaSpecFixture{
				name: "disabled kafka",
				spec: apiv2.WBKafkaSpec{
					Enabled: false,
				},
				expectNil: true,
			}),
			Entry("minimal enabled configuration", kafkaSpecFixture{
				name: "minimal configuration",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "test-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    3,
					Config: apiv2.WBKafkaConfig{
						ReplicationConfig: apiv2.WBKafkaReplicationConfig{
							OffsetsTopicRF:           3,
							TransactionStateRF:       3,
							TransactionStateISR:      2,
							DefaultReplicationFactor: 3,
							MinInSyncReplicas:        2,
						},
					},
				},
				expectNil:                        false,
				expectedName:                     "test-kafka",
				expectedNamespace:                testingOwnerNamespace,
				expectedReplicas:                 0,
				expectedVersion:                  common.KafkaVersion,
				expectedMetadataVersion:          common.KafkaMetadataVersion,
				expectedListeners:                2,
				expectedOffsetsTopicRF:           "3",
				expectedTransactionStateRF:       "3",
				expectedTransactionStateISR:      "2",
				expectedDefaultReplicationFactor: "3",
				expectedMinInSyncReplicas:        "2",
				expectedNodePoolAnnotation:       true,
				expectedOwnerReferences:          1,
			}),
			Entry("production configuration with different replication factors", kafkaSpecFixture{
				name: "production configuration",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "prod-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "100Gi",
					Replicas:    5,
					Config: apiv2.WBKafkaConfig{
						ReplicationConfig: apiv2.WBKafkaReplicationConfig{
							OffsetsTopicRF:           5,
							TransactionStateRF:       5,
							TransactionStateISR:      3,
							DefaultReplicationFactor: 5,
							MinInSyncReplicas:        3,
						},
					},
				},
				expectNil:                        false,
				expectedName:                     "prod-kafka",
				expectedNamespace:                testingOwnerNamespace,
				expectedReplicas:                 0,
				expectedVersion:                  common.KafkaVersion,
				expectedMetadataVersion:          common.KafkaMetadataVersion,
				expectedListeners:                2,
				expectedOffsetsTopicRF:           "5",
				expectedTransactionStateRF:       "5",
				expectedTransactionStateISR:      "3",
				expectedDefaultReplicationFactor: "5",
				expectedMinInSyncReplicas:        "3",
				expectedNodePoolAnnotation:       true,
				expectedOwnerReferences:          1,
			}),
			Entry("single replica configuration", kafkaSpecFixture{
				name: "single replica",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "dev-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "5Gi",
					Replicas:    1,
					Config: apiv2.WBKafkaConfig{
						ReplicationConfig: apiv2.WBKafkaReplicationConfig{
							OffsetsTopicRF:           1,
							TransactionStateRF:       1,
							TransactionStateISR:      1,
							DefaultReplicationFactor: 1,
							MinInSyncReplicas:        1,
						},
					},
				},
				expectNil:                        false,
				expectedName:                     "dev-kafka",
				expectedNamespace:                testingOwnerNamespace,
				expectedReplicas:                 0,
				expectedVersion:                  common.KafkaVersion,
				expectedMetadataVersion:          common.KafkaMetadataVersion,
				expectedListeners:                2,
				expectedOffsetsTopicRF:           "1",
				expectedTransactionStateRF:       "1",
				expectedTransactionStateISR:      "1",
				expectedDefaultReplicationFactor: "1",
				expectedMinInSyncReplicas:        "1",
				expectedNodePoolAnnotation:       true,
				expectedOwnerReferences:          1,
			}),
		)

		Context("when owner reference cannot be set", func() {
			It("should return an error", func() {
				spec := apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "test-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    3,
					Config: apiv2.WBKafkaConfig{
						ReplicationConfig: apiv2.WBKafkaReplicationConfig{
							OffsetsTopicRF:           3,
							TransactionStateRF:       3,
							TransactionStateISR:      2,
							DefaultReplicationFactor: 3,
							MinInSyncReplicas:        2,
						},
					},
				}

				nilScheme := runtime.NewScheme()

				result, err := ToKafkaVendorSpec(ctx, spec, owner, nilScheme)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to set owner reference"))
				Expect(result).To(BeNil())
			})
		})

		Context("KRaft mode requirements", func() {
			It("should always set Kafka replicas to 0 when using node pools", func() {
				spec := apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "kraft-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "20Gi",
					Replicas:    5,
					Config: apiv2.WBKafkaConfig{
						ReplicationConfig: apiv2.WBKafkaReplicationConfig{
							OffsetsTopicRF:           3,
							TransactionStateRF:       3,
							TransactionStateISR:      2,
							DefaultReplicationFactor: 3,
							MinInSyncReplicas:        2,
						},
					},
				}

				result, err := ToKafkaVendorSpec(ctx, spec, owner, testScheme)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Spec.Kafka.Replicas).To(Equal(int32(0)))
				Expect(result.Annotations).To(HaveKeyWithValue("strimzi.io/node-pools", "enabled"))
			})
		})
	})

	Describe("ToKafkaNodePoolVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(kafkav1beta2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		type nodePoolFixture struct {
			name                    string
			spec                    apiv2.WBKafkaSpec
			expectedName            string
			expectedNamespace       string
			expectedReplicas        int32
			expectedRoles           []string
			expectedStorageSize     string
			expectedStorageType     string
			expectedDeleteClaim     bool
			expectResources         bool
			expectedCPURequest      string
			expectedMemoryRequest   string
			expectedCPULimit        string
			expectedMemoryLimit     string
			expectedOwnerReferences int
		}

		DescribeTable("node pool spec translation scenarios",
			func(fixture nodePoolFixture) {
				result, err := ToKafkaNodePoolVendorSpec(ctx, fixture.spec, owner, testScheme)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				Expect(result.Name).To(Equal(fixture.expectedName))
				Expect(result.Namespace).To(Equal(fixture.expectedNamespace))
				Expect(result.Labels).To(HaveKeyWithValue("strimzi.io/cluster", strimzi.KafkaName(fixture.spec.Name)))

				Expect(result.Spec.Replicas).To(Equal(fixture.expectedReplicas))
				Expect(result.Spec.Roles).To(ConsistOf(fixture.expectedRoles))

				Expect(result.Spec.Storage.Type).To(Equal("jbod"))
				Expect(result.Spec.Storage.Volumes).To(HaveLen(1))
				Expect(result.Spec.Storage.Volumes[0].ID).To(Equal(int32(0)))
				Expect(result.Spec.Storage.Volumes[0].Type).To(Equal(fixture.expectedStorageType))
				Expect(result.Spec.Storage.Volumes[0].Size).To(Equal(fixture.expectedStorageSize))
				Expect(result.Spec.Storage.Volumes[0].DeleteClaim).To(Equal(fixture.expectedDeleteClaim))

				if fixture.expectResources {
					Expect(result.Spec.Resources).ToNot(BeNil())
					if fixture.expectedCPURequest != "" {
						cpuRequest := result.Spec.Resources.Requests[corev1.ResourceCPU]
						Expect(cpuRequest.String()).To(Equal(fixture.expectedCPURequest))
					}
					if fixture.expectedMemoryRequest != "" {
						memRequest := result.Spec.Resources.Requests[corev1.ResourceMemory]
						Expect(memRequest.String()).To(Equal(fixture.expectedMemoryRequest))
					}
					if fixture.expectedCPULimit != "" {
						cpuLimit := result.Spec.Resources.Limits[corev1.ResourceCPU]
						Expect(cpuLimit.String()).To(Equal(fixture.expectedCPULimit))
					}
					if fixture.expectedMemoryLimit != "" {
						memLimit := result.Spec.Resources.Limits[corev1.ResourceMemory]
						Expect(memLimit.String()).To(Equal(fixture.expectedMemoryLimit))
					}
				} else {
					Expect(result.Spec.Resources).To(BeNil())
				}

				Expect(result.OwnerReferences).To(HaveLen(fixture.expectedOwnerReferences))
				if fixture.expectedOwnerReferences > 0 {
					Expect(result.OwnerReferences[0].UID).To(Equal(owner.UID))
					Expect(result.OwnerReferences[0].Name).To(Equal(owner.Name))
				}
			},
			Entry("minimal node pool configuration", nodePoolFixture{
				name: "minimal node pool",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "test-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    3,
				},
				expectedName:            "test-kafka-node-pool",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        3,
				expectedRoles:           []string{strimzi.RoleBroker, strimzi.RoleController},
				expectedStorageSize:     "10Gi",
				expectedStorageType:     strimzi.StorageType,
				expectedDeleteClaim:     strimzi.StorageDeleteClaim,
				expectResources:         false,
				expectedOwnerReferences: 1,
			}),
			Entry("node pool with resources", nodePoolFixture{
				name: "node pool with resources",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "prod-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "100Gi",
					Replicas:    5,
					Config: apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2000m"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4000m"),
								corev1.ResourceMemory: resource.MustParse("16Gi"),
							},
						},
					},
				},
				expectedName:            "prod-kafka-node-pool",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        5,
				expectedRoles:           []string{strimzi.RoleBroker, strimzi.RoleController},
				expectedStorageSize:     "100Gi",
				expectedStorageType:     strimzi.StorageType,
				expectedDeleteClaim:     strimzi.StorageDeleteClaim,
				expectResources:         true,
				expectedCPURequest:      "2",
				expectedMemoryRequest:   "8Gi",
				expectedCPULimit:        "4",
				expectedMemoryLimit:     "16Gi",
				expectedOwnerReferences: 1,
			}),
			Entry("single replica node pool", nodePoolFixture{
				name: "single replica node pool",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "dev-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "5Gi",
					Replicas:    1,
				},
				expectedName:            "dev-kafka-node-pool",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        1,
				expectedRoles:           []string{strimzi.RoleBroker, strimzi.RoleController},
				expectedStorageSize:     "5Gi",
				expectedStorageType:     strimzi.StorageType,
				expectedDeleteClaim:     strimzi.StorageDeleteClaim,
				expectResources:         false,
				expectedOwnerReferences: 1,
			}),
			Entry("node pool with only requests", nodePoolFixture{
				name: "node pool with only requests",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "limited-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "20Gi",
					Replicas:    3,
					Config: apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
						},
					},
				},
				expectedName:            "limited-kafka-node-pool",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        3,
				expectedRoles:           []string{strimzi.RoleBroker, strimzi.RoleController},
				expectedStorageSize:     "20Gi",
				expectedStorageType:     strimzi.StorageType,
				expectedDeleteClaim:     strimzi.StorageDeleteClaim,
				expectResources:         true,
				expectedCPURequest:      "1",
				expectedMemoryRequest:   "4Gi",
				expectedCPULimit:        "",
				expectedMemoryLimit:     "",
				expectedOwnerReferences: 1,
			}),
			Entry("node pool with only limits", nodePoolFixture{
				name: "node pool with only limits",
				spec: apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "max-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "50Gi",
					Replicas:    3,
					Config: apiv2.WBKafkaConfig{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("3000m"),
								corev1.ResourceMemory: resource.MustParse("12Gi"),
							},
						},
					},
				},
				expectedName:            "max-kafka-node-pool",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        3,
				expectedRoles:           []string{strimzi.RoleBroker, strimzi.RoleController},
				expectedStorageSize:     "50Gi",
				expectedStorageType:     strimzi.StorageType,
				expectedDeleteClaim:     strimzi.StorageDeleteClaim,
				expectResources:         true,
				expectedCPURequest:      "",
				expectedMemoryRequest:   "",
				expectedCPULimit:        "3",
				expectedMemoryLimit:     "12Gi",
				expectedOwnerReferences: 1,
			}),
		)

		Context("when owner reference cannot be set", func() {
			It("should return an error", func() {
				spec := apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "test-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    3,
				}

				nilScheme := runtime.NewScheme()

				result, err := ToKafkaNodePoolVendorSpec(ctx, spec, owner, nilScheme)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to set owner reference"))
				Expect(result).To(BeNil())
			})
		})

		Context("KRaft mode roles", func() {
			It("should include both broker and controller roles", func() {
				spec := apiv2.WBKafkaSpec{
					Enabled:     true,
					Name:        "kraft-kafka",
					Namespace:   testingOwnerNamespace,
					StorageSize: "20Gi",
					Replicas:    3,
				}

				result, err := ToKafkaNodePoolVendorSpec(ctx, spec, owner, testScheme)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Spec.Roles).To(ContainElements(strimzi.RoleBroker, strimzi.RoleController))
				Expect(result.Spec.Roles).To(HaveLen(2))
			})
		})
	})
})
