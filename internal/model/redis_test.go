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
						Sentinel: SentinelConfig{
							Enabled: true,
						},
					}
					Expect(config.IsHighAvailability()).To(BeTrue())
				})
			})

			Context("when Sentinel is not enabled", func() {
				It("should return false", func() {
					config := RedisConfig{
						Sentinel: SentinelConfig{
							Enabled: false,
						},
					}
					Expect(config.IsHighAvailability()).To(BeFalse())
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
						sentinelHost := "redis-sentinel.example.com"
						sentinelPort := "26379"
						masterName := "mymaster"
						connInfo := RedisSentinelConnInfo{
							SentinelHost: sentinelHost,
							SentinelPort: sentinelPort,
							MasterName:   masterName,
						}
						status := NewRedisSentinelConnDetail(connInfo)
						detail := RedisStatusDetail{status}
						connDetail, ok := detail.ToRedisSentinelConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.SentinelHost).To(Equal(sentinelHost))
						Expect(connDetail.connInfo.SentinelPort).To(Equal(sentinelPort))
						Expect(connDetail.connInfo.MasterName).To(Equal(masterName))
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
						host := "redis.example.com"
						port := "6379"
						connInfo := RedisStandaloneConnInfo{
							Host: host,
							Port: port,
						}
						status := NewRedisStandaloneConnDetail(connInfo)
						detail := RedisStatusDetail{status}
						connDetail, ok := detail.ToRedisStandaloneConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal(host))
						Expect(connDetail.connInfo.Port).To(Equal(port))
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
				sentinelHost := "test-host"
				sentinelPort := "26379"
				masterName := "testmaster"
				connInfo := RedisSentinelConnInfo{
					SentinelHost: sentinelHost,
					SentinelPort: sentinelPort,
					MasterName:   masterName,
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
				host := "test-host"
				port := "6379"
				connInfo := RedisStandaloneConnInfo{
					Host: host,
					Port: port,
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
				sentinelHost := "redis-sentinel.example.com"
				sentinelPort := "26379"
				masterName := "mymaster"
				results := InitResults()
				connInfo := RedisSentinelConnInfo{
					SentinelHost: sentinelHost,
					SentinelPort: sentinelPort,
					MasterName:   masterName,
				}
				connStatus := NewRedisSentinelConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Connection.RedisSentinelHost).To(Equal(sentinelHost))
				Expect(status.Connection.RedisSentinelPort).To(Equal(sentinelPort))
				Expect(status.Connection.RedisMasterName).To(Equal(masterName))
			})
		})

		Context("when results have Standalone connection status", func() {
			It("should populate connection info in status", func() {
				host := "redis.example.com"
				port := "6379"
				results := InitResults()
				connInfo := RedisStandaloneConnInfo{
					Host: host,
					Port: port,
				}
				connStatus := NewRedisStandaloneConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractRedisStatus(ctx)
				Expect(status.Connection.RedisHost).To(Equal(host))
				Expect(status.Connection.RedisPort).To(Equal(port))
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

	Describe("BuildRedisDefaults", func() {
		const testOwnerNamespace = "test-namespace"

		Context("when size is Dev", func() {
			It("should return a redis config with storage only and no sentinel", func() {
				config, err := BuildRedisDefaults(SizeDev, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(resource.MustParse(DevStorageRequest)))
				Expect(config.Sentinel.Enabled).To(BeFalse())
				Expect(config.Requests).To(BeEmpty())
				Expect(config.Limits).To(BeEmpty())
			})
		})

		Context("when size is Small", func() {
			It("should return a redis config with full resource requirements and sentinel", func() {
				config, err := BuildRedisDefaults(SizeSmall, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(resource.MustParse(SmallStorageRequest)))
				Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallReplicaCpuRequest)))
				Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallReplicaCpuLimit)))
				Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallReplicaMemoryRequest)))
				Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallReplicaMemoryLimit)))
				Expect(config.Sentinel.Enabled).To(BeTrue())
				Expect(config.Sentinel.MasterGroupName).To(Equal(DefaultSentinelGroup))
				Expect(config.Sentinel.ReplicaCount).To(Equal(ReplicaSentinelCount))
				Expect(config.Sentinel.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallSentinelCpuRequest)))
				Expect(config.Sentinel.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallSentinelCpuLimit)))
				Expect(config.Sentinel.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallSentinelMemoryRequest)))
				Expect(config.Sentinel.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallSentinelMemoryLimit)))
			})
		})

		Context("when size is invalid", func() {
			It("should return an error", func() {
				_, err := BuildRedisDefaults(Size("invalid"), testOwnerNamespace)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid profile"))
			})
		})
	})
})
