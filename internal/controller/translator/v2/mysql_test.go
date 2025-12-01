package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/infra/mysql/percona"
	"github.com/wandb/operator/internal/controller/translator/common"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("MySQL Translator", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("TranslateMySQLStatus", func() {
		type statusFixture struct {
			name               string
			input              common.MySQLStatus
			expectedReady      bool
			expectedState      apiv2.WBStateType
			expectedHost       string
			expectedPort       string
			expectedUser       string
			expectedConditions int
		}

		DescribeTable("status translation scenarios",
			func(fixture statusFixture) {
				result := TranslateMySQLStatus(ctx, fixture.input)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.State).To(Equal(fixture.expectedState))
				Expect(result.Connection.MySQLHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.MySQLPort).To(Equal(fixture.expectedPort))
				Expect(result.Connection.MySQLUser).To(Equal(fixture.expectedUser))
				Expect(result.Conditions).To(HaveLen(fixture.expectedConditions))
				Expect(result.LastReconciled).ToNot(BeZero())
			},
			Entry("ready status with connection", statusFixture{
				name: "ready with connection",
				input: common.MySQLStatus{
					Ready: true,
					Connection: common.MySQLConnection{
						Host: "mysql.example.com",
						Port: "3306",
						User: "root",
					},
					Conditions: []common.MySQLCondition{
						common.NewMySQLCondition(common.MySQLConnectionCode, "Connected"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateReady,
				expectedHost:       "mysql.example.com",
				expectedPort:       "3306",
				expectedUser:       "root",
				expectedConditions: 1,
			}),
			Entry("creating status", statusFixture{
				name: "creating",
				input: common.MySQLStatus{
					Ready: false,
					Connection: common.MySQLConnection{
						Host: "",
						Port: "",
						User: "",
					},
					Conditions: []common.MySQLCondition{
						common.NewMySQLCondition(common.MySQLCreatedCode, "Creating MySQL cluster"),
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
				input: common.MySQLStatus{
					Ready: true,
					Connection: common.MySQLConnection{
						Host: "mysql.example.com",
						Port: "3306",
						User: "root",
					},
					Conditions: []common.MySQLCondition{
						common.NewMySQLCondition(common.MySQLUpdatedCode, "Updating configuration"),
					},
				},
				expectedReady:      true,
				expectedState:      apiv2.WBStateUpdating,
				expectedHost:       "mysql.example.com",
				expectedPort:       "3306",
				expectedUser:       "root",
				expectedConditions: 1,
			}),
			Entry("deleting status", statusFixture{
				name: "deleting",
				input: common.MySQLStatus{
					Ready: false,
					Connection: common.MySQLConnection{
						Host: "mysql.example.com",
						Port: "3306",
						User: "root",
					},
					Conditions: []common.MySQLCondition{
						common.NewMySQLCondition(common.MySQLDeletedCode, "Deleting resources"),
					},
				},
				expectedReady:      false,
				expectedState:      apiv2.WBStateDeleting,
				expectedHost:       "mysql.example.com",
				expectedPort:       "3306",
				expectedUser:       "root",
				expectedConditions: 1,
			}),
		)
	})

	Describe("ExtractMySQLStatus", func() {
		type extractFixture struct {
			name          string
			conditions    []common.MySQLCondition
			expectedReady bool
			expectedHost  string
			expectedPort  string
			expectedUser  string
		}

		DescribeTable("condition extraction scenarios",
			func(fixture extractFixture) {
				result := ExtractMySQLStatus(ctx, fixture.conditions)

				Expect(result.Ready).To(Equal(fixture.expectedReady))
				Expect(result.Connection.MySQLHost).To(Equal(fixture.expectedHost))
				Expect(result.Connection.MySQLPort).To(Equal(fixture.expectedPort))
				Expect(result.Connection.MySQLUser).To(Equal(fixture.expectedUser))
			},
			Entry("connection info available", extractFixture{
				name: "with connection info",
				conditions: []common.MySQLCondition{
					common.NewMySQLConnCondition(common.MySQLConnInfo{
						Host: "mysql-svc.default.svc.cluster.local",
						Port: "3306",
						User: "wandb",
					}),
				},
				expectedReady: true,
				expectedHost:  "mysql-svc.default.svc.cluster.local",
				expectedPort:  "3306",
				expectedUser:  "wandb",
			}),
			Entry("no connection info", extractFixture{
				name: "no connection info",
				conditions: []common.MySQLCondition{
					common.NewMySQLCondition(common.MySQLCreatedCode, "Created"),
				},
				expectedReady: false,
				expectedHost:  "",
				expectedPort:  "",
				expectedUser:  "",
			}),
		)
	})

	Describe("ToMySQLVendorSpec", func() {
		var (
			testScheme *runtime.Scheme
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			testScheme = runtime.NewScheme()
			Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			Expect(apiv2.AddToScheme(testScheme)).To(Succeed())
			Expect(pxcv1.SchemeBuilder.AddToScheme(testScheme)).To(Succeed())

			owner = &apiv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-wandb",
					Namespace: testingOwnerNamespace,
					UID:       "test-uid-12345",
				},
			}
		})

		type mysqlSpecFixture struct {
			name                    string
			spec                    apiv2.WBMySQLSpec
			expectNil               bool
			expectError             bool
			expectedName            string
			expectedNamespace       string
			expectedReplicas        int32
			expectedStorageSize     string
			expectedImage           string
			expectedProxySQLEnabled bool
			expectedProxySQLSize    int32
			expectedTLSEnabled      bool
			expectedUnsafePXCSize   bool
			expectedUnsafeProxySize bool
			expectedUnsafeTLS       bool
			expectedHAProxyDisabled bool
			expectedLogCollector    bool
			expectResources         bool
			expectedCPURequest      string
			expectedMemoryRequest   string
			expectedCPULimit        string
			expectedMemoryLimit     string
			expectedOwnerReferences int
		}

		DescribeTable("mysql spec translation scenarios",
			func(fixture mysqlSpecFixture) {
				result, err := ToMySQLVendorSpec(ctx, fixture.spec, owner, testScheme)

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
				Expect(result.Labels).To(HaveKeyWithValue("app", percona.ClusterName(fixture.spec.Name)))

				Expect(result.Spec.CRVersion).To(Equal(common.PXCCRVersion))
				Expect(result.Spec.Unsafe.PXCSize).To(Equal(fixture.expectedUnsafePXCSize))
				Expect(result.Spec.Unsafe.TLS).To(Equal(fixture.expectedUnsafeTLS))
				Expect(result.Spec.Unsafe.ProxySize).To(Equal(fixture.expectedUnsafeProxySize))

				Expect(result.Spec.PXC).ToNot(BeNil())
				Expect(result.Spec.PXC.PodSpec).ToNot(BeNil())
				Expect(result.Spec.PXC.PodSpec.Size).To(Equal(fixture.expectedReplicas))
				Expect(result.Spec.PXC.PodSpec.Image).To(Equal(fixture.expectedImage))

				Expect(result.Spec.PXC.PodSpec.VolumeSpec).ToNot(BeNil())
				Expect(result.Spec.PXC.PodSpec.VolumeSpec.PersistentVolumeClaim).ToNot(BeNil())
				storageRequest := result.Spec.PXC.PodSpec.VolumeSpec.PersistentVolumeClaim.Resources.Requests[corev1.ResourceStorage]
				Expect(storageRequest.String()).To(Equal(fixture.expectedStorageSize))

				Expect(result.Spec.TLS).ToNot(BeNil())
				Expect(*result.Spec.TLS.Enabled).To(Equal(fixture.expectedTLSEnabled))

				if fixture.expectedProxySQLEnabled {
					Expect(result.Spec.ProxySQL).ToNot(BeNil())
					Expect(result.Spec.ProxySQL.PodSpec.Enabled).To(BeTrue())
					Expect(result.Spec.ProxySQL.PodSpec.Size).To(Equal(fixture.expectedProxySQLSize))
					Expect(result.Spec.ProxySQL.PodSpec.Image).To(Equal(common.ProxySQLImage))
					Expect(result.Spec.ProxySQL.PodSpec.VolumeSpec).ToNot(BeNil())
					Expect(result.Spec.ProxySQL.PodSpec.VolumeSpec.EmptyDir).ToNot(BeNil())
				} else {
					Expect(result.Spec.ProxySQL).To(BeNil())
				}

				if fixture.expectedHAProxyDisabled {
					Expect(result.Spec.HAProxy).ToNot(BeNil())
					Expect(result.Spec.HAProxy.PodSpec.Enabled).To(BeFalse())
				}

				if fixture.expectedLogCollector {
					Expect(result.Spec.LogCollector).ToNot(BeNil())
					Expect(result.Spec.LogCollector.Enabled).To(BeTrue())
					Expect(result.Spec.LogCollector.Image).To(Equal(common.LogCollectorImg))
				} else {
					Expect(result.Spec.LogCollector).To(BeNil())
				}

				if fixture.expectResources {
					Expect(result.Spec.PXC.PodSpec.Resources).ToNot(Equal(corev1.ResourceRequirements{}))
					if fixture.expectedCPURequest != "" {
						cpuRequest := result.Spec.PXC.PodSpec.Resources.Requests[corev1.ResourceCPU]
						Expect(cpuRequest.String()).To(Equal(fixture.expectedCPURequest))
					}
					if fixture.expectedMemoryRequest != "" {
						memRequest := result.Spec.PXC.PodSpec.Resources.Requests[corev1.ResourceMemory]
						Expect(memRequest.String()).To(Equal(fixture.expectedMemoryRequest))
					}
					if fixture.expectedCPULimit != "" {
						cpuLimit := result.Spec.PXC.PodSpec.Resources.Limits[corev1.ResourceCPU]
						Expect(cpuLimit.String()).To(Equal(fixture.expectedCPULimit))
					}
					if fixture.expectedMemoryLimit != "" {
						memLimit := result.Spec.PXC.PodSpec.Resources.Limits[corev1.ResourceMemory]
						Expect(memLimit.String()).To(Equal(fixture.expectedMemoryLimit))
					}
				}

				Expect(result.OwnerReferences).To(HaveLen(fixture.expectedOwnerReferences))
				if fixture.expectedOwnerReferences > 0 {
					Expect(result.OwnerReferences[0].UID).To(Equal(owner.UID))
					Expect(result.OwnerReferences[0].Name).To(Equal(owner.Name))
				}
			},
			Entry("disabled mysql returns nil", mysqlSpecFixture{
				name: "disabled mysql",
				spec: apiv2.WBMySQLSpec{
					Enabled: false,
				},
				expectNil: true,
			}),
			Entry("invalid storage size returns error", mysqlSpecFixture{
				name: "invalid storage",
				spec: apiv2.WBMySQLSpec{
					Enabled:     true,
					Name:        "test-mysql",
					Namespace:   testingOwnerNamespace,
					StorageSize: "invalid-size",
					Replicas:    1,
				},
				expectError: true,
			}),
			Entry("single replica dev configuration", mysqlSpecFixture{
				name: "dev configuration",
				spec: apiv2.WBMySQLSpec{
					Enabled:     true,
					Name:        "dev-mysql",
					Namespace:   testingOwnerNamespace,
					StorageSize: "10Gi",
					Replicas:    1,
				},
				expectNil:               false,
				expectedName:            "dev-mysql-cluster",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        1,
				expectedStorageSize:     "10Gi",
				expectedImage:           common.DevPXCImage,
				expectedProxySQLEnabled: false,
				expectedTLSEnabled:      false,
				expectedUnsafePXCSize:   true,
				expectedUnsafeProxySize: true,
				expectedUnsafeTLS:       true,
				expectedHAProxyDisabled: true,
				expectedLogCollector:    true,
				expectResources:         false,
				expectedOwnerReferences: 1,
			}),
			Entry("two replica configuration with ProxySQL", mysqlSpecFixture{
				name: "two replica prod",
				spec: apiv2.WBMySQLSpec{
					Enabled:     true,
					Name:        "ha-mysql",
					Namespace:   testingOwnerNamespace,
					StorageSize: "50Gi",
					Replicas:    2,
				},
				expectNil:               false,
				expectedName:            "ha-mysql-cluster",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        2,
				expectedStorageSize:     "50Gi",
				expectedImage:           common.ProdPXCImage,
				expectedProxySQLEnabled: true,
				expectedProxySQLSize:    2,
				expectedTLSEnabled:      true,
				expectedUnsafePXCSize:   false,
				expectedUnsafeProxySize: false,
				expectedUnsafeTLS:       false,
				expectedHAProxyDisabled: false,
				expectedLogCollector:    false,
				expectResources:         false,
				expectedOwnerReferences: 1,
			}),
			Entry("three+ replica prod configuration", mysqlSpecFixture{
				name: "prod configuration",
				spec: apiv2.WBMySQLSpec{
					Enabled:     true,
					Name:        "prod-mysql",
					Namespace:   testingOwnerNamespace,
					StorageSize: "100Gi",
					Replicas:    3,
				},
				expectNil:               false,
				expectedName:            "prod-mysql-cluster",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        3,
				expectedStorageSize:     "100Gi",
				expectedImage:           common.ProdPXCImage,
				expectedProxySQLEnabled: true,
				expectedProxySQLSize:    3,
				expectedTLSEnabled:      true,
				expectedUnsafePXCSize:   false,
				expectedUnsafeProxySize: false,
				expectedUnsafeTLS:       false,
				expectedHAProxyDisabled: false,
				expectedLogCollector:    false,
				expectResources:         false,
				expectedOwnerReferences: 1,
			}),
			Entry("configuration with resources", mysqlSpecFixture{
				name: "with resources",
				spec: apiv2.WBMySQLSpec{
					Enabled:     true,
					Name:        "resource-mysql",
					Namespace:   testingOwnerNamespace,
					StorageSize: "20Gi",
					Replicas:    3,
					Config: apiv2.WBMySQLConfig{
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
				expectedName:            "resource-mysql-cluster",
				expectedNamespace:       testingOwnerNamespace,
				expectedReplicas:        3,
				expectedStorageSize:     "20Gi",
				expectedImage:           common.ProdPXCImage,
				expectedProxySQLEnabled: true,
				expectedProxySQLSize:    3,
				expectedTLSEnabled:      true,
				expectedUnsafePXCSize:   false,
				expectedUnsafeProxySize: false,
				expectedUnsafeTLS:       false,
				expectResources:         true,
				expectedCPURequest:      "2",
				expectedMemoryRequest:   "4Gi",
				expectedCPULimit:        "4",
				expectedMemoryLimit:     "8Gi",
				expectedOwnerReferences: 1,
			}),
		)
	})
})
