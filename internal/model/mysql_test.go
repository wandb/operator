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

var _ = Describe("MySQL Model", func() {
	Describe("MySQLConfig", func() {
		Describe("IsHighAvailability", func() {
			Context("when replicas is greater than 1", func() {
				It("should return true", func() {
					config := MySQLConfig{Replicas: 3}
					Expect(config.IsHighAvailability()).To(BeTrue())
				})
			})

			Context("when replicas is equal to 1", func() {
				It("should return false", func() {
					config := MySQLConfig{Replicas: 1}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})

			Context("when replicas is 0", func() {
				It("should return false", func() {
					config := MySQLConfig{Replicas: 0}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})
		})
	})

	Describe("GetMySQLReplicaCountForSize", func() {
		Context("when size is dev", func() {
			It("should return 1 replica", func() {
				count, err := GetMySQLReplicaCountForSize(apiv2.WBSizeDev)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int32(1)))
			})
		})

		Context("when size is small", func() {
			It("should return 3 replicas", func() {
				count, err := GetMySQLReplicaCountForSize(apiv2.WBSizeSmall)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int32(3)))
			})
		})

		Context("when size is unsupported", func() {
			It("should return error", func() {
				count, err := GetMySQLReplicaCountForSize(apiv2.WBSize("large"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for MySQL"))
				Expect(count).To(Equal(int32(0)))
			})
		})
	})

	Describe("GetMySQLConfigForSize", func() {
		Context("when size is dev", func() {
			It("should return dev config", func() {
				config, err := GetMySQLConfigForSize(apiv2.WBSizeDev)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.PXCImage).To(Equal(DevPXCImage))
				Expect(config.ProxySQLEnabled).To(BeFalse())
				Expect(config.ProxySQLReplicas).To(Equal(int32(0)))
				Expect(config.ProxySQLImage).To(BeEmpty())
				Expect(config.TLSEnabled).To(BeFalse())
				Expect(config.LogCollectorEnabled).To(BeTrue())
				Expect(config.LogCollectorImage).To(Equal(LogCollectorImage))
				Expect(config.AllowUnsafePXCSize).To(BeTrue())
				Expect(config.AllowUnsafeProxySize).To(BeTrue())
			})
		})

		Context("when size is small", func() {
			It("should return small config", func() {
				config, err := GetMySQLConfigForSize(apiv2.WBSizeSmall)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.PXCImage).To(Equal(SmallPXCImage))
				Expect(config.ProxySQLEnabled).To(BeTrue())
				Expect(config.ProxySQLReplicas).To(Equal(int32(3)))
				Expect(config.ProxySQLImage).To(Equal(ProxySQLImage))
				Expect(config.TLSEnabled).To(BeTrue())
				Expect(config.LogCollectorEnabled).To(BeFalse())
				Expect(config.LogCollectorImage).To(BeEmpty())
				Expect(config.AllowUnsafePXCSize).To(BeFalse())
				Expect(config.AllowUnsafeProxySize).To(BeFalse())
			})
		})

		Context("when size is unsupported", func() {
			It("should return error", func() {
				config, err := GetMySQLConfigForSize(apiv2.WBSize("medium"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for MySQL"))
				Expect(config).To(Equal(MySQLSizeConfig{}))
			})
		})
	})

	Describe("MySQL Error", func() {
		Describe("NewMySQLError", func() {
			It("should create error with correct fields", func() {
				err := NewMySQLError(MySQLErrFailedToCreate, "test reason")
				Expect(err.infraName).To(Equal(MySQL))
				Expect(err.code).To(Equal(string(MySQLErrFailedToCreate)))
				Expect(err.reason).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewMySQLError(MySQLErrFailedToUpdate, "update failed")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("mysql"))
				Expect(errStr).To(ContainSubstring("update failed"))
			})
		})

		Describe("MySQLInfraError", func() {
			Describe("mysqlCode", func() {
				It("should return the error code", func() {
					infraErr := NewMySQLError(MySQLErrFailedToDelete, "delete failed")
					mysqlErr := MySQLInfraError{infraErr}
					Expect(mysqlErr.mysqlCode()).To(Equal(MySQLErrFailedToDelete))
				})
			})
		})

		Describe("ToMySQLInfraError", func() {
			Context("when error is a MySQL infra error", func() {
				It("should convert successfully", func() {
					err := NewMySQLError(MySQLErrFailedToGetConfig, "config error")
					mysqlErr, ok := ToMySQLInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(mysqlErr.mysqlCode()).To(Equal(MySQLErrFailedToGetConfig))
					Expect(mysqlErr.reason).To(Equal("config error"))
				})
			})

			Context("when error is not an infra error", func() {
				It("should return false", func() {
					err := fmt.Errorf("regular error")
					_, ok := ToMySQLInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})

			Context("when error is an infra error but not MySQL", func() {
				It("should return false", func() {
					err := InfraError{
						infraName: Redis,
						code:      "SomeCode",
						reason:    "some reason",
					}
					_, ok := ToMySQLInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("MySQL Status", func() {
		Describe("NewMySQLStatus", func() {
			It("should create status with correct fields", func() {
				status := NewMySQLStatus(MySQLCreated, "MySQL created")
				Expect(status.infraName).To(Equal(MySQL))
				Expect(status.code).To(Equal(string(MySQLCreated)))
				Expect(status.message).To(Equal("MySQL created"))
			})
		})

		Describe("MySQLStatusDetail", func() {
			Describe("mysqlCode", func() {
				It("should return the status code", func() {
					status := NewMySQLStatus(MySQLUpdated, "updated")
					detail := MySQLStatusDetail{status}
					Expect(detail.mysqlCode()).To(Equal(MySQLUpdated))
				})
			})

			Describe("ToMySQLConnDetail", func() {
				Context("when status is connection type with connection info", func() {
					It("should convert successfully", func() {
						host := "mysql.example.com"
						port := "3306"
						user := "admin"
						connInfo := MySQLConnInfo{
							Host: host,
							Port: port,
							User: user,
						}
						status := NewMySQLConnDetail(connInfo)
						detail := MySQLStatusDetail{status}
						connDetail, ok := detail.ToMySQLConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal(host))
						Expect(connDetail.connInfo.Port).To(Equal(port))
						Expect(connDetail.connInfo.User).To(Equal(user))
					})
				})

				Context("when status is not connection type", func() {
					It("should return false", func() {
						status := NewMySQLStatus(MySQLCreated, "created")
						detail := MySQLStatusDetail{status}
						_, ok := detail.ToMySQLConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						status := InfraStatus{
							infraName: MySQL,
							code:      string(MySQLConnection),
							message:   "connection",
							hidden:    "not a MySQLConnInfo",
						}
						detail := MySQLStatusDetail{status}
						connDetail, ok := detail.ToMySQLConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(BeEmpty())
						Expect(connDetail.connInfo.Port).To(BeEmpty())
						Expect(connDetail.connInfo.User).To(BeEmpty())
					})
				})
			})
		})

		Describe("NewMySQLConnDetail", func() {
			It("should create connection detail with info", func() {
				host := "test-host"
				port := "3306"
				user := "test-user"
				connInfo := MySQLConnInfo{
					Host: host,
					Port: port,
					User: user,
				}
				status := NewMySQLConnDetail(connInfo)
				Expect(status.infraName).To(Equal(MySQL))
				Expect(status.code).To(Equal(string(MySQLConnection)))
				Expect(status.message).To(Equal("MySQL connection info"))
				Expect(status.hidden).To(Equal(connInfo))
			})
		})

		Describe("InfraStatus.ToMySQLStatusDetail", func() {
			Context("when infra status is for MySQL", func() {
				It("should convert successfully", func() {
					status := InfraStatus{
						infraName: MySQL,
						code:      "TestCode",
						message:   "test message",
						hidden:    "hidden data",
					}
					detail, ok := status.ToMySQLStatusDetail()
					Expect(ok).To(BeTrue())
					Expect(detail.infraName).To(Equal(MySQL))
					Expect(detail.code).To(Equal("TestCode"))
					Expect(detail.message).To(Equal("test message"))
					Expect(detail.hidden).To(Equal("hidden data"))
				})
			})

			Context("when infra status is not for MySQL", func() {
				It("should return false", func() {
					status := InfraStatus{
						infraName: Redis,
						code:      "TestCode",
						message:   "test message",
					}
					_, ok := status.ToMySQLStatusDetail()
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Results.ExtractMySQLStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return ready state", func() {
				results := InitResults()
				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have MySQL errors", func() {
			It("should include errors in status details with error state", func() {
				results := InitResults()
				err := NewMySQLError(MySQLErrFailedToCreate, "failed to create")
				results.AddErrors(err)

				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateError))
				Expect(status.Details[0].Code).To(Equal(string(MySQLErrFailedToCreate)))
				Expect(status.Details[0].Message).To(Equal("failed to create"))
			})
		})

		Context("when results have non-MySQL errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := InfraError{
					infraName: Redis,
					code:      "RedisError",
					reason:    "redis failed",
				}
				results.AddErrors(err)

				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have MySQL statuses", func() {
			It("should include statuses in details with ready state", func() {
				results := InitResults()
				infraStatus := NewMySQLStatus(MySQLCreated, "MySQL created successfully")
				results.AddStatuses(infraStatus)

				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details[0].Code).To(Equal(string(MySQLCreated)))
				Expect(status.Details[0].Message).To(Equal("MySQL created successfully"))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status", func() {
				host := "mysql.example.com"
				port := "3306"
				user := "admin"
				results := InitResults()
				connInfo := MySQLConnInfo{
					Host: host,
					Port: port,
					User: user,
				}
				connStatus := NewMySQLConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Connection.MySQLHost).To(Equal(host))
				Expect(status.Connection.MySQLPort).To(Equal(port))
				Expect(status.Connection.MySQLUser).To(Equal(user))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both in details with error state", func() {
				results := InitResults()
				err := NewMySQLError(MySQLErrFailedToUpdate, "update failed")
				infraStatus := NewMySQLStatus(MySQLCreated, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				results := InitResults()
				err1 := NewMySQLError(MySQLErrFailedToCreate, "create failed")
				err2 := NewMySQLError(MySQLErrFailedToUpdate, "update failed")
				results.AddErrors(err1, err2)

				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple statuses including connection", func() {
			It("should populate connection and other statuses", func() {
				host := "test-host"
				port := "3306"
				user := "test-user"
				results := InitResults()
				connInfo := MySQLConnInfo{
					Host: host,
					Port: port,
					User: user,
				}
				connStatus := NewMySQLConnDetail(connInfo)
				createdStatus := NewMySQLStatus(MySQLCreated, "created")
				updatedStatus := NewMySQLStatus(MySQLUpdated, "updated")
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := results.ExtractMySQLStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Connection.MySQLHost).To(Equal(host))
				Expect(status.Connection.MySQLPort).To(Equal(port))
				Expect(status.Connection.MySQLUser).To(Equal(user))
				Expect(status.Details).To(HaveLen(2))
			})
		})
	})

	Describe("Error codes", func() {
		It("should have distinct error codes", func() {
			codes := []MySQLErrorCode{
				MySQLErrFailedToGetConfig,
				MySQLErrFailedToInitialize,
				MySQLErrFailedToCreate,
				MySQLErrFailedToUpdate,
				MySQLErrFailedToDelete,
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
			codes := []MySQLInfraCode{
				MySQLCreated,
				MySQLUpdated,
				MySQLDeleted,
				MySQLConnection,
			}

			for i := 0; i < len(codes); i++ {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})

	Describe("BuildMySQLDefaults", func() {
		const testOwnerNamespace = "test-namespace"

		Context("when size is Dev", func() {
			It("should return a MySQL config with storage only and no resources", func() {
				config, err := BuildMySQLDefaults(SizeDev, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(DevMySQLStorageSize))
				Expect(config.Replicas).To(Equal(int32(1)))
				Expect(config.PXCImage).To(Equal(DevPXCImage))
				Expect(config.ProxySQLEnabled).To(BeFalse())
				Expect(config.TLSEnabled).To(BeFalse())
				Expect(config.LogCollectorEnabled).To(BeTrue())
				Expect(config.Resources.Requests).To(BeEmpty())
				Expect(config.Resources.Limits).To(BeEmpty())
			})
		})

		Context("when size is Small", func() {
			It("should return a MySQL config with full resource requirements", func() {
				config, err := BuildMySQLDefaults(SizeSmall, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(SmallMySQLStorageSize))
				Expect(config.Replicas).To(Equal(int32(3)))
				Expect(config.PXCImage).To(Equal(SmallPXCImage))
				Expect(config.ProxySQLEnabled).To(BeTrue())
				Expect(config.ProxySQLReplicas).To(Equal(int32(3)))
				Expect(config.TLSEnabled).To(BeTrue())
				Expect(config.LogCollectorEnabled).To(BeFalse())
				Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallMySQLCpuRequest)))
				Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallMySQLCpuLimit)))
				Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallMySQLMemoryRequest)))
				Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallMySQLMemoryLimit)))
			})
		})

		Context("when size is invalid", func() {
			It("should return an error", func() {
				_, err := BuildMySQLDefaults(Size("invalid"), testOwnerNamespace)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for MySQL"))
			})
		})
	})
})
