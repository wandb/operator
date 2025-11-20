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

var _ = Describe("WeightsAndBiasesCustomDefaulter - Redis", func() {
	var (
		defaulter *WeightsAndBiasesCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		defaulter = &WeightsAndBiasesCustomDefaulter{}
		ctx = context.Background()
	})

	Describe("Size dev - Redis defaults", func() {
		Context("when Redis spec is empty", func() {
			It("should apply all dev defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Redis.StorageSize).To(Equal(defaults.DevStorageRequest))
				Expect(wandb.Spec.Redis.Config).To(BeNil())
				Expect(wandb.Spec.Redis.Sentinel).To(BeNil())
			})
		})

		Context("when Redis has custom namespace", func() {
			It("should keep custom namespace", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Redis: apiv2.WBRedisSpec{
							Enabled:   true,
							Namespace: "custom-redis-namespace",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Namespace).To(Equal("custom-redis-namespace"))
			})
		})

		Context("when Redis has custom StorageSize", func() {
			It("should keep custom StorageSize", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "20Gi",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.StorageSize).To(Equal("20Gi"))
				Expect(wandb.Spec.Redis.StorageSize).ToNot(Equal(defaults.DevStorageRequest))
			})
		})

		Context("when Redis is disabled", func() {
			It("should still apply defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Redis: apiv2.WBRedisSpec{
							Enabled: false,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Redis.StorageSize).To(Equal(defaults.DevStorageRequest))
			})
		})
	})

	Describe("Size small - Redis defaults", func() {
		Context("when Redis spec is empty", func() {
			It("should apply all small defaults including resources", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Redis.StorageSize).To(Equal(defaults.SmallStorageRequest))

				Expect(wandb.Spec.Redis.Config).ToNot(BeNil())
				Expect(wandb.Spec.Redis.Config.Resources.Requests).ToNot(BeNil())
				Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallReplicaCpuRequest)))
				Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallReplicaMemoryRequest)))
				Expect(wandb.Spec.Redis.Config.Resources.Limits).ToNot(BeNil())
				Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallReplicaCpuLimit)))
				Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallReplicaMemoryLimit)))
			})
		})

		Context("when Redis has partial resources", func() {
			It("should merge with defaults", func() {
				customCPU := resource.MustParse("2000m")
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Config: apiv2.WBRedisConfig{
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

				Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPU))
				Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallReplicaMemoryRequest)))
				Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallReplicaCpuLimit)))
				Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallReplicaMemoryLimit)))
			})
		})

		Context("when Sentinel is enabled in default config", func() {
			It("should create Sentinel with defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Sentinel).ToNot(BeNil())
				Expect(wandb.Spec.Redis.Sentinel.Enabled).To(BeTrue())
				Expect(wandb.Spec.Redis.Sentinel.Config).ToNot(BeNil())
				Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).To(Equal(defaults.DefaultSentinelGroup))

				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests).ToNot(BeNil())
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallSentinelCpuRequest)))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallSentinelMemoryRequest)))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits).ToNot(BeNil())
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallSentinelCpuLimit)))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallSentinelMemoryLimit)))
			})
		})

		Context("when Sentinel is already configured with custom values", func() {
			It("should keep custom values and merge defaults", func() {
				customMasterName := "custom-master"
				customCPU := resource.MustParse("300m")
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
								Config: apiv2.WBRedisSentinelConfig{
									MasterName: customMasterName,
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU: customCPU,
										},
									},
								},
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).To(Equal(customMasterName))
				Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).ToNot(Equal(defaults.DefaultSentinelGroup))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPU))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallSentinelMemoryRequest)))
			})
		})

		Context("when Sentinel is disabled by user", func() {
			It("should not enable Sentinel", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: false,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Sentinel.Enabled).To(BeFalse())
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
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Size).To(Equal(apiv2.WBSizeDev))
				Expect(wandb.Spec.Redis.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Redis.StorageSize).To(Equal(defaults.DevStorageRequest))
			})
		})
	})

	Describe("Complete spec override", func() {
		Context("when all Redis fields are provided", func() {
			It("should not override any user values", func() {
				customNamespace := "custom-namespace"
				customStorage := "50Gi"
				customCPU := resource.MustParse("3000m")
				customMemory := resource.MustParse("6Gi")
				customCPULimit := resource.MustParse("4000m")
				customMemoryLimit := resource.MustParse("8Gi")
				customMasterName := "my-custom-master"
				customSentinelCPU := resource.MustParse("500m")
				customSentinelMemory := resource.MustParse("512Mi")
				customSentinelCPULimit := resource.MustParse("750m")
				customSentinelMemoryLimit := resource.MustParse("768Mi")

				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							Namespace:   customNamespace,
							StorageSize: customStorage,
							Config: apiv2.WBRedisConfig{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    customCPU,
										corev1.ResourceMemory: customMemory,
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    customCPULimit,
										corev1.ResourceMemory: customMemoryLimit,
									},
								},
							},
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
								Config: apiv2.WBRedisSentinelConfig{
									MasterName: customMasterName,
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    customSentinelCPU,
											corev1.ResourceMemory: customSentinelMemory,
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    customSentinelCPULimit,
											corev1.ResourceMemory: customSentinelMemoryLimit,
										},
									},
								},
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Redis.Namespace).To(Equal(customNamespace))
				Expect(wandb.Spec.Redis.StorageSize).To(Equal(customStorage))
				Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPU))
				Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(customMemory))
				Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(customCPULimit))
				Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customMemoryLimit))
				Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).To(Equal(customMasterName))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customSentinelCPU))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(customSentinelMemory))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(customSentinelCPULimit))
				Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customSentinelMemoryLimit))

				Expect(wandb.Spec.Redis.StorageSize).ToNot(Equal(defaults.SmallStorageRequest))
				Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(resource.MustParse(defaults.SmallReplicaCpuRequest)))
			})
		})
	})
})
