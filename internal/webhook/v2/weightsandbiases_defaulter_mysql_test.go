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

var _ = Describe("WeightsAndBiasesCustomDefaulter - MySQL", func() {
	var (
		defaulter *WeightsAndBiasesCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		defaulter = &WeightsAndBiasesCustomDefaulter{}
		ctx = context.Background()
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
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.MySQL.StorageSize).To(Equal(defaults.DevMySQLStorageSize))
				Expect(wandb.Spec.MySQL.Replicas).To(Equal(int32(1)))
				Expect(wandb.Spec.MySQL.Config).To(BeNil())
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
							Enabled:   true,
							Namespace: "custom-mysql-namespace",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.Namespace).To(Equal("custom-mysql-namespace"))
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
							Enabled:     true,
							StorageSize: "50Gi",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.StorageSize).To(Equal("50Gi"))
				Expect(wandb.Spec.MySQL.StorageSize).ToNot(Equal(defaults.DevMySQLStorageSize))
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
							Enabled: false,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.MySQL.StorageSize).To(Equal(defaults.DevMySQLStorageSize))
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
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.MySQL.StorageSize).To(Equal(defaults.SmallMySQLStorageSize))
				Expect(wandb.Spec.MySQL.Replicas).To(Equal(int32(3)))

				Expect(wandb.Spec.MySQL.Config).ToNot(BeNil())
				Expect(wandb.Spec.MySQL.Config.Resources.Requests).ToNot(BeNil())
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallMySQLCpuRequest)))
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallMySQLMemoryRequest)))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits).ToNot(BeNil())
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallMySQLCpuLimit)))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallMySQLMemoryLimit)))
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
							Enabled: true,
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPU))
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallMySQLMemoryRequest)))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallMySQLCpuLimit)))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallMySQLMemoryLimit)))
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
							Enabled: true,
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPURequest))
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(customMemoryRequest))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(customCPULimit))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customMemoryLimit))

				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(resource.MustParse(defaults.SmallMySQLCpuRequest)))
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).ToNot(Equal(resource.MustParse(defaults.SmallMySQLMemoryRequest)))
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
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Size).To(Equal(apiv2.WBSizeDev))
				Expect(wandb.Spec.MySQL.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.MySQL.StorageSize).To(Equal(defaults.DevMySQLStorageSize))
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
							Enabled:     true,
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
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.MySQL.Namespace).To(Equal(customNamespace))
				Expect(wandb.Spec.MySQL.StorageSize).To(Equal(customStorage))
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPURequest))
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(customMemoryRequest))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(customCPULimit))
				Expect(wandb.Spec.MySQL.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customMemoryLimit))

				Expect(wandb.Spec.MySQL.StorageSize).ToNot(Equal(defaults.SmallMySQLStorageSize))
				Expect(wandb.Spec.MySQL.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(resource.MustParse(defaults.SmallMySQLCpuRequest)))
			})
		})
	})
})
