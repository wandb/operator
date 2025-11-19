package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/defaults"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - Kafka", func() {
	var (
		defaulter *WeightsAndBiasesCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		defaulter = &WeightsAndBiasesCustomDefaulter{}
		ctx = context.Background()
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Kafka.StorageSize).To(Equal(defaults.DevKafkaStorageSize))
				Expect(wandb.Spec.Kafka.Replicas).To(Equal(int32(1)))
				Expect(wandb.Spec.Kafka.Config).To(BeNil())
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Namespace).To(Equal("custom-kafka-namespace"))
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.StorageSize).To(Equal("20Gi"))
				Expect(wandb.Spec.Kafka.StorageSize).ToNot(Equal(defaults.DevKafkaStorageSize))
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Replicas).To(Equal(int32(5)))
				Expect(wandb.Spec.Kafka.Replicas).ToNot(Equal(int32(1)))
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Kafka.StorageSize).To(Equal(defaults.DevKafkaStorageSize))
				Expect(wandb.Spec.Kafka.Replicas).To(Equal(int32(1)))
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Kafka.StorageSize).To(Equal(defaults.SmallKafkaStorageSize))
				Expect(wandb.Spec.Kafka.Replicas).To(Equal(int32(3)))

				Expect(wandb.Spec.Kafka.Config).ToNot(BeNil())
				Expect(wandb.Spec.Kafka.Config.Resources.Requests).ToNot(BeNil())
				Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallKafkaCpuRequest)))
				Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallKafkaMemoryRequest)))
				Expect(wandb.Spec.Kafka.Config.Resources.Limits).ToNot(BeNil())
				Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallKafkaCpuLimit)))
				Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallKafkaMemoryLimit)))
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
							Config: &apiv2.WBKafkaConfig{
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPU))
				Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallKafkaMemoryRequest)))
				Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallKafkaCpuLimit)))
				Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallKafkaMemoryLimit)))
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Replicas).To(Equal(int32(3)))
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Size).To(Equal(apiv2.WBSizeDev))
				Expect(wandb.Spec.Kafka.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Kafka.StorageSize).To(Equal(defaults.DevKafkaStorageSize))
				Expect(wandb.Spec.Kafka.Replicas).To(Equal(int32(1)))
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
							Config: &apiv2.WBKafkaConfig{
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Kafka.Namespace).To(Equal(customNamespace))
				Expect(wandb.Spec.Kafka.StorageSize).To(Equal(customStorage))
				Expect(wandb.Spec.Kafka.Replicas).To(Equal(customReplicas))
				Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPURequest))
				Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(customMemoryRequest))
				Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(customCPULimit))
				Expect(wandb.Spec.Kafka.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customMemoryLimit))

				Expect(wandb.Spec.Kafka.StorageSize).ToNot(Equal(defaults.SmallKafkaStorageSize))
				Expect(wandb.Spec.Kafka.Replicas).ToNot(Equal(int32(3)))
				Expect(wandb.Spec.Kafka.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(resource.MustParse(defaults.SmallKafkaCpuRequest)))
			})
		})
	})
})
