package altinity

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/wandb/operator/internal/controller/translator/common"
	chiv2 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClickHouseConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ClickHouse Altinity Conditions Suite")
}

var _ = Describe("ClickHouse Altinity Conditions", func() {
	var (
		ctx    context.Context
		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(chiv2.AddToScheme(scheme)).To(Succeed())
	})

	Describe("GetConditions", func() {
		It("should create connection condition when installation exists", func() {
			specName := "test-clickhouse"
			specNamespace := "default"
			namespacedName := types.NamespacedName{
				Name:      specName,
				Namespace: specNamespace,
			}

			chi := &chiv2.ClickHouseInstallation{
				ObjectMeta: metav1.ObjectMeta{
					Name:      InstallationName(specName),
					Namespace: specNamespace,
				},
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(chi).
				Build()

			result, err := GetConditions(ctx, fakeClient, namespacedName)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].Code()).To(Equal(string(common.ClickHouseConnectionCode)))

			status := common.ExtractClickHouseStatus(ctx, result)
			Expect(status.Connection.Host).To(Equal(fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, specNamespace)))
			Expect(status.Connection.Port).To(Equal(strconv.Itoa(ClickHouseNativePort)))
			Expect(status.Connection.User).To(Equal(ClickHouseUser))
			Expect(status.Ready).To(BeTrue())
		})

		It("should still create condition when installation doesn't exist", func() {
			namespacedName := types.NamespacedName{
				Name:      "missing-chi",
				Namespace: "test-ns",
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			result, err := GetConditions(ctx, fakeClient, namespacedName)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))

			status := common.ExtractClickHouseStatus(ctx, result)
			Expect(status.Connection.Host).To(Equal(fmt.Sprintf("%s.%s.svc.cluster.local", ServiceName, "test-ns")))
		})

		Context("connection string formatting", func() {
			It("should use ServiceName constant in host", func() {
				namespacedName := types.NamespacedName{
					Name:      "my-chi",
					Namespace: "my-namespace",
				}

				chi := &chiv2.ClickHouseInstallation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      InstallationName("my-chi"),
						Namespace: "my-namespace",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(chi).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractClickHouseStatus(ctx, result)
				Expect(status.Connection.Host).To(Equal(fmt.Sprintf("%s.my-namespace.svc.cluster.local", ServiceName)))
			})

			It("should use ClickHouseNativePort for port", func() {
				namespacedName := types.NamespacedName{
					Name:      "test-chi",
					Namespace: "test-ns",
				}

				chi := &chiv2.ClickHouseInstallation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      InstallationName("test-chi"),
						Namespace: "test-ns",
					},
				}

				fakeClient := fake.NewClientBuilder().
					WithScheme(scheme).
					WithRuntimeObjects(chi).
					Build()

				result, err := GetConditions(ctx, fakeClient, namespacedName)

				Expect(err).ToNot(HaveOccurred())
				status := common.ExtractClickHouseStatus(ctx, result)
				Expect(status.Connection.Port).To(Equal(strconv.Itoa(ClickHouseNativePort)))
				Expect(status.Connection.User).To(Equal(ClickHouseUser))
			})
		})
	})
})
