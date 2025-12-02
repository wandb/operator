package percona

import (
	"context"
	"fmt"
	"testing"

	"github.com/wandb/operator/internal/controller/translator/common"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMySQLConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MySQL Percona Conditions Suite")
}

var _ = Describe("MySQL Percona Conditions", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(pxcv1.SchemeBuilder.AddToScheme(scheme)).To(Succeed())
	})

	Describe("GetConditions", func() {
		It("should create connection condition when cluster exists", func() {
			specName := "test-mysql"
			specNamespace := "default"
			namespacedName := types.NamespacedName{
				Name:      specName,
				Namespace: specNamespace,
			}

			pxc := &pxcv1.PerconaXtraDBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ClusterName(specName),
					Namespace: specNamespace,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(pxc).
				Build()

			result, err := GetConditions(ctx, fakeClient, namespacedName)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Code()).To(Equal(string(common.MySQLConnectionCode)))

			status := common.ExtractMySQLStatus(ctx, result)
			Expect(status.Connection.Host).To(Equal(fmt.Sprintf("%s.%s.svc.cluster.local", ClusterName(specName), specNamespace)))
			Expect(status.Connection.Port).To(Equal("3306"))
			Expect(status.Connection.User).To(Equal("root"))
			Expect(status.Ready).To(BeTrue())
		})

		It("should still create condition when cluster doesn't exist", func() {
			namespacedName := types.NamespacedName{
				Name:      "missing-mysql",
				Namespace: "test-ns",
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			result, err := GetConditions(ctx, fakeClient, namespacedName)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
		})

		Context("connection string formatting", func() {
			It("should use cluster name in host for both dev and HA mode", func() {
				specName := "my-mysql"
				namespacedName := types.NamespacedName{
					Name:      specName,
					Namespace: "my-namespace",
				}

				pxc := &pxcv1.PerconaXtraDBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ClusterName(specName),
						Namespace: "my-namespace",
					},
					Spec: pxcv1.PerconaXtraDBClusterSpec{
						ProxySQL: &pxcv1.ProxySQLSpec{
							PodSpec: pxcv1.PodSpec{
								Enabled: true,
							},
						},
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(pxc).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractMySQLStatus(ctx, result)
				expectedHost := fmt.Sprintf("%s.my-namespace.svc.cluster.local", ClusterName(specName))
				Expect(status.Connection.Host).To(Equal(expectedHost))
			})

			It("should use port 3306", func() {
				namespacedName := types.NamespacedName{
					Name:      "test-mysql",
					Namespace: "test-ns",
				}

				pxc := &pxcv1.PerconaXtraDBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ClusterName("test-mysql"),
						Namespace: "test-ns",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(pxc).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractMySQLStatus(ctx, result)
				Expect(status.Connection.Port).To(Equal("3306"))
			})
		})
	})
})
