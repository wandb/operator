package common

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Interface", func() {
	Describe("IsInfraError", func() {
		Context("when error is an InfraError", func() {
			It("should return true with no specific infraNames", func() {
				err := NewMySQLError(MySQLErrFailedToCreateCode, "test error")
				Expect(IsInfraError(err)).To(BeTrue())
			})

			It("should return true when infraName matches", func() {
				err := NewMySQLError(MySQLErrFailedToCreateCode, "test error")
				Expect(IsInfraError(err, MySQL)).To(BeTrue())
			})

			It("should return true when one of multiple infraNames matches", func() {
				err := NewRedisError(RedisDeploymentConflictCode, "test error")
				Expect(IsInfraError(err, MySQL, Redis, Kafka)).To(BeTrue())
			})

			It("should return false when infraName does not match", func() {
				err := NewMySQLError(MySQLErrFailedToCreateCode, "test error")
				Expect(IsInfraError(err, Redis)).To(BeFalse())
			})
		})

		Context("when error is not an InfraError", func() {
			It("should return false", func() {
				err := fmt.Errorf("regular error")
				Expect(IsInfraError(err)).To(BeFalse())
			})

			It("should return false even with infraNames specified", func() {
				err := fmt.Errorf("regular error")
				Expect(IsInfraError(err, MySQL, Redis)).To(BeFalse())
			})
		})
	})

	Describe("HasCriticalError", func() {
		Context("when error list contains non-InfraErrors", func() {
			It("should return true", func() {
				errors := []error{
					NewMySQLError(MySQLErrFailedToCreateCode, "infra error"),
					fmt.Errorf("critical error"),
				}
				Expect(HasCriticalError(errors)).To(BeTrue())
			})

			It("should return true when only non-InfraErrors present", func() {
				errors := []error{
					fmt.Errorf("critical error 1"),
					fmt.Errorf("critical error 2"),
				}
				Expect(HasCriticalError(errors)).To(BeTrue())
			})
		})

		Context("when error list contains only InfraErrors", func() {
			It("should return false", func() {
				errors := []error{
					NewMySQLError(MySQLErrFailedToCreateCode, "error 1"),
					NewRedisError(RedisDeploymentConflictCode, "error 2"),
				}
				Expect(HasCriticalError(errors)).To(BeFalse())
			})
		})

		Context("when error list is empty", func() {
			It("should return false", func() {
				errors := []error{}
				Expect(HasCriticalError(errors)).To(BeFalse())
			})
		})
	})

	Describe("IsCriticalError", func() {
		Context("when error is an InfraError", func() {
			It("should return false", func() {
				err := NewMySQLError(MySQLErrFailedToCreateCode, "test error")
				Expect(IsCriticalError(err)).To(BeFalse())
			})
		})

		Context("when error is not an InfraError", func() {
			It("should return true", func() {
				err := fmt.Errorf("critical error")
				Expect(IsCriticalError(err)).To(BeTrue())
			})
		})
	})

	Describe("ToRedisStatusDetail", func() {
		Context("when InfraStatusDetail is for Redis", func() {
			It("should convert successfully", func() {
				status := NewRedisStatusDetail(RedisSentinelCreatedCode, "Redis created")
				detail, ok := status.ToRedisStatusDetail()
				Expect(ok).To(BeTrue())
				Expect(detail.InfraName()).To(Equal(Redis))
				Expect(detail.Code()).To(Equal(string(RedisSentinelCreatedCode)))
				Expect(detail.Message()).To(Equal("Redis created"))
			})
		})

		Context("when InfraStatusDetail is not for Redis", func() {
			It("should return false", func() {
				status := NewMySQLStatusDetail(MySQLCreatedCode, "MySQL created")
				_, ok := status.ToRedisStatusDetail()
				Expect(ok).To(BeFalse())
			})
		})
	})

	Describe("Status check functions", func() {
		Describe("IsRedisStatus", func() {
			It("should return true for Redis status", func() {
				status := NewRedisStatusDetail(RedisSentinelCreatedCode, "test")
				Expect(IsRedisStatus(status)).To(BeTrue())
			})

			It("should return false for non-Redis status", func() {
				status := NewMySQLStatusDetail(MySQLCreatedCode, "test")
				Expect(IsRedisStatus(status)).To(BeFalse())
			})
		})

		Describe("IsMySQLStatus", func() {
			It("should return true for MySQL status", func() {
				status := NewMySQLStatusDetail(MySQLCreatedCode, "test")
				Expect(IsMySQLStatus(status)).To(BeTrue())
			})

			It("should return false for non-MySQL status", func() {
				status := NewRedisStatusDetail(RedisSentinelCreatedCode, "test")
				Expect(IsMySQLStatus(status)).To(BeFalse())
			})
		})

		Describe("IsKafkaStatus", func() {
			It("should return true for Kafka status", func() {
				status := NewKafkaStatusDetail(KafkaCreatedCode, "test")
				Expect(IsKafkaStatus(status)).To(BeTrue())
			})

			It("should return false for non-Kafka status", func() {
				status := NewMySQLStatusDetail(MySQLCreatedCode, "test")
				Expect(IsKafkaStatus(status)).To(BeFalse())
			})
		})

		Describe("IsClickhouseStatus", func() {
			It("should return true for ClickHouse status", func() {
				status := NewClickHouseStatusDetail(ClickHouseCreatedCode, "test")
				Expect(IsClickhouseStatus(status)).To(BeTrue())
			})

			It("should return false for non-ClickHouse status", func() {
				status := NewMySQLStatusDetail(MySQLCreatedCode, "test")
				Expect(IsClickhouseStatus(status)).To(BeFalse())
			})
		})

		Describe("IsMinioStatus", func() {
			It("should return true for Minio status", func() {
				status := NewMinioStatusDetail(MinioCreatedCode, "test")
				Expect(IsMinioStatus(status)).To(BeTrue())
			})

			It("should return false for non-Minio status", func() {
				status := NewMySQLStatusDetail(MySQLCreatedCode, "test")
				Expect(IsMinioStatus(status)).To(BeFalse())
			})
		})
	})

	Describe("Results", func() {
		Describe("HasCriticalError", func() {
			It("should return true when error list has critical errors", func() {
				results := InitResults()
				results.AddErrors(fmt.Errorf("critical error"))
				Expect(results.HasCriticalError()).To(BeTrue())
			})

			It("should return false when error list has only InfraErrors", func() {
				results := InitResults()
				results.AddErrors(NewMySQLError(MySQLErrFailedToCreateCode, "test"))
				Expect(results.HasCriticalError()).To(BeFalse())
			})
		})

		Describe("GetCriticalErrors", func() {
			It("should return only critical errors", func() {
				results := InitResults()
				criticalErr := fmt.Errorf("critical error")
				infraErr := NewMySQLError(MySQLErrFailedToCreateCode, "infra error")
				results.AddErrors(criticalErr, infraErr)

				criticalErrors := results.GetCriticalErrors()
				Expect(criticalErrors).To(HaveLen(1))
				Expect(criticalErrors[0]).To(Equal(criticalErr))
			})

			It("should return empty list when no critical errors", func() {
				results := InitResults()
				results.AddErrors(NewMySQLError(MySQLErrFailedToCreateCode, "test"))
				Expect(results.GetCriticalErrors()).To(BeEmpty())
			})
		})

		Describe("Merge", func() {
			It("should merge results into other", func() {
				results1 := InitResults()
				results1.AddErrors(fmt.Errorf("error 1"))
				results1.AddStatuses(NewMySQLStatusDetail(MySQLCreatedCode, "status 1"))

				results2 := InitResults()
				results2.AddErrors(fmt.Errorf("error 2"))
				results2.AddStatuses(NewRedisStatusDetail(RedisSentinelCreatedCode, "status 2"))

				results1.Merge(results2)

				Expect(results2.ErrorList).To(HaveLen(2))
				Expect(results2.StatusList).To(HaveLen(2))
			})

			It("should handle nil other gracefully", func() {
				results := InitResults()
				results.AddErrors(fmt.Errorf("error"))
				Expect(func() { results.Merge(nil) }).NotTo(Panic())
			})
		})
	})
})
