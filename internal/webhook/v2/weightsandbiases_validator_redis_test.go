package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiasesCustomValidator - Redis", func() {
	var (
		validator *WeightsAndBiasesCustomValidator
		ctx       context.Context
	)

	BeforeEach(func() {
		validator = &WeightsAndBiasesCustomValidator{}
		ctx = context.Background()
	})

	Describe("ValidateCreate - Redis spec validation", func() {
		Context("when Redis is disabled", func() {
			It("should allow creation without validation errors", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: false,
						},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when Redis is enabled with valid storageSize", func() {
			It("should allow creation with valid resource quantity", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
						},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when Redis has invalid storageSize", func() {
			It("should reject creation with invalid resource quantity", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "invalid-size",
						},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, wandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.storageSize"))
				Expect(err.Error()).To(ContainSubstring("must be a valid resource quantity"))
			})
		})

		Context("when Redis has empty storageSize", func() {
			It("should allow creation since empty storageSize is valid", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "",
						},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when Redis Sentinel is enabled with Redis enabled", func() {
			It("should allow creation", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, wandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when Redis Sentinel is enabled but Redis is disabled", func() {
			It("should reject creation", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: false,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, wandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.sentinel.enabled"))
				Expect(err.Error()).To(ContainSubstring("Redis Sentinel cannot be enabled when Redis is disabled"))
			})
		})

		Context("when Redis has multiple validation errors", func() {
			It("should report all validation errors", func() {
				wandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "not-a-valid-size",
						},
					},
				}

				warnings, err := validator.ValidateCreate(ctx, wandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.storageSize"))
			})
		})
	})

	Describe("ValidateUpdate - Redis changes validation", func() {
		Context("when Redis is disabled in both old and new", func() {
			It("should allow update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: false,
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: false,
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when storageSize is initially empty and being set for the first time", func() {
			It("should allow the update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "",
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when storageSize is being changed after initial set", func() {
			It("should reject the update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "20Gi",
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.storageSize"))
				Expect(err.Error()).To(ContainSubstring("storageSize may not be changed"))
			})
		})

		Context("when storageSize remains the same", func() {
			It("should allow the update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when namespace is being changed", func() {
			It("should reject the update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:   true,
							Namespace: "redis-ns-1",
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:   true,
							Namespace: "redis-ns-2",
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.namespace"))
				Expect(err.Error()).To(ContainSubstring("namespace may not be changed"))
			})
		})

		Context("when namespace remains the same", func() {
			It("should allow the update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:   true,
							Namespace: "redis-ns-1",
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:   true,
							Namespace: "redis-ns-1",
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when Sentinel is toggled from disabled to enabled", func() {
			It("should reject the update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: false,
							},
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.sentinel.enabled"))
				Expect(err.Error()).To(ContainSubstring("Redis Sentinel cannot be toggled between enabled and disabled"))
			})
		})

		Context("when Sentinel is toggled from enabled to disabled", func() {
			It("should reject the update", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: false,
							},
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.sentinel.enabled"))
				Expect(err.Error()).To(ContainSubstring("Redis Sentinel cannot be toggled between enabled and disabled"))
			})
		})

		Context("when Sentinel remains in the same state", func() {
			It("should allow the update when both are enabled", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should allow the update when both are disabled", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: false,
							},
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: true,
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: false,
							},
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).ToNot(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("when update includes invalid new storageSize", func() {
			It("should report both spec validation and change validation errors", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "invalid-size",
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.storageSize"))
			})
		})

		Context("when multiple immutable fields are changed", func() {
			It("should report all validation errors", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
							Namespace:   "ns1",
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: false,
							},
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "20Gi",
							Namespace:   "ns2",
							Sentinel: apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.storageSize"))
				Expect(err.Error()).To(ContainSubstring("spec.redis.namespace"))
				Expect(err.Error()).To(ContainSubstring("spec.redis.sentinel.enabled"))
			})
		})

		Context("when Redis is newly enabled in an update", func() {
			It("should validate the new spec", func() {
				oldWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled: false,
						},
					},
				}

				newWandb := &apiv2.WeightsAndBiases{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-wandb",
						Namespace: "test-namespace",
					},
					Spec: apiv2.WeightsAndBiasesSpec{
						Redis: apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "invalid-size",
						},
					},
				}

				warnings, err := validator.ValidateUpdate(ctx, oldWandb, newWandb)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())
				Expect(err.Error()).To(ContainSubstring("spec.redis.storageSize"))
				Expect(err.Error()).To(ContainSubstring("must be a valid resource quantity"))
			})
		})
	})
})
