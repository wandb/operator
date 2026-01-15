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

var _ = Describe("WeightsAndBiasesCustomDefaulter - MySQL", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	Describe("Size dev - MySQL defaults", func() {
		Context("when MySQL spec is empty", func() {
			It("should apply all dev defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.MySQL.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.MySQL.StorageSize).To(g.Equal(defaults.DevMySQLStorageSize))
				g.Expect(wandb.Spec.MySQL.Replicas).To(g.Equal(int32(1)))
			})
		})

		Context("when MySQL has custom namespace", func() {
			It("should keep custom namespace", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							Namespace: "custom-mysql-namespace",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.MySQL.Namespace).To(g.Equal("custom-mysql-namespace"))
			})
		})

		Context("when MySQL has custom StorageSize", func() {
			It("should keep custom StorageSize", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							StorageSize: "50Gi",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.MySQL.StorageSize).To(g.Equal("50Gi"))
				g.Expect(wandb.Spec.MySQL.StorageSize).ToNot(g.Equal(defaults.DevMySQLStorageSize))
			})
		})

		Context("when MySQL is disabled", func() {
			It("should still apply defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: false,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.MySQL.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.MySQL.StorageSize).To(g.Equal(defaults.DevMySQLStorageSize))
			})
		})
	})

	Describe("Size small - MySQL defaults", func() {
		Context("when MySQL spec is empty", func() {
			It("should apply all small defaults including resources", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.MySQL.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.MySQL.StorageSize).To(g.Equal(defaults.SmallMySQLStorageSize))
				g.Expect(wandb.Spec.MySQL.Replicas).To(g.Equal(int32(3)))

				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests).ToNot(g.BeNil())
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallMySQLCpuRequest)))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallMySQLMemoryRequest)))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits).ToNot(g.BeNil())
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallMySQLCpuLimit)))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallMySQLMemoryLimit)))
			})
		})

		Context("when MySQL has partial resources", func() {
			It("should merge with defaults", func() {
				customCPU := resource.MustParse("2000m")
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							Config: apiv2.WBMySQLConfig{
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

				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPU))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallMySQLMemoryRequest)))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallMySQLCpuLimit)))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallMySQLMemoryLimit)))
			})
		})

		Context("when MySQL has all resources set", func() {
			It("should not override any resource values", func() {
				customCPURequest := resource.MustParse("2000m")
				customMemoryRequest := resource.MustParse("3Gi")
				customCPULimit := resource.MustParse("3000m")
				customMemoryLimit := resource.MustParse("4Gi")

				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							Config: apiv2.WBMySQLConfig{
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

				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPURequest))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(customMemoryRequest))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(customCPULimit))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customMemoryLimit))

				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(g.Equal(resource.MustParse(defaults.SmallMySQLCpuRequest)))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(g.Equal(resource.MustParse(defaults.SmallMySQLMemoryRequest)))
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
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Size).To(g.Equal(apiv2.WBSizeDev))
				g.Expect(wandb.Spec.MySQL.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.MySQL.StorageSize).To(g.Equal(defaults.DevMySQLStorageSize))
			})
		})
	})

	Describe("Complete spec override", func() {
		Context("when all MySQL fields are provided", func() {
			It("should not override any user values", func() {
				customNamespace := "custom-namespace"
				customStorage := "100Gi"
				customCPURequest := resource.MustParse("4000m")
				customMemoryRequest := resource.MustParse("8Gi")
				customCPULimit := resource.MustParse("6000m")
				customMemoryLimit := resource.MustParse("12Gi")

				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						MySQL: apiv2.WBMySQLSpec{
							WBInfraSpec: apiv2.WBInfraSpec{
								Enabled: true,
							},
							Namespace:   customNamespace,
							StorageSize: customStorage,
							Config: apiv2.WBMySQLConfig{
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

				g.Expect(wandb.Spec.MySQL.Namespace).To(g.Equal(customNamespace))
				g.Expect(wandb.Spec.MySQL.StorageSize).To(g.Equal(customStorage))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPURequest))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(customMemoryRequest))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(customCPULimit))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customMemoryLimit))

				g.Expect(wandb.Spec.MySQL.StorageSize).ToNot(g.Equal(defaults.SmallMySQLStorageSize))
				g.Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(g.Equal(resource.MustParse(defaults.SmallMySQLCpuRequest)))
			})
		})
	})
})
