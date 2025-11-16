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
		Describe("GetKafkaConfig", func() {
			Context("when merged Kafka is nil", func() {
				It("should return empty config", func() {
					builder := &InfraConfigBuilder{}
					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Enabled).To(BeFalse())
					Expect(config.Namespace).To(BeEmpty())
					Expect(config.StorageSize).To(BeEmpty())
					Expect(config.Replicas).To(BeZero())
				})
			})

			Context("when merged Kafka has values with dev size", func() {
				It("should return config with dev defaults", func() {
					spec := &apiv2.WBKafkaSpec{
						Enabled:     true,
						Namespace:   "test-namespace",
						StorageSize: "1Gi",
					}
					builder := &InfraConfigBuilder{
						mergedKafka: spec,
						size:        apiv2.WBSizeDev,
					}

					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal("test-namespace"))
					Expect(config.StorageSize).To(Equal("1Gi"))
					Expect(config.Replicas).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.DefaultReplicationFactor).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.MinInSyncReplicas).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.OffsetsTopicRF).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.TransactionStateRF).To(Equal(int32(1)))
					Expect(config.ReplicationConfig.TransactionStateISR).To(Equal(int32(1)))
				})
			})

			Context("when merged Kafka has values with small size", func() {
				It("should return config with small defaults", func() {
					spec := &apiv2.WBKafkaSpec{
						Enabled:     true,
						Namespace:   "prod-namespace",
						StorageSize: "5Gi",
					}
					builder := &InfraConfigBuilder{
						mergedKafka: spec,
						size:        apiv2.WBSizeSmall,
					}

					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal("prod-namespace"))
					Expect(config.StorageSize).To(Equal("5Gi"))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.DefaultReplicationFactor).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.MinInSyncReplicas).To(Equal(int32(2)))
					Expect(config.ReplicationConfig.OffsetsTopicRF).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.TransactionStateRF).To(Equal(int32(3)))
					Expect(config.ReplicationConfig.TransactionStateISR).To(Equal(int32(2)))
				})
			})

			Context("when merged Kafka has resource config", func() {
				It("should include resources in config", func() {
					spec := &apiv2.WBKafkaSpec{
						Enabled:     true,
						Namespace:   "test-namespace",
						StorageSize: "10Gi",
						Config: &apiv2.WBKafkaConfig{
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("500m"),
									v1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("1000m"),
									v1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
						},
					}
					builder := &InfraConfigBuilder{
						mergedKafka: spec,
						size:        apiv2.WBSizeSmall,
					}

					config, err := builder.GetKafkaConfig()
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Resources.Requests.Cpu().String()).To(Equal("500m"))
					Expect(config.Resources.Requests.Memory().String()).To(Equal("1Gi"))
					Expect(config.Resources.Limits.Cpu().String()).To(Equal("1"))
					Expect(config.Resources.Limits.Memory().String()).To(Equal("2Gi"))
				})
			})

			Context("when size is unsupported", func() {
				It("should return error", func() {
					spec := &apiv2.WBKafkaSpec{
						Enabled:     true,
						Namespace:   "test-namespace",
						StorageSize: "10Gi",
					}
					builder := &InfraConfigBuilder{
						mergedKafka: spec,
						size:        apiv2.WBSize("large"),
					}

					config, err := builder.GetKafkaConfig()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unsupported size for Kafka"))
					Expect(config.Replicas).To(Equal(int32(0)))
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
						connInfo := KafkaConnInfo{
							Host: "kafka.example.com",
							Port: "9092",
						}
						status := NewKafkaConnDetail(connInfo)
						detail := KafkaStatusDetail{status}

						connDetail, ok := detail.ToKafkaConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal("kafka.example.com"))
						Expect(connDetail.connInfo.Port).To(Equal("9092"))
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
				connInfo := KafkaConnInfo{
					Host: "kafka-broker.example.com",
					Port: "9092",
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

		Context("when results have Kafka updated status", func() {
			It("should include status in details with updating state", func() {
				results := InitResults()
				infraStatus := NewKafkaStatus(KafkaUpdated, "Kafka updated")
				results.AddStatuses(infraStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			})
		})

		Context("when results have Kafka deleted status", func() {
			It("should include status in details with deleting state", func() {
				results := InitResults()
				infraStatus := NewKafkaStatus(KafkaDeleted, "Kafka deleted")
				results.AddStatuses(infraStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			})
		})

		Context("when results have node pool created status", func() {
			It("should include status in details with updating state", func() {
				results := InitResults()
				infraStatus := NewKafkaStatus(KafkaNodePoolCreated, "NodePool created")
				results.AddStatuses(infraStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			})
		})

		Context("when results have node pool updated status", func() {
			It("should include status in details with updating state", func() {
				results := InitResults()
				infraStatus := NewKafkaStatus(KafkaNodePoolUpdated, "NodePool updated")
				results.AddStatuses(infraStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateUpdating))
			})
		})

		Context("when results have node pool deleted status", func() {
			It("should include status in details with deleting state", func() {
				results := InitResults()
				infraStatus := NewKafkaStatus(KafkaNodePoolDeleted, "NodePool deleted")
				results.AddStatuses(infraStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateDeleting))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status", func() {
				results := InitResults()
				connInfo := KafkaConnInfo{
					Host: "kafka.example.com",
					Port: "9092",
				}
				connStatus := NewKafkaConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.KafkaHost).To(Equal("kafka.example.com"))
				Expect(status.Connection.KafkaPort).To(Equal("9092"))
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
				results := InitResults()
				connInfo := KafkaConnInfo{
					Host: "test-host",
					Port: "9092",
				}
				connStatus := NewKafkaConnDetail(connInfo)
				createdStatus := NewKafkaStatus(KafkaCreated, "created")
				updatedStatus := NewKafkaStatus(KafkaUpdated, "updated")
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
				Expect(status.Connection.KafkaHost).To(Equal("test-host"))
				Expect(status.Connection.KafkaPort).To(Equal("9092"))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have deleting and error states", func() {
			It("should mark as not ready and pick worst state", func() {
				results := InitResults()
				err := NewKafkaError(KafkaErrFailedToDelete, "delete failed")
				deleteStatus := NewKafkaStatus(KafkaDeleted, "deleting")
				results.AddErrors(err)
				results.AddStatuses(deleteStatus)

				status := results.ExtractKafkaStatus(ctx)
				Expect(status.Ready).To(BeFalse())
				Expect(status.State).To(Equal(apiv2.WBStateType("")))
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
