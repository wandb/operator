package v2_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v2 "github.com/wandb/operator/internal/controller/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Infra Sizing", func() {
	Context("ResolveInfraSizing", func() {
		It("should apply default sizing baseline", func() {
			infraConfigs := map[string]serverManifest.InfraConfig{
				"default": {
					Sizing: map[apiv2.Size]serverManifest.SizingConfig{
						"default": {
							Replicas:   1,
							VolumeSize: "10Gi",
						},
					},
				},
			}
			defaultInfraConfig := infraConfigs["default"]
			result := v2.ResolveInfraSizing(defaultInfraConfig.Sizing, "small", false)
			Expect(result.Replicas).To(Equal(int32(1)))
			Expect(result.VolumeSize).To(Equal("10Gi"))
		})

		It("should override default with size-specific values", func() {
			infraConfigs := map[string]serverManifest.InfraConfig{
				"default": {
					Sizing: map[apiv2.Size]serverManifest.SizingConfig{
						"default": {
							Replicas:   1,
							VolumeSize: "10Gi",
						},
						"small": {
							Replicas:   3,
							VolumeSize: "100Gi",
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("2"),
								},
							},
						},
					},
				},
			}
			defaultInfraConfig := infraConfigs["default"]
			result := v2.ResolveInfraSizing(defaultInfraConfig.Sizing, "small", false)
			Expect(result.Replicas).To(Equal(int32(3)))
			Expect(result.VolumeSize).To(Equal("100Gi"))
			Expect(result.Resources).NotTo(BeNil())
			Expect(result.Resources.Requests.Cpu().String()).To(Equal("2"))
		})

		It("should merge resources from default and size-specific sizing", func() {
			infraConfigs := map[string]serverManifest.InfraConfig{
				"default": {
					Sizing: map[apiv2.Size]serverManifest.SizingConfig{
						"default": {
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("2Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("2"),
								},
							},
						},
						"small": {
							Resources: &corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("4"),
								},
							},
						},
					},
				},
			}
			defaultInfraConfig := infraConfigs["default"]
			result := v2.ResolveInfraSizing(defaultInfraConfig.Sizing, "small", false)
			Expect(result).NotTo(BeNil())
			// CPU request overridden by size-specific
			Expect(result.Resources.Requests.Cpu().String()).To(Equal("4"))
			// Memory request preserved from default
			Expect(result.Resources.Requests.Memory().String()).To(Equal("2Gi"))
			// CPU limit preserved from default
			Expect(result.Resources.Limits.Cpu().String()).To(Equal("2"))
		})
	})

	Context("ApplyInfraSizing", func() {
		It("should apply manifest sizing to empty spec fields", func() {
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: "small",
				},
			}
			manifest := serverManifest.Manifest{
				Mysql: map[string]serverManifest.InfraConfig{
					"default": {
						Sizing: map[apiv2.Size]serverManifest.SizingConfig{
							"small": {
								Replicas: 3,
								Resources: &corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("2"),
										corev1.ResourceMemory: resource.MustParse("8Gi"),
									},
								},
							},
						},
					},
				},
			}
			v2.ApplyInfraSizing(wandb, manifest)
			Expect(wandb.Spec.MySQL.Replicas).To(Equal(int32(3)))
			Expect(wandb.Spec.MySQL.Config.Resources.Requests.Cpu().String()).To(Equal("2"))
		})

		It("should not override user-specified spec fields", func() {
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Size: "small",
					MySQL: apiv2.MySQLSpec{
						Replicas:    5,
						StorageSize: "50Gi",
					},
				},
			}
			manifest := serverManifest.Manifest{
				Mysql: map[string]serverManifest.InfraConfig{
					"default": {
						Sizing: map[apiv2.Size]serverManifest.SizingConfig{
							"small": {
								Replicas:   3,
								VolumeSize: "100Gi",
							},
						},
					},
				},
			}
			v2.ApplyInfraSizing(wandb, manifest)
			Expect(wandb.Spec.MySQL.Replicas).To(Equal(int32(5)))
			Expect(wandb.Spec.MySQL.StorageSize).To(Equal("50Gi"))
		})
	})

	Context("ResolveKafkaSizing", func() {
		It("should merge default and size-specific kafka sizing", func() {
			kafkaConfig := serverManifest.KafkaConfig{
				Sizing: map[apiv2.Size]serverManifest.KafkaSizingConfig{
					"default": {
						SizingConfig: serverManifest.SizingConfig{
							Replicas:   1,
							VolumeSize: "10Gi",
						},
					},
					"small": {
						SizingConfig: serverManifest.SizingConfig{
							Replicas:   3,
							VolumeSize: "100Gi",
						},
					},
				},
			}
			result := v2.ResolveKafkaSizing(kafkaConfig.Sizing, "small")
			Expect(result).NotTo(BeNil())
			Expect(result.Replicas).To(Equal(int32(3)))
			Expect(result.VolumeSize).To(Equal("100Gi"))
		})
	})
})
