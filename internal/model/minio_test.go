package model

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Minio Model", func() {
	Describe("MinioConfig", func() {
		Describe("IsHighAvailability", func() {
			Context("when servers is greater than 1", func() {
				It("should return true", func() {
					config := MinioConfig{
						Servers: 3,
					}
					Expect(config.IsHighAvailability()).To(BeTrue())
				})
			})

			Context("when servers is 1", func() {
				It("should return false", func() {
					config := MinioConfig{
						Servers: 1,
					}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})

			Context("when servers is 0", func() {
				It("should return false", func() {
					config := MinioConfig{
						Servers: 0,
					}
					Expect(config.IsHighAvailability()).To(BeFalse())
				})
			})
		})
	})

	Describe("Minio Error", func() {
		Describe("NewMinioError", func() {
			It("should create error with correct fields", func() {
				err := NewMinioError(MinioErrFailedToCreateCode, "test reason")
				Expect(err.InfraName()).To(Equal(Minio))
				Expect(err.Code()).To(Equal(string(MinioErrFailedToCreateCode)))
				Expect(err.Reason()).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewMinioError(MinioErrFailedToUpdateCode, "update error")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("minio"))
				Expect(errStr).To(ContainSubstring("update error"))
			})
		})

		Describe("MinioInfraError", func() {
			Describe("minioCode", func() {
				It("should return the error code", func() {
					infraErr := NewMinioError(MinioErrFailedToDeleteCode, "test error")
					minioErr := MinioInfraError{infraErr}
					Expect(minioErr.minioCode()).To(Equal(MinioErrFailedToDeleteCode))
				})
			})
		})

		Describe("ToMinioInfraError", func() {
			Context("when error is a Minio infra error", func() {
				It("should convert successfully", func() {
					err := NewMinioError(MinioErrFailedToCreateCode, "create failed")
					minioErr, ok := ToMinioInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(minioErr.minioCode()).To(Equal(MinioErrFailedToCreateCode))
					Expect(minioErr.reason).To(Equal("create failed"))
				})
			})

			Context("when error is not an infra error", func() {
				It("should return false", func() {
					err := fmt.Errorf("regular error")
					_, ok := ToMinioInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})

			Context("when error is an infra error but not Minio", func() {
				It("should return false", func() {
					err := NewInfraError(Redis, "SomeCode", "some reason")
					_, ok := ToMinioInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Minio Status", func() {
		Describe("NewMinioStatusDetail", func() {
			It("should create status with correct fields", func() {
				status := NewMinioStatusDetail(MinioCreatedCode, "Minio created")
				Expect(status.InfraName()).To(Equal(Minio))
				Expect(status.Code()).To(Equal(string(MinioCreatedCode)))
				Expect(status.Message()).To(Equal("Minio created"))
			})
		})

		Describe("MinioStatusDetail", func() {
			Describe("minioCode", func() {
				It("should return the status code", func() {
					status := NewMinioStatusDetail(MinioCreatedCode, "created")
					detail := MinioStatusDetail{status}
					Expect(detail.minioCode()).To(Equal(MinioCreatedCode))
				})
			})

			Describe("ToMinioConnDetail", func() {
				Context("when status is connection type with connection info", func() {
					It("should convert successfully", func() {
						host := "minio.example.com"
						port := "9000"
						accessKey := "test-access-key"
						connInfo := MinioConnInfo{
							Host:      host,
							Port:      port,
							AccessKey: accessKey,
						}
						status := NewMinioConnDetail(connInfo)
						detail := MinioStatusDetail{status}
						connDetail, ok := detail.ToMinioConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal(host))
						Expect(connDetail.connInfo.Port).To(Equal(port))
						Expect(connDetail.connInfo.AccessKey).To(Equal(accessKey))
					})
				})

				Context("when status is not connection type", func() {
					It("should return false", func() {
						status := NewMinioStatusDetail(MinioCreatedCode, "created")
						detail := MinioStatusDetail{status}
						_, ok := detail.ToMinioConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						status := NewInfraStatusDetail(Minio, string(MinioConnectionCode), "connection", "not a MinioConnInfo")
						detail := MinioStatusDetail{status}
						connDetail, ok := detail.ToMinioConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(BeEmpty())
						Expect(connDetail.connInfo.Port).To(BeEmpty())
						Expect(connDetail.connInfo.AccessKey).To(BeEmpty())
					})
				})
			})
		})

		Describe("NewMinioConnDetail", func() {
			It("should create connection detail with info", func() {
				host := "test-host"
				port := "9000"
				accessKey := "test-key"
				connInfo := MinioConnInfo{
					Host:      host,
					Port:      port,
					AccessKey: accessKey,
				}
				status := NewMinioConnDetail(connInfo)
				Expect(status.InfraName()).To(Equal(Minio))
				Expect(status.Code()).To(Equal(string(MinioConnectionCode)))
				Expect(status.Message()).To(Equal("Minio connection info"))
				Expect(status.Hidden()).To(Equal(connInfo))
			})
		})
	})

	Describe("Results.ExtractMinioStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return not ready state with no connection", func() {
				results := InitResults()
				status := ExtractMinioStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(BeEmpty())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have Minio errors", func() {
			It("should include errors and not be ready", func() {
				results := InitResults()
				err := NewMinioError(MinioErrFailedToCreateCode, "create failed")
				results.AddErrors(err)

				status := ExtractMinioStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Errors[0].Code()).To(Equal(string(MinioErrFailedToCreateCode)))
				Expect(status.Errors[0].Reason()).To(Equal("create failed"))
			})
		})

		Context("when results have non-Minio errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := NewInfraError(MySQL, "MySQLError", "mysql failed")
				results.AddErrors(err)

				status := ExtractMinioStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(BeEmpty())
			})
		})

		Context("when results have Minio statuses", func() {
			It("should include statuses in details", func() {
				results := InitResults()
				infraStatus := NewMinioStatusDetail(MinioCreatedCode, "Minio created successfully")
				results.AddStatuses(infraStatus)

				status := ExtractMinioStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].Code()).To(Equal(string(MinioCreatedCode)))
				Expect(status.Details[0].Message()).To(Equal("Minio created successfully"))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status and be ready", func() {
				host := "minio.example.com"
				port := "9000"
				accessKey := "test-access-key"
				results := InitResults()
				connInfo := MinioConnInfo{
					Host:      host,
					Port:      port,
					AccessKey: accessKey,
				}
				connStatus := NewMinioConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := ExtractMinioStatus(ctx, results)
				Expect(status.Ready).To(BeTrue())
				Expect(status.Connection.Host).To(Equal(host))
				Expect(status.Connection.Port).To(Equal(port))
				Expect(status.Connection.AccessKey).To(Equal(accessKey))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both errors and details, not be ready", func() {
				results := InitResults()
				err := NewMinioError(MinioErrFailedToUpdateCode, "update failed")
				infraStatus := NewMinioStatusDetail(MinioCreatedCode, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := ExtractMinioStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(1))
				Expect(status.Details).To(HaveLen(1))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				results := InitResults()
				err1 := NewMinioError(MinioErrFailedToCreateCode, "create failed")
				err2 := NewMinioError(MinioErrFailedToUpdateCode, "update failed")
				results.AddErrors(err1, err2)

				status := ExtractMinioStatus(ctx, results)
				Expect(status.Ready).To(BeFalse())
				Expect(status.Errors).To(HaveLen(2))
			})
		})
	})

	Describe("BuildMinioDefaults", func() {
		const testOwnerNamespace = "test-namespace"

		Context("when size is Dev", func() {
			It("should return complete dev defaults", func() {
				config, err := BuildMinioDefaults(SizeDev, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(DevMinioStorageSize))
				Expect(config.Servers).To(Equal(int32(1)))
				Expect(config.VolumesPerServer).To(Equal(int32(1)))
				Expect(config.Image).To(Equal(MinioImage))
				Expect(config.Resources.Requests).To(BeEmpty())
				Expect(config.Resources.Limits).To(BeEmpty())
			})
		})

		Context("when size is Small", func() {
			It("should return complete small defaults with all resource fields", func() {
				config, err := BuildMinioDefaults(SizeSmall, testOwnerNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(config.Enabled).To(BeTrue())
				Expect(config.Namespace).To(Equal(testOwnerNamespace))
				Expect(config.StorageSize).To(Equal(SmallMinioStorageSize))
				Expect(config.Servers).To(Equal(int32(3)))
				Expect(config.VolumesPerServer).To(Equal(int32(4)))
				Expect(config.Image).To(Equal(MinioImage))
				Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallMinioCpuRequest)))
				Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(SmallMinioCpuLimit)))
				Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallMinioMemoryRequest)))
				Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(SmallMinioMemoryLimit)))
			})
		})

		Context("when size is invalid", func() {
			It("should return error", func() {
				_, err := BuildMinioDefaults(Size("invalid"), testOwnerNamespace)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported size for Minio"))
			})
		})
	})
})
