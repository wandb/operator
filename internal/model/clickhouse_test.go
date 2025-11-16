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
					config := ClickHouseConfig{Replicas: 3}
					Expect(config.IsHighAvailability()).To(BeTrue())
				})
			})

			Context("when replicas is equal to 1", func() {
				It("should return false", func() {
					config := ClickHouseConfig{Replicas: 1}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})

			Context("when replicas is 0", func() {
				It("should return false", func() {
					config := ClickHouseConfig{Replicas: 0}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})
		})
	})

	Describe("InfraConfigBuilder", func() {
		Describe("GetClickHouseConfig", func() {
			Context("when merged ClickHouse is nil", func() {
				It("should return empty config", func() {
					builder := &InfraConfigBuilder{}
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Enabled).To(BeFalse())
					Expect(config.Namespace).To(BeEmpty())
					Expect(config.StorageSize).To(BeEmpty())
					Expect(config.Replicas).To(BeZero())
					Expect(config.Version).To(BeEmpty())
				})
			})

			Context("when merged ClickHouse has values", func() {
				It("should return config with all values", func() {
					spec := &apiv2.WBClickHouseSpec{
						Enabled:     true,
						Namespace:   "test-namespace",
						StorageSize: "20Gi",
						Replicas:    3,
						Version:     "24.1",
					}
					builder := &InfraConfigBuilder{
						mergedClickHouse: spec,
					}

					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal("test-namespace"))
					Expect(config.StorageSize).To(Equal("20Gi"))
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Version).To(Equal("24.1"))
				})
			})

			Context("when merged ClickHouse has config with resources", func() {
				It("should return config with resources", func() {
					resources := v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("500m"),
							v1.ResourceMemory: resource.MustParse("1Gi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1000m"),
							v1.ResourceMemory: resource.MustParse("2Gi"),
						},
					}
					spec := &apiv2.WBClickHouseSpec{
						Enabled:     true,
						Namespace:   "test-namespace",
						StorageSize: "20Gi",
						Config: &apiv2.WBClickHouseConfig{
							Resources: resources,
						},
					}
					builder := &InfraConfigBuilder{
						mergedClickHouse: spec,
					}

					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())
					Expect(config.Resources.Requests).To(Equal(resources.Requests))
					Expect(config.Resources.Limits).To(Equal(resources.Limits))
				})
			})
		})

		Describe("AddClickHouseSpec", func() {
			Context("when build defaults succeeds", func() {
				It("should add merged spec to builder", func() {
					actual := apiv2.WBClickHouseSpec{
						Enabled:     true,
						StorageSize: "20Gi",
					}
					builder := BuildInfraConfig().AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					Expect(builder.mergedClickHouse).ToNot(BeNil())
					Expect(builder.size).To(Equal(apiv2.WBSizeSmall))
				})
			})

			Context("when build spec succeeds with dev size", func() {
				It("should set size to dev", func() {
					actual := apiv2.WBClickHouseSpec{
						Enabled: true,
					}
					builder := BuildInfraConfig().AddClickHouseSpec(&actual, apiv2.WBSizeDev)

					Expect(builder.errors).To(BeEmpty())
					Expect(builder.size).To(Equal(apiv2.WBSizeDev))
				})
			})
		})
	})

	Describe("ClickHouse Error", func() {
		Describe("NewClickHouseError", func() {
			It("should create error with correct fields", func() {
				err := NewClickHouseError(ClickHouseErrFailedToCreate, "test reason")
				Expect(err.infraName).To(Equal(Clickhouse))
				Expect(err.code).To(Equal(string(ClickHouseErrFailedToCreate)))
				Expect(err.reason).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewClickHouseError(ClickHouseErrFailedToUpdate, "update failed")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("clickhouse"))
				Expect(errStr).To(ContainSubstring("update failed"))
			})
		})

		Describe("ClickHouseInfraError", func() {
			Describe("clickhouseCode", func() {
				It("should return the error code", func() {
					infraErr := NewClickHouseError(ClickHouseErrFailedToDelete, "delete failed")
					chErr := ClickHouseInfraError{infraErr}
					Expect(chErr.clickhouseCode()).To(Equal(ClickHouseErrFailedToDelete))
				})
			})
		})

		Describe("ToClickHouseInfraError", func() {
			Context("when error is a ClickHouse infra error", func() {
				It("should convert successfully", func() {
					err := NewClickHouseError(ClickHouseErrFailedToGetConfig, "config error")
					chErr, ok := ToClickHouseInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(chErr.clickhouseCode()).To(Equal(ClickHouseErrFailedToGetConfig))
					Expect(chErr.reason).To(Equal("config error"))
				})
			})

			Context("when error is not an infra error", func() {
				It("should return false", func() {
					err := fmt.Errorf("regular error")
					_, ok := ToClickHouseInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})

			Context("when error is an infra error but not ClickHouse", func() {
				It("should return false", func() {
					err := InfraError{
						infraName: Redis,
						code:      "SomeCode",
						reason:    "some reason",
					}
					_, ok := ToClickHouseInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("ClickHouse Status", func() {
		Describe("NewClickHouseStatus", func() {
			It("should create status with correct fields", func() {
				status := NewClickHouseStatus(ClickHouseCreated, "ClickHouse created")
				Expect(status.infraName).To(Equal(Clickhouse))
				Expect(status.code).To(Equal(string(ClickHouseCreated)))
				Expect(status.message).To(Equal("ClickHouse created"))
			})
		})

		Describe("ClickHouseStatusDetail", func() {
			Describe("clickhouseCode", func() {
				It("should return the status code", func() {
					status := NewClickHouseStatus(ClickHouseUpdated, "updated")
					detail := ClickHouseStatusDetail{status}
					Expect(detail.clickhouseCode()).To(Equal(ClickHouseUpdated))
				})
			})

			Describe("ToClickHouseConnDetail", func() {
				Context("when status is connection type with connection info", func() {
					It("should convert successfully", func() {
						connInfo := ClickHouseConnInfo{
							Host: "clickhouse.example.com",
							Port: "9000",
							User: "admin",
						}
						status := NewClickHouseConnDetail(connInfo)
						detail := ClickHouseStatusDetail{status}
						connDetail, ok := detail.ToClickHouseConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal("clickhouse.example.com"))
						Expect(connDetail.connInfo.Port).To(Equal("9000"))
						Expect(connDetail.connInfo.User).To(Equal("admin"))
					})
				})

				Context("when status is not connection type", func() {
					It("should return false", func() {
						status := NewClickHouseStatus(ClickHouseCreated, "created")
						detail := ClickHouseStatusDetail{status}
						_, ok := detail.ToClickHouseConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						status := InfraStatus{
							infraName: Clickhouse,
							code:      string(ClickHouseConnection),
							message:   "connection",
							hidden:    "not a ClickHouseConnInfo",
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
				connInfo := ClickHouseConnInfo{
					Host: "test-host",
					Port: "8123",
					User: "test-user",
				}
				status := NewClickHouseConnDetail(connInfo)
				Expect(status.infraName).To(Equal(Clickhouse))
				Expect(status.code).To(Equal(string(ClickHouseConnection)))
				Expect(status.message).To(Equal("ClickHouse connection info"))
				Expect(status.hidden).To(Equal(connInfo))
			})
		})

		Describe("InfraStatus.ToClickHouseStatusDetail", func() {
			Context("when infra status is for ClickHouse", func() {
				It("should convert successfully", func() {
					status := InfraStatus{
						infraName: Clickhouse,
						code:      "TestCode",
						message:   "test message",
						hidden:    "hidden data",
					}
					detail, ok := status.ToClickHouseStatusDetail()
					Expect(ok).To(BeTrue())
					Expect(detail.infraName).To(Equal(Clickhouse))
					Expect(detail.code).To(Equal("TestCode"))
					Expect(detail.message).To(Equal("test message"))
					Expect(detail.hidden).To(Equal("hidden data"))
				})
			})

			Context("when infra status is not for ClickHouse", func() {
				It("should return false", func() {
					status := InfraStatus{
						infraName: Redis,
						code:      "TestCode",
						message:   "test message",
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
				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have ClickHouse errors", func() {
			It("should include errors in status details with error state", func() {
				results := InitResults()
				err := NewClickHouseError(ClickHouseErrFailedToCreate, "failed to create")
				results.AddErrors(err)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateError))
				Expect(status.Details[0].Code).To(Equal(string(ClickHouseErrFailedToCreate)))
				Expect(status.Details[0].Message).To(Equal("failed to create"))
			})
		})

		Context("when results have non-ClickHouse errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := InfraError{
					infraName: Redis,
					code:      "RedisError",
					reason:    "redis failed",
				}
				results.AddErrors(err)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have ClickHouse statuses", func() {
			It("should include statuses in details with ready state", func() {
				results := InitResults()
				infraStatus := NewClickHouseStatus(ClickHouseCreated, "ClickHouse created successfully")
				results.AddStatuses(infraStatus)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details[0].Code).To(Equal(string(ClickHouseCreated)))
				Expect(status.Details[0].Message).To(Equal("ClickHouse created successfully"))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status", func() {
				results := InitResults()
				connInfo := ClickHouseConnInfo{
					Host: "clickhouse.example.com",
					Port: "9000",
					User: "admin",
				}
				connStatus := NewClickHouseConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Connection.ClickHouseHost).To(Equal("clickhouse.example.com"))
				Expect(status.Connection.ClickHousePort).To(Equal("9000"))
				Expect(status.Connection.ClickHouseUser).To(Equal("admin"))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both in details with error state", func() {
				results := InitResults()
				err := NewClickHouseError(ClickHouseErrFailedToUpdate, "update failed")
				infraStatus := NewClickHouseStatus(ClickHouseCreated, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				results := InitResults()
				err1 := NewClickHouseError(ClickHouseErrFailedToCreate, "create failed")
				err2 := NewClickHouseError(ClickHouseErrFailedToUpdate, "update failed")
				results.AddErrors(err1, err2)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple statuses including connection", func() {
			It("should populate connection and other statuses", func() {
				results := InitResults()
				connInfo := ClickHouseConnInfo{
					Host: "test-host",
					Port: "8123",
					User: "test-user",
				}
				connStatus := NewClickHouseConnDetail(connInfo)
				createdStatus := NewClickHouseStatus(ClickHouseCreated, "created")
				updatedStatus := NewClickHouseStatus(ClickHouseUpdated, "updated")
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Connection.ClickHouseHost).To(Equal("test-host"))
				Expect(status.Connection.ClickHousePort).To(Equal("8123"))
				Expect(status.Connection.ClickHouseUser).To(Equal("test-user"))
				Expect(status.Details).To(HaveLen(2))
			})
		})
	})

	Describe("Error codes", func() {
		It("should have distinct error codes", func() {
			codes := []ClickHouseErrorCode{
				ClickHouseErrFailedToGetConfig,
				ClickHouseErrFailedToInitialize,
				ClickHouseErrFailedToCreate,
				ClickHouseErrFailedToUpdate,
				ClickHouseErrFailedToDelete,
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
				ClickHouseCreated,
				ClickHouseUpdated,
				ClickHouseDeleted,
				ClickHouseConnection,
			}

			for i := 0; i < len(codes); i++ {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})
})
