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

var _ = Describe("WeightsAndBiasesCustomDefaulter - Redis", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Redis.StorageSize).To(g.Equal(defaults.DevStorageRequest))
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests).To(g.BeEmpty())
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits).To(g.BeEmpty())
				g.Expect(wandb.Spec.Redis.Sentinel.Enabled).To(g.BeFalse())
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests).To(g.BeEmpty())
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits).To(g.BeEmpty())
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							Namespace: "custom-redis-namespace",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal("custom-redis-namespace"))
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							StorageSize: "20Gi",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.StorageSize).To(g.Equal("20Gi"))
				g.Expect(wandb.Spec.Redis.StorageSize).ToNot(g.Equal(defaults.DevStorageRequest))
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: false,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Redis.StorageSize).To(g.Equal(defaults.DevStorageRequest))
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Redis.StorageSize).To(g.Equal(defaults.SmallStorageRequest))

				g.Expect(wandb.Spec.Redis.Config).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallReplicaCpuRequest)))
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallReplicaMemoryRequest)))
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallReplicaCpuLimit)))
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallReplicaMemoryLimit)))
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
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
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPU))
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallReplicaMemoryRequest)))
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallReplicaCpuLimit)))
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallReplicaMemoryLimit)))
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Sentinel).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Redis.Sentinel.Enabled).To(g.BeTrue())
				g.Expect(wandb.Spec.Redis.Sentinel.Config).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).To(g.Equal(defaults.DefaultSentinelGroup))

				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallSentinelCpuRequest)))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallSentinelMemoryRequest)))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallSentinelCpuLimit)))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallSentinelMemoryLimit)))
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
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
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).To(g.Equal(customMasterName))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).ToNot(g.Equal(defaults.DefaultSentinelGroup))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPU))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallSentinelMemoryRequest)))
			})
		})

		Context("when Sentinel is disabled by user, overriding the default of True", func() {
			It("should CONTINUE to enable Sentinel", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Redis: apiv2.WBRedisSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: false,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Sentinel.Enabled).To(g.BeTrue())
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Size).To(g.Equal(apiv2.WBSizeDev))
				g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Redis.StorageSize).To(g.Equal(defaults.DevStorageRequest))
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
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
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
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Redis.Namespace).To(g.Equal(customNamespace))
				g.Expect(wandb.Spec.Redis.StorageSize).To(g.Equal(customStorage))
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPU))
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(customMemory))
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(customCPULimit))
				g.Expect(wandb.Spec.Redis.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customMemoryLimit))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.MasterName).To(g.Equal(customMasterName))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customSentinelCPU))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(customSentinelMemory))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(customSentinelCPULimit))
				g.Expect(wandb.Spec.Redis.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customSentinelMemoryLimit))

				g.Expect(wandb.Spec.Redis.StorageSize).ToNot(g.Equal(defaults.SmallStorageRequest))
				g.Expect(wandb.Spec.Redis.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(g.Equal(resource.MustParse(defaults.SmallReplicaCpuRequest)))
			})
		})
	})
})
