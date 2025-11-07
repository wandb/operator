package redis

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("wbRedisSpecDefaults", func() {
	Describe("wbRedisSentinelEnabled", func() {
		Context("when Sentinel is nil", func() {
			It("should return false", func() {
				spec := v2.WBRedisSpec{
					Sentinel: nil,
				}
				result := wbRedisSentinelEnabled(spec)
				Expect(result).To(BeFalse())
			})
		})

		Context("when Sentinel is disabled", func() {
			It("should return false", func() {
				spec := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: false,
					},
				}
				result := wbRedisSentinelEnabled(spec)
				Expect(result).To(BeFalse())
			})
		})

		Context("when Sentinel is enabled", func() {
			It("should return true", func() {
				spec := v2.WBRedisSpec{
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
					},
				}
				result := wbRedisSentinelEnabled(spec)
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("wbRedisSpecDefaults", func() {
		Context("when profile is Dev", func() {
			It("should return a redis spec with storage only and no sentinel", func() {
				spec, err := wbRedisSpecDefaults(v2.WBProfileDev)
				Expect(err).ToNot(HaveOccurred())
				Expect(spec).ToNot(BeNil())
				Expect(spec.Enabled).To(BeTrue())
				Expect(spec.Config).ToNot(BeNil())
				Expect(spec.Sentinel).To(BeNil())

				storageRequest, err := resource.ParseQuantity(DevStorageRequest)
				Expect(err).ToNot(HaveOccurred())
				Expect(spec.Config.Resources.Requests[corev1.ResourceStorage]).To(Equal(storageRequest))

				_, hasCPURequest := spec.Config.Resources.Requests[corev1.ResourceCPU]
				Expect(hasCPURequest).To(BeFalse())
				_, hasCPULimit := spec.Config.Resources.Limits[corev1.ResourceCPU]
				Expect(hasCPULimit).To(BeFalse())
				_, hasMemoryRequest := spec.Config.Resources.Requests[corev1.ResourceMemory]
				Expect(hasMemoryRequest).To(BeFalse())
				_, hasMemoryLimit := spec.Config.Resources.Limits[corev1.ResourceMemory]
				Expect(hasMemoryLimit).To(BeFalse())
			})
		})

		Context("when profile is Small", func() {
			It("should return a redis spec with full resource requirements and sentinel", func() {
				spec, err := wbRedisSpecDefaults(v2.WBProfileSmall)
				Expect(err).ToNot(HaveOccurred())
				Expect(spec).ToNot(BeNil())
				Expect(spec.Enabled).To(BeTrue())
				Expect(spec.Config).ToNot(BeNil())

				storageRequest, err := resource.ParseQuantity(SmallStorageRequest)
				Expect(err).ToNot(HaveOccurred())
				cpuRequest, err := resource.ParseQuantity(SmallReplicaCpuRequest)
				Expect(err).ToNot(HaveOccurred())
				cpuLimit, err := resource.ParseQuantity(SmallReplicaCpuLimit)
				Expect(err).ToNot(HaveOccurred())
				memoryRequest, err := resource.ParseQuantity(SmallReplicaMemoryRequest)
				Expect(err).ToNot(HaveOccurred())
				memoryLimit, err := resource.ParseQuantity(SmallReplicaMemoryLimit)
				Expect(err).ToNot(HaveOccurred())

				Expect(spec.Config.Resources.Requests[corev1.ResourceStorage]).To(Equal(storageRequest))
				Expect(spec.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(cpuRequest))
				Expect(spec.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(cpuLimit))
				Expect(spec.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(memoryRequest))
				Expect(spec.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(memoryLimit))

				Expect(spec.Sentinel).ToNot(BeNil())
				Expect(spec.Sentinel.Config).ToNot(BeNil())

				sentinelCpuRequest, err := resource.ParseQuantity(SmallSentinelCpuRequest)
				Expect(err).ToNot(HaveOccurred())
				sentinelCpuLimit, err := resource.ParseQuantity(SmallSentinelCpuLimit)
				Expect(err).ToNot(HaveOccurred())
				sentinelMemoryRequest, err := resource.ParseQuantity(SmallSentinelMemoryRequest)
				Expect(err).ToNot(HaveOccurred())
				sentinelMemoryLimit, err := resource.ParseQuantity(SmallSentinelMemoryLimit)
				Expect(err).ToNot(HaveOccurred())

				Expect(spec.Sentinel.Config.Resources.Requests[corev1.ResourceCPU]).To(Equal(sentinelCpuRequest))
				Expect(spec.Sentinel.Config.Resources.Limits[corev1.ResourceCPU]).To(Equal(sentinelCpuLimit))
				Expect(spec.Sentinel.Config.Resources.Requests[corev1.ResourceMemory]).To(Equal(sentinelMemoryRequest))
				Expect(spec.Sentinel.Config.Resources.Limits[corev1.ResourceMemory]).To(Equal(sentinelMemoryLimit))
			})
		})

		Context("when profile is invalid", func() {
			It("should return an error", func() {
				spec, err := wbRedisSpecDefaults(v2.WBProfile("invalid"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid profile"))
				Expect(spec).To(BeNil())
			})
		})
	})

	Describe("Constants", func() {
		It("should have valid resource quantity constants", func() {
			quantities := map[string]string{
				"DevStorageRequest":          DevStorageRequest,
				"SmallStorageRequest":        SmallStorageRequest,
				"SmallReplicaCpuRequest":     SmallReplicaCpuRequest,
				"SmallReplicaCpuLimit":       SmallReplicaCpuLimit,
				"SmallReplicaMemoryRequest":  SmallReplicaMemoryRequest,
				"SmallReplicaMemoryLimit":    SmallReplicaMemoryLimit,
				"SmallSentinelCpuRequest":    SmallSentinelCpuRequest,
				"SmallSentinelCpuLimit":      SmallSentinelCpuLimit,
				"SmallSentinelMemoryRequest": SmallSentinelMemoryRequest,
				"SmallSentinelMemoryLimit":   SmallSentinelMemoryLimit,
			}

			for name, value := range quantities {
				_, err := resource.ParseQuantity(value)
				Expect(err).ToNot(HaveOccurred(), "Failed to parse %s: %s", name, value)
			}
		})

		It("should have valid replica sentinel count", func() {
			Expect(ReplicaSentinelCount).To(Equal(3))
		})

		It("should have valid default sentinel group", func() {
			Expect(DefaultSentinelGroup).To(Equal("gorilla"))
		})
	})
})
