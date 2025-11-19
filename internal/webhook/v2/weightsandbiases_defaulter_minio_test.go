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

var _ = Describe("WeightsAndBiasesCustomDefaulter - Minio", func() {
	var (
		defaulter *WeightsAndBiasesCustomDefaulter
		ctx       context.Context
	)

	BeforeEach(func() {
		defaulter = &WeightsAndBiasesCustomDefaulter{}
		ctx = context.Background()
	})

	Describe("Size dev - Minio defaults", func() {
		Context("when Minio spec is empty", func() {
			It("should apply all dev defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Minio: apiv2.WBMinioSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Minio.StorageSize).To(Equal(defaults.DevMinioStorageSize))
				Expect(wandb.Spec.Minio.Replicas).To(Equal(int32(1)))
				Expect(wandb.Spec.Minio.Config).To(BeNil())
			})
		})

		Context("when Minio has custom namespace", func() {
			It("should keep custom namespace", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Minio: apiv2.WBMinioSpec{
							Enabled:   true,
							Namespace: "custom-minio-namespace",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.Namespace).To(Equal("custom-minio-namespace"))
			})
		})

		Context("when Minio has custom StorageSize", func() {
			It("should keep custom StorageSize", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Minio: apiv2.WBMinioSpec{
							Enabled:     true,
							StorageSize: "50Gi",
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.StorageSize).To(Equal("50Gi"))
				Expect(wandb.Spec.Minio.StorageSize).ToNot(Equal(defaults.DevMinioStorageSize))
			})
		})

		Context("when Minio has custom Replicas", func() {
			It("should keep custom Replicas", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Minio: apiv2.WBMinioSpec{
							Enabled:  true,
							Replicas: 4,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.Replicas).To(Equal(int32(4)))
				Expect(wandb.Spec.Minio.Replicas).ToNot(Equal(int32(1)))
			})
		})

		Context("when Minio is disabled", func() {
			It("should still apply defaults", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Minio: apiv2.WBMinioSpec{
							Enabled: false,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Minio.StorageSize).To(Equal(defaults.DevMinioStorageSize))
				Expect(wandb.Spec.Minio.Replicas).To(Equal(int32(1)))
			})
		})
	})

	Describe("Size small - Minio defaults", func() {
		Context("when Minio spec is empty", func() {
			It("should apply all small defaults including resources", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Minio: apiv2.WBMinioSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Minio.StorageSize).To(Equal(defaults.SmallMinioStorageSize))
				Expect(wandb.Spec.Minio.Replicas).To(Equal(int32(3)))

				Expect(wandb.Spec.Minio.Config).ToNot(BeNil())
				Expect(wandb.Spec.Minio.Config.Resources.Requests).ToNot(BeNil())
				Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallMinioCpuRequest)))
				Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallMinioMemoryRequest)))
				Expect(wandb.Spec.Minio.Config.Resources.Limits).ToNot(BeNil())
				Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallMinioCpuLimit)))
				Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallMinioMemoryLimit)))
			})
		})

		Context("when Minio has partial resources", func() {
			It("should merge with defaults", func() {
				customMemory := resource.MustParse("3Gi")
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Minio: apiv2.WBMinioSpec{
							Enabled: true,
							Config: &apiv2.WBMinioConfig{
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: customMemory,
									},
								},
							},
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallMinioCpuRequest)))
				Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse(defaults.SmallMinioMemoryRequest)))
				Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(resource.MustParse(defaults.SmallMinioCpuLimit)))
				Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customMemory))
			})
		})

		Context("when Minio Replicas is explicitly set to 0", func() {
			It("should default Replicas to 3", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Minio: apiv2.WBMinioSpec{
							Enabled:  true,
							Replicas: 0,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Minio.Replicas).To(Equal(int32(3)))
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
						Minio: apiv2.WBMinioSpec{
							Enabled: true,
						},
					},
				}

				err := defaulter.Default(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())

				Expect(wandb.Spec.Size).To(Equal(apiv2.WBSizeDev))
				Expect(wandb.Spec.Minio.Namespace).To(Equal("test-namespace"))
				Expect(wandb.Spec.Minio.StorageSize).To(Equal(defaults.DevMinioStorageSize))
				Expect(wandb.Spec.Minio.Replicas).To(Equal(int32(1)))
			})
		})
	})

	Describe("Complete spec override", func() {
		Context("when all Minio fields are provided", func() {
			It("should not override any user values", func() {
				customNamespace := "custom-namespace"
				customStorage := "200Gi"
				customReplicas := int32(8)
				customCPURequest := resource.MustParse("2000m")
				customMemoryRequest := resource.MustParse("4Gi")
				customCPULimit := resource.MustParse("3000m")
				customMemoryLimit := resource.MustParse("6Gi")

				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeSmall,
						Minio: apiv2.WBMinioSpec{
							Enabled:     true,
							Namespace:   customNamespace,
							StorageSize: customStorage,
							Replicas:    customReplicas,
							Config: &apiv2.WBMinioConfig{
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

				Expect(wandb.Spec.Minio.Namespace).To(Equal(customNamespace))
				Expect(wandb.Spec.Minio.StorageSize).To(Equal(customStorage))
				Expect(wandb.Spec.Minio.Replicas).To(Equal(customReplicas))
				Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(customCPURequest))
				Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(customMemoryRequest))
				Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(customCPULimit))
				Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(customMemoryLimit))

				Expect(wandb.Spec.Minio.StorageSize).ToNot(Equal(defaults.SmallMinioStorageSize))
				Expect(wandb.Spec.Minio.Replicas).ToNot(Equal(int32(3)))
				Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(Equal(resource.MustParse(defaults.SmallMinioCpuRequest)))
			})
		})
	})
})
