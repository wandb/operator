package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/minio/tenant"
	"github.com/wandb/operator/internal/controller/translator/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("Minio Translator", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("TranslateMinioStatus", func() {
		type statusFixture struct {
			name               string
			input              common.MinioStatus
			expectedReady      bool
			expectedState      apiv2.WBStateType
			expectedHost       string
			expectedPort       string
			expectedAccessKey  string
			expectedConditions int
		}

		DescribeTable("status translation scenarios",
			func(fixture statusFixture) {
				result := TranslateMinioStatus(ctx, fixture.input)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.State).To(Equal(fixture.expectedState))
				Expect(result.Connection.MinioHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.MinioPort).To(Equal(fixture.expectedPort))
				Expect(result.Connection.MinioAccessKey).To(Equal(fixture.expectedAccessKey))
				Expect(result.Conditions).To(HaveLen(fixture.expectedConditions))
				Expect(result.LastReconciled).ToNot(BeZero())
			},
			Entry("ready status with connection", statusFixture{
				name: "ready with connection",
				input: common.MinioStatus{
					Ready: true,
					Connection: common.MinioConnection{
						Host:      "minio.example.com",
						Port:      "9000",
						AccessKey: "admin",
					},
					Conditions: []common.MinioCondition{
						common.NewMinioCondition(common.MinioConnectionCode, "Connected"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateReady,
				expectedHost:       "minio.example.com",
				expectedPort:       "9000",
				expectedAccessKey:  "admin",
				expectedConditions: 1,
			}),
			Entry("creating status", statusFixture{
				name: "creating",
				input: common.MinioStatus{
					Ready: false,
					Connection: common.MinioConnection{
						Host:      "",
						Port:      "",
						AccessKey: "",
					},
					Conditions: []common.MinioCondition{
						common.NewMinioCondition(common.MinioCreatedCode, "Creating Minio tenant"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "",
				expectedPort:       "",
				expectedAccessKey:  "",
				expectedConditions: 1,
			}),
			Entry("updating status", statusFixture{
				name: "updating",
				input: common.MinioStatus{
					Ready: true,
					Connection: common.MinioConnection{
						Host:      "minio.example.com",
						Port:      "9000",
						AccessKey: "admin",
					},
					Conditions: []common.MinioCondition{
						common.NewMinioCondition(common.MinioUpdatedCode, "Updating configuration"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "minio.example.com",
				expectedPort:       "9000",
				expectedAccessKey:  "admin",
				expectedConditions: 1,
			}),
			Entry("deleting status", statusFixture{
				name: "deleting",
				input: common.MinioStatus{
					Ready: false,
					Connection: common.MinioConnection{
						Host:      "minio.example.com",
						Port:      "9000",
						AccessKey: "admin",
					},
					Conditions: []common.MinioCondition{
						common.NewMinioCondition(common.MinioDeletedCode, "Deleting resources"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateDeleting,
				expectedHost:       "minio.example.com",
				expectedPort:       "9000",
				expectedAccessKey:  "admin",
				expectedConditions: 1,
			}),
			Entry("empty status", statusFixture{
				name: "empty status",
				input: common.MinioStatus{
					Ready:      false,
					Connection: common.MinioConnection{},
					Conditions: []common.MinioCondition{},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateUnknown,
				expectedHost:       "",
				expectedPort:       "",
				expectedAccessKey:  "",
				expectedConditions: 0,
			}),
		)
	})

	Describe("ExtractMinioStatus", func() {
		type extractFixture struct {
			name              string
			conditions        []common.MinioCondition
			expectedReady     bool
			expectedHost      string
			expectedPort      string
			expectedAccessKey string
		}

		DescribeTable("condition extraction scenarios",
			func(fixture extractFixture) {
				result := ExtractMinioStatus(ctx, fixture.conditions)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.Connection.MinioHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.MinioPort).To(Equal(fixture.expectedPort))
				Expect(result.Connection.MinioAccessKey).To(Equal(fixture.expectedAccessKey))
			},
			Entry("connection info available", extractFixture{
				name: "with connection info",
				conditions: []common.MinioCondition{
					common.NewMinioConnCondition(common.MinioConnInfo{
						Host:      "minio-svc.default.svc.cluster.local",
						Port:      "9000",
						AccessKey: "minioadmin",
					}),
				},
				expectedReady:     true,
				expectedHost:      "minio-svc.default.svc.cluster.local",
				expectedPort:      "9000",
				expectedAccessKey: "minioadmin",
			}),
			Entry("no connection info", extractFixture{
				name: "no connection info",
				conditions: []common.MinioCondition{
					common.NewMinioCondition(common.MinioCreatedCode, "Created"),
				},
				expectedReady:     false,
				expectedHost:      "",
				expectedPort:      "",
				expectedAccessKey: "",
			}),
		)
	})

	Describe("ToMinioVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(miniov2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		type minioSpecFixture struct {
			name                     string
			spec                     apiv2.WBMinioSpec
			expectNil                bool
			expectError              bool
			expectedName             string
			expectedNamespace        string
			expectedServers          int32
			expectedVolumesPerServer int32
			expectedStorageSize      string
			expectedImage            string
			expectResources          bool
			expectedCPURequest       string
			expectedMemoryRequest    string
			expectedCPULimit         string
			expectedMemoryLimit      string
			expectedOwnerReferences  int
		}

		DescribeTable("minio spec translation scenarios",
			func(fixture minioSpecFixture) {
				result, err := ToMinioVendorSpec(ctx, fixture.spec, owner, testScheme)

				if fixture.expectError {
					Expect(err).To(HaveOccurred())
					return
				}

				if fixture.expectNil {
					Expect(result).To(BeNil())
					Expect(err).ToNot(HaveOccurred())
					return
				}

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())

				Expect(result.Name).To(Equal(fixture.expectedName))
				Expect(result.Namespace).To(Equal(fixture.expectedNamespace))
				Expect(result.Labels).To(HaveKeyWithValue("app", tenant.TenantName(fixture.spec.Name)))

				Expect(result.Spec.Image).To(Equal(fixture.expectedImage))
				Expect(result.Spec.Configuration).ToNot(BeNil())
				Expect(result.Spec.Configuration.Name).To(Equal(tenant.ConfigName(fixture.spec.Name)))

				Expect(result.Spec.Pools).To(HaveLen(1))
				pool := result.Spec.Pools[0]
				Expect(pool.Name).To(Equal(tenant.PoolName(fixture.spec.Name)))
				Expect(pool.Servers).To(Equal(fixture.expectedServers))
				Expect(pool.VolumesPerServer).To(Equal(fixture.expectedVolumesPerServer))

				Expect(pool.VolumeClaimTemplate).ToNot(BeNil())
				storageRequest := pool.VolumeClaimTemplate.Spec.Resources.Requests[corev1.ResourceStorage]
				Expect(storageRequest.String()).To(Equal(fixture.expectedStorageSize))
				Expect(pool.VolumeClaimTemplate.Spec.AccessModes).To(ContainElement(corev1.ReadWriteOnce))

				if fixture.expectResources {
					if fixture.expectedCPURequest != "" {
						cpuRequest := pool.Resources.Requests[corev1.ResourceCPU]
						Expect(cpuRequest.String()).To(Equal(fixture.expectedCPURequest))
					}
					if fixture.expectedMemoryRequest != "" {
						memRequest := pool.Resources.Requests[corev1.ResourceMemory]
						Expect(memRequest.String()).To(Equal(fixture.expectedMemoryRequest))
					}
					if fixture.expectedCPULimit != "" {
						cpuLimit := pool.Resources.Limits[corev1.ResourceCPU]
						Expect(cpuLimit.String()).To(Equal(fixture.expectedCPULimit))
					}
					if fixture.expectedMemoryLimit != "" {
						memLimit := pool.Resources.Limits[corev1.ResourceMemory]
						Expect(memLimit.String()).To(Equal(fixture.expectedMemoryLimit))
					}
				}

				Expect(result.OwnerReferences).To(HaveLen(fixture.expectedOwnerReferences))
				if fixture.expectedOwnerReferences > 0 {
					Expect(result.OwnerReferences[0].UID).To(Equal(owner.UID))
					Expect(result.OwnerReferences[0].Name).To(Equal(owner.Name))
				}
			},
			Entry("disabled minio returns nil", minioSpecFixture{
				name: "disabled minio",
				spec: apiv2.WBMinioSpec{
					Enabled: false,
				},
				expectNil: true,
			}),
			Entry("invalid storage size returns error", minioSpecFixture{
				name: "invalid storage",
				spec: apiv2.WBMinioSpec{
					Enabled:     true,
					Name:        "test-minio",
					Namespace:   testingOwnerNamespace,
					StorageSize: "invalid-size",
					Replicas:    1,
				},
				expectError: true,
			}),
			Entry("single replica dev configuration", minioSpecFixture{
				name: "dev configuration",
				spec: apiv2.WBMinioSpec{
					Enabled:     true,
					Name:        "dev-minio",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    1,
				},
				expectNil:                false,
				expectedName:             "dev-minio",
				expectedNamespace:        testingOwnerNamespace,
				expectedServers:          1,
				expectedVolumesPerServer: common.DevVolumesPerServer,
				expectedStorageSize:      "10Gi",
				expectedImage:            common.MinioImage,
				expectResources:          false,
				expectedOwnerReferences:  1,
			}),
			Entry("multi-replica prod configuration", minioSpecFixture{
				name: "prod configuration",
				spec: apiv2.WBMinioSpec{
					Enabled:     true,
					Name:        "prod-minio",
					Namespace:   testingOwnerNamespace,
					StorageSize: "100Gi",
					Replicas:    4,
				},
				expectNil:                false,
				expectedName:             "prod-minio",
				expectedNamespace:        testingOwnerNamespace,
				expectedServers:          4,
				expectedVolumesPerServer: common.ProdVolumesPerServer,
				expectedStorageSize:      "100Gi",
				expectedImage:            common.MinioImage,
				expectResources:          false,
				expectedOwnerReferences:  1,
			}),
			Entry("configuration with resources", minioSpecFixture{
				name: "with resources",
				spec: apiv2.WBMinioSpec{
					Enabled:     true,
					Name:        "resource-minio",
					Namespace:   testingOwnerNamespace,
					StorageSize: "50Gi",
					Replicas:    2,
					Config: apiv2.WBMinioConfig{
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
				expectNil:                false,
				expectedName:             "resource-minio",
				expectedNamespace:        testingOwnerNamespace,
				expectedServers:          2,
				expectedVolumesPerServer: common.ProdVolumesPerServer,
				expectedStorageSize:      "50Gi",
				expectedImage:            common.MinioImage,
				expectResources:          true,
				expectedCPURequest:       "2",
				expectedMemoryRequest:    "4Gi",
				expectedCPULimit:         "4",
				expectedMemoryLimit:      "8Gi",
				expectedOwnerReferences:  1,
			}),
		)
	})

	Describe("ToMinioConfigSecret", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		It("should create config secret with correct credentials", func() {
			spec := apiv2.WBMinioSpec{
				Name:      "test-minio",
				Namespace: testingOwnerNamespace,
			}

			result, err := ToMinioConfigSecret(ctx, spec, owner, testScheme)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).ToNot(BeNil())
			Expect(result.Name).To(Equal(tenant.ConfigName("test-minio")))
			Expect(result.Namespace).To(Equal(testingOwnerNamespace))
			Expect(result.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(result.StringData).To(HaveKey("config.env"))
			Expect(result.StringData["config.env"]).To(ContainSubstring("MINIO_ROOT_USER"))
			Expect(result.StringData["config.env"]).To(ContainSubstring("MINIO_ROOT_PASSWORD"))
			Expect(result.StringData["config.env"]).To(ContainSubstring("MINIO_BROWSER"))
			Expect(result.OwnerReferences).To(HaveLen(1))
		})
	})
})
