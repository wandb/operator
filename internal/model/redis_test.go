package model

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Redis Model", func() {
	Describe("RedisConfig", func() {
		Describe("IsHighAvailability", func() {
			Context("when Sentinel is enabled", func() {
				It("should return true", func() {
					config := RedisConfig{
						Sentinel: sentinelConfig{
							Enabled: true,
						},
					}
					Expect(config.IsHighAvailability()).To(BeTrue())
				})
			})

			Context("when Sentinel is not enabled", func() {
				It("should return false", func() {
					config := RedisConfig{
						Sentinel: sentinelConfig{
							Enabled: false,
						},
					}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})
		})

		Describe("GetRedisConfig", func() {
			Context("when mergedRedis is nil", func() {
				It("should return empty config", func() {
					builder := &InfraConfigBuilder{}
					config, err := builder.GetRedisConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Enabled).To(BeFalse())
					Expect(config.Namespace).To(BeEmpty())
				})
			})

			Context("when mergedRedis has basic config without sentinel", func() {
				It("should return config with basic fields", func() {
					builder := &InfraConfigBuilder{
						mergedRedis: &apiv2.WBRedisSpec{
							Enabled:     true,
							Namespace:   "test-namespace",
							StorageSize: "10Gi",
						},
					}
					config, err := builder.GetRedisConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal("test-namespace"))
					Expect(config.StorageSize.String()).To(Equal("10Gi"))
					Expect(config.Sentinel.Enabled).To(BeFalse())
				})
			})

			Context("when mergedRedis has config with resources", func() {
				It("should populate resource limits and requests", func() {
					resources := v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("500m"),
							v1.ResourceMemory: resource.MustParse("512Mi"),
						},
					}
					builder := &InfraConfigBuilder{
						mergedRedis: &apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "5Gi",
							Config: &apiv2.WBRedisConfig{
								Resources: resources,
							},
						},
					}
					config, err := builder.GetRedisConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Requests).NotTo(BeEmpty())
					Expect(config.Limits).NotTo(BeEmpty())
				})
			})

			Context("when mergedRedis has sentinel enabled", func() {
				It("should populate sentinel config", func() {
					builder := &InfraConfigBuilder{
						mergedRedis: &apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
							Sentinel: &apiv2.WBRedisSentinelSpec{
								Enabled: true,
							},
						},
					}
					config, err := builder.GetRedisConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Sentinel.Enabled).To(BeTrue())
					Expect(config.Sentinel.ReplicaCount).To(Equal(ReplicaSentinelCount))
				})
			})

			Context("when mergedRedis has sentinel with full config", func() {
				It("should populate all sentinel fields including resources", func() {
					sentinelResources := v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("50m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("200m"),
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					}
					builder := &InfraConfigBuilder{
						mergedRedis: &apiv2.WBRedisSpec{
							Enabled:     true,
							StorageSize: "10Gi",
							Sentinel: &apiv2.WBRedisSentinelSpec{
								Enabled: true,
								Config: &apiv2.WBRedisSentinelConfig{
									MasterName: "test-master",
									Resources:  sentinelResources,
								},
							},
						},
					}
					config, err := builder.GetRedisConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Sentinel.Enabled).To(BeTrue())
					Expect(config.Sentinel.MasterGroupName).To(Equal("test-master"))
					Expect(config.Sentinel.ReplicaCount).To(Equal(ReplicaSentinelCount))
					Expect(config.Sentinel.Requests).NotTo(BeEmpty())
					Expect(config.Sentinel.Limits).NotTo(BeEmpty())
				})
			})
		})

		Describe("AddRedisSpec", func() {
			Context("when merging with dev size", func() {
				It("should successfully merge and store spec", func() {
					actual := apiv2.WBRedisSpec{
						Enabled: true,
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddRedisSpec(&actual, apiv2.WBSizeDev)
					Expect(result).To(Equal(builder))
					Expect(builder.mergedRedis).NotTo(BeNil())
					Expect(builder.mergedRedis.Enabled).To(BeTrue())
					Expect(builder.errors).To(BeEmpty())
				})
			})

			Context("when merging with small size", func() {
				It("should successfully merge with small defaults", func() {
					actual := apiv2.WBRedisSpec{
						Enabled: true,
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddRedisSpec(&actual, apiv2.WBSizeSmall)
					Expect(result).To(Equal(builder))
					Expect(builder.mergedRedis).NotTo(BeNil())
					Expect(builder.mergedRedis.Enabled).To(BeTrue())
					Expect(builder.errors).To(BeEmpty())
				})
			})

			Context("when building defaults fails", func() {
				It("should append error and return builder", func() {
					actual := apiv2.WBRedisSpec{
						Enabled: true,
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddRedisSpec(&actual, "invalid-size")
					Expect(result).To(Equal(builder))
					Expect(builder.errors).NotTo(BeEmpty())
				})
			})

			Context("when actual spec has custom values", func() {
				It("should preserve custom values during merge", func() {
					actual := apiv2.WBRedisSpec{
						Enabled:     true,
						Namespace:   "custom-ns",
						StorageSize: "100Gi",
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddRedisSpec(&actual, apiv2.WBSizeDev)
					Expect(result).To(Equal(builder))
					Expect(builder.mergedRedis).NotTo(BeNil())
					Expect(builder.mergedRedis.Namespace).To(Equal("custom-ns"))
					Expect(builder.mergedRedis.StorageSize).To(Equal("100Gi"))
					Expect(builder.errors).To(BeEmpty())
				})
			})
		})
	})

	Describe("Redis Error", func() {
		Describe("NewRedisError", func() {
			It("should create error with correct fields", func() {
				err := NewRedisError(RedisDeploymentConflict, "test reason")
				Expect(err.infraName).To(Equal(Redis))
				Expect(err.code).To(Equal(string(RedisDeploymentConflict)))
				Expect(err.reason).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewRedisError(RedisDeploymentConflict, "conflict error")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("DeploymentConflict"))
				Expect(errStr).To(ContainSubstring("redis"))
				Expect(errStr).To(ContainSubstring("conflict error"))
			})
		})

		Describe("RedisInfraError", func() {
			Describe("redisCode", func() {
				It("should return the error code", func() {
					infraErr := NewRedisError(RedisDeploymentConflict, "test error")
					redisErr := RedisInfraError{infraErr}
					Expect(redisErr.redisCode()).To(Equal(RedisDeploymentConflict))
				})
			})
		})

		Describe("ToRedisInfraError", func() {
			Context("when error is a Redis infra error", func() {
				It("should convert successfully", func() {
					err := NewRedisError(RedisDeploymentConflict, "deployment conflict")
					redisErr, ok := ToRedisInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(redisErr.redisCode()).To(Equal(RedisDeploymentConflict))
					Expect(redisErr.reason).To(Equal("deployment conflict"))
				})
			})

			Context("when error is not an infra error", func() {
				It("should return false", func() {
					err := fmt.Errorf("regular error")
					_, ok := ToRedisInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})

			Context("when error is an infra error but not Redis", func() {
				It("should return false", func() {
					err := InfraError{
						infraName: MySQL,
						code:      "SomeCode",
						reason:    "some reason",
					}
					_, ok := ToRedisInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Redis Status", func() {
		Describe("NewRedisStatus", func() {
			It("should create status with correct fields", func() {
				status := NewRedisStatus(RedisSentinelCreated, "Redis Sentinel created")
				Expect(status.infraName).To(Equal(Redis))
				Expect(status.code).To(Equal(string(RedisSentinelCreated)))
				Expect(status.message).To(Equal("Redis Sentinel created"))
			})
		})

		Describe("RedisStatusDetail", func() {
			Describe("redisCode", func() {
				It("should return the status code", func() {
					status := NewRedisStatus(RedisSentinelCreated, "created")
					detail := RedisStatusDetail{status}
					Expect(detail.redisCode()).To(Equal(RedisSentinelCreated))
				})
			})

			Describe("ToRedisSentinelConnDetail", func() {
				Context("when status is Sentinel connection type with connection info", func() {
					It("should convert successfully", func() {
						connInfo := RedisSentinelConnInfo{
							SentinelHost: "redis-sentinel.example.com",
							SentinelPort: "26379",
							MasterName:   "mymaster",
						}
						status := NewRedisSentinelConnDetail(connInfo)
						detail := RedisStatusDetail{status}
						connDetail, ok := detail.ToRedisSentinelConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.SentinelHost).To(Equal("redis-sentinel.example.com"))
						Expect(connDetail.connInfo.SentinelPort).To(Equal("26379"))
						Expect(connDetail.connInfo.MasterName).To(Equal("mymaster"))
					})
				})

				Context("when status is not Sentinel connection type", func() {
					It("should return false", func() {
						status := NewRedisStatus(RedisSentinelCreated, "created")
						detail := RedisStatusDetail{status}
						_, ok := detail.ToRedisSentinelConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						status := InfraStatus{
							infraName: Redis,
							code:      string(RedisSentinelConnection),
							message:   "connection",
							hidden:    "not a RedisSentinelConnInfo",
						}
						detail := RedisStatusDetail{status}
						connDetail, ok := detail.ToRedisSentinelConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.SentinelHost).To(BeEmpty())
						Expect(connDetail.connInfo.SentinelPort).To(BeEmpty())
						Expect(connDetail.connInfo.MasterName).To(BeEmpty())
					})
				})
			})

			Describe("ToRedisStandaloneConnDetail", func() {
				Context("when status is Standalone connection type with connection info", func() {
					It("should convert successfully", func() {
						connInfo := RedisStandaloneConnInfo{
							Host: "redis.example.com",
							Port: "6379",
						}
						status := NewRedisStandaloneConnDetail(connInfo)
						detail := RedisStatusDetail{status}
						connDetail, ok := detail.ToRedisStandaloneConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal("redis.example.com"))
						Expect(connDetail.connInfo.Port).To(Equal("6379"))
					})
				})

				Context("when status is not Standalone connection type", func() {
					It("should return false", func() {
						status := NewRedisStatus(RedisStandaloneCreated, "created")
						detail := RedisStatusDetail{status}
						_, ok := detail.ToRedisStandaloneConnDetail()
						Expect(ok).To(BeFalse())
					})
				})
			})
		})

		Describe("NewRedisSentinelConnDetail", func() {
			It("should create Sentinel connection detail with info", func() {
				connInfo := RedisSentinelConnInfo{
					SentinelHost: "test-host",
					SentinelPort: "26379",
					MasterName:   "testmaster",
				}
				status := NewRedisSentinelConnDetail(connInfo)
				Expect(status.infraName).To(Equal(Redis))
				Expect(status.code).To(Equal(string(RedisSentinelConnection)))
				Expect(status.message).To(ContainSubstring("redis://"))
				Expect(status.hidden).To(Equal(connInfo))
			})
		})

		Describe("NewRedisStandaloneConnDetail", func() {
			It("should create Standalone connection detail with info", func() {
				connInfo := RedisStandaloneConnInfo{
					Host: "test-host",
					Port: "6379",
				}
				status := NewRedisStandaloneConnDetail(connInfo)
				Expect(status.infraName).To(Equal(Redis))
				Expect(status.code).To(Equal(string(RedisStandaloneConnection)))
				Expect(status.message).To(Equal("redis://test-host:6379"))
				Expect(status.hidden).To(Equal(connInfo))
			})
		})
	})

	Describe("Results.ExtractRedisStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return default state", func() {
				results := InitResults()
				status := results.ExtractRedisStatus(ctx)
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have Redis errors", func() {
			It("should include errors in status details with error state", func() {
				results := InitResults()
				err := NewRedisError(RedisDeploymentConflict, "deployment conflict")
				results.AddErrors(err)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateError))
				Expect(status.Details[0].Code).To(Equal(string(RedisDeploymentConflict)))
				Expect(status.Details[0].Message).To(Equal("deployment conflict"))
			})
		})

		Context("when results have non-Redis errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := InfraError{
					infraName: MySQL,
					code:      "MySQLError",
					reason:    "mysql failed",
				}
				results.AddErrors(err)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have Redis statuses", func() {
			It("should include statuses in details with updating state", func() {
				results := InitResults()
				infraStatus := NewRedisStatus(RedisSentinelCreated, "Redis Sentinel created successfully")
				results.AddStatuses(infraStatus)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateUpdating))
				Expect(status.Details[0].Code).To(Equal(string(RedisSentinelCreated)))
				Expect(status.Details[0].Message).To(Equal("Redis Sentinel created successfully"))
			})
		})

		Context("when results have Sentinel connection status", func() {
			It("should populate connection info in status", func() {
				results := InitResults()
				connInfo := RedisSentinelConnInfo{
					SentinelHost: "redis-sentinel.example.com",
					SentinelPort: "26379",
					MasterName:   "mymaster",
				}
				connStatus := NewRedisSentinelConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Connection.RedisSentinelHost).To(Equal("redis-sentinel.example.com"))
				Expect(status.Connection.RedisSentinelPort).To(Equal("26379"))
				Expect(status.Connection.RedisMasterName).To(Equal("mymaster"))
			})
		})

		Context("when results have Standalone connection status", func() {
			It("should populate connection info in status", func() {
				results := InitResults()
				connInfo := RedisStandaloneConnInfo{
					Host: "redis.example.com",
					Port: "6379",
				}
				connStatus := NewRedisStandaloneConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Connection.RedisHost).To(Equal("redis.example.com"))
				Expect(status.Connection.RedisPort).To(Equal("6379"))
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both in details", func() {
				results := InitResults()
				err := NewRedisError(RedisDeploymentConflict, "conflict")
				infraStatus := NewRedisStatus(RedisSentinelCreated, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Details).To(HaveLen(2))
			})
		})
	})
})
