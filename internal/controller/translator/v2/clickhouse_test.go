package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/clickhouse/altinity"
	"github.com/wandb/operator/internal/controller/translator/common"
	chiv2 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("ClickHouse Translator", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("TranslateClickHouseStatus", func() {
		type statusFixture struct {
			name               string
			input              common.ClickHouseStatus
			expectedReady      bool
			expectedState      apiv2.WBStateType
			expectedHost       string
			expectedPort       string
			expectedUser       string
			expectedConditions int
		}

		DescribeTable("status translation scenarios",
			func(fixture statusFixture) {
				result := TranslateClickHouseStatus(ctx, fixture.input)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.State).To(Equal(fixture.expectedState))
				Expect(result.Connection.ClickHouseHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.ClickHousePort).To(Equal(fixture.expectedPort))
				Expect(result.Connection.ClickHouseUser).To(Equal(fixture.expectedUser))
				Expect(result.Conditions).To(HaveLen(fixture.expectedConditions))
				Expect(result.LastReconciled).ToNot(BeZero())
			},
			Entry("ready status with connection", statusFixture{
				name: "ready with connection",
				input: common.ClickHouseStatus{
					Ready: true,
					Connection: common.ClickHouseConnection{
						Host: "clickhouse.example.com",
						Port: "9000",
						User: "test_user",
					},
					Conditions: []common.ClickHouseCondition{
						common.NewClickHouseCondition(common.ClickHouseConnectionCode, "Connected"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateReady,
				expectedHost:       "clickhouse.example.com",
				expectedPort:       "9000",
				expectedUser:       "test_user",
				expectedConditions: 1,
			}),
			Entry("not ready status", statusFixture{
				name: "not ready",
				input: common.ClickHouseStatus{
					Ready: false,
					Connection: common.ClickHouseConnection{
						Host: "",
						Port: "",
						User: "",
					},
					Conditions: []common.ClickHouseCondition{
						common.NewClickHouseCondition(common.ClickHouseCreatedCode, "Creating"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "",
				expectedPort:       "",
				expectedUser:       "",
				expectedConditions: 1,
			}),
			Entry("updating status", statusFixture{
				name: "updating",
				input: common.ClickHouseStatus{
					Ready: true,
					Connection: common.ClickHouseConnection{
						Host: "clickhouse.example.com",
						Port: "9000",
						User: "test_user",
					},
					Conditions: []common.ClickHouseCondition{
						common.NewClickHouseCondition(common.ClickHouseUpdatedCode, "Updating configuration"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "clickhouse.example.com",
				expectedPort:       "9000",
				expectedUser:       "test_user",
				expectedConditions: 1,
			}),
			Entry("deleting status", statusFixture{
				name: "deleting",
				input: common.ClickHouseStatus{
					Ready: false,
					Connection: common.ClickHouseConnection{
						Host: "clickhouse.example.com",
						Port: "9000",
						User: "test_user",
					},
					Conditions: []common.ClickHouseCondition{
						common.NewClickHouseCondition(common.ClickHouseDeletedCode, "Deleting resources"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateDeleting,
				expectedHost:       "clickhouse.example.com",
				expectedPort:       "9000",
				expectedUser:       "test_user",
				expectedConditions: 1,
			}),
			Entry("multiple conditions with worst state taking precedence", statusFixture{
				name: "multiple conditions",
				input: common.ClickHouseStatus{
					Ready: true,
					Connection: common.ClickHouseConnection{
						Host: "clickhouse.example.com",
						Port: "9000",
						User: "test_user",
					},
					Conditions: []common.ClickHouseCondition{
						common.NewClickHouseCondition(common.ClickHouseCreatedCode, "Created"),
						common.NewClickHouseCondition(common.ClickHouseConnectionCode, "Connected"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "clickhouse.example.com",
				expectedPort:       "9000",
				expectedUser:       "test_user",
				expectedConditions: 2,
			}),
			Entry("empty status", statusFixture{
				name: "empty status",
				input: common.ClickHouseStatus{
					Ready:      false,
					Connection: common.ClickHouseConnection{},
					Conditions: []common.ClickHouseCondition{},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateUnknown,
				expectedHost:       "",
				expectedPort:       "",
				expectedUser:       "",
				expectedConditions: 0,
			}),
		)
	})

	Describe("ExtractClickHouseStatus", func() {
		type extractFixture struct {
			name          string
			conditions    []common.ClickHouseCondition
			expectedReady bool
			expectedHost  string
			expectedPort  string
			expectedUser  string
		}

		DescribeTable("condition extraction scenarios",
			func(fixture extractFixture) {
				result := ExtractClickHouseStatus(ctx, fixture.conditions)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.Connection.ClickHouseHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.ClickHousePort).To(Equal(fixture.expectedPort))
				Expect(result.Connection.ClickHouseUser).To(Equal(fixture.expectedUser))
			},
			Entry("connection info available", extractFixture{
				name: "with connection info",
				conditions: []common.ClickHouseCondition{
					common.NewClickHouseConnCondition(common.ClickHouseConnInfo{
						Host: "clickhouse-svc.default.svc.cluster.local",
						Port: "9000",
						User: "admin",
					}),
				},
				expectedReady: true,
				expectedHost:  "clickhouse-svc.default.svc.cluster.local",
				expectedPort:  "9000",
				expectedUser:  "admin",
			}),
			Entry("no connection info", extractFixture{
				name: "no connection info",
				conditions: []common.ClickHouseCondition{
					common.NewClickHouseCondition(common.ClickHouseCreatedCode, "Created"),
				},
				expectedReady: false,
				expectedHost:  "",
				expectedPort:  "",
				expectedUser:  "",
			}),
			Entry("mixed conditions", extractFixture{
				name: "mixed conditions with connection",
				conditions: []common.ClickHouseCondition{
					common.NewClickHouseCondition(common.ClickHouseCreatedCode, "Created"),
					common.NewClickHouseConnCondition(common.ClickHouseConnInfo{
						Host: "ch-host",
						Port: "8123",
						User: "default",
					}),
					common.NewClickHouseCondition(common.ClickHouseUpdatedCode, "Updated"),
				},
				expectedReady: true,
				expectedHost:  "ch-host",
				expectedPort:  "8123",
				expectedUser:  "default",
			}),
			Entry("empty conditions", extractFixture{
				name:          "empty conditions",
				conditions:    []common.ClickHouseCondition{},
				expectedReady: false,
				expectedHost:  "",
				expectedPort:  "",
				expectedUser:  "",
			}),
		)
	})

	Describe("ToClickHouseVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(chiv2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		type vendorSpecFixture struct {
			name                    string
			spec                    apiv2.WBClickHouseSpec
			expectNil               bool
			expectedName            string
			expectedNamespace       string
			expectedReplicas        int
			expectedStorageSize     string
			expectPodTemplate       bool
			expectedCPURequest      string
			expectedMemoryRequest   string
			expectedCPULimit        string
			expectedMemoryLimit     string
			expectedVolumeTemplate  string
			expectedClusterName     string
			expectedShardsCount     int
			expectedOwnerReferences int
		}

		DescribeTable("vendor spec translation scenarios",
			func(fixture vendorSpecFixture) {
				result, err := ToClickHouseVendorSpec(ctx, fixture.spec, owner, testScheme)

				if fixture.expectNil {
					Expect(result).To(BeNil())
					Expect(err).ToNot(HaveOccurred())
					return
				}

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				Expect(result.Name).To(Equal(fixture.expectedName))
				Expect(result.Namespace).To(Equal(fixture.expectedNamespace))
				Expect(result.Labels).To(HaveKeyWithValue("app", altinity.CHIName))

				Expect(result.Spec.Configuration).ToNot(BeNil())
				Expect(result.Spec.Configuration.Clusters).To(HaveLen(1))
				Expect(result.Spec.Configuration.Clusters[0].Name).To(Equal(fixture.expectedClusterName))
				Expect(result.Spec.Configuration.Clusters[0].Layout.ShardsCount).To(Equal(fixture.expectedShardsCount))
				Expect(result.Spec.Configuration.Clusters[0].Layout.ReplicasCount).To(Equal(fixture.expectedReplicas))

				Expect(result.Spec.Configuration.Users).ToNot(BeNil())

				Expect(result.Spec.Defaults).ToNot(BeNil())
				Expect(result.Spec.Defaults.Templates).ToNot(BeNil())
				Expect(result.Spec.Defaults.Templates.DataVolumeClaimTemplate).To(Equal(fixture.expectedVolumeTemplate))

				Expect(result.Spec.Templates).ToNot(BeNil())
				Expect(result.Spec.Templates.VolumeClaimTemplates).To(HaveLen(1))
				Expect(result.Spec.Templates.VolumeClaimTemplates[0].Name).To(Equal(fixture.expectedVolumeTemplate))
				storageRequest := result.Spec.Templates.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
				Expect(storageRequest.String()).To(Equal(fixture.expectedStorageSize))
				Expect(result.Spec.Templates.VolumeClaimTemplates[0].Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))

				if fixture.expectPodTemplate {
					Expect(result.Spec.Templates.PodTemplates).To(HaveLen(1))
					podTemplate := result.Spec.Templates.PodTemplates[0]
					Expect(podTemplate.Spec.Containers).To(HaveLen(1))
					Expect(podTemplate.Spec.Containers[0].Name).To(Equal("clickhouse"))

					resources := podTemplate.Spec.Containers[0].Resources
					if fixture.expectedCPURequest != "" {
						cpuRequest := resources.Requests[corev1.ResourceCPU]
						Expect(cpuRequest.String()).To(Equal(fixture.expectedCPURequest))
					}
					if fixture.expectedMemoryRequest != "" {
						memRequest := resources.Requests[corev1.ResourceMemory]
						Expect(memRequest.String()).To(Equal(fixture.expectedMemoryRequest))
					}
					if fixture.expectedCPULimit != "" {
						cpuLimit := resources.Limits[corev1.ResourceCPU]
						Expect(cpuLimit.String()).To(Equal(fixture.expectedCPULimit))
					}
					if fixture.expectedMemoryLimit != "" {
						memLimit := resources.Limits[corev1.ResourceMemory]
						Expect(memLimit.String()).To(Equal(fixture.expectedMemoryLimit))
					}
				} else {
					Expect(result.Spec.Templates.PodTemplates).To(BeEmpty())
				}

				Expect(result.OwnerReferences).To(HaveLen(fixture.expectedOwnerReferences))
				if fixture.expectedOwnerReferences > 0 {
					Expect(result.OwnerReferences[0].UID).To(Equal(owner.UID))
					Expect(result.OwnerReferences[0].Name).To(Equal(owner.Name))
				}
			},
			Entry("disabled clickhouse returns nil", vendorSpecFixture{
				name: "disabled clickhouse",
				spec: apiv2.WBClickHouseSpec{
					Enabled: false,
				},
				expectNil: true,
			}),
			Entry("minimal enabled configuration", vendorSpecFixture{
				name: "minimal configuration",
				spec: apiv2.WBClickHouseSpec{
					Enabled:     true,
					Name:        "test-ch",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    1,
				},
				expectNil:               false,
				expectedName:            "test-ch-install",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        1,
				expectedStorageSize:     "10Gi",
				expectPodTemplate:       false,
				expectedVolumeTemplate:  "test-ch-voltempl",
				expectedClusterName:     "test-ch",
				expectedShardsCount:     1,
				expectedOwnerReferences: 1,
			}),
			Entry("configuration with resources", vendorSpecFixture{
				name: "with resources",
				spec: apiv2.WBClickHouseSpec{
					Enabled:     true,
					Name:        "prod-ch",
					Namespace:   testingOwnerNamespace,
					StorageSize: "100Gi",
					Replicas:    3,
					Config: apiv2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("2000m"),
								corev1.ResourceMemory: resource.MustParse("4Gi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4000m"),
								corev1.ResourceMemory: resource.MustParse("8Gi"),
							},
						},
					},
				},
				expectNil:               false,
				expectedName:            "prod-ch-install",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        3,
				expectedStorageSize:     "100Gi",
				expectPodTemplate:       true,
				expectedCPURequest:      "2",
				expectedMemoryRequest:   "4Gi",
				expectedCPULimit:        "4",
				expectedMemoryLimit:     "8Gi",
				expectedVolumeTemplate:  "prod-ch-voltempl",
				expectedClusterName:     "prod-ch",
				expectedShardsCount:     1,
				expectedOwnerReferences: 1,
			}),
			Entry("configuration with only requests", vendorSpecFixture{
				name: "only requests",
				spec: apiv2.WBClickHouseSpec{
					Enabled:     true,
					Name:        "dev-ch",
					Namespace:   testingOwnerNamespace,
					StorageSize: "5Gi",
					Replicas:    1,
					Config: apiv2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
				expectNil:               false,
				expectedName:            "dev-ch-install",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        1,
				expectedStorageSize:     "5Gi",
				expectPodTemplate:       true,
				expectedCPURequest:      "500m",
				expectedMemoryRequest:   "1Gi",
				expectedCPULimit:        "",
				expectedMemoryLimit:     "",
				expectedVolumeTemplate:  "dev-ch-voltempl",
				expectedClusterName:     "dev-ch",
				expectedShardsCount:     1,
				expectedOwnerReferences: 1,
			}),
			Entry("configuration with only limits", vendorSpecFixture{
				name: "only limits",
				spec: apiv2.WBClickHouseSpec{
					Enabled:     true,
					Name:        "limited-ch",
					Namespace:   testingOwnerNamespace,
					StorageSize: "20Gi",
					Replicas:    2,
					Config: apiv2.WBClickHouseConfig{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("3000m"),
								corev1.ResourceMemory: resource.MustParse("6Gi"),
							},
						},
					},
				},
				expectNil:               false,
				expectedName:            "limited-ch-install",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        2,
				expectedStorageSize:     "20Gi",
				expectPodTemplate:       true,
				expectedCPURequest:      "",
				expectedMemoryRequest:   "",
				expectedCPULimit:        "3",
				expectedMemoryLimit:     "6Gi",
				expectedVolumeTemplate:  "limited-ch-voltempl",
				expectedClusterName:     "limited-ch",
				expectedShardsCount:     1,
				expectedOwnerReferences: 1,
			}),
			Entry("large storage configuration", vendorSpecFixture{
				name: "large storage",
				spec: apiv2.WBClickHouseSpec{
					Enabled:     true,
					Name:        "large-ch",
					Namespace:   testingOwnerNamespace,
					StorageSize: "1Ti",
					Replicas:    5,
				},
				expectNil:               false,
				expectedName:            "large-ch-install",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        5,
				expectedStorageSize:     "1Ti",
				expectPodTemplate:       false,
				expectedVolumeTemplate:  "large-ch-voltempl",
				expectedClusterName:     "large-ch",
				expectedShardsCount:     1,
				expectedOwnerReferences: 1,
			}),
		)

		Context("when owner reference cannot be set", func() {
			It("should return an error", func() {
				spec := apiv2.WBClickHouseSpec{
					Enabled:     true,
					Name:        "test-ch",
					Namespace:   "test",
					StorageSize: "10Gi",
					Replicas:    1,
				}

				nilScheme := runtime.NewScheme()

				result, err := ToClickHouseVendorSpec(ctx, spec, owner, nilScheme)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to set owner reference"))
				Expect(result).To(BeNil())
			})
		})

		Context("password and user configuration", func() {
			It("should configure user settings with password hash", func() {
				spec := apiv2.WBClickHouseSpec{
					Enabled:     true,
					Name:        "secure-ch",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    1,
				}

				result, err := ToClickHouseVendorSpec(ctx, spec, owner, testScheme)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.Spec.Configuration.Users).ToNot(BeNil())

				passwordKey := altinity.ClickHouseUser + "/password_sha256_hex"
				passwordSetting := result.Spec.Configuration.Users.Get(passwordKey)
				Expect(passwordSetting).ToNot(BeNil())

				networkKey := altinity.ClickHouseUser + "/networks/ip"
				networkSetting := result.Spec.Configuration.Users.Get(networkKey)
				Expect(networkSetting).ToNot(BeNil())
			})
		})
	})
})
