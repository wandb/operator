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

	Describe("InfraConfigBuilder", func() {
		Describe("AddClickHouseSpec and GetClickHouseConfig", func() {
			Context("with dev size and empty actual spec", func() {
				It("should use all dev defaults except Enabled and Replicas", func() {
					actual := apiv2.WBClickHouseSpec{
						Enabled:  true,
						Replicas: int32(1),
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeDev)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Replicas).To(Equal(int32(1)))
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.DevClickHouseStorageSize))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests).To(BeEmpty())
					Expect(config.Resources.Limits).To(BeEmpty())
				})
			})

			Context("with small size and empty actual spec", func() {
				It("should use all small defaults including resources except Enabled and Replicas", func() {
					actual := apiv2.WBClickHouseSpec{
						Enabled:  true,
						Replicas: int32(3),
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Replicas).To(Equal(int32(3)))
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})

			Context("with small size and storage override", func() {
				It("should use override storage and default resources", func() {
					storageSizeOverride := "50Gi"
					replicasFromActual := int32(3)
					actual := apiv2.WBClickHouseSpec{
						Enabled:     true,
						Replicas:    replicasFromActual,
						StorageSize: storageSizeOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Replicas).To(Equal(replicasFromActual))
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(storageSizeOverride))
					Expect(config.StorageSize).NotTo(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})

			Context("with small size and version override", func() {
				It("should use override version and default resources", func() {
					versionOverride := "24.8"
					replicasFromActual := int32(3)
					actual := apiv2.WBClickHouseSpec{
						Enabled:  true,
						Replicas: replicasFromActual,
						Version:  versionOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Replicas).To(Equal(replicasFromActual))
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Version).To(Equal(versionOverride))
					Expect(config.Version).NotTo(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})

			Context("with small size and replicas override", func() {
				It("should use override replicas and default resources", func() {
					replicasOverride := int32(5)
					actual := apiv2.WBClickHouseSpec{
						Enabled:  true,
						Replicas: replicasOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Replicas).To(Equal(replicasOverride))
					Expect(config.Replicas).NotTo(Equal(int32(3)))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})

			Context("with small size and resource overrides", func() {
				It("should use override resources and default storage/version", func() {
					replicasFromActual := int32(3)
					cpuRequestOverride := "2"
					cpuLimitOverride := "4"
					memoryRequestOverride := "4Gi"
					memoryLimitOverride := "8Gi"
					actual := apiv2.WBClickHouseSpec{
						Enabled:  true,
						Replicas: replicasFromActual,
						Config: &apiv2.WBClickHouseConfig{
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
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Replicas).To(Equal(replicasFromActual))
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})

			Context("with small size and all overrides", func() {
				It("should use all override values", func() {
					storageSizeOverride := "100Gi"
					replicasOverride := int32(7)
					versionOverride := "25.1"
					cpuRequestOverride := "3"
					cpuLimitOverride := "6"
					memoryRequestOverride := "8Gi"
					memoryLimitOverride := "16Gi"
					actual := apiv2.WBClickHouseSpec{
						Enabled:     true,
						StorageSize: storageSizeOverride,
						Replicas:    replicasOverride,
						Version:     versionOverride,
						Config: &apiv2.WBClickHouseConfig{
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
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(storageSizeOverride))
					Expect(config.StorageSize).NotTo(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Replicas).To(Equal(replicasOverride))
					Expect(config.Replicas).NotTo(Equal(int32(3)))
					Expect(config.Version).To(Equal(versionOverride))
					Expect(config.Version).NotTo(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryRequestOverride)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(memoryLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})

			Context("with dev size and namespace override via spec", func() {
				It("should use override namespace and dev defaults", func() {
					namespaceOverride := "custom-clickhouse-namespace"
					replicasFromActual := int32(1)
					actual := apiv2.WBClickHouseSpec{
						Enabled:   true,
						Replicas:  replicasFromActual,
						Namespace: namespaceOverride,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeDev)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Replicas).To(Equal(replicasFromActual))
					Expect(config.Namespace).To(Equal(namespaceOverride))
					Expect(config.Namespace).NotTo(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.DevClickHouseStorageSize))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests).To(BeEmpty())
					Expect(config.Resources.Limits).To(BeEmpty())
				})
			})

			Context("with small size and partial resource overrides", func() {
				It("should merge override and default resources", func() {
					replicasFromActual := int32(3)
					cpuLimitOverride := "2"
					actual := apiv2.WBClickHouseSpec{
						Enabled:  true,
						Replicas: replicasFromActual,
						Config: &apiv2.WBClickHouseConfig{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									v1.ResourceCPU: resource.MustParse(cpuLimitOverride),
								},
							},
						},
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeTrue())
					Expect(config.Replicas).To(Equal(replicasFromActual))
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(cpuLimitOverride)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).NotTo(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})

			Context("with disabled spec", func() {
				It("should respect enabled false and use defaults for other fields", func() {
					replicasFromActual := int32(0)
					actual := apiv2.WBClickHouseSpec{
						Enabled:  false,
						Replicas: replicasFromActual,
					}
					builder := BuildInfraConfig(testingOwnerNamespace).AddClickHouseSpec(&actual, apiv2.WBSizeSmall)

					Expect(builder.errors).To(BeEmpty())
					config, err := builder.GetClickHouseConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(config.Enabled).To(BeFalse())
					Expect(config.Replicas).To(Equal(replicasFromActual))
					Expect(config.Namespace).To(Equal(testingOwnerNamespace))
					Expect(config.StorageSize).To(Equal(translatorv2.SmallClickHouseStorageSize))
					Expect(config.Version).To(Equal(translatorv2.ClickHouseVersion))
					Expect(config.Resources.Requests[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuRequest)))
					Expect(config.Resources.Requests[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryRequest)))
					Expect(config.Resources.Limits[v1.ResourceCPU]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseCpuLimit)))
					Expect(config.Resources.Limits[v1.ResourceMemory]).To(Equal(resource.MustParse(translatorv2.SmallClickHouseMemoryLimit)))
				})
			})
		})
	})

	Describe("ClickHouse Error", func() {
		Describe("NewClickHouseError", func() {
			It("should create error with correct fields", func() {
				reason := "test reason"
				err := NewClickHouseError(ClickHouseErrFailedToCreate, reason)
				Expect(err.infraName).To(Equal(Clickhouse))
				Expect(err.code).To(Equal(string(ClickHouseErrFailedToCreate)))
				Expect(err.reason).To(Equal(reason))
			})

			It("should implement error interface", func() {
				reason := "update failed"
				err := NewClickHouseError(ClickHouseErrFailedToUpdate, reason)
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
					infraErr := NewClickHouseError(ClickHouseErrFailedToDelete, reason)
					chErr := ClickHouseInfraError{infraErr}
					Expect(chErr.clickhouseCode()).To(Equal(ClickHouseErrFailedToDelete))
				})
			})
		})

		Describe("ToClickHouseInfraError", func() {
			Context("when error is a ClickHouse infra error", func() {
				It("should convert successfully", func() {
					reason := "config error"
					err := NewClickHouseError(ClickHouseErrFailedToGetConfig, reason)
					chErr, ok := ToClickHouseInfraError(err)
					Expect(ok).To(BeTrue())
					Expect(chErr.clickhouseCode()).To(Equal(ClickHouseErrFailedToGetConfig))
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
						infraName: Redis,
						code:      code,
						reason:    reason,
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
				message := "ClickHouse created"
				status := NewClickHouseStatus(ClickHouseCreated, message)
				Expect(status.infraName).To(Equal(Clickhouse))
				Expect(status.code).To(Equal(string(ClickHouseCreated)))
				Expect(status.message).To(Equal(message))
			})
		})

		Describe("ClickHouseStatusDetail", func() {
			Describe("clickhouseCode", func() {
				It("should return the status code", func() {
					message := "updated"
					status := NewClickHouseStatus(ClickHouseUpdated, message)
					detail := ClickHouseStatusDetail{status}
					Expect(detail.clickhouseCode()).To(Equal(ClickHouseUpdated))
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
						status := NewClickHouseStatus(ClickHouseCreated, message)
						detail := ClickHouseStatusDetail{status}
						_, ok := detail.ToClickHouseConnDetail()
						Expect(ok).To(BeFalse())
					})
				})

				Context("when status is connection type but missing connection info", func() {
					It("should return empty connection info but ok true", func() {
						message := "connection"
						hidden := "not a ClickHouseConnInfo"
						status := InfraStatus{
							infraName: Clickhouse,
							code:      string(ClickHouseConnection),
							message:   message,
							hidden:    hidden,
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
				Expect(status.infraName).To(Equal(Clickhouse))
				Expect(status.code).To(Equal(string(ClickHouseConnection)))
				Expect(status.message).To(Equal("ClickHouse connection info"))
				Expect(status.hidden).To(Equal(connInfo))
			})
		})

		Describe("InfraStatus.ToClickHouseStatusDetail", func() {
			Context("when infra status is for ClickHouse", func() {
				It("should convert successfully", func() {
					code := "TestCode"
					message := "test message"
					hidden := "hidden data"
					status := InfraStatus{
						infraName: Clickhouse,
						code:      code,
						message:   message,
						hidden:    hidden,
					}
					detail, ok := status.ToClickHouseStatusDetail()
					Expect(ok).To(BeTrue())
					Expect(detail.infraName).To(Equal(Clickhouse))
					Expect(detail.code).To(Equal(code))
					Expect(detail.message).To(Equal(message))
					Expect(detail.hidden).To(Equal(hidden))
				})
			})

			Context("when infra status is not for ClickHouse", func() {
				It("should return false", func() {
					code := "TestCode"
					message := "test message"
					status := InfraStatus{
						infraName: Redis,
						code:      code,
						message:   message,
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
				reason := "failed to create"
				results := InitResults()
				err := NewClickHouseError(ClickHouseErrFailedToCreate, reason)
				results.AddErrors(err)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateError))
				Expect(status.Details[0].Code).To(Equal(string(ClickHouseErrFailedToCreate)))
				Expect(status.Details[0].Message).To(Equal(reason))
			})
		})

		Context("when results have non-ClickHouse errors", func() {
			It("should not include them in status", func() {
				code := "RedisError"
				reason := "redis failed"
				results := InitResults()
				err := InfraError{
					infraName: Redis,
					code:      code,
					reason:    reason,
				}
				results.AddErrors(err)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(BeEmpty())
			})
		})

		Context("when results have ClickHouse statuses", func() {
			It("should include statuses in details with ready state", func() {
				message := "ClickHouse created successfully"
				results := InitResults()
				infraStatus := NewClickHouseStatus(ClickHouseCreated, message)
				results.AddStatuses(infraStatus)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details).To(HaveLen(1))
				Expect(status.Details[0].State).To(Equal(apiv2.WBStateReady))
				Expect(status.Details[0].Code).To(Equal(string(ClickHouseCreated)))
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

				status := results.ExtractClickHouseStatus(ctx)
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
				err := NewClickHouseError(ClickHouseErrFailedToUpdate, errorReason)
				infraStatus := NewClickHouseStatus(ClickHouseCreated, statusMessage)
				results.AddErrors(err)
				results.AddStatuses(infraStatus)

				status := results.ExtractClickHouseStatus(ctx)
				Expect(status.State).To(Equal(apiv2.WBStateError))
				Expect(status.Details).To(HaveLen(2))
			})
		})

		Context("when results have multiple errors", func() {
			It("should include all errors", func() {
				createReason := "create failed"
				updateReason := "update failed"
				results := InitResults()
				err1 := NewClickHouseError(ClickHouseErrFailedToCreate, createReason)
				err2 := NewClickHouseError(ClickHouseErrFailedToUpdate, updateReason)
				results.AddErrors(err1, err2)

				status := results.ExtractClickHouseStatus(ctx)
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
				createdStatus := NewClickHouseStatus(ClickHouseCreated, createdMessage)
				updatedStatus := NewClickHouseStatus(ClickHouseUpdated, updatedMessage)
				results.AddStatuses(connStatus, createdStatus, updatedStatus)

				status := results.ExtractClickHouseStatus(ctx)
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
