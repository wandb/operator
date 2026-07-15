package reconciler_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/reconciler"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

var _ = Describe("ReconcileV2 Sizing", func() {
	Context("resolveResources", func() {
		It("should apply resources from the 'default' key if present", func() {
			app := serverManifest.Application{
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"default": {
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:          "small",
					RequireLimits: true,
				},
			}

			res := v2.ResolveResources(app, wandb, nil)
			Expect(res).NotTo(BeNil())
			Expect(res.Requests.Cpu().String()).To(Equal("100m"))
		})

		It("should override default with size-specific config", func() {
			app := serverManifest.Application{
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"default": {
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
					"small": {
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("200m"),
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:          "small",
					RequireLimits: true,
				},
			}

			res := v2.ResolveResources(app, wandb, nil)
			Expect(res).NotTo(BeNil())
			Expect(res.Requests.Cpu().String()).To(Equal("200m"))
		})

		It("should apply container-specific overrides last", func() {
			app := serverManifest.Application{
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"small": {
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("200m"),
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:          "small",
					RequireLimits: true,
				},
			}
			containerRes := &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("300m"),
				},
			}

			res := v2.ResolveResources(app, wandb, containerRes)
			Expect(res).NotTo(BeNil())
			Expect(res.Requests.Cpu().String()).To(Equal("300m"))
		})

		It("should clear limits if RequireLimits is false", func() {
			app := serverManifest.Application{
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"default": {
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("200m"),
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:          "small",
					RequireLimits: false,
				},
			}

			res := v2.ResolveResources(app, wandb, nil)
			Expect(res).NotTo(BeNil())
			Expect(res.Requests.Cpu().String()).To(Equal("100m"))
			Expect(res.Limits).To(BeNil())
		})

		It("should apply legacy overrides over sizing-derived resources", func() {
			app := serverManifest.Application{
				Name: "api",
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"default": {
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("200m"),
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:          "small",
					RequireLimits: true,
					Wandb: apiv2.WandbAppSpec{
						LegacyOverrides: map[string]apiv2.LegacyOverrides{
							"api": {
								Resources: &corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("2"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("4"),
									},
								},
							},
						},
					},
				},
			}

			res := v2.ResolveResources(app, wandb, nil)
			Expect(res).NotTo(BeNil())
			// Override wins per resource name; untouched fields survive.
			Expect(res.Requests.Cpu().String()).To(Equal("2"))
			Expect(res.Requests.Memory().String()).To(Equal("128Mi"))
			Expect(res.Limits.Cpu().String()).To(Equal("4"))
		})

		It("should strip legacy override limits when RequireLimits is false", func() {
			app := serverManifest.Application{Name: "api"}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:          "small",
					RequireLimits: false,
					Wandb: apiv2.WandbAppSpec{
						LegacyOverrides: map[string]apiv2.LegacyOverrides{
							"api": {
								Resources: &corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("2"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("4"),
									},
								},
							},
						},
					},
				},
			}

			res := v2.ResolveResources(app, wandb, nil)
			Expect(res).NotTo(BeNil())
			Expect(res.Requests.Cpu().String()).To(Equal("2"))
			Expect(res.Limits).To(BeNil())
		})

		It("should not apply another application's legacy override", func() {
			app := serverManifest.Application{
				Name: "weave",
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"default": {
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size:          "small",
					RequireLimits: true,
					Wandb: apiv2.WandbAppSpec{
						LegacyOverrides: map[string]apiv2.LegacyOverrides{
							"api": {
								Resources: &corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU: resource.MustParse("2"),
									},
								},
							},
						},
					},
				},
			}

			res := v2.ResolveResources(app, wandb, nil)
			Expect(res).NotTo(BeNil())
			Expect(res.Requests.Cpu().String()).To(Equal("100m"))
		})
	})

	Context("ResolveAutoscaling", func() {
		It("should use default autoscaling if size-specific is missing", func() {
			app := serverManifest.Application{
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"default": {
						Autoscaling: &serverManifest.AutoscalingConfig{
							Horizontal: autoscalingv2.HorizontalPodAutoscalerSpec{
								MaxReplicas: 10,
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: "small",
				},
			}

			hpa := v2.ResolveAutoscaling(app, wandb)
			Expect(hpa).NotTo(BeNil())
			Expect(hpa.MaxReplicas).To(Equal(int32(10)))
		})

		It("should merge size-specific autoscaling onto default", func() {
			app := serverManifest.Application{
				Sizing: map[apiv2.Size]serverManifest.SizingConfig{
					"default": {
						Autoscaling: &serverManifest.AutoscalingConfig{
							Horizontal: autoscalingv2.HorizontalPodAutoscalerSpec{
								MinReplicas: ptr.To(int32(2)),
								MaxReplicas: 10,
							},
						},
					},
					"small": {
						Autoscaling: &serverManifest.AutoscalingConfig{
							Horizontal: autoscalingv2.HorizontalPodAutoscalerSpec{
								MaxReplicas: 5,
							},
						},
					},
				},
			}
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: "small",
				},
			}

			hpa := v2.ResolveAutoscaling(app, wandb)
			Expect(hpa).NotTo(BeNil())
			Expect(*hpa.MinReplicas).To(Equal(int32(2)))
			Expect(hpa.MaxReplicas).To(Equal(int32(5)))
		})
	})
})
