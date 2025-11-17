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

	Describe("InfraConfigBuilder", func() {
		Describe("AddMinioSpec and GetMinioConfig", func() {
			Context("with dev size and empty actual spec", func() {
				It("should use all dev defaults except Enabled, Namespace, and Replicas", func() {
					namespaceOverride := "custom-minio-namespace"
					replicasFromActual := int32(1)
					actual := apiv2.WBMinioSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Replicas:  replicasFromActual,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeDev)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.DevMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(1)))
					Expect(config.VolumesPerServer).To(Equal(int32(1)))
					Expect(config.Image).To(Equal(MinioImage))
					Expect(config.Resources.Requests).To(BeEmpty())
					Expect(config.Resources.Limits).To(BeEmpty())
				})
			})

			Context("with small size and empty actual spec", func() {
				It("should use all small defaults including resources except Enabled, Namespace, and Replicas", func() {
					namespaceOverride := "custom-minio-namespace"
					replicasFromActual := int32(3)
					actual := apiv2.WBMinioSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Replicas:  replicasFromActual,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Image).To(Equal(MinioImage))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallMinioCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallMinioCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryLimit)))
				})
			})

			Context("with small size and storage override", func() {
				It("should use override storage and default resources", func() {
					storageSizeOverride := "50Gi"
					namespaceOverride := "custom-minio-namespace"
					replicasFromActual := int32(3)
					actual := apiv2.WBMinioSpec{
						Enabled:     true,
						Namespace:   namespaceOverride,
						Replicas:    replicasFromActual,
						StorageSize: storageSizeOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(storageSizeOverride))
					Expect(config.StorageSize).NotTo(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Image).To(Equal(MinioImage))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallMinioCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallMinioCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryLimit)))
				})
			})

			Context("with small size and namespace using default", func() {
				It("should use default namespace when not provided", func() {
					replicasFromActual := int32(3)
					actual := apiv2.WBMinioSpec{
						Enabled:  true,
						Replicas: replicasFromActual,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
				})
			})

			Context("with small size and replicas override", func() {
				It("should use override replicas value", func() {
					namespaceOverride := "custom-minio-namespace"
					replicasOverride := int32(5)
					actual := apiv2.WBMinioSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Replicas:  replicasOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
				})
			})

			Context("with small size and resource overrides", func() {
				It("should use override resources and default storage", func() {
					cpuRequestOverride := "2"
					cpuLimitOverride := "4"
					memoryRequestOverride := "4Gi"
					memoryLimitOverride := "8Gi"
					namespaceOverride := "custom-minio-namespace"
					replicasFromActual := int32(3)
					actual := apiv2.WBMinioSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Replicas:  replicasFromActual,
						Config: &apiv2.WBMinioConfig{
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
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Image).To(Equal(MinioImage))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioMemoryLimit)))
				})
			})

			Context("with small size and partial resource overrides", func() {
				It("should merge override and default resources", func() {
					cpuLimitOverride := "2"
					namespaceOverride := "custom-minio-namespace"
					replicasFromActual := int32(3)
					actual := apiv2.WBMinioSpec{
						Enabled:   true,
						Namespace: namespaceOverride,
						Replicas:  replicasFromActual,
						Config: &apiv2.WBMinioConfig{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse(cpuLimitOverride),
								},
							},
						},
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallMinioCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryLimit)))
				})
			})

			Context("with small size and all overrides", func() {
				It("should use all override values", func() {
					storageSizeOverride := "100Gi"
					namespaceOverride := "custom-minio-namespace"
					replicasOverride := int32(7)
					cpuRequestOverride := "3"
					cpuLimitOverride := "6"
					memoryRequestOverride := "8Gi"
					memoryLimitOverride := "16Gi"
					actual := apiv2.WBMinioSpec{
						Enabled:     true,
						Namespace:   namespaceOverride,
						Replicas:    replicasOverride,
						StorageSize: storageSizeOverride,
						Config: &apiv2.WBMinioConfig{
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
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(storageSizeOverride))
					Expect(config.StorageSize).NotTo(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Image).To(Equal(MinioImage))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallMinioMemoryLimit)))
				})
			})

			Context("with disabled spec", func() {
				It("should respect enabled false and use defaults for other fields", func() {
					namespaceOverride := "custom-minio-namespace"
					replicasFromActual := int32(0)
					actual := apiv2.WBMinioSpec{
						Enabled:   false,
						Namespace: namespaceOverride,
						Replicas:  replicasFromActual,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddMinioSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetMinioConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeFalse())
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallMinioStorageSize))
					Expect(config.Servers).To(Equal(int32(3)))
					Expect(config.VolumesPerServer).To(Equal(int32(4)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallMinioCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallMinioCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallMinioMemoryLimit)))
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
				host := "test-host"
				port := "9000"
				accessKey := "test-key"
				connInfo := MinioConnInfo{
					Host:      host,
					Port:      port,
					AccessKey: accessKey,
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

				status := results.ExtractMinioStatus(ctx)
				Expect(status.Connection.MinioHost).To(Equal(host))
				Expect(status.Connection.MinioPort).To(Equal(port))
				Expect(status.Connection.MinioAccessKey).To(Equal(accessKey))
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
