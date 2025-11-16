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

		Describe("GetMinioConfig", func() {
			Context("when mergedMinio is nil", func() {
				It("should return empty config", func() {
					builder := &InfraConfigBuilder{}
					config, err := builder.GetMinioConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Enabled).To(BeFalse())
					Expect(config.Namespace).To(BeEmpty())
				})
			})

			Context("when mergedMinio has basic config for dev size", func() {
				It("should return config with basic fields and dev defaults", func() {
					builder := &InfraConfigBuilder{
						mergedMinio: &apiv2.WBMinioSpec{
							Enabled:     true,
							Namespace:   "test-namespace",
							StorageSize: "50Gi",
						},
						size: apiv2.WBSizeDev,
					}
					config, err := builder.GetMinioConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal("test-namespace"))
					Expect(config.StorageSize).To(Equal("50Gi"))
					Expect(config.Servers).To(Equal(int32(1)))
					Expect(config.VolumesPerServer).To(Equal(int32(1)))
					Expect(config.Image).To(Equal(MinioImage))
				})
			})

			Context("when mergedMinio has config for small size", func() {
				It("should return config with small size defaults", func() {
					builder := &InfraConfigBuilder{
						mergedMinio: &apiv2.WBMinioSpec{
							Enabled:     true,
							StorageSize: "100Gi",
						},
						size: apiv2.WBSizeSmall,
					}
					config, err := builder.GetMinioConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Enabled).To(BeTrue())
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Image).To(Equal(MinioImage))
				})
			})

			Context("when mergedMinio has config with resources", func() {
				It("should populate resource limits and requests", func() {
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
					builder := &InfraConfigBuilder{
						mergedMinio: &apiv2.WBMinioSpec{
							Enabled:     true,
							StorageSize: "50Gi",
							Config: &apiv2.WBMinioConfig{
								Resources: resources,
							},
						},
						size: apiv2.WBSizeDev,
					}
					config, err := builder.GetMinioConfig()
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Resources.Requests).NotTo(BeEmpty())
					Expect(config.Resources.Limits).NotTo(BeEmpty())
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse("500m")))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))
				})
			})

			Context("when size is invalid", func() {
				It("should return error", func() {
					builder := &InfraConfigBuilder{
						mergedMinio: &apiv2.WBMinioSpec{
							Enabled: true,
						},
						size: "invalid-size",
					}
					_, err := builder.GetMinioConfig()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unsupported size"))
				})
			})
		})

		Describe("AddMinioSpec", func() {
			Context("when merging with dev size", func() {
				It("should successfully merge and store spec", func() {
					actual := apiv2.WBMinioSpec{
						Enabled: true,
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddMinioSpec(&actual, apiv2.WBSizeDev)
					Expect(result).To(Equal(builder))
					Expect(builder.mergedMinio).NotTo(BeNil())
					Expect(builder.mergedMinio.Enabled).To(BeTrue())
					Expect(builder.size).To(Equal(apiv2.WBSizeDev))
					Expect(builder.errors).To(BeEmpty())
				})
			})

			Context("when merging with small size", func() {
				It("should successfully merge with small defaults", func() {
					actual := apiv2.WBMinioSpec{
						Enabled: true,
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddMinioSpec(&actual, apiv2.WBSizeSmall)
					Expect(result).To(Equal(builder))
					Expect(builder.mergedMinio).NotTo(BeNil())
					Expect(builder.mergedMinio.Enabled).To(BeTrue())
					Expect(builder.size).To(Equal(apiv2.WBSizeSmall))
					Expect(builder.errors).To(BeEmpty())
				})
			})

			Context("when building defaults fails", func() {
				It("should append error and return builder", func() {
					actual := apiv2.WBMinioSpec{
						Enabled: true,
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddMinioSpec(&actual, "invalid-size")
					Expect(result).To(Equal(builder))
					Expect(builder.errors).NotTo(BeEmpty())
				})
			})

			Context("when actual spec has custom values", func() {
				It("should preserve custom values during merge", func() {
					actual := apiv2.WBMinioSpec{
						Enabled:     true,
						Namespace:   "custom-ns",
						StorageSize: "500Gi",
					}
					builder := &InfraConfigBuilder{}
					result := builder.AddMinioSpec(&actual, apiv2.WBSizeDev)
					Expect(result).To(Equal(builder))
					Expect(builder.mergedMinio).NotTo(BeNil())
					Expect(builder.mergedMinio.Namespace).To(Equal("custom-ns"))
					Expect(builder.mergedMinio.StorageSize).To(Equal("500Gi"))
					Expect(builder.errors).To(BeEmpty())
				})
			})
		})

		Describe("GetMinioConfigForSize", func() {
			Context("when size is dev", func() {
				It("should return dev configuration", func() {
					config, err := GetMinioConfigForSize(apiv2.WBSizeDev)
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Servers).To(Equal(int32(1)))
					Expect(config.VolumesPerServer).To(Equal(int32(1)))
					Expect(config.Image).To(Equal(MinioImage))
				})
			})

			Context("when size is small", func() {
				It("should return small configuration", func() {
					config, err := GetMinioConfigForSize(apiv2.WBSizeSmall)
					Expect(err).NotTo(HaveOccurred())
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Image).To(Equal(MinioImage))
				})
			})

			Context("when size is invalid", func() {
				It("should return error", func() {
					_, err := GetMinioConfigForSize("invalid-size")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("unsupported size for Minio"))
					Expect(err.Error()).To(ContainSubstring("only 'dev' and 'small' are supported"))
				})
			})
		})
	})

	Describe("Minio Error", func() {
		Describe("NewMinioError", func() {
			It("should create error with correct fields", func() {
				err := NewMinioError(MinioErrFailedToCreate, "test reason")
				Expect(err.infraName).To(Equal(Minio))
				Expect(err.code).To(Equal(string(MinioErrFailedToCreate)))
				Expect(err.reason).To(Equal("test reason"))
			})

			It("should implement error interface", func() {
				err := NewMinioError(MinioErrFailedToUpdate, "update error")
				errStr := err.Error()
				Expect(errStr).To(ContainSubstring("FailedToUpdate"))
				Expect(errStr).To(ContainSubstring("minio"))
				Expect(errStr).To(ContainSubstring("update error"))
			})
		})

		Describe("MinioInfraError", func() {
			Describe("minioCode", func() {
				It("should return the error code", func() {
					infraErr := NewMinioError(MinioErrFailedToDelete, "test error")
					minioErr := MinioInfraError{infraErr}
					Expect(minioErr.minioCode()).To(Equal(MinioErrFailedToDelete))
				})
			})
		})

		Describe("ToMinioInfraError", func() {
			Context("when error is a Minio infra error", func() {
				It("should convert successfully", func() {
					err := NewMinioError(MinioErrFailedToCreate, "create failed")
					minioErr, ok := ToMinioInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(minioErr.minioCode()).To(Equal(MinioErrFailedToCreate))
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
					err := InfraError{
						infraName: Redis,
						code:      "SomeCode",
						reason:    "some reason",
					}
					_, ok := ToMinioInfraError(err)
					Expect(ok).To(BeFalse())
				})
			})
		})
	})

	Describe("Minio Status", func() {
		Describe("NewMinioStatus", func() {
			It("should create status with correct fields", func() {
				status := NewMinioStatus(MinioCreated, "Minio created")
				Expect(status.infraName).To(Equal(Minio))
				Expect(status.code).To(Equal(string(MinioCreated)))
				Expect(status.message).To(Equal("Minio created"))
			})
		})

		Describe("MinioStatusDetail", func() {
			Describe("minioCode", func() {
				It("should return the status code", func() {
					status := NewMinioStatus(MinioCreated, "created")
					detail := MinioStatusDetail{status}
					Expect(detail.minioCode()).To(Equal(MinioCreated))
				})
			})

			Describe("ToMinioConnDetail", func() {
				Context("when status is connection type with connection info", func() {
					It("should convert successfully", func() {
						connInfo := MinioConnInfo{
							Host:      "minio.example.com",
							Port:      "9000",
							AccessKey: "test-access-key",
						}
						status := NewMinioConnDetail(connInfo)
						detail := MinioStatusDetail{status}
						connDetail, ok := detail.ToMinioConnDetail()
						Expect(ok).To(BeTrue())
						Expect(connDetail.connInfo.Host).To(Equal("minio.example.com"))
						Expect(connDetail.connInfo.Port).To(Equal("9000"))
						Expect(connDetail.connInfo.AccessKey).To(Equal("test-access-key"))
					})
				})

				Context("when status is not connection type", func() {
					It("should return false", func() {
						status := NewMinioStatus(MinioCreated, "created")
						detail := MinioStatusDetail{status}
						_, ok := detail.ToMinioConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						status := InfraStatus{
							infraName: Minio,
							code:      string(MinioConnection),
							message:   "connection",
							hidden:    "not a MinioConnInfo",
						}
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
				connInfo := MinioConnInfo{
					Host:      "test-host",
					Port:      "9000",
					AccessKey: "test-key",
				}
				status := NewMinioConnDetail(connInfo)
				Expect(status.infraName).To(Equal(Minio))
				Expect(status.code).To(Equal(string(MinioConnection)))
				Expect(status.message).To(Equal("Minio connection info"))
				Expect(status.hidden).To(Equal(connInfo))
			})
		})
	})

	Describe("Results.ExtractMinioStatus", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when results have no errors or statuses", func() {
			It("should return default state as ready", func() {
				results := InitResults()
				status := results.ExtractMinioStatus(ctx)
				Expect(status.Details).To(BeEmpty())
				Expect(status.State).To(Equal(apiv2.WBStateReady))
			})
		})

		Context("when results have Minio errors", func() {
			It("should include errors in status details with error state", func() {
				results := InitResults()
				err := NewMinioError(MinioErrFailedToCreate, "create failed")
				results.AddErrors(err)

				status := results.ExtractMinioStatus(ctx)
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateError))
				Expect(status.Details[0].Code).To(Equal(string(MinioErrFailedToCreate)))
				Expect(status.Details[0].Message).To(Equal("create failed"))
				Expect(status.State).To(Equal(apiv2.WBStateError))
			})
		})

		Context("when results have non-Minio errors", func() {
			It("should not include them in status", func() {
				results := InitResults()
				err := InfraError{
					infraName: MySQL,
					code:      "MySQLError",
					reason:    "mysql failed",
				}
				results.AddErrors(err)

				status := results.ExtractMinioStatus(ctx)
				Expect(status.Details).To(BeEmpty())
				Expect(status.State).To(Equal(apiv2.WBStateReady))
			})
		})

		Context("when results have Minio statuses", func() {
			It("should include statuses in details with ready state", func() {
				results := InitResults()
				infraStatus := NewMinioStatus(MinioCreated, "Minio created successfully")
				results.AddStatuses(infraStatus)

				status := results.ExtractMinioStatus(ctx)
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details[0].Code).To(Equal(string(MinioCreated)))
				Expect(status.Details[0].Message).To(Equal("Minio created successfully"))
				Expect(status.State).To(Equal(apiv2.WBStateReady))
			})
		})

		Context("when results have connection status", func() {
			It("should populate connection info in status", func() {
				results := InitResults()
				connInfo := MinioConnInfo{
					Host:      "minio.example.com",
					Port:      "9000",
					AccessKey: "test-access-key",
				}
				connStatus := NewMinioConnDetail(connInfo)
				results.AddStatuses(connStatus)

				status := results.ExtractMinioStatus(ctx)
				Expect(status.Connection.MinioHost).To(Equal("minio.example.com"))
				Expect(status.Connection.MinioPort).To(Equal("9000"))
				Expect(status.Connection.MinioAccessKey).To(Equal("test-access-key"))
			})
		})

		Context("when results have both errors and statuses", func() {
			It("should include both in details and set state to error", func() {
				results := InitResults()
				err := NewMinioError(MinioErrFailedToUpdate, "update failed")
				infraStatus := NewMinioStatus(MinioCreated, "created")
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := results.ExtractMinioStatus(ctx)
				Expect(status.Details).To(HaveLen(2))
				Expect(status.State).To(Equal(apiv2.WBStateError))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors and maintain error state", func() {
				results := InitResults()
				err1 := NewMinioError(MinioErrFailedToCreate, "create failed")
				err2 := NewMinioError(MinioErrFailedToUpdate, "update failed")
				results.AddErrors(err1, err2)

				status := results.ExtractMinioStatus(ctx)
				Expect(status.Details).To(HaveLen(2))
				Expect(status.State).To(Equal(apiv2.WBStateError))
			})
		})
	})
})
