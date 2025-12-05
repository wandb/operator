package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/defaults"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - Kafka", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	Describe("Size dev - Kafka defaults", func() {
		Context("when Kafka spec is empty", func() {
			It("should apply all dev defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Kafka: apiv2.WBKafkaSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Kafka.StorageSize).To(g.Equal(defaults.DevKafkaStorageSize))
				g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests).To(g.BeEmpty())
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits).To(g.BeEmpty())
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.MinInSyncReplicas).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.OffsetsTopicRF).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateRF).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateISR).To(g.Equal(int32(1)))
			})
		})

		Context("when Kafka has custom namespace", func() {
			It("should keep custom namespace", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Kafka: apiv2.WBKafkaSpec{
							Enabled:   true,
							Namespace: "custom-kafka-namespace",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal("custom-kafka-namespace"))
			})
		})

		Context("when Kafka has custom StorageSize", func() {
			It("should keep custom StorageSize", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Kafka: apiv2.WBKafkaSpec{
							Enabled:     true,
							StorageSize: "20Gi",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.StorageSize).To(g.Equal("20Gi"))
				g.Expect(wandb.Spec.Kafka.StorageSize).ToNot(g.Equal(defaults.DevKafkaStorageSize))
			})
		})

		Context("when Kafka has custom Replicas", func() {
			It("should keep custom Replicas", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Kafka: apiv2.WBKafkaSpec{
							Enabled:  true,
							Replicas: 5,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(int32(5)))
				g.Expect(wandb.Spec.Kafka.Replicas).ToNot(g.Equal(int32(1)))
			})
		})

		Context("when Kafka is disabled", func() {
			It("should still apply defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Kafka: apiv2.WBKafkaSpec{
							Enabled: false,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Kafka.StorageSize).To(g.Equal(defaults.DevKafkaStorageSize))
				g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(int32(1)))
			})
		})
	})

	Describe("Size small - Kafka defaults", func() {
		Context("when Kafka spec is empty", func() {
			It("should apply all small defaults including resources", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Kafka: apiv2.WBKafkaSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Kafka.StorageSize).To(g.Equal(defaults.SmallKafkaStorageSize))
				g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(int32(3)))

				g.Expect(wandb.Spec.Kafka.Config).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallKafkaCpuRequest)))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallKafkaMemoryRequest)))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallKafkaCpuLimit)))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallKafkaMemoryLimit)))
			})
		})

		Context("when Kafka has partial resources", func() {
			It("should merge with defaults", func() {
				customCPU := resource.MustParse("2000m")
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Kafka: apiv2.WBKafkaSpec{
							Enabled: true,
							Config: apiv2.WBKafkaConfig{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: customCPU,
									},
								},
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPU))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallKafkaMemoryRequest)))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallKafkaCpuLimit)))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallKafkaMemoryLimit)))

				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor).To(g.Equal(int32(3)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.MinInSyncReplicas).To(g.Equal(int32(2)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.OffsetsTopicRF).To(g.Equal(int32(3)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateRF).To(g.Equal(int32(3)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateISR).To(g.Equal(int32(2)))
			})
		})

		Context("when Kafka has custom ReplicationConfig", func() {
			It("should keep custom values and merge with defaults", func() {
				customDefaultRF := int32(5)
				customMinISR := int32(3)
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Kafka: apiv2.WBKafkaSpec{
							Enabled: true,
							Config: apiv2.WBKafkaConfig{
								ReplicationConfig: apiv2.WBKafkaReplicationConfig{
									DefaultReplicationFactor: customDefaultRF,
									MinInSyncReplicas:        customMinISR,
								},
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.DefaultReplicationFactor).To(g.Equal(customDefaultRF))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.MinInSyncReplicas).To(g.Equal(customMinISR))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.OffsetsTopicRF).To(g.Equal(int32(3)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateRF).To(g.Equal(int32(3)))
				g.Expect(wandb.Spec.Kafka.Config.ReplicationConfig.TransactionStateISR).To(g.Equal(int32(2)))
			})
		})

		Context("when Kafka Replicas is explicitly set to 0", func() {
			It("should default Replicas to 3", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Kafka: apiv2.WBKafkaSpec{
							Enabled:  true,
							Replicas: 0,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(int32(3)))
			})
		})
	})

	Describe("No size specified - defaults to dev", func() {
		Context("when Size is empty", func() {
			It("should default Size to dev and apply dev defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Kafka: apiv2.WBKafkaSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Size).To(g.Equal(apiv2.WBSizeDev))
				g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Kafka.StorageSize).To(g.Equal(defaults.DevKafkaStorageSize))
				g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(int32(1)))
			})
		})
	})

	Describe("Complete spec override", func() {
		Context("when all Kafka fields are provided", func() {
			It("should not override any user values", func() {
				customNamespace := "custom-namespace"
				customStorage := "100Gi"
				customReplicas := int32(7)
				customCPURequest := resource.MustParse("3000m")
				customMemoryRequest := resource.MustParse("4Gi")
				customCPULimit := resource.MustParse("4000m")
				customMemoryLimit := resource.MustParse("6Gi")

				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Kafka: apiv2.WBKafkaSpec{
							Enabled:     true,
							Namespace:   customNamespace,
							StorageSize: customStorage,
							Replicas:    customReplicas,
							Config: apiv2.WBKafkaConfig{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    customCPURequest,
										corev1.ResourceMemory: customMemoryRequest,
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    customCPULimit,
										corev1.ResourceMemory: customMemoryLimit,
									},
								},
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Kafka.Namespace).To(g.Equal(customNamespace))
				g.Expect(wandb.Spec.Kafka.StorageSize).To(g.Equal(customStorage))
				g.Expect(wandb.Spec.Kafka.Replicas).To(g.Equal(customReplicas))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPURequest))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(customMemoryRequest))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(customCPULimit))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customMemoryLimit))

				g.Expect(wandb.Spec.Kafka.StorageSize).ToNot(g.Equal(defaults.SmallKafkaStorageSize))
				g.Expect(wandb.Spec.Kafka.Replicas).ToNot(g.Equal(int32(3)))
				g.Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(g.Equal(resource.MustParse(defaults.SmallKafkaCpuRequest)))
			})
		})
	})
})
