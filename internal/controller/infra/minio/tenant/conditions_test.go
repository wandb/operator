package tenant

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/wandb/operator/internal/controller/translator/common"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Minio Tenant Conditions Suite")
}

var _ = Describe("Minio Tenant Conditions", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(miniov2.AddToScheme(scheme)).To(Succeed())
	})

	Describe("GetConditions", func() {
		type conditionsFixture struct {
			name               string
			specName           string
			specNamespace      string
			tenantExists       bool
			secretExists       bool
			expectedConditions int
			expectedHost       string
			expectedPort       string
			expectedAccessKey  string
			expectError        bool
		}

		DescribeTable("connection info extraction scenarios",
			func(fixture conditionsFixture) {
				namespacedName := types.NamespacedName{
					Name:      fixture.specName,
					Namespace: fixture.specNamespace,
				}

				var objects []runtime.Object

				if fixture.tenantExists {
					tenant := &miniov2.Tenant{
						ObjectMeta: metav1.ObjectMeta{
							Name:      TenantName(fixture.specName),
							Namespace: fixture.specNamespace,
						},
					}
					objects = append(objects, tenant)
				}

				if fixture.secretExists {
					secret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      ConfigName(fixture.specName),
							Namespace: fixture.specNamespace,
						},
					}
					objects = append(objects, secret)
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(objects...).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				if fixture.expectError {
					Expect(err).To(HaveOccurred())
					return
				}

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(fixture.expectedConditions))

				if fixture.expectedConditions > 0 {
					Expect(result[0].Code()).To(Equal(string(common.MinioConnectionCode)))

					status := common.ExtractMinioStatus(ctx, result)
					Expect(status.Connection.Host).To(Equal(fixture.expectedHost))
					Expect(status.Connection.Port).To(Equal(fixture.expectedPort))
					Expect(status.Connection.AccessKey).To(Equal(fixture.expectedAccessKey))
					Expect(status.Ready).To(BeTrue())
				}
			},
			Entry("both tenant and secret exist", conditionsFixture{
				name:               "happy path",
				specName:           "test-minio",
				specNamespace:      "default",
				tenantExists:       true,
				secretExists:       true,
				expectedConditions: 1,
				expectedHost:       "test-minio-hl.default.svc.cluster.local",
				expectedPort:       strconv.Itoa(MinioPort),
				expectedAccessKey:  MinioAccessKey,
				expectError:        false,
			}),
			Entry("tenant doesn't exist still creates condition", conditionsFixture{
				name:               "no tenant",
				specName:           "missing-minio",
				specNamespace:      "default",
				tenantExists:       false,
				secretExists:       true,
				expectedConditions: 1,
				expectedHost:       "missing-minio-hl.default.svc.cluster.local",
				expectedPort:       strconv.Itoa(MinioPort),
				expectedAccessKey:  MinioAccessKey,
				expectError:        false,
			}),
			Entry("secret doesn't exist still creates condition", conditionsFixture{
				name:               "no secret",
				specName:           "test-minio",
				specNamespace:      "default",
				tenantExists:       true,
				secretExists:       false,
				expectedConditions: 1,
				expectedHost:       "test-minio-hl.default.svc.cluster.local",
				expectedPort:       strconv.Itoa(MinioPort),
				expectedAccessKey:  MinioAccessKey,
				expectError:        false,
			}),
			Entry("neither resource exists still creates condition", conditionsFixture{
				name:               "no resources",
				specName:           "missing-minio",
				specNamespace:      "default",
				tenantExists:       false,
				secretExists:       false,
				expectedConditions: 1,
				expectedHost:       "missing-minio-hl.default.svc.cluster.local",
				expectedPort:       strconv.Itoa(MinioPort),
				expectedAccessKey:  MinioAccessKey,
				expectError:        false,
			}),
		)

		Context("connection string formatting", func() {
			It("should format host as {serviceName}.{namespace}.svc.cluster.local", func() {
				namespacedName := types.NamespacedName{
					Name:      "my-minio",
					Namespace: "my-namespace",
				}

				tenant := &miniov2.Tenant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      TenantName("my-minio"),
						Namespace: "my-namespace",
					},
				}

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ConfigName("my-minio"),
						Namespace: "my-namespace",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(tenant, secret).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(1))

				status := common.ExtractMinioStatus(ctx, result)
				expectedHost := fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName("my-minio"), "my-namespace")
				Expect(status.Connection.Host).To(Equal(expectedHost))
				Expect(status.Connection.Host).To(Equal("my-minio-hl.my-namespace.svc.cluster.local"))
			})

			It("should use correct port number", func() {
				namespacedName := types.NamespacedName{
					Name:      "test-minio",
					Namespace: "test-ns",
				}

				tenant := &miniov2.Tenant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      TenantName("test-minio"),
						Namespace: "test-ns",
					},
				}

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ConfigName("test-minio"),
						Namespace: "test-ns",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(tenant, secret).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(1))

				status := common.ExtractMinioStatus(ctx, result)
				Expect(status.Connection.Port).To(Equal(strconv.Itoa(MinioPort)))
				Expect(status.Connection.Port).To(Equal("443"))
			})

			It("should use correct access key", func() {
				namespacedName := types.NamespacedName{
					Name:      "test-minio",
					Namespace: "test-ns",
				}

				tenant := &miniov2.Tenant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      TenantName("test-minio"),
						Namespace: "test-ns",
					},
				}

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ConfigName("test-minio"),
						Namespace: "test-ns",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(tenant, secret).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(1))

				status := common.ExtractMinioStatus(ctx, result)
				Expect(status.Connection.AccessKey).To(Equal(MinioAccessKey))
			})
		})
	})
})
