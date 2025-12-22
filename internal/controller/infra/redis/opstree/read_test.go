package opstree

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/translator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("computeStandaloneStatusSummary", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("should compute correct status",
		func(podsRunning map[string]bool, initialStatus *translator.RedisStatus, expectedReady bool, expectedState string) {
			status := initialStatus

			computeStandaloneStatusSummary(ctx, podsRunning, status)

			Expect(status.Ready).To(Equal(expectedReady), "Ready status mismatch")
			Expect(status.State).To(Equal(expectedState), "State mismatch")

			if initialStatus.Connection.URL.Name != "" {
				Expect(status.Connection.URL.Name).To(Equal(initialStatus.Connection.URL.Name), "Connection should be preserved")
			}

			if len(initialStatus.Conditions) > 0 {
				Expect(status.Conditions).To(Equal(initialStatus.Conditions), "Conditions should be preserved")
			}
		},
		Entry("standalone pod running",
			map[string]bool{"redis-0": true},
			&translator.RedisStatus{},
			true,
			"Ready",
		),
		Entry("standalone pod not running",
			map[string]bool{"redis-0": false},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("no pods",
			map[string]bool{},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("nil pods map",
			nil,
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("status with existing connection should preserve it",
			map[string]bool{"redis-0": true},
			&translator.RedisStatus{
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
			map[string]bool{"redis-0": true},
			&translator.RedisStatus{
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
	)
})

var _ = Describe("computeSentinelStatusSummary", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("should compute correct status",
		func(sentinelPodsRunning, replicationPodsRunning map[string]bool, initialStatus *translator.RedisStatus, expectedReady bool, expectedState string) {
			status := initialStatus

			computeSentinelStatusSummary(ctx, sentinelPodsRunning, replicationPodsRunning, status)

			Expect(status.Ready).To(Equal(expectedReady), "Ready status mismatch")
			Expect(status.State).To(Equal(expectedState), "State mismatch")

			if initialStatus.Connection.URL.Name != "" {
				Expect(status.Connection.URL.Name).To(Equal(initialStatus.Connection.URL.Name), "Connection should be preserved")
			}

			if len(initialStatus.Conditions) > 0 {
				Expect(status.Conditions).To(Equal(initialStatus.Conditions), "Conditions should be preserved")
			}
		},
		Entry("all sentinel and replication pods running",
			map[string]bool{"sentinel-0": true, "sentinel-1": true, "sentinel-2": true},
			map[string]bool{"replica-0": true, "replica-1": true},
			&translator.RedisStatus{},
			true,
			"Ready",
		),
		Entry("sentinel pod not running",
			map[string]bool{"sentinel-0": true, "sentinel-1": false, "sentinel-2": true},
			map[string]bool{"replica-0": true, "replica-1": true},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("replication pod not running",
			map[string]bool{"sentinel-0": true, "sentinel-1": true, "sentinel-2": true},
			map[string]bool{"replica-0": true, "replica-1": false},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("no sentinel pods",
			map[string]bool{},
			map[string]bool{"replica-0": true, "replica-1": true},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("no replication pods",
			map[string]bool{"sentinel-0": true, "sentinel-1": true, "sentinel-2": true},
			map[string]bool{},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("all pods not running",
			map[string]bool{"sentinel-0": false, "sentinel-1": false},
			map[string]bool{"replica-0": false, "replica-1": false},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("nil sentinel pods map",
			nil,
			map[string]bool{"replica-0": true},
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("nil replication pods map",
			map[string]bool{"sentinel-0": true},
			nil,
			&translator.RedisStatus{},
			false,
			"NotReady",
		),
		Entry("status with existing connection should preserve it",
			map[string]bool{"sentinel-0": true},
			map[string]bool{"replica-0": true},
			&translator.RedisStatus{
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
			map[string]bool{"sentinel-0": true},
			map[string]bool{"replica-0": true},
			&translator.RedisStatus{
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
	)
})
