package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/redis/opstree"
	"github.com/wandb/operator/internal/controller/translator/common"
	"github.com/wandb/operator/internal/defaults"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Redis Translator", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("TranslateRedisStatus", func() {
		type statusFixture struct {
			name                   string
			input                  common.RedisStatus
			expectedReady          bool
			expectedState          apiv2.WBStateType
			expectedHost           string
			expectedPort           string
			expectedSentinelHost   string
			expectedSentinelPort   string
			expectedSentinelMaster string
			expectedConditions     int
		}

		DescribeTable("status translation scenarios",
			func(fixture statusFixture) {
				result := TranslateRedisStatus(ctx, fixture.input)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.State).To(Equal(fixture.expectedState))
				Expect(result.Connection.RedisHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.RedisPort).To(Equal(fixture.expectedPort))
				Expect(result.Connection.RedisSentinelHost).To(Equal(fixture.expectedSentinelHost))
				Expect(result.Connection.RedisSentinelPort).To(Equal(fixture.expectedSentinelPort))
				Expect(result.Connection.RedisMasterName).To(Equal(fixture.expectedSentinelMaster))
				Expect(result.Conditions).To(HaveLen(fixture.expectedConditions))
				Expect(result.LastReconciled).ToNot(BeZero())
			},
			Entry("standalone ready status", statusFixture{
				name: "standalone ready",
				input: common.RedisStatus{
					Ready: true,
					Connection: common.RedisConnection{
						RedisHost: "redis.example.com",
						RedisPort: "6379",
					},
					Conditions: []common.RedisCondition{
						common.NewRedisCondition(common.RedisStandaloneConnectionCode, "Connected"),
					},
				},
				expectedReady:          true,
				expectedState:          apiv2.WBStateReady,
				expectedHost:           "redis.example.com",
				expectedPort:           "6379",
				expectedSentinelHost:   "",
				expectedSentinelPort:   "",
				expectedSentinelMaster: "",
				expectedConditions:     1,
			}),
			Entry("sentinel ready status", statusFixture{
				name: "sentinel ready",
				input: common.RedisStatus{
					Ready: true,
					Connection: common.RedisConnection{
						RedisHost:      "redis-replication.example.com",
						RedisPort:      "6379",
						SentinelHost:   "redis-sentinel.example.com",
						SentinelPort:   "26379",
						SentinelMaster: "mymaster",
					},
					Conditions: []common.RedisCondition{
						common.NewRedisCondition(common.RedisSentinelConnectionCode, "Connected"),
					},
				},
				expectedReady:          true,
				expectedState:          apiv2.WBStateReady,
				expectedHost:           "redis-replication.example.com",
				expectedPort:           "6379",
				expectedSentinelHost:   "redis-sentinel.example.com",
				expectedSentinelPort:   "26379",
				expectedSentinelMaster: "mymaster",
				expectedConditions:     1,
			}),
			Entry("sentinel creating status", statusFixture{
				name: "sentinel creating",
				input: common.RedisStatus{
					Ready:      false,
					Connection: common.RedisConnection{},
					Conditions: []common.RedisCondition{
						common.NewRedisCondition(common.RedisSentinelCreatedCode, "Creating sentinel"),
					},
				},
				expectedReady:          false,
				expectedState:          apiv2.WBStateUpdating,
				expectedHost:           "",
				expectedPort:           "",
				expectedSentinelHost:   "",
				expectedSentinelPort:   "",
				expectedSentinelMaster: "",
				expectedConditions:     1,
			}),
			Entry("replication creating status", statusFixture{
				name: "replication creating",
				input: common.RedisStatus{
					Ready:      false,
					Connection: common.RedisConnection{},
					Conditions: []common.RedisCondition{
						common.NewRedisCondition(common.RedisReplicationCreatedCode, "Creating replication"),
					},
				},
				expectedReady:          false,
				expectedState:          apiv2.WBStateUpdating,
				expectedHost:           "",
				expectedPort:           "",
				expectedSentinelHost:   "",
				expectedSentinelPort:   "",
				expectedSentinelMaster: "",
				expectedConditions:     1,
			}),
			Entry("standalone deleting status", statusFixture{
				name: "standalone deleting",
				input: common.RedisStatus{
					Ready:      false,
					Connection: common.RedisConnection{},
					Conditions: []common.RedisCondition{
						common.NewRedisCondition(common.RedisStandaloneDeletedCode, "Deleting standalone"),
					},
				},
				expectedReady:          false,
				expectedState:          apiv2.WBStateDeleting,
				expectedHost:           "",
				expectedPort:           "",
				expectedSentinelHost:   "",
				expectedSentinelPort:   "",
				expectedSentinelMaster: "",
				expectedConditions:     1,
			}),
		)
	})

	Describe("ToRedisStandaloneVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(redisv1beta2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		It("should return nil when redis is disabled", func() {
			spec := apiv2.WBRedisSpec{
				Enabled: false,
			}

			result, err := ToRedisStandaloneVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return nil when sentinel is enabled", func() {
			spec := apiv2.WBRedisSpec{
				Enabled: true,
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: true,
				},
			}

			result, err := ToRedisStandaloneVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should create standalone redis with minimal config", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Name:        "test-redis",
				Namespace:   testingOwnerNamespace,
				StorageSize: "10Gi",
			}

			result, err := ToRedisStandaloneVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Name).To(Equal(opstree.StandaloneName("test-redis")))
			Expect(result.Namespace).To(Equal(testingOwnerNamespace))
			Expect(result.Spec.KubernetesConfig.Image).To(Equal(common.RedisStandaloneImage))
			Expect(result.Spec.KubernetesConfig.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

			Expect(result.Spec.Storage).ToNot(BeNil())
			storageRequest := result.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(storageRequest.String()).To(Equal("10Gi"))

			Expect(result.OwnerReferences).To(HaveLen(1))
		})

		It("should create standalone redis with resources", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Name:        "resource-redis",
				Namespace:   testingOwnerNamespace,
				StorageSize: "20Gi",
				Config: apiv2.WBRedisConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			}

			result, err := ToRedisStandaloneVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Spec.KubernetesConfig.Resources).ToNot(BeNil())

			cpuRequest := result.Spec.KubernetesConfig.Resources.Requests[corev1.ResourceCPU]
			Expect(cpuRequest.String()).To(Equal("500m"))
			memRequest := result.Spec.KubernetesConfig.Resources.Requests[corev1.ResourceMemory]
			Expect(memRequest.String()).To(Equal("1Gi"))
		})

		It("should return error for invalid storage size", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Name:        "test-redis",
				Namespace:   testingOwnerNamespace,
				StorageSize: "invalid-size",
			}

			result, err := ToRedisStandaloneVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("ToRedisSentinelVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(redissentinelv1beta2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		It("should return nil when redis is disabled", func() {
			spec := apiv2.WBRedisSpec{
				Enabled: false,
			}

			result, err := ToRedisSentinelVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return nil when sentinel is disabled", func() {
			spec := apiv2.WBRedisSpec{
				Enabled: true,
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: false,
				},
			}

			result, err := ToRedisSentinelVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should create sentinel with default config", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:   true,
				Name:      "test-redis",
				Namespace: testingOwnerNamespace,
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: true,
				},
			}

			result, err := ToRedisSentinelVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Name).To(Equal(opstree.SentinelName("test-redis")))
			Expect(result.Namespace).To(Equal(testingOwnerNamespace))
			Expect(*result.Spec.Size).To(Equal(int32(defaults.ReplicaSentinelCount)))
			Expect(result.Spec.KubernetesConfig.Image).To(Equal(common.RedisSentinelImage))

			Expect(result.Spec.RedisSentinelConfig).ToNot(BeNil())
			Expect(result.Spec.RedisSentinelConfig.RedisReplicationName).To(Equal(opstree.ReplicationName("test-redis")))
			Expect(result.Spec.RedisSentinelConfig.MasterGroupName).To(Equal(DefaultSentinelGroup))

			Expect(result.OwnerReferences).To(HaveLen(1))
		})

		It("should use custom master name when provided", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:   true,
				Name:      "test-redis",
				Namespace: testingOwnerNamespace,
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: true,
					Config: apiv2.WBRedisSentinelConfig{
						MasterName: "custom-master",
					},
				},
			}

			result, err := ToRedisSentinelVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Spec.RedisSentinelConfig.MasterGroupName).To(Equal("custom-master"))
		})

		It("should create sentinel with resources", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:   true,
				Name:      "resource-redis",
				Namespace: testingOwnerNamespace,
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: true,
					Config: apiv2.WBRedisSentinelConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			}

			result, err := ToRedisSentinelVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Spec.KubernetesConfig.Resources).ToNot(BeNil())

			cpuRequest := result.Spec.KubernetesConfig.Resources.Requests[corev1.ResourceCPU]
			Expect(cpuRequest.String()).To(Equal("250m"))
		})
	})

	Describe("ToRedisReplicationVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(redisreplicationv1beta2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		It("should return nil when redis is disabled", func() {
			spec := apiv2.WBRedisSpec{
				Enabled: false,
			}

			result, err := ToRedisReplicationVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return nil when sentinel is disabled", func() {
			spec := apiv2.WBRedisSpec{
				Enabled: true,
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: false,
				},
			}

			result, err := ToRedisReplicationVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should create replication with default config", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Name:        "test-redis",
				Namespace:   testingOwnerNamespace,
				StorageSize: "10Gi",
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: true,
				},
			}

			result, err := ToRedisReplicationVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Name).To(Equal(opstree.ReplicationName("test-redis")))
			Expect(result.Namespace).To(Equal(testingOwnerNamespace))
			Expect(*result.Spec.Size).To(Equal(int32(defaults.ReplicaSentinelCount)))
			Expect(result.Spec.KubernetesConfig.Image).To(Equal(common.RedisReplicationImage))

			Expect(result.Spec.Storage).ToNot(BeNil())
			storageRequest := result.Spec.Storage.VolumeClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]
			Expect(storageRequest.String()).To(Equal("10Gi"))

			Expect(result.OwnerReferences).To(HaveLen(1))
		})

		It("should create replication with resources", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Name:        "resource-redis",
				Namespace:   testingOwnerNamespace,
				StorageSize: "20Gi",
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: true,
				},
				Config: apiv2.WBRedisConfig{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			}

			result, err := ToRedisReplicationVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Spec.KubernetesConfig.Resources).ToNot(BeNil())

			cpuRequest := result.Spec.KubernetesConfig.Resources.Requests[corev1.ResourceCPU]
			Expect(cpuRequest.String()).To(Equal("500m"))
			memRequest := result.Spec.KubernetesConfig.Resources.Requests[corev1.ResourceMemory]
			Expect(memRequest.String()).To(Equal("1Gi"))
		})

		It("should return error for invalid storage size", func() {
			spec := apiv2.WBRedisSpec{
				Enabled:     true,
				Name:        "test-redis",
				Namespace:   testingOwnerNamespace,
				StorageSize: "invalid-size",
				Sentinel: apiv2.WBRedisSentinelSpec{
					Enabled: true,
				},
			}

			result, err := ToRedisReplicationVendorSpec(ctx, spec, owner, testScheme)

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})
})
