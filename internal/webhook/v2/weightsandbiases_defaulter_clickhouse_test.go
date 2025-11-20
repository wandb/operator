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

var _ = Describe("WeightsAndBiasesCustomDefaulter - ClickHouse", func() {
	var (
		defaulter *WeightsAndBiasesCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		defaulter = &WeightsAndBiasesCustomDefaulter{}
		ctx = context.Background()
	})

	Describe("Size dev - ClickHouse defaults", func() {
		Context("when ClickHouse spec is empty", func() {
			It("should apply all dev defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.ClickHouse.StorageSize).To(Equal(defaults.DevClickHouseStorageSize))
				Expect(wandb.Spec.ClickHouse.Replicas).To(Equal(int32(1)))
				Expect(wandb.Spec.ClickHouse.Version).To(Equal(defaults.ClickHouseVersion))
				Expect(wandb.Spec.ClickHouse.Config).To(BeNil())
			})
		})

		Context("when ClickHouse has custom namespace", func() {
			It("should keep custom namespace", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled:   true,
							Namespace: "custom-clickhouse-namespace",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Namespace).To(Equal("custom-clickhouse-namespace"))
			})
		})

		Context("when ClickHouse has custom StorageSize", func() {
			It("should keep custom StorageSize", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled:     true,
							StorageSize: "100Gi",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.StorageSize).To(Equal("100Gi"))
				Expect(wandb.Spec.ClickHouse.StorageSize).ToNot(Equal(defaults.DevClickHouseStorageSize))
			})
		})

		Context("when ClickHouse has custom Replicas", func() {
			It("should keep custom Replicas", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled:  true,
							Replicas: 2,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Replicas).To(Equal(int32(2)))
				Expect(wandb.Spec.ClickHouse.Replicas).ToNot(Equal(int32(1)))
			})
		})

		Context("when ClickHouse has custom Version", func() {
			It("should keep custom Version", func() {
				customVersion := "24.1"
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled: true,
							Version: customVersion,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Version).To(Equal(customVersion))
				Expect(wandb.Spec.ClickHouse.Version).ToNot(Equal(defaults.ClickHouseVersion))
			})
		})

		Context("when ClickHouse is disabled", func() {
			It("should still apply defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled: false,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.ClickHouse.StorageSize).To(Equal(defaults.DevClickHouseStorageSize))
				Expect(wandb.Spec.ClickHouse.Replicas).To(Equal(int32(1)))
				Expect(wandb.Spec.ClickHouse.Version).To(Equal(defaults.ClickHouseVersion))
			})
		})
	})

	Describe("Size small - ClickHouse defaults", func() {
		Context("when ClickHouse spec is empty", func() {
			It("should apply all small defaults including resources", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.ClickHouse.StorageSize).To(Equal(defaults.SmallClickHouseStorageSize))
				Expect(wandb.Spec.ClickHouse.Replicas).To(Equal(int32(3)))
				Expect(wandb.Spec.ClickHouse.Version).To(Equal(defaults.ClickHouseVersion))

				Expect(wandb.Spec.ClickHouse.Config).ToNot(BeNil())
				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests).ToNot(BeNil())
				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallClickHouseCpuRequest)))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallClickHouseMemoryRequest)))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Limits).ToNot(BeNil())
				Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallClickHouseCpuLimit)))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallClickHouseMemoryLimit)))
			})
		})

		Context("when ClickHouse has partial resources", func() {
			It("should merge with defaults", func() {
				customCPU := resource.MustParse("1500m")
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled: true,
							Config: apiv2.WBClickHouseConfig{
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

				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPU))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallClickHouseMemoryRequest)))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallClickHouseCpuLimit)))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallClickHouseMemoryLimit)))
			})
		})

		Context("when ClickHouse Replicas is explicitly set to 0", func() {
			It("should default Replicas to 3", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled:  true,
							Replicas: 0,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Replicas).To(Equal(int32(3)))
			})
		})

		Context("when ClickHouse Version is empty", func() {
			It("should default Version", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled: true,
							Version: "",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.ClickHouse.Version).To(Equal(defaults.ClickHouseVersion))
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
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Size).To(Equal(apiv2.WBSizeDev))
				Expect(wandb.Spec.ClickHouse.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.ClickHouse.StorageSize).To(Equal(defaults.DevClickHouseStorageSize))
				Expect(wandb.Spec.ClickHouse.Replicas).To(Equal(int32(1)))
				Expect(wandb.Spec.ClickHouse.Version).To(Equal(defaults.ClickHouseVersion))
			})
		})
	})

	Describe("Complete spec override", func() {
		Context("when all ClickHouse fields are provided", func() {
			It("should not override any user values", func() {
				customNamespace := "custom-namespace"
				customStorage := "500Gi"
				customReplicas := int32(5)
				customVersion := "24.5"
				customCPURequest := resource.MustParse("2000m")
				customMemoryRequest := resource.MustParse("8Gi")
				customCPULimit := resource.MustParse("3000m")
				customMemoryLimit := resource.MustParse("12Gi")

				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						ClickHouse: apiv2.WBClickHouseSpec{
							Enabled:     true,
							Namespace:   customNamespace,
							StorageSize: customStorage,
							Replicas:    customReplicas,
							Version:     customVersion,
							Config: apiv2.WBClickHouseConfig{
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

				Expect(wandb.Spec.ClickHouse.Namespace).To(Equal(customNamespace))
				Expect(wandb.Spec.ClickHouse.StorageSize).To(Equal(customStorage))
				Expect(wandb.Spec.ClickHouse.Replicas).To(Equal(customReplicas))
				Expect(wandb.Spec.ClickHouse.Version).To(Equal(customVersion))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPURequest))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(customMemoryRequest))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(customCPULimit))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customMemoryLimit))

				Expect(wandb.Spec.ClickHouse.StorageSize).ToNot(Equal(defaults.SmallClickHouseStorageSize))
				Expect(wandb.Spec.ClickHouse.Replicas).ToNot(Equal(int32(3)))
				Expect(wandb.Spec.ClickHouse.Version).ToNot(Equal(defaults.ClickHouseVersion))
				Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(resource.MustParse(defaults.SmallClickHouseCpuRequest)))
			})
		})
	})
})
