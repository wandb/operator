package model

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	Describe("Kafka Error", func() {
		Describe("NewKafkaError", func() {
			It("should create error with correct fields", func() {
				err := NewKafkaError(KafkaErrFailedToCreateCode, "test reason")
				Expect(err.InfraName()).To(Equal(Kafka))
				Expect(err.Code()).To(Equal(string(KafkaErrFailedToCreateCode)))
				Expect(err.Reason()).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewKafkaError(KafkaErrFailedToUpdateCode, "update failed")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("kafka"))
				Expect(errStr).To(ContainSubstring("update failed"))
			})
		})

		Describe("KafkaInfraError", func() {
			Describe("kafkaCode", func() {
				It("should return the error code", func() {
					infraErr := NewKafkaError(KafkaErrFailedToDeleteCode, "delete failed")
					kafkaErr := KafkaInfraError{infraErr}
					Expect(KafkaErrorCode(kafkaErr.Code())).To(Equal(KafkaErrFailedToDeleteCode))
				})
			})
		})

		Describe("ToKafkaInfraError", func() {
			Context("when error is a Kafka infra error", func() {
				It("should convert successfully", func() {
					err := NewKafkaError(KafkaErrFailedToGetConfigCode, "config error")
					kafkaErr, ok := ToKafkaInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(KafkaErrorCode(kafkaErr.Code())).To(Equal(KafkaErrFailedToGetConfigCode))
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
					err := NewInfraError(Redis, "SomeCode", "some reason")
					_, ok := ToKafkaInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Kafka Status", func() {
		Describe("NewKafkaStatusDetail", func() {
			It("should create status with correct fields", func() {
				status := NewKafkaStatusDetail(KafkaCreatedCode, "Kafka created")
				Expect(status.InfraName()).To(Equal(Kafka))
				Expect(status.Code()).To(Equal(string(KafkaCreatedCode)))
				Expect(status.Message()).To(Equal("Kafka created"))
			})
		})

		Describe("KafkaStatusDetail", func() {
			Describe("kafkaCode", func() {
				It("should return the status code", func() {
					status := NewKafkaStatusDetail(KafkaUpdatedCode, "updated")
					detail := KafkaStatusDetail{status}
					Expect(KafkaInfraCode(detail.Code())).To(Equal(KafkaUpdatedCode))
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
						status := NewKafkaStatusDetail(KafkaCreatedCode, "created")
						detail := KafkaStatusDetail{status}

						_, ok := detail.ToKafkaConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return true but with empty connection info", func() {
						status := NewInfraStatusDetail(Kafka, string(KafkaConnectionCode), "kafka://host:port", nil)
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

				Expect(status.InfraName()).To(Equal(Kafka))
				Expect(status.Code()).To(Equal(string(KafkaConnectionCode)))
				Expect(status.Message()).To(Equal("kafka://kafka-broker.example.com:9092"))
			})
		})
	})

	Describe("Results.ExtractKafkaStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return not ready state with no connection", func() {
				results := InitResults()
				status := ExtractKafkaStatus(ctx, results)

				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(BeEmpty())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have Kafka errors", func() {
			It("should include errors and not be ready", func() {
				results := InitResults()
				err := NewKafkaError(KafkaErrFailedToCreateCode, "failed to create")
				results.AddErrors(err)

				status := ExtractKafkaStatus(ctx, results)

				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Errors[0].Code()).To(Equal(string(KafkaErrFailedToCreateCode)))
				Expect(status.Errors[0].Reason()).To(Equal("failed to create"))
			})
		})

		Context("when results have non-Kafka errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := NewInfraError(Redis, "RedisError", "redis failed")
				results.AddErrors(err)

				status := ExtractKafkaStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(BeEmpty())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have Kafka created status", func() {
			It("should include status in details", func() {
				results := InitResults()
				infraStatus := NewKafkaStatusDetail(KafkaCreatedCode, "Kafka created successfully")
				results.AddStatuses(infraStatus)

				status := ExtractKafkaStatus(ctx, results)

				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].Code()).To(Equal(string(KafkaCreatedCode)))
				Expect(status.Details[0].Message()).To(Equal("Kafka created successfully"))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status and be ready", func() {
				host := "kafka.example.com"
				port := "9092"
				results := InitResults()
				connInfo := KafkaConnInfo{
					Host: host,
					Port: port,
				}
				connStatus := NewKafkaConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := ExtractKafkaStatus(ctx, results)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.Host).To(Equal(host))
				Expect(status.Connection.Port).To(Equal(port))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both errors and details, not be ready", func() {
				results := InitResults()
				err := NewKafkaError(KafkaErrFailedToUpdateCode, "update failed")
				infraStatus := NewKafkaStatusDetail(KafkaCreatedCode, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := ExtractKafkaStatus(ctx, results)

				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Details).To(HaveLen(1))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				results := InitResults()
				err1 := NewKafkaError(KafkaErrFailedToCreateCode, "create failed")
				err2 := NewKafkaError(KafkaErrFailedToUpdateCode, "update failed")
				results.AddErrors(err1, err2)

				status := ExtractKafkaStatus(ctx, results)

				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(2))
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
				createdStatus := NewKafkaStatusDetail(KafkaCreatedCode, "created")
				updatedStatus := NewKafkaStatusDetail(KafkaUpdatedCode, "updated")
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := ExtractKafkaStatus(ctx, results)

				Expect(status.Connection.Host).To(Equal(host))
				Expect(status.Connection.Port).To(Equal(port))
				Expect(status.Details).To(HaveLen(2))
			})
		})
	})

	Describe("Error codes", func() {
		It("should have distinct error codes", func() {
			codes := []KafkaErrorCode{
				KafkaErrFailedToGetConfigCode,
				KafkaErrFailedToInitializeCode,
				KafkaErrFailedToCreateCode,
				KafkaErrFailedToUpdateCode,
				KafkaErrFailedToDeleteCode,
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
				KafkaCreatedCode,
				KafkaUpdatedCode,
				KafkaDeletedCode,
				KafkaNodePoolCreatedCode,
				KafkaNodePoolUpdatedCode,
				KafkaNodePoolDeletedCode,
				KafkaConnectionCode,
			}

			for i := range len(codes) {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})

	Describe("BuildKafkaDefaults", func() {
		const testOwnerNamespace = "test-namespace"

		Context("when size is Dev", func() {
			It("should return complete dev defaults", func() {
				config, err := BuildKafkaDefaults(SizeDev, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(DevKafkaStorageSize))
				Expect(config.Replicas).To(Equal(int32(1)))
				Expect(config.ReplicationConfig.DefaultReplicationFactor).To(Equal(int32(1)))
				Expect(config.Resources.Requests).To(BeEmpty())
				Expect(config.Resources.Limits).To(BeEmpty())
			})
		})

		Context("when size is Small", func() {
			It("should return complete small defaults with all resource fields", func() {
				config, err := BuildKafkaDefaults(SizeSmall, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(SmallKafkaStorageSize))
				Expect(config.Replicas).To(Equal(int32(3)))
				Expect(config.ReplicationConfig.DefaultReplicationFactor).To(Equal(int32(3)))
				Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallKafkaCpuRequest)))
				Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallKafkaCpuLimit)))
				Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallKafkaMemoryRequest)))
				Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallKafkaMemoryLimit)))
			})
		})

		Context("when size is invalid", func() {
			It("should return error", func() {
				_, err := BuildKafkaDefaults(Size("invalid"), testOwnerNamespace)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for Kafka"))
			})
		})
	})
})
