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

var _ = Describe("WeightsAndBiasesCustomDefaulter - ClickHouse", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.ClickHouse.StorageSize).To(g.Equal(defaults.DevClickHouseStorageSize))
				g.Expect(wandb.Spec.ClickHouse.Replicas).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.ClickHouse.Version).To(g.Equal(defaults.ClickHouseVersion))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests).To(g.BeEmpty())
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits).To(g.BeEmpty())
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Namespace).To(g.Equal("custom-clickhouse-namespace"))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.StorageSize).To(g.Equal("100Gi"))
				g.Expect(wandb.Spec.ClickHouse.StorageSize).ToNot(g.Equal(defaults.DevClickHouseStorageSize))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Replicas).To(g.Equal(int32(2)))
				g.Expect(wandb.Spec.ClickHouse.Replicas).ToNot(g.Equal(int32(1)))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Version).To(g.Equal(customVersion))
				g.Expect(wandb.Spec.ClickHouse.Version).ToNot(g.Equal(defaults.ClickHouseVersion))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.ClickHouse.StorageSize).To(g.Equal(defaults.DevClickHouseStorageSize))
				g.Expect(wandb.Spec.ClickHouse.Replicas).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.ClickHouse.Version).To(g.Equal(defaults.ClickHouseVersion))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.ClickHouse.StorageSize).To(g.Equal(defaults.SmallClickHouseStorageSize))
				g.Expect(wandb.Spec.ClickHouse.Replicas).To(g.Equal(int32(3)))
				g.Expect(wandb.Spec.ClickHouse.Version).To(g.Equal(defaults.ClickHouseVersion))

				g.Expect(wandb.Spec.ClickHouse.Config).ToNot(g.BeNil())
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests).ToNot(g.BeNil())
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallClickHouseCpuRequest)))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallClickHouseMemoryRequest)))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits).ToNot(g.BeNil())
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallClickHouseCpuLimit)))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallClickHouseMemoryLimit)))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPU))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallClickHouseMemoryRequest)))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallClickHouseCpuLimit)))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallClickHouseMemoryLimit)))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Replicas).To(g.Equal(int32(3)))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Version).To(g.Equal(defaults.ClickHouseVersion))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Size).To(g.Equal(apiv2.WBSizeDev))
				g.Expect(wandb.Spec.ClickHouse.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.ClickHouse.StorageSize).To(g.Equal(defaults.DevClickHouseStorageSize))
				g.Expect(wandb.Spec.ClickHouse.Replicas).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.ClickHouse.Version).To(g.Equal(defaults.ClickHouseVersion))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.ClickHouse.Namespace).To(g.Equal(customNamespace))
				g.Expect(wandb.Spec.ClickHouse.StorageSize).To(g.Equal(customStorage))
				g.Expect(wandb.Spec.ClickHouse.Replicas).To(g.Equal(customReplicas))
				g.Expect(wandb.Spec.ClickHouse.Version).To(g.Equal(customVersion))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPURequest))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(customMemoryRequest))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(customCPULimit))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customMemoryLimit))

				g.Expect(wandb.Spec.ClickHouse.StorageSize).ToNot(g.Equal(defaults.SmallClickHouseStorageSize))
				g.Expect(wandb.Spec.ClickHouse.Replicas).ToNot(g.Equal(int32(3)))
				g.Expect(wandb.Spec.ClickHouse.Version).ToNot(g.Equal(defaults.ClickHouseVersion))
				g.Expect(wandb.Spec.ClickHouse.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(g.Equal(resource.MustParse(defaults.SmallClickHouseCpuRequest)))
			})
		})
	})
})
