package common

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MySQL Model", func() {

	Describe("MySQL Error", func() {
		Describe("NewMySQLError", func() {
			It("should create error with correct fields", func() {
				err := NewMySQLError(MySQLErrFailedToCreateCode, "test reason")
				Expect(err.InfraName()).To(Equal(MySQL))
				Expect(err.Code()).To(Equal(string(MySQLErrFailedToCreateCode)))
				Expect(err.Reason()).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewMySQLError(MySQLErrFailedToUpdateCode, "update failed")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("mysql"))
				Expect(errStr).To(ContainSubstring("update failed"))
			})
		})

		Describe("MySQLInfraError", func() {
			Describe("mysqlCode", func() {
				It("should return the error code", func() {
					infraErr := NewMySQLError(MySQLErrFailedToDeleteCode, "delete failed")
					mysqlErr := MySQLInfraError{infraErr}
					Expect(MySQLErrorCode(mysqlErr.Code())).To(Equal(MySQLErrFailedToDeleteCode))
				})
			})
		})

		Describe("ToMySQLInfraError", func() {
			Context("when error is a MySQL infra error", func() {
				It("should convert successfully", func() {
					err := NewMySQLError(MySQLErrFailedToGetConfigCode, "config error")
					mysqlErr, ok := ToMySQLInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(MySQLErrorCode(mysqlErr.Code())).To(Equal(MySQLErrFailedToGetConfigCode))
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
					err := NewInfraError(Redis, "SomeCode", "some reason")
					_, ok := ToMySQLInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("MySQL Status", func() {
		Describe("NewMySQLStatus", func() {
			It("should create status with correct fields", func() {
				status := NewMySQLStatusDetail(MySQLCreatedCode, "MySQL created")
				Expect(status.InfraName()).To(Equal(MySQL))
				Expect(status.Code()).To(Equal(string(MySQLCreatedCode)))
				Expect(status.Message()).To(Equal("MySQL created"))
			})
		})

		Describe("MySQLStatusDetail", func() {
			Describe("mysqlCode", func() {
				It("should return the status code", func() {
					status := NewMySQLStatusDetail(MySQLUpdatedCode, "updated")
					detail := MySQLStatusDetail{status}
					Expect(MySQLInfraCode(detail.Code())).To(Equal(MySQLUpdatedCode))
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
						status := NewMySQLStatusDetail(MySQLCreatedCode, "created")
						detail := MySQLStatusDetail{status}
						_, ok := detail.ToMySQLConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						status := NewInfraStatusDetail(MySQL, string(MySQLConnectionCode), "connection", "not a MySQLConnInfo")
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
				Expect(status.InfraName()).To(Equal(MySQL))
				Expect(status.Code()).To(Equal(string(MySQLConnectionCode)))
				Expect(status.Message()).To(Equal("MySQL connection info"))
				Expect(status.Hidden()).To(Equal(connInfo))
			})
		})

		Describe("InfraStatusDetail.ToMySQLStatusDetail", func() {
			Context("when infra status is for MySQL", func() {
				It("should convert successfully", func() {
					status := NewInfraStatusDetail(MySQL, "TestCode", "test message", "hidden data")
					detail, ok := status.ToMySQLStatusDetail()
					Expect(ok).To(BeTrue())
					Expect(detail.InfraName()).To(Equal(MySQL))
					Expect(detail.Code()).To(Equal("TestCode"))
					Expect(detail.Message()).To(Equal("test message"))
					Expect(detail.Hidden()).To(Equal("hidden data"))
				})
			})

			Context("when infra status is not for MySQL", func() {
				It("should return false", func() {
					status := NewInfraStatusDetail(Redis, "TestCode", "test message", nil)
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
			It("should return not ready state with no connection", func() {
				results := InitResults()
				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(BeEmpty())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have MySQL errors", func() {
			It("should include errors and not be ready", func() {
				results := InitResults()
				err := NewMySQLError(MySQLErrFailedToCreateCode, "failed to create")
				results.AddErrors(err)

				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Errors[0].Code()).To(Equal(string(MySQLErrFailedToCreateCode)))
				Expect(status.Errors[0].Reason()).To(Equal("failed to create"))
			})
		})

		Context("when results have non-MySQL errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := NewInfraError(Redis, "RedisError", "redis failed")
				results.AddErrors(err)

				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have MySQL statuses", func() {
			It("should include statuses in details", func() {
				results := InitResults()
				infraStatus := NewMySQLStatusDetail(MySQLCreatedCode, "MySQL created successfully")
				results.AddStatuses(infraStatus)

				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].Code()).To(Equal(string(MySQLCreatedCode)))
				Expect(status.Details[0].Message()).To(Equal("MySQL created successfully"))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status and be ready", func() {
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

				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.Host).To(Equal(host))
				Expect(status.Connection.Port).To(Equal(port))
				Expect(status.Connection.User).To(Equal(user))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both errors and details, not be ready", func() {
				results := InitResults()
				err := NewMySQLError(MySQLErrFailedToUpdateCode, "update failed")
				infraStatus := NewMySQLStatusDetail(MySQLCreatedCode, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Details).To(HaveLen(1))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				results := InitResults()
				err1 := NewMySQLError(MySQLErrFailedToCreateCode, "create failed")
				err2 := NewMySQLError(MySQLErrFailedToUpdateCode, "update failed")
				results.AddErrors(err1, err2)

				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(2))
			})
		})

		Context("when results have multiple statuses including connection", func() {
			It("should populate connection and other statuses and be ready", func() {
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
				createdStatus := NewMySQLStatusDetail(MySQLCreatedCode, "created")
				updatedStatus := NewMySQLStatusDetail(MySQLUpdatedCode, "updated")
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := ExtractMySQLStatus(ctx, results)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.Host).To(Equal(host))
				Expect(status.Connection.Port).To(Equal(port))
				Expect(status.Connection.User).To(Equal(user))
				Expect(status.Details).To(HaveLen(2))
			})
		})
	})

	Describe("Error codes", func() {
		It("should have distinct error codes", func() {
			codes := []MySQLErrorCode{
				MySQLErrFailedToGetConfigCode,
				MySQLErrFailedToInitializeCode,
				MySQLErrFailedToCreateCode,
				MySQLErrFailedToUpdateCode,
				MySQLErrFailedToDeleteCode,
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
				MySQLCreatedCode,
				MySQLUpdatedCode,
				MySQLDeletedCode,
				MySQLConnectionCode,
			}

			for i := 0; i < len(codes); i++ {
				for j := i + 1; j < len(codes); j++ {
					Expect(codes[i]).NotTo(Equal(codes[j]))
				}
			}
		})
	})
})
