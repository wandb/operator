package model

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	translatorv2 "github.com/wandb/operator/internal/controller/translator/v2"
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
	})

	Describe("InfraConfigBuilder", func() {
		Describe("AddRedisSpec and GetRedisConfig", func() {
			Context("with dev size and empty actual spec", func() {
				It("should use all dev defaults except Enabled and Namespace", func() {
					namespaceOverride := "custom-redis-namespace"
					actual := apiv2.WBRedisSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeDev)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(translatorv2.DevStorageRequest)))
					Expect(config.Requests).To(BeEmpty())
					Expect(config.Limits).To(BeEmpty())
					Expect(config.Sentinel.Enabled).To(BeFalse())
					Expect(config.Sentinel.MasterGroupName).To(BeEmpty())
					Expect(config.Sentinel.ReplicaCount).To(Equal(0))
					Expect(config.Sentinel.Requests).To(BeEmpty())
					Expect(config.Sentinel.Limits).To(BeEmpty())
				})
			})

			Context("with small size and empty actual spec", func() {
				It("should use all small defaults including resources except Enabled and Namespace", func() {
					namespaceOverride := "custom-redis-namespace"
					actual := apiv2.WBRedisSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(translatorv2.SmallStorageRequest)))
					Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuRequest)))
					Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryRequest)))
					Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuLimit)))
					Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryLimit)))
					Expect(config.Sentinel.Enabled).To(BeTrue())
					Expect(config.Sentinel.ReplicaCount).To(Equal(translatorv2.ReplicaSentinelCount))
					Expect(config.Sentinel.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallSentinelCpuRequest)))
					Expect(config.Sentinel.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallSentinelMemoryRequest)))
					Expect(config.Sentinel.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallSentinelCpuLimit)))
					Expect(config.Sentinel.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallSentinelMemoryLimit)))
				})
			})

			Context("with small size and storage override", func() {
				It("should use override storage and default resources", func() {
					storageSizeOverride := "50Gi"
					namespaceOverride := "custom-redis-namespace"
					actual := apiv2.WBRedisSpec{
						Enabled:     true,
						Namespace:   namespaceOverride,
						StorageSize: storageSizeOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(storageSizeOverride)))
					Expect(config.StorageSize).NotTo(Equal(resource.MustParse(translatorv2.SmallStorageRequest)))
					Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuRequest)))
					Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryRequest)))
					Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuLimit)))
					Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryLimit)))
					Expect(config.Sentinel.Enabled).To(BeTrue())
				})
			})

			Context("with small size and resource overrides", func() {
				It("should use override resources and default storage", func() {
					cpuRequestOverride := "2"
					cpuLimitOverride := "4"
					memoryRequestOverride := "4Gi"
					memoryLimitOverride := "8Gi"
					namespaceOverride := "custom-redis-namespace"
					actual := apiv2.WBRedisSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Config: &apiv2.WBRedisConfig{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse(cpuRequestOverride),
									v1.ResourceMemory: resource.MustParse(memoryRequestOverride),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse(cpuLimitOverride),
									v1.ResourceMemory: resource.MustParse(memoryLimitOverride),
								},
							},
						},
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(translatorv2.SmallStorageRequest)))
					Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaCpuRequest)))
					Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryRequest)))
					Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaCpuLimit)))
					Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryLimit)))
				})
			})

			Context("with small size and partial resource overrides", func() {
				It("should merge override and default resources", func() {
					cpuLimitOverride := "2"
					namespaceOverride := "custom-redis-namespace"
					actual := apiv2.WBRedisSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Config: &apiv2.WBRedisConfig{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse(cpuLimitOverride),
								},
							},
						},
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(translatorv2.SmallStorageRequest)))
					Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuRequest)))
					Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryRequest)))
					Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaCpuLimit)))
					Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryLimit)))
				})
			})

			Context("with small size and all overrides including sentinel", func() {
				It("should use all override values", func() {
					storageSizeOverride := "100Gi"
					namespaceOverride := "custom-redis-namespace"
					cpuRequestOverride := "3"
					cpuLimitOverride := "6"
					memoryRequestOverride := "8Gi"
					memoryLimitOverride := "16Gi"
					sentinelCpuRequestOverride := "1"
					sentinelCpuLimitOverride := "2"
					sentinelMemoryRequestOverride := "2Gi"
					sentinelMemoryLimitOverride := "4Gi"
					masterNameOverride := "custom-master"
					actual := apiv2.WBRedisSpec{
						Enabled:     true,
						Namespace:   namespaceOverride,
						StorageSize: storageSizeOverride,
						Config: &apiv2.WBRedisConfig{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse(cpuRequestOverride),
									v1.ResourceMemory: resource.MustParse(memoryRequestOverride),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse(cpuLimitOverride),
									v1.ResourceMemory: resource.MustParse(memoryLimitOverride),
								},
							},
						},
						Sentinel: &apiv2.WBRedisSentinelSpec{
							Enabled: true,
							Config: &apiv2.WBRedisSentinelConfig{
								MasterName: masterNameOverride,
								Resources: v1.ResourceRequirements{
									Requests: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse(sentinelCpuRequestOverride),
										v1.ResourceMemory: resource.MustParse(sentinelMemoryRequestOverride),
									},
									Limits: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse(sentinelCpuLimitOverride),
										v1.ResourceMemory: resource.MustParse(sentinelMemoryLimitOverride),
									},
								},
							},
						},
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(storageSizeOverride)))
					Expect(config.StorageSize).NotTo(Equal(resource.MustParse(translatorv2.SmallStorageRequest)))
					Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaCpuRequest)))
					Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryRequest)))
					Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaCpuLimit)))
					Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryLimit)))
					Expect(config.Sentinel.Enabled).To(BeTrue())
					Expect(config.Sentinel.MasterGroupName).To(Equal(masterNameOverride))
					Expect(config.Sentinel.ReplicaCount).To(Equal(translatorv2.ReplicaSentinelCount))
					Expect(config.Sentinel.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(sentinelCpuRequestOverride)))
					Expect(config.Sentinel.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallSentinelCpuRequest)))
					Expect(config.Sentinel.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(sentinelMemoryRequestOverride)))
					Expect(config.Sentinel.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallSentinelMemoryRequest)))
					Expect(config.Sentinel.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(sentinelCpuLimitOverride)))
					Expect(config.Sentinel.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallSentinelCpuLimit)))
					Expect(config.Sentinel.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(sentinelMemoryLimitOverride)))
					Expect(config.Sentinel.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallSentinelMemoryLimit)))
				})
			})

			Context("with disabled spec", func() {
				It("should respect enabled false and use defaults for other fields", func() {
					namespaceOverride := "custom-redis-namespace"
					actual := apiv2.WBRedisSpec{
						Enabled:   false,
						Namespace: namespaceOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeFalse())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(translatorv2.SmallStorageRequest)))
					Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuRequest)))
					Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryRequest)))
					Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuLimit)))
					Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryLimit)))
				})
			})

			Context("with small size and sentinel disabled explicitly", func() {
				It("should disable sentinel but use other small defaults", func() {
					namespaceOverride := "custom-redis-namespace"
					actual := apiv2.WBRedisSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Sentinel: &apiv2.WBRedisSentinelSpec{
							Enabled: false,
						},
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddRedisSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetRedisConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(resource.MustParse(translatorv2.SmallStorageRequest)))
					Expect(config.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuRequest)))
					Expect(config.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryRequest)))
					Expect(config.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallReplicaCpuLimit)))
					Expect(config.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallReplicaMemoryLimit)))
					Expect(config.Sentinel.Enabled).To(BeFalse())
					Expect(config.Sentinel.ReplicaCount).To(Equal(translatorv2.ReplicaSentinelCount))
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
})
