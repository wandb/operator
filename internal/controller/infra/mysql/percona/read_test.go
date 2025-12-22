package percona

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/translator"
	pxcv1 "github.com/wandb/operator/internal/vendored/percona-operator/pxc/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("computeStatusSummary", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("should compute correct status",
		func(clusterCR *pxcv1.PerconaXtraDBCluster, initialStatus *translator.MysqlStatus, expectedReady bool, expectedState string) {
			status := initialStatus

			computeStatusSummary(ctx, clusterCR, status)

			Expect(status.Ready).To(Equal(expectedReady), "Ready status mismatch")
			Expect(status.State).To(Equal(expectedState), "State mismatch")

			if initialStatus.Connection.URL.Name != "" {
				Expect(status.Connection.URL.Name).To(Equal(initialStatus.Connection.URL.Name), "Connection should be preserved")
			}

			if len(initialStatus.Conditions) > 0 {
				Expect(status.Conditions).To(Equal(initialStatus.Conditions), "Conditions should be preserved")
			}
		},
		Entry("clusterCR is nil",
			nil,
			&translator.MysqlStatus{},
			false,
			"Not Installed",
		),
		Entry("cluster status is ready",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStateReady,
				},
			},
			&translator.MysqlStatus{},
			true,
			"Ready",
		),
		Entry("cluster status is initializing",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStateInit,
				},
			},
			&translator.MysqlStatus{},
			false,
			"Initializing",
		),
		Entry("cluster status is paused",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStatePaused,
				},
			},
			&translator.MysqlStatus{},
			false,
			"Paused",
		),
		Entry("cluster status is stopping",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStateStopping,
				},
			},
			&translator.MysqlStatus{},
			false,
			"Stopping",
		),
		Entry("cluster status is error",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStateError,
				},
			},
			&translator.MysqlStatus{},
			false,
			"Error",
		),
		Entry("status with existing connection should preserve it",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStateReady,
				},
			},
			&translator.MysqlStatus{
				Connection: translator.InfraConnection{
					URL: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "existing-secret",
						},
						Key: "url",
					},
				},
			},
			true,
			"Ready",
		),
		Entry("status with existing conditions should preserve them",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStateReady,
				},
			},
			&translator.MysqlStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Custom",
						Status: metav1.ConditionTrue,
					},
				},
			},
			true,
			"Ready",
		),
		Entry("cluster with empty status defaults to capitalized empty string",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: "",
				},
			},
			&translator.MysqlStatus{},
			false,
			"",
		),
		Entry("cluster status is ready with host set",
			&pxcv1.PerconaXtraDBCluster{
				Status: pxcv1.PerconaXtraDBClusterStatus{
					Status: pxcv1.AppStateReady,
					Host:   "mysql-cluster.namespace.svc.cluster.local",
				},
			},
			&translator.MysqlStatus{},
			true,
			"Ready",
		),
	)
})
