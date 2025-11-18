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

var _ = Describe("ClickHouse Model", func() {
	Describe("ClickHouseConfig", func() {
		Describe("IsHighAvailability", func() {
			Context("when replicas is greater than 1", func() {
				It("should return true", func() {
					replicasOverride := int32(3)
					config := ClickHouseConfig{Replicas: replicasOverride}
					Expect(config.IsHighAvailability()).To(BeTrue())
				})
			})

			Context("when replicas is equal to 1", func() {
				It("should return false", func() {
					replicasOverride := int32(1)
					config := ClickHouseConfig{Replicas: replicasOverride}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})

			Context("when replicas is 0", func() {
				It("should return false", func() {
					replicasOverride := int32(0)
					config := ClickHouseConfig{Replicas: replicasOverride}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})
		})
	})

	Describe("ClickHouse Error", func() {
		Describe("NewClickHouseError", func() {
			It("should create error with correct fields", func() {
				reason := "test reason"
				err := NewClickHouseError(ClickHouseErrFailedToCreateCode, reason)
				Expect(err.InfraName).To(Equal(Clickhouse))
				Expect(err.code).To(Equal(string(ClickHouseErrFailedToCreateCode)))
				Expect(err.reason).To(Equal(reason))
			})

			It("should implement error interface", func() {
				reason := "update failed"
				err := NewClickHouseError(ClickHouseErrFailedToUpdateCode, reason)
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("clickhouse"))
				Expect(errStr).To(ContainSubstring(reason))
			})
		})

		Describe("ClickHouseInfraError", func() {
			Describe("clickhouseCode", func() {
				It("should return the error code", func() {
					reason := "delete failed"
					infraErr := NewClickHouseError(ClickHouseErrFailedToDeleteCode, reason)
					chErr := ClickHouseInfraError{infraErr}
					Expect(chErr.clickhouseCode()).To(Equal(ClickHouseErrFailedToDeleteCode))
				})
			})
		})

		Describe("ToClickHouseInfraError", func() {
			Context("when error is a ClickHouse infra error", func() {
				It("should convert successfully", func() {
					reason := "config error"
					err := NewClickHouseError(ClickHouseErrFailedToGetConfigCode, reason)
					chErr, ok := ToClickHouseInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(chErr.clickhouseCode()).To(Equal(ClickHouseErrFailedToGetConfigCode))
					Expect(chErr.reason).To(Equal(reason))
				})
			})

			Context("when error is not an infra error", func() {
				It("should return false", func() {
					message := "regular error"
					err := fmt.Errorf("%s", message)
					_, ok := ToClickHouseInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})

			Context("when error is an infra error but not ClickHouse", func() {
				It("should return false", func() {
					code := "SomeCode"
					reason := "some reason"
					err := InfraError{
						InfraName: Redis,
						Code:      code,
						reason:    reason,
					}
					_, ok := ToClickHouseInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("ClickHouse Status", func() {
		Describe("NewClickHouseStatusDetail", func() {
			It("should create status with correct fields", func() {
				message := "ClickHouse created"
				status := NewClickHouseStatusDetail(ClickHouseCreatedCode, message)
				Expect(status.InfraName).To(Equal(Clickhouse))
				Expect(status.code).To(Equal(string(ClickHouseCreatedCode)))
				Expect(status.message).To(Equal(message))
			})
		})

		Describe("ClickHouseStatusDetail", func() {
			Describe("clickhouseCode", func() {
				It("should return the status code", func() {
					message := "updated"
					status := NewClickHouseStatusDetail(ClickHouseUpdatedCode, message)
					detail := ClickHouseStatusDetail{status}
					Expect(detail.clickhouseCode()).To(Equal(ClickHouseUpdatedCode))
				})
			})

			Describe("ToClickHouseConnDetail", func() {
				Context("when status is connection type with connection info", func() {
					It("should convert successfully", func() {
						host := "clickhouse.example.com"
						port := "9000"
						user := "admin"
						connInfo := ClickHouseConnInfo{
							Host: host,
							Port: port,
							User: user,
						}
						status := NewClickHouseConnDetail(connInfo)
						detail := ClickHouseStatusDetail{status}
						connDetail, ok := detail.ToClickHouseConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal(host))
						Expect(connDetail.connInfo.Port).To(Equal(port))
						Expect(connDetail.connInfo.User).To(Equal(user))
					})
				})

				Context("when status is not connection type", func() {
					It("should return false", func() {
						message := "created"
						status := NewClickHouseStatusDetail(ClickHouseCreatedCode, message)
						detail := ClickHouseStatusDetail{status}
						_, ok := detail.ToClickHouseConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						message := "connection"
						hidden := "not a ClickHouseConnInfo"
						status := InfraStatusDetail{
							InfraName: Clickhouse,
							Code:      string(ClickHouseConnectionCode),
							Message:   message,
							Hidden:    hidden,
						}
						detail := ClickHouseStatusDetail{status}
						connDetail, ok := detail.ToClickHouseConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(BeEmpty())
						Expect(connDetail.connInfo.Port).To(BeEmpty())
						Expect(connDetail.connInfo.User).To(BeEmpty())
					})
				})
			})
		})

		Describe("NewClickHouseConnDetail", func() {
			It("should create connection detail with info", func() {
				host := "test-host"
				port := "8123"
				user := "test-user"
				connInfo := ClickHouseConnInfo{
					Host: host,
					Port: port,
					User: user,
				}
				status := NewClickHouseConnDetail(connInfo)
				Expect(status.InfraName).To(Equal(Clickhouse))
				Expect(status.code).To(Equal(string(ClickHouseConnectionCode)))
				Expect(status.message).To(Equal("ClickHouse connection info"))
				Expect(status.hidden).To(Equal(connInfo))
			})
		})

		Describe("InfraStatusDetail.ToClickHouseStatusDetail", func() {
			Context("when infra status is for ClickHouse", func() {
				It("should convert successfully", func() {
					code := "TestCode"
					message := "test message"
					hidden := "hidden data"
					status := InfraStatusDetail{
						InfraName: Clickhouse,
						Code:      code,
						Message:   message,
						Hidden:    hidden,
					}
					detail, ok := status.ToClickHouseStatusDetail()
					Expect(ok).To(BeTrue())
					Expect(detail.InfraName).To(Equal(Clickhouse))
					Expect(detail.code).To(Equal(code))
					Expect(detail.message).To(Equal(message))
					Expect(detail.hidden).To(Equal(hidden))
				})
			})

			Context("when infra status is not for ClickHouse", func() {
				It("should return false", func() {
					code := "TestCode"
					message := "test message"
					status := InfraStatusDetail{
						InfraName: Redis,
						Code:      code,
						Message:   message,
					}
					_, ok := status.ToClickHouseStatusDetail()
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Results.ExtractClickHouseStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return ready state", func() {
				results := InitResults()
				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have ClickHouse errors", func() {
			It("should include errors in status details with error state", func() {
				reason := "failed to create"
				results := InitResults()
				err := NewClickHouseError(ClickHouseErrFailedToCreateCode, reason)
				results.AddErrors(err)

				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateError))
				Expect(status.Details[0].Code).To(Equal(string(ClickHouseErrFailedToCreateCode)))
				Expect(status.Details[0].Message).To(Equal(reason))
			})
		})

		Context("when results have non-ClickHouse errors", func() {
			It("should not include them in status", func() {
				code := "RedisError"
				reason := "redis failed"
				results := InitResults()
				err := InfraError{
					InfraName: Redis,
					Code:      code,
					reason:    reason,
				}
				results.AddErrors(err)

				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have ClickHouse statuses", func() {
			It("should include statuses in details with ready state", func() {
				message := "ClickHouse created successfully"
				results := InitResults()
				infraStatus := NewClickHouseStatusDetail(ClickHouseCreatedCode, message)
				results.AddStatuses(infraStatus)

				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details[0].Code).To(Equal(string(ClickHouseCreatedCode)))
				Expect(status.Details[0].Message).To(Equal(message))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status", func() {
				host := "clickhouse.example.com"
				port := "9000"
				user := "admin"
				results := InitResults()
				connInfo := ClickHouseConnInfo{
					Host: host,
					Port: port,
					User: user,
				}
				connStatus := NewClickHouseConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Connection.ClickHouseHost).To(Equal(host))
				Expect(status.Connection.ClickHousePort).To(Equal(port))
				Expect(status.Connection.ClickHouseUser).To(Equal(user))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both in details with error state", func() {
				errorReason := "update failed"
				statusMessage := "created"
				results := InitResults()
				err := NewClickHouseError(ClickHouseErrFailedToUpdateCode, errorReason)
				infraStatus := NewClickHouseStatusDetail(ClickHouseCreatedCode, statusMessage)
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				createReason := "create failed"
				updateReason := "update failed"
				results := InitResults()
				err1 := NewClickHouseError(ClickHouseErrFailedToCreateCode, createReason)
				err2 := NewClickHouseError(ClickHouseErrFailedToUpdateCode, updateReason)
				results.AddErrors(err1, err2)

				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple statuses including connection", func() {
			It("should populate connection and other statuses", func() {
				host := "test-host"
				port := "8123"
				user := "test-user"
				createdMessage := "created"
				updatedMessage := "updated"
				results := InitResults()
				connInfo := ClickHouseConnInfo{
					Host: host,
					Port: port,
					User: user,
				}
				connStatus := NewClickHouseConnDetail(connInfo)
				createdStatus := NewClickHouseStatusDetail(ClickHouseCreatedCode, createdMessage)
				updatedStatus := NewClickHouseStatusDetail(ClickHouseUpdatedCode, updatedMessage)
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := ExtractClickHouseStatus(ctx, results)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Connection.ClickHouseHost).To(Equal(host))
				Expect(status.Connection.ClickHousePort).To(Equal(port))
				Expect(status.Connection.ClickHouseUser).To(Equal(user))
				Expect(status.Details).To(HaveLen(2))
			})
		})
	})

	Describe("Error codes", func() {
		It("should have distinct error codes", func() {
			codes := []ClickHouseErrorCode{
				ClickHouseErrFailedToGetConfigCode,
				ClickHouseErrFailedToInitializeCode,
				ClickHouseErrFailedToCreateCode,
				ClickHouseErrFailedToUpdateCode,
				ClickHouseErrFailedToDeleteCode,
			}

			for i := 0; i < len(codes); i++ {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})

	Describe("Status codes", func() {
		It("should have distinct status codes", func() {
			codes := []ClickHouseInfraCode{
				ClickHouseCreatedCode,
				ClickHouseUpdatedCode,
				ClickHouseDeletedCode,
				ClickHouseConnectionCode,
			}

			for i := 0; i < len(codes); i++ {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})

	Describe("BuildClickHouseDefaults", func() {
		const testOwnerNamespace = "test-namespace"

		Context("when size is Dev", func() {
			It("should return dev defaults with 1 replica", func() {
				config, err := BuildClickHouseDefaults(SizeDev, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Version).To(Equal(ClickHouseVersion))
				Expect(config.StorageSize).To(Equal(DevClickHouseStorageSize))
				Expect(config.Replicas).To(Equal(int32(1)))
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.Resources.Requests).To(BeEmpty())
				Expect(config.Resources.Limits).To(BeEmpty())
			})
		})

		Context("when size is Small", func() {
			It("should return small defaults with 3 replicas and resources", func() {
				config, err := BuildClickHouseDefaults(SizeSmall, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Version).To(Equal(ClickHouseVersion))
				Expect(config.StorageSize).To(Equal(SmallClickHouseStorageSize))
				Expect(config.Replicas).To(Equal(int32(3)))
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallClickHouseCpuRequest)))
				Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallClickHouseCpuLimit)))
				Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallClickHouseMemoryRequest)))
				Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallClickHouseMemoryLimit)))
			})
		})

		Context("when size is invalid", func() {
			It("should return error", func() {
				_, err := BuildClickHouseDefaults(Size("invalid"), testOwnerNamespace)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for ClickHouse"))
			})
		})
	})
})
