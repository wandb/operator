package altinity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/translator"
	chiv1 "github.com/wandb/operator/internal/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("computeStatusSummary", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("should compute correct status",
		func(chiCR *chiv1.ClickHouseInstallation, podsRunning map[string]bool, initialStatus *translator.ClickHouseStatus, expectedReady bool, expectedState string) {
			status := initialStatus

			computeStatusSummary(ctx, chiCR, podsRunning, status)

			Expect(status.Ready).To(Equal(expectedReady), "Ready status mismatch")
			Expect(status.State).To(Equal(expectedState), "State mismatch")

			if initialStatus.Connection.URL.Name != "" {
				Expect(status.Connection.URL.Name).To(Equal(initialStatus.Connection.URL.Name), "Connection should be preserved")
			}

			if len(initialStatus.Conditions) > 0 {
				Expect(status.Conditions).To(Equal(initialStatus.Conditions), "Conditions should be preserved")
			}
		},
		Entry("all pods running",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{"pod-0": true, "pod-1": true, "pod-2": true},
			&translator.ClickHouseStatus{},
			true,
			"Ready",
		),
		Entry("some pods not running",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{"pod-0": true, "pod-1": false, "pod-2": true},
			&translator.ClickHouseStatus{},
			false,
			"NotReady",
		),
		Entry("no pods running",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{"pod-0": false, "pod-1": false},
			&translator.ClickHouseStatus{},
			false,
			"NotReady",
		),
		Entry("empty pods map",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{},
			&translator.ClickHouseStatus{},
			false,
			"NotReady",
		),
		Entry("nil pods map",
			&chiv1.ClickHouseInstallation{},
			nil,
			&translator.ClickHouseStatus{},
			false,
			"NotReady",
		),
		Entry("single pod running",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{"pod-0": true},
			&translator.ClickHouseStatus{},
			true,
			"Ready",
		),
		Entry("single pod not running",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{"pod-0": false},
			&translator.ClickHouseStatus{},
			false,
			"NotReady",
		),
		Entry("status with existing connection should preserve it",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{"pod-0": true},
			&translator.ClickHouseStatus{
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
			&chiv1.ClickHouseInstallation{},
			map[string]bool{"pod-0": true},
			&translator.ClickHouseStatus{
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
		Entry("large number of pods all running",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{
				"pod-0": true, "pod-1": true, "pod-2": true, "pod-3": true,
				"pod-4": true, "pod-5": true, "pod-6": true, "pod-7": true,
			},
			&translator.ClickHouseStatus{},
			true,
			"Ready",
		),
		Entry("large number of pods with one not running",
			&chiv1.ClickHouseInstallation{},
			map[string]bool{
				"pod-0": true, "pod-1": true, "pod-2": true, "pod-3": true,
				"pod-4": true, "pod-5": false, "pod-6": true, "pod-7": true,
			},
			&translator.ClickHouseStatus{},
			false,
			"NotReady",
		),
	)
})
