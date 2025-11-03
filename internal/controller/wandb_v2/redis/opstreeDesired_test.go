package redis

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("OpstreeDesired", func() {
	var (
		namespacedName types.NamespacedName
		storageSize    resource.Quantity
	)

	BeforeEach(func() {
		namespacedName = types.NamespacedName{
			Name:      "test-redis",
			Namespace: "test-namespace",
		}
		var err error
		storageSize, err = resource.ParseQuantity("1Gi")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("desiredOpstreeRedis", func() {
		Context("when WBRedisSpec is nil", func() {
			It("should return an error", func() {
				redis, err := desiredOpstreeRedis(namespacedName, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("WBRedisSpec is nil"))
				Expect(redis).To(BeNil())
			})
		})

		Context("when WBRedisSpec.Config is nil", func() {
			It("should return an error", func() {
				wbSpec := &v2.WBRedisSpec{}
				redis, err := desiredOpstreeRedis(namespacedName, wbSpec)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("WBRedisSpec.Config is nil"))
				Expect(redis).To(BeNil())
			})
		})

		Context("when Sentinel is enabled", func() {
			It("should return nil without error", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
					},
				}
				redis, err := desiredOpstreeRedis(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(redis).To(BeNil())
			})
		})

		Context("when Sentinel is disabled", func() {
			It("should return a Redis resource", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
				}
				redis, err := desiredOpstreeRedis(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(redis).ToNot(BeNil())
				Expect(redis.Name).To(Equal(namespacedName.Name))
				Expect(redis.Namespace).To(Equal(namespacedName.Namespace))
				Expect(redis.Spec.KubernetesConfig.Image).To(Equal(OpstreeImage))
				Expect(redis.Spec.KubernetesConfig.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				Expect(redis.Spec.Storage).ToNot(BeNil())
				Expect(redis.Spec.Storage.VolumeClaimTemplate.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))
				Expect(redis.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(storageSize))
			})
		})

		Context("when Sentinel is nil", func() {
			It("should return a Redis resource", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
					Sentinel: nil,
				}
				redis, err := desiredOpstreeRedis(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(redis).ToNot(BeNil())
			})
		})
	})

	Describe("desiredOpstreeSentinel", func() {
		Context("when WBRedisSpec is nil", func() {
			It("should return an error", func() {
				sentinel, err := desiredOpstreeSentinel(namespacedName, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("WBRedisSpec is nil"))
				Expect(sentinel).To(BeNil())
			})
		})

		Context("when WBRedisSpec.Config is nil", func() {
			It("should return an error", func() {
				wbSpec := &v2.WBRedisSpec{}
				sentinel, err := desiredOpstreeSentinel(namespacedName, wbSpec)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("WBRedisSpec.Config is nil"))
				Expect(sentinel).To(BeNil())
			})
		})

		Context("when Sentinel is disabled", func() {
			It("should return nil without error", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
				}
				sentinel, err := desiredOpstreeSentinel(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(sentinel).To(BeNil())
			})
		})

		Context("when Sentinel is nil", func() {
			It("should return nil without error", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
					Sentinel: nil,
				}
				sentinel, err := desiredOpstreeSentinel(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(sentinel).To(BeNil())
			})
		})

		Context("when Sentinel is enabled", func() {
			It("should return a RedisSentinel resource", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
					},
				}
				sentinel, err := desiredOpstreeSentinel(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(sentinel).ToNot(BeNil())
				Expect(sentinel.Name).To(Equal(namespacedName.Name))
				Expect(sentinel.Namespace).To(Equal(namespacedName.Namespace))
				Expect(sentinel.Spec.KubernetesConfig.Image).To(Equal(OpstreeSentinelImage))
				Expect(sentinel.Spec.KubernetesConfig.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				Expect(*sentinel.Spec.Size).To(Equal(int32(ReplicaSentinelCount)))
				Expect(sentinel.Spec.RedisSentinelConfig).ToNot(BeNil())
				Expect(sentinel.Spec.RedisSentinelConfig.RedisReplicationName).To(Equal(NamePrefix))
				Expect(sentinel.Spec.RedisSentinelConfig.MasterGroupName).To(Equal(DefaultSentinelGroup))
			})
		})
	})

	Describe("desiredOpstreeReplication", func() {
		Context("when WBRedisSpec is nil", func() {
			It("should return an error", func() {
				replication, err := desiredOpstreeReplication(namespacedName, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("WBRedisSpec is nil"))
				Expect(replication).To(BeNil())
			})
		})

		Context("when WBRedisSpec.Config is nil", func() {
			It("should return an error", func() {
				wbSpec := &v2.WBRedisSpec{}
				replication, err := desiredOpstreeReplication(namespacedName, wbSpec)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("WBRedisSpec.Config is nil"))
				Expect(replication).To(BeNil())
			})
		})

		Context("when Sentinel is disabled", func() {
			It("should return nil without error", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
				}
				replication, err := desiredOpstreeReplication(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(replication).To(BeNil())
			})
		})

		Context("when Sentinel is nil", func() {
			It("should return nil without error", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
					Sentinel: nil,
				}
				replication, err := desiredOpstreeReplication(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(replication).To(BeNil())
			})
		})

		Context("when Sentinel is enabled", func() {
			It("should return a RedisReplication resource", func() {
				wbSpec := &v2.WBRedisSpec{
					Config: &v2.WBRedisConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: storageSize,
							},
						},
					},
					Sentinel: &v2.WBRedisSentinelSpec{
						Enabled: true,
					},
				}
				replication, err := desiredOpstreeReplication(namespacedName, wbSpec)
				Expect(err).ToNot(HaveOccurred())
				Expect(replication).ToNot(BeNil())
				Expect(replication.Name).To(Equal(namespacedName.Name))
				Expect(replication.Namespace).To(Equal(namespacedName.Namespace))
				Expect(replication.Spec.KubernetesConfig.Image).To(Equal(OpstreeImage))
				Expect(replication.Spec.KubernetesConfig.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
				Expect(*replication.Spec.Size).To(Equal(int32(ReplicaSentinelCount)))
				Expect(replication.Spec.Storage).ToNot(BeNil())
				Expect(replication.Spec.Storage.VolumeClaimTemplate.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))
				Expect(replication.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(storageSize))
			})
		})
	})

	Describe("desiredOpstreeNamespacedName", func() {
		Context("when request has namespace", func() {
			It("should return NamespacedName with request namespace and NamePrefix", func() {
				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "custom-namespace",
						Name:      "some-name",
					},
				}
				result := desiredOpstreeNamespacedName(req)
				Expect(result.Namespace).To(Equal("custom-namespace"))
				Expect(result.Name).To(Equal(NamePrefix))
			})
		})

		Context("when request has empty namespace", func() {
			It("should return NamespacedName with default namespace and NamePrefix", func() {
				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: "",
						Name:      "some-name",
					},
				}
				result := desiredOpstreeNamespacedName(req)
				Expect(result.Namespace).To(Equal(DefaultNamespace))
				Expect(result.Name).To(Equal(NamePrefix))
			})
		})
	})
})
