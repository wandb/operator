package tenant

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/translator"
	miniov2 "github.com/wandb/operator/internal/vendored/minio-operator/minio.min.io/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("computeStatusSummary", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("should compute correct status",
		func(tenantCR *miniov2.Tenant, initialStatus *translator.MinioStatus, expectedReady bool, expectedState string) {
			status := initialStatus

			computeStatusSummary(ctx, tenantCR, status)

			Expect(status.Ready).To(Equal(expectedReady), "Ready status mismatch")
			Expect(status.State).To(Equal(expectedState), "State mismatch")

			if initialStatus.Connection.URL.Name != "" {
				Expect(status.Connection.URL.Name).To(Equal(initialStatus.Connection.URL.Name), "Connection should be preserved")
			}

			if len(initialStatus.Conditions) > 0 {
				Expect(status.Conditions).To(Equal(initialStatus.Conditions), "Conditions should be preserved")
			}
		},
		Entry("tenantCR is nil",
			nil,
			&translator.MinioStatus{},
			false,
			"Not Installed",
		),
		Entry("health status is green",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus: miniov2.HealthStatusGreen,
				},
			},
			&translator.MinioStatus{},
			true,
			"Ready",
		),
		Entry("health status is yellow",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus: miniov2.HealthStatusYellow,
				},
			},
			&translator.MinioStatus{},
			true,
			"Degraded",
		),
		Entry("health status is red",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus: miniov2.HealthStatusRed,
				},
			},
			&translator.MinioStatus{},
			false,
			"Error",
		),
		Entry("health status is empty (default case)",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus: "",
				},
			},
			&translator.MinioStatus{},
			false,
			"NotReady",
		),
		Entry("health status is unknown value (default case)",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus: "unknown",
				},
			},
			&translator.MinioStatus{},
			false,
			"NotReady",
		),
		Entry("status with existing connection should preserve it",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus: miniov2.HealthStatusGreen,
				},
			},
			&translator.MinioStatus{
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
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus: miniov2.HealthStatusGreen,
				},
			},
			&translator.MinioStatus{
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
		Entry("yellow status with health message",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus:  miniov2.HealthStatusYellow,
					HealthMessage: "Some drives are offline",
				},
			},
			&translator.MinioStatus{},
			true,
			"Degraded",
		),
		Entry("red status with health message",
			&miniov2.Tenant{
				Status: miniov2.TenantStatus{
					HealthStatus:  miniov2.HealthStatusRed,
					HealthMessage: "Lost write quorum",
				},
			},
			&translator.MinioStatus{},
			false,
			"Error",
		),
	)
})
