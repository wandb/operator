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

var _ = Describe("WeightsAndBiasesCustomDefaulter - Minio", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Minio.StorageSize).To(g.Equal(defaults.DevMinioStorageSize))
				g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(int32(1)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests).To(g.BeEmpty())
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits).To(g.BeEmpty())
				g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).To(g.Equal(defaults.DefaultMinioBrowserSetting))
				g.Expect(wandb.Spec.Minio.Config.RootUser).To(g.Equal(defaults.DefaultMinioRootUser))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal("custom-minio-namespace"))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.StorageSize).To(g.Equal("50Gi"))
				g.Expect(wandb.Spec.Minio.StorageSize).ToNot(g.Equal(defaults.DevMinioStorageSize))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(int32(4)))
				g.Expect(wandb.Spec.Minio.Replicas).ToNot(g.Equal(int32(1)))
			})
		})

		Context("when Minio has custom MinioBrowserSetting", func() {
			It("should keep custom MinioBrowserSetting", func() {
				customBrowserSetting := "off"
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Minio: apiv2.WBMinioSpec{
							Enabled: true,
							Config: apiv2.WBMinioConfig{
								MinioBrowserSetting: customBrowserSetting,
							},
						},
					},
				}

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).To(g.Equal(customBrowserSetting))
				g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).ToNot(g.Equal(defaults.DefaultMinioBrowserSetting))
			})
		})

		Context("when Minio has custom RootUser", func() {
			It("should keep custom RootUser", func() {
				customRootUser := "custom-admin"
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Size: apiv2.WBSizeDev,
						Minio: apiv2.WBMinioSpec{
							Enabled: true,
							Config: apiv2.WBMinioConfig{
								RootUser: customRootUser,
							},
						},
					},
				}

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Config.RootUser).To(g.Equal(customRootUser))
				g.Expect(wandb.Spec.Minio.Config.RootUser).ToNot(g.Equal(defaults.DefaultMinioRootUser))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Minio.StorageSize).To(g.Equal(defaults.DevMinioStorageSize))
				g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(int32(1)))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Minio.StorageSize).To(g.Equal(defaults.SmallMinioStorageSize))
				g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(int32(3)))

				g.Expect(wandb.Spec.Minio.Config).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallMinioCpuRequest)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallMinioMemoryRequest)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits).ToNot(g.BeNil())
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallMinioCpuLimit)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallMinioMemoryLimit)))
				g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).To(g.Equal(defaults.DefaultMinioBrowserSetting))
				g.Expect(wandb.Spec.Minio.Config.RootUser).To(g.Equal(defaults.DefaultMinioRootUser))
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
							Config: apiv2.WBMinioConfig{
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: customMemory,
									},
								},
							},
						},
					},
				}

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallMinioCpuRequest)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(resource.MustParse(defaults.SmallMinioMemoryRequest)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(resource.MustParse(defaults.SmallMinioCpuLimit)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customMemory))
				g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).To(g.Equal(defaults.DefaultMinioBrowserSetting))
				g.Expect(wandb.Spec.Minio.Config.RootUser).To(g.Equal(defaults.DefaultMinioRootUser))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(int32(3)))
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

				err := Default(ctx, wandb)
				g.Expect(err).ToNot(g.HaveOccurred())

				g.Expect(wandb.Spec.Size).To(g.Equal(apiv2.WBSizeDev))
				g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal("test-namespace"))
				g.Expect(wandb.Spec.Minio.StorageSize).To(g.Equal(defaults.DevMinioStorageSize))
				g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(int32(1)))
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
				customBrowserSetting := "off"
				customRootUser := "superadmin"

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
							Config: apiv2.WBMinioConfig{
								MinioBrowserSetting: customBrowserSetting,
								RootUser:            customRootUser,
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

				g.Expect(wandb.Spec.Minio.Namespace).To(g.Equal(customNamespace))
				g.Expect(wandb.Spec.Minio.StorageSize).To(g.Equal(customStorage))
				g.Expect(wandb.Spec.Minio.Replicas).To(g.Equal(customReplicas))
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).To(g.Equal(customCPURequest))
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceMemory]).To(g.Equal(customMemoryRequest))
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceCPU]).To(g.Equal(customCPULimit))
				g.Expect(wandb.Spec.Minio.Config.Resources.Limits[corev1.ResourceMemory]).To(g.Equal(customMemoryLimit))
				g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).To(g.Equal(customBrowserSetting))
				g.Expect(wandb.Spec.Minio.Config.RootUser).To(g.Equal(customRootUser))

				g.Expect(wandb.Spec.Minio.StorageSize).ToNot(g.Equal(defaults.SmallMinioStorageSize))
				g.Expect(wandb.Spec.Minio.Replicas).ToNot(g.Equal(int32(3)))
				g.Expect(wandb.Spec.Minio.Config.Resources.Requests[corev1.ResourceCPU]).ToNot(g.Equal(resource.MustParse(defaults.SmallMinioCpuRequest)))
				g.Expect(wandb.Spec.Minio.Config.MinioBrowserSetting).ToNot(g.Equal(defaults.DefaultMinioBrowserSetting))
				g.Expect(wandb.Spec.Minio.Config.RootUser).ToNot(g.Equal(defaults.DefaultMinioRootUser))
			})
		})
	})
})
