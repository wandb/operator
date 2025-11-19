package common

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Redis Model", func() {
	Describe("Redis Error", func() {
		Describe("NewRedisError", func() {
			It("should create error with correct fields", func() {
				err := NewRedisError(RedisDeploymentConflictCode, "test reason")
				Expect(err.InfraName()).To(Equal(Redis))
				Expect(err.Code()).To(Equal(string(RedisDeploymentConflictCode)))
				Expect(err.Reason()).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewRedisError(RedisDeploymentConflictCode, "conflict error")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("DeploymentConflict"))
				Expect(errStr).To(ContainSubstring("redis"))
				Expect(errStr).To(ContainSubstring("conflict error"))
			})
		})

		Describe("RedisInfraError", func() {
			Describe("redisCode", func() {
				It("should return the error code", func() {
					infraErr := NewRedisError(RedisDeploymentConflictCode, "test error")
					redisErr := RedisInfraError{infraErr}
					Expect(RedisErrorCode(redisErr.Code())).To(Equal(RedisDeploymentConflictCode))
				})
			})
		})

		Describe("ToRedisInfraError", func() {
			Context("when error is a Redis infra error", func() {
				It("should convert successfully", func() {
					err := NewRedisError(RedisDeploymentConflictCode, "deployment conflict")
					redisErr, ok := ToRedisInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(RedisErrorCode(redisErr.Code())).To(Equal(RedisDeploymentConflictCode))
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
					err := NewInfraError(MySQL, "SomeCode", "some reason")
					_, ok := ToRedisInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Redis Status", func() {
		Describe("NewRedisStatus", func() {
			It("should create status with correct fields", func() {
				status := NewRedisStatusDetail(RedisSentinelCreatedCode, "Redis Sentinel created")
				Expect(status.InfraName()).To(Equal(Redis))
				Expect(status.Code()).To(Equal(string(RedisSentinelCreatedCode)))
				Expect(status.Message()).To(Equal("Redis Sentinel created"))
			})
		})

		Describe("RedisStatusDetail", func() {
			Describe("redisCode", func() {
				It("should return the status code", func() {
					status := NewRedisStatusDetail(RedisSentinelCreatedCode, "created")
					detail := RedisStatusDetail{status}
					Expect(RedisInfraCode(detail.Code())).To(Equal(RedisSentinelCreatedCode))
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
						status := NewRedisStatusDetail(RedisSentinelCreatedCode, "created")
						detail := RedisStatusDetail{status}
						_, ok := detail.ToRedisSentinelConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						status := NewInfraStatusDetail(Redis, string(RedisSentinelConnectionCode), "connection", "not a RedisSentinelConnInfo")
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
						status := NewRedisStatusDetail(RedisStandaloneCreatedCode, "created")
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
				Expect(status.InfraName()).To(Equal(Redis))
				Expect(status.Code()).To(Equal(string(RedisSentinelConnectionCode)))
				Expect(status.Message()).To(ContainSubstring("redis://"))
				Expect(status.Hidden()).To(Equal(connInfo))
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
				Expect(status.InfraName()).To(Equal(Redis))
				Expect(status.Code()).To(Equal(string(RedisStandaloneConnectionCode)))
				Expect(status.Message()).To(Equal("redis://test-host:6379"))
				Expect(status.Hidden()).To(Equal(connInfo))
			})
		})
	})

	Describe("Results.ExtractRedisStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return not ready state with no connection", func() {
				results := InitResults()
				status := ExtractRedisStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(BeEmpty())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have Redis errors", func() {
			It("should include errors and not be ready", func() {
				results := InitResults()
				err := NewRedisError(RedisDeploymentConflictCode, "deployment conflict")
				results.AddErrors(err)

				status := ExtractRedisStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Errors[0].Code()).To(Equal(string(RedisDeploymentConflictCode)))
				Expect(status.Errors[0].Reason()).To(Equal("deployment conflict"))
			})
		})

		Context("when results have non-Redis errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := NewInfraError(MySQL, "MySQLError", "mysql failed")
				results.AddErrors(err)

				status := ExtractRedisStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have Redis statuses", func() {
			It("should include statuses in details", func() {
				results := InitResults()
				infraStatus := NewRedisStatusDetail(RedisSentinelCreatedCode, "Redis Sentinel created successfully")
				results.AddStatuses(infraStatus)

				status := ExtractRedisStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].Code()).To(Equal(string(RedisSentinelCreatedCode)))
				Expect(status.Details[0].Message()).To(Equal("Redis Sentinel created successfully"))
			})
		})

		Context("when results have Sentinel connection status", func() {
			It("should populate connection info in status and be ready", func() {
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

				status := ExtractRedisStatus(ctx, results)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.SentinelHost).To(Equal(sentinelHost))
				Expect(status.Connection.SentinelPort).To(Equal(sentinelPort))
				Expect(status.Connection.SentinelMaster).To(Equal(masterName))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have Standalone connection status", func() {
			It("should populate connection info in status and be ready", func() {
				host := "redis.example.com"
				port := "6379"
				results := InitResults()
				connInfo := RedisStandaloneConnInfo{
					Host: host,
					Port: port,
				}
				connStatus := NewRedisStandaloneConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := ExtractRedisStatus(ctx, results)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.RedisHost).To(Equal(host))
				Expect(status.Connection.RedisPort).To(Equal(port))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both errors and details, not be ready", func() {
				results := InitResults()
				err := NewRedisError(RedisDeploymentConflictCode, "conflict")
				infraStatus := NewRedisStatusDetail(RedisSentinelCreatedCode, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := ExtractRedisStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Details).To(HaveLen(1))
			})
		})
	})
})
