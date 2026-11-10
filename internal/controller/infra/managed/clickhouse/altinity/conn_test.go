package altinity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ClickHouse connection topology", func() {
	chiWithReplicas := func(replicas int) *chiv1.ClickHouseInstallation {
		return &chiv1.ClickHouseInstallation{
			ObjectMeta: metav1.ObjectMeta{Name: "wandb-chi", Namespace: "wandb"},
			Spec: chiv1.ChiSpec{
				Configuration: &chiv1.Configuration{
					Clusters: []*chiv1.Cluster{{
						Name:   chiClusterName,
						Layout: &chiv1.ChiClusterLayout{ShardsCount: ShardsCount, ReplicasCount: replicas},
					}},
				},
			},
			Status: &chiv1.Status{Endpoint: "clickhouse.wandb.svc"},
		}
	}

	// Weave only creates ReplicatedMergeTree tables when told the cluster is
	// replicated, and the topology comes off the live CHI: a spec change hasn't
	// reached ClickHouse until the CHI carries it.
	DescribeTable("derives replication from the live CHI layout",
		func(replicas int, expected bool) {
			connInfo := readConnectionDetails(chiWithReplicas(replicas))
			Expect(connInfo).ToNot(BeNil())
			Expect(connInfo.Replicated).To(Equal(expected))
		},
		Entry("a single replica is not replicated", 1, false),
		Entry("two replicas are replicated", 2, true),
		Entry("three replicas are replicated", 3, true),
	)

	It("publishes the CHI cluster name, not the Service-name derivation", func() {
		connInfo := readConnectionDetails(chiWithReplicas(2))
		Expect(connInfo.ClusterName).To(Equal(CHIClusterName()))
		Expect(connInfo.ClusterName).ToNot(Equal(ClusterName("wandb-chi")))
	})

	It("reports no topology for a CHI with no cluster layout", func() {
		chi := chiWithReplicas(2)
		chi.Spec.Configuration.Clusters[0].Layout = nil

		connInfo := readConnectionDetails(chi)
		Expect(connInfo.Replicated).To(BeFalse())
		Expect(connInfo.ClusterName).To(BeEmpty())
	})

	Describe("writeClickHouseConnInfo", func() {
		var (
			ctx        context.Context
			nsnBuilder *NsNameBuilder
			owner      *apiv2.WeightsAndBiases
		)

		BeforeEach(func() {
			ctx = context.Background()
			nsnBuilder = createNsNameBuilder(types.NamespacedName{Name: "wandb-chi", Namespace: "wandb"})
			owner = &apiv2.WeightsAndBiases{
				TypeMeta:   metav1.TypeMeta{APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases"},
				ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb", UID: "wandb-uid"},
			}
		})

		newClient := func() *fake.ClientBuilder {
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(apiv2.AddToScheme(scheme)).To(Succeed())
			return fake.NewClientBuilder().WithScheme(scheme)
		}

		// The connection Secret is the only thing applications read, so topology
		// has to travel in it alongside host and database.
		It("writes topology into the connection Secret and returns selectors", func() {
			c := newClient().Build()

			conn, err := writeClickHouseConnInfo(ctx, c, owner, nsnBuilder, readConnectionDetails(chiWithReplicas(3)))
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(c.Get(ctx, nsnBuilder.ConnectionNsName(), secret)).To(Succeed())
			Expect(secret.StringData).To(HaveKeyWithValue("Replicated", "true"))
			Expect(secret.StringData).To(HaveKeyWithValue("ClusterName", chiClusterName))

			Expect(conn.Replicated.SecretKeyRef().Name).To(Equal(nsnBuilder.ConnectionNsName().Name))
			Expect(conn.Replicated.SecretKeyRef().Key).To(Equal("Replicated"))
			Expect(conn.ClusterName.SecretKeyRef().Key).To(Equal("ClusterName"))
		})

		// An unreplicated cluster must say so explicitly: applications need to
		// distinguish "not replicated" from "nothing published".
		It("writes an explicit false for a single-replica cluster", func() {
			c := newClient().Build()

			_, err := writeClickHouseConnInfo(ctx, c, owner, nsnBuilder, readConnectionDetails(chiWithReplicas(1)))
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			Expect(c.Get(ctx, nsnBuilder.ConnectionNsName(), secret)).To(Succeed())
			Expect(secret.StringData).To(HaveKeyWithValue("Replicated", "false"))
		})
	})
})
