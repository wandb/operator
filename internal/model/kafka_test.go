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

var _ = Describe("Kafka Model", func() {
	Describe("KafkaConfig", func() {
		Describe("IsHighAvailability", func() {
			Context("when replicas is greater than 1", func() {
				It("should return true", func() {
					config := KafkaConfig{Replicas: 3}
					Expect(config.IsHighAvailability()).To(BeTrue())
				})
			})

			Context("when replicas is equal to 1", func() {
				It("should return false", func() {
					config := KafkaConfig{Replicas: 1}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})

			Context("when replicas is 0", func() {
				It("should return false", func() {
					config := KafkaConfig{Replicas: 0}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})
		})
	})

	Describe("InfraConfigBuilder", func() {
		Describe("AddKafkaSpec and GetKafkaConfig", func() {
			Context("with dev size and empty actual spec", func() {
				It("should use all dev defaults except Enabled and Namespace", func() {
					namespaceOverride := "custom-kafka-namespace"
					actual := apiv2.WBKafkaSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeDev)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.DevKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(1)))
					Expect(config.Resources.Requests).To(BeEmpty())
					Expect(config.Resources.Limits).To(BeEmpty())
					Expect(config.ReplicationConfig.DefaultReplicationFactor).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.MinInSyncReplicas).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.OffsetsTopicRF).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.TransactionStateRF).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.TransactionStateISR).To(Equal(int32(1)))
				})
			})

			Context("with small size and empty actual spec", func() {
				It("should use all small defaults including resources except Enabled and Namespace", func() {
					namespaceOverride := "custom-kafka-namespace"
					actual := apiv2.WBKafkaSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallKafkaCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallKafkaCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryLimit)))
					Expect(config.ReplicationConfig.DefaultReplicationFactor).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.MinInSyncReplicas).To(Equal(int32(2)))
					Expect(config.ReplicationConfig.OffsetsTopicRF).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.TransactionStateRF).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.TransactionStateISR).To(Equal(int32(2)))
				})
			})

			Context("with small size and storage override", func() {
				It("should use override storage and default resources", func() {
					storageSizeOverride := "50Gi"
					namespaceOverride := "custom-kafka-namespace"
					actual := apiv2.WBKafkaSpec{
						Enabled:     true,
						Namespace:   namespaceOverride,
						StorageSize: storageSizeOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(storageSizeOverride))
					Expect(config.StorageSize).NotTo(Equal(translatorv2.SmallKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallKafkaCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallKafkaCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryLimit)))
				})
			})

			Context("with small size and namespace using default", func() {
				It("should use default namespace when not provided", func() {
					actual := apiv2.WBKafkaSpec{
						Enabled: true,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(3)))
				})
			})

			Context("with small size and resource overrides", func() {
				It("should use override resources and default storage", func() {
					cpuRequestOverride := "2"
					cpuLimitOverride := "4"
					memoryRequestOverride := "4Gi"
					memoryLimitOverride := "8Gi"
					namespaceOverride := "custom-kafka-namespace"
					actual := apiv2.WBKafkaSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Config: &apiv2.WBKafkaConfig{
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
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryLimit)))
				})
			})

			Context("with small size and partial resource overrides", func() {
				It("should merge override and default resources", func() {
					cpuLimitOverride := "2"
					namespaceOverride := "custom-kafka-namespace"
					actual := apiv2.WBKafkaSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Config: &apiv2.WBKafkaConfig{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse(cpuLimitOverride),
								},
							},
						},
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallKafkaCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryLimit)))
				})
			})

			Context("with small size and all overrides", func() {
				It("should use all override values", func() {
					storageSizeOverride := "100Gi"
					namespaceOverride := "custom-kafka-namespace"
					cpuRequestOverride := "3"
					cpuLimitOverride := "6"
					memoryRequestOverride := "8Gi"
					memoryLimitOverride := "16Gi"
					actual := apiv2.WBKafkaSpec{
						Enabled:     true,
						Namespace:   namespaceOverride,
						StorageSize: storageSizeOverride,
						Config: &apiv2.WBKafkaConfig{
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
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(storageSizeOverride))
					Expect(config.StorageSize).NotTo(Equal(translatorv2.SmallKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryLimit)))
					Expect(config.ReplicationConfig.DefaultReplicationFactor).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.MinInSyncReplicas).To(Equal(int32(2)))
				})
			})

			Context("with disabled spec", func() {
				It("should respect enabled false and use defaults for other fields", func() {
					namespaceOverride := "custom-kafka-namespace"
					actual := apiv2.WBKafkaSpec{
						Enabled:   false,
						Namespace: namespaceOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddKafkaSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeFalse())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallKafkaStorageSize))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallKafkaCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallKafkaCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallKafkaMemoryLimit)))
				})
			})
		})
	})

	Describe("GetReplicaCountForSize", func() {
		Context("when size is dev", func() {
			It("should return 1 replica", func() {
				count, err := GetReplicaCountForSize(apiv2.WBSizeDev)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int32(1)))
			})
		})

		Context("when size is small", func() {
			It("should return 3 replicas", func() {
				count, err := GetReplicaCountForSize(apiv2.WBSizeSmall)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int32(3)))
			})
		})

		Context("when size is unsupported", func() {
			It("should return error", func() {
				count, err := GetReplicaCountForSize(apiv2.WBSize("large"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for Kafka"))
				Expect(count).To(Equal(int32(0)))
			})
		})
	})

	Describe("GetReplicationConfigForSize", func() {
		Context("when size is dev", func() {
			It("should return dev replication config", func() {
				config, err := GetReplicationConfigForSize(apiv2.WBSizeDev)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.DefaultReplicationFactor).To(Equal(int32(1)))
				Expect(config.MinInSyncReplicas).To(Equal(int32(1)))
				Expect(config.OffsetsTopicRF).To(Equal(int32(1)))
				Expect(config.TransactionStateRF).To(Equal(int32(1)))
				Expect(config.TransactionStateISR).To(Equal(int32(1)))
			})
		})

		Context("when size is small", func() {
			It("should return small replication config", func() {
				config, err := GetReplicationConfigForSize(apiv2.WBSizeSmall)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.DefaultReplicationFactor).To(Equal(int32(3)))
				Expect(config.MinInSyncReplicas).To(Equal(int32(2)))
				Expect(config.OffsetsTopicRF).To(Equal(int32(3)))
				Expect(config.TransactionStateRF).To(Equal(int32(3)))
				Expect(config.TransactionStateISR).To(Equal(int32(2)))
			})
		})

		Context("when size is unsupported", func() {
			It("should return error", func() {
				config, err := GetReplicationConfigForSize(apiv2.WBSize("medium"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for Kafka"))
				Expect(config).To(Equal(KafkaReplicationConfig{}))
			})
		})
	})

	Describe("Kafka Error", func() {
		Describe("NewKafkaError", func() {
			It("should create error with correct fields", func() {
				err := NewKafkaError(KafkaErrFailedToCreate, "test reason")
				Expect(err.infraName).To(Equal(Kafka))
				Expect(err.code).To(Equal(string(KafkaErrFailedToCreate)))
				Expect(err.reason).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewKafkaError(KafkaErrFailedToUpdate, "update failed")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("kafka"))
				Expect(errStr).To(ContainSubstring("update failed"))
			})
		})

		Describe("KafkaInfraError", func() {
			Describe("kafkaCode", func() {
				It("should return the error code", func() {
					infraErr := NewKafkaError(KafkaErrFailedToDelete, "delete failed")
					kafkaErr := KafkaInfraError{infraErr}
					Expect(kafkaErr.kafkaCode()).To(Equal(KafkaErrFailedToDelete))
				})
			})
		})

		Describe("ToKafkaInfraError", func() {
			Context("when error is a Kafka infra error", func() {
				It("should convert successfully", func() {
					err := NewKafkaError(KafkaErrFailedToGetConfig, "config error")
					kafkaErr, ok := ToKafkaInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(kafkaErr.kafkaCode()).To(Equal(KafkaErrFailedToGetConfig))
					Expect(kafkaErr.reason).To(Equal("config error"))
				})
			})

			Context("when error is not an infra error", func() {
				It("should return false", func() {
					err := fmt.Errorf("regular error")
					_, ok := ToKafkaInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})

			Context("when error is an infra error but not Kafka", func() {
				It("should return false", func() {
					err := InfraError{
						infraName: Redis,
						code:      "SomeCode",
						reason:    "some reason",
					}
					_, ok := ToKafkaInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Kafka Status", func() {
		Describe("NewKafkaStatus", func() {
			It("should create status with correct fields", func() {
				status := NewKafkaStatus(KafkaCreated, "Kafka created")
				Expect(status.infraName).To(Equal(Kafka))
				Expect(status.code).To(Equal(string(KafkaCreated)))
				Expect(status.message).To(Equal("Kafka created"))
			})
		})

		Describe("KafkaStatusDetail", func() {
			Describe("kafkaCode", func() {
				It("should return the status code", func() {
					status := NewKafkaStatus(KafkaUpdated, "updated")
					detail := KafkaStatusDetail{status}
					Expect(detail.kafkaCode()).To(Equal(KafkaUpdated))
				})
			})

			Describe("ToKafkaConnDetail", func() {
				Context("when status is connection type with connection info", func() {
					It("should convert successfully", func() {
						host := "kafka.example.com"
						port := "9092"
						connInfo := KafkaConnInfo{
							Host: host,
							Port: port,
						}
						status := NewKafkaConnDetail(connInfo)
						detail := KafkaStatusDetail{status}

						connDetail, ok := detail.ToKafkaConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal(host))
						Expect(connDetail.connInfo.Port).To(Equal(port))
					})
				})

				Context("when status is not connection type", func() {
					It("should return false", func() {
						status := NewKafkaStatus(KafkaCreated, "created")
						detail := KafkaStatusDetail{status}

						_, ok := detail.ToKafkaConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return true but with empty connection info", func() {
						status := InfraStatus{
							infraName: Kafka,
							code:      string(KafkaConnection),
							message:   "kafka://host:port",
							hidden:    nil,
						}
						detail := KafkaStatusDetail{status}

						connDetail, ok := detail.ToKafkaConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(BeEmpty())
						Expect(connDetail.connInfo.Port).To(BeEmpty())
					})
				})
			})
		})

		Describe("NewKafkaConnDetail", func() {
			It("should create connection status with correct fields", func() {
				host := "kafka-broker.example.com"
				port := "9092"
				connInfo := KafkaConnInfo{
					Host: host,
					Port: port,
				}
				status := NewKafkaConnDetail(connInfo)

				Expect(status.infraName).To(Equal(Kafka))
				Expect(status.code).To(Equal(string(KafkaConnection)))
				Expect(status.message).To(Equal("kafka://kafka-broker.example.com:9092"))
			})
		})
	})

	Describe("Results.ExtractKafkaStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return ready state", func() {
				results := InitResults()
				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Ready).To(BeTrue())
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have Kafka errors", func() {
			It("should include errors in status details with error state", func() {
				results := InitResults()
				err := NewKafkaError(KafkaErrFailedToCreate, "failed to create")
				results.AddErrors(err)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateError))
				Expect(status.Details[0].Code).To(Equal(string(KafkaErrFailedToCreate)))
				Expect(status.Details[0].Message).To(Equal("failed to create"))
			})
		})

		Context("when results have non-Kafka errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := InfraError{
					infraName: Redis,
					code:      "RedisError",
					reason:    "redis failed",
				}
				results.AddErrors(err)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have Kafka created status", func() {
			It("should include status in details with updating state", func() {
				results := InitResults()
				infraStatus := NewKafkaStatus(KafkaCreated, "Kafka created successfully")
				results.AddStatuses(infraStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Ready).To(BeTrue())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateUpdating))
				Expect(status.Details[0].Code).To(Equal(string(KafkaCreated)))
				Expect(status.Details[0].Message).To(Equal("Kafka created successfully"))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status", func() {
				host := "kafka.example.com"
				port := "9092"
				results := InitResults()
				connInfo := KafkaConnInfo{
					Host: host,
					Port: port,
				}
				connStatus := NewKafkaConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.KafkaHost).To(Equal(host))
				Expect(status.Connection.KafkaPort).To(Equal(port))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both in details with error state", func() {
				results := InitResults()
				err := NewKafkaError(KafkaErrFailedToUpdate, "update failed")
				infraStatus := NewKafkaStatus(KafkaCreated, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				results := InitResults()
				err1 := NewKafkaError(KafkaErrFailedToCreate, "create failed")
				err2 := NewKafkaError(KafkaErrFailedToUpdate, "update failed")
				results.AddErrors(err1, err2)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple statuses including connection", func() {
			It("should populate connection and other statuses", func() {
				host := "test-host"
				port := "9092"
				results := InitResults()
				connInfo := KafkaConnInfo{
					Host: host,
					Port: port,
				}
				connStatus := NewKafkaConnDetail(connInfo)
				createdStatus := NewKafkaStatus(KafkaCreated, "created")
				updatedStatus := NewKafkaStatus(KafkaUpdated, "updated")
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Connection.KafkaHost).To(Equal(host))
				Expect(status.Connection.KafkaPort).To(Equal(port))
				Expect(status.Details).To(HaveLen(2))
			})
		})
	})

	Describe("Error codes", func() {
		It("should have distinct error codes", func() {
			codes := []KafkaErrorCode{
				KafkaErrFailedToGetConfig,
				KafkaErrFailedToInitialize,
				KafkaErrFailedToCreate,
				KafkaErrFailedToUpdate,
				KafkaErrFailedToDelete,
			}

			for i := range len(codes) {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})

	Describe("Status codes", func() {
		It("should have distinct status codes", func() {
			codes := []KafkaInfraCode{
				KafkaCreated,
				KafkaUpdated,
				KafkaDeleted,
				KafkaNodePoolCreated,
				KafkaNodePoolUpdated,
				KafkaNodePoolDeleted,
				KafkaConnection,
			}

			for i := range len(codes) {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})
})
