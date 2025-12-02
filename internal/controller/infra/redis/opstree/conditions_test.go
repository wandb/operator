package opstree

import (
	"context"
	"testing"

	"github.com/wandb/operator/internal/controller/translator/common"
	redisv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redis/v1beta2"
	redisreplicationv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redisreplication/v1beta2"
	redissentinelv1beta2 "github.com/wandb/operator/internal/vendored/redis-operator/redissentinel/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRedisConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Redis OpsTre e Conditions Suite")
}

var _ = Describe("Redis OpsTre e Conditions", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(redisv1beta2.AddToScheme(scheme)).To(Succeed())
		Expect(redissentinelv1beta2.AddToScheme(scheme)).To(Succeed())
		Expect(redisreplicationv1beta2.AddToScheme(scheme)).To(Succeed())
	})

	Describe("GetConditions", func() {
		Context("standalone mode", func() {
			It("should create both standalone and sentinel connections", func() {
				specName := "test-redis"
				specNamespace := "default"
				namespacedName := types.NamespacedName{
					Name:      specName,
					Namespace: specNamespace,
				}

				standalone := &redisv1beta2.Redis{
					ObjectMeta: metav1.ObjectMeta{
						Name:      StandaloneName(specName),
						Namespace: specNamespace,
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(standalone).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(2))

				status := common.ExtractRedisStatus(ctx, result)
				Expect(status.Connection.RedisHost).To(Equal("wandb-redis." + specNamespace + ".svc.cluster.local"))
				Expect(status.Connection.RedisPort).To(Equal("6379"))
			})
		})

		Context("sentinel mode", func() {
			It("should create both standalone and sentinel connections", func() {
				specName := "test-redis"
				specNamespace := "default"
				namespacedName := types.NamespacedName{
					Name:      specName,
					Namespace: specNamespace,
				}

				sentinel := &redissentinelv1beta2.RedisSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      SentinelName(specName),
						Namespace: specNamespace,
					},
				}

				replication := &redisreplicationv1beta2.RedisReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ReplicationName(specName),
						Namespace: specNamespace,
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(sentinel, replication).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(2))

				status := common.ExtractRedisStatus(ctx, result)
				Expect(status.Connection.SentinelHost).To(Equal("wandb-redis-sentinel." + specNamespace + ".svc.cluster.local"))
				Expect(status.Connection.SentinelPort).To(Equal("26379"))
				Expect(status.Connection.SentinelMaster).To(Equal("gorilla"))
			})

			It("should create both conditions even with only sentinel", func() {
				specName := "test-redis"
				specNamespace := "default"
				namespacedName := types.NamespacedName{
					Name:      specName,
					Namespace: specNamespace,
				}

				sentinel := &redissentinelv1beta2.RedisSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      SentinelName(specName),
						Namespace: specNamespace,
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(sentinel).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(2))
			})

			It("should create both conditions even with only replication", func() {
				specName := "test-redis"
				specNamespace := "default"
				namespacedName := types.NamespacedName{
					Name:      specName,
					Namespace: specNamespace,
				}

				replication := &redisreplicationv1beta2.RedisReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ReplicationName(specName),
						Namespace: specNamespace,
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(replication).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(2))
			})
		})

		Context("both standalone and sentinel exist", func() {
			It("should create both connection types", func() {
				specName := "test-redis"
				specNamespace := "default"
				namespacedName := types.NamespacedName{
					Name:      specName,
					Namespace: specNamespace,
				}

				standalone := &redisv1beta2.Redis{
					ObjectMeta: metav1.ObjectMeta{
						Name:      StandaloneName(specName),
						Namespace: specNamespace,
					},
				}

				sentinel := &redissentinelv1beta2.RedisSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      SentinelName(specName),
						Namespace: specNamespace,
					},
				}

				replication := &redisreplicationv1beta2.RedisReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ReplicationName(specName),
						Namespace: specNamespace,
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(standalone, sentinel, replication).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(2))
			})
		})

		Context("connection string formatting", func() {
			It("should format standalone host correctly", func() {
				namespacedName := types.NamespacedName{
					Name:      "my-redis",
					Namespace: "my-namespace",
				}

				standalone := &redisv1beta2.Redis{
					ObjectMeta: metav1.ObjectMeta{
						Name:      StandaloneName("my-redis"),
						Namespace: "my-namespace",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(standalone).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractRedisStatus(ctx, result)
				Expect(status.Connection.RedisHost).To(Equal("wandb-redis.my-namespace.svc.cluster.local"))
				Expect(status.Connection.RedisPort).To(Equal("6379"))
			})

			It("should format sentinel host correctly", func() {
				namespacedName := types.NamespacedName{
					Name:      "my-redis",
					Namespace: "my-namespace",
				}

				sentinel := &redissentinelv1beta2.RedisSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      SentinelName("my-redis"),
						Namespace: "my-namespace",
					},
				}

				replication := &redisreplicationv1beta2.RedisReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ReplicationName("my-redis"),
						Namespace: "my-namespace",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(sentinel, replication).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractRedisStatus(ctx, result)
				Expect(status.Connection.SentinelHost).To(Equal("wandb-redis-sentinel.my-namespace.svc.cluster.local"))
				Expect(status.Connection.SentinelPort).To(Equal("26379"))
				Expect(status.Connection.SentinelMaster).To(Equal("gorilla"))
			})
		})
	})
})
