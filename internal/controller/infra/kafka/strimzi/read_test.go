package strimzi

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/translator"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("computeStatusSummary", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	DescribeTable("should compute correct status",
		func(kafkaCR *strimziv1.Kafka, initialStatus *translator.KafkaStatus, expectedReady bool, expectedState string) {
			status := initialStatus

			computeStatusSummary(ctx, kafkaCR, status)

			Expect(status.Ready).To(Equal(expectedReady), "Ready status mismatch")
			Expect(status.State).To(Equal(expectedState), "State mismatch")

			if initialStatus.Connection.URL.Name != "" {
				Expect(status.Connection.URL.Name).To(Equal(initialStatus.Connection.URL.Name), "Connection should be preserved")
			}

			if len(initialStatus.Conditions) > 0 {
				Expect(status.Conditions).To(Equal(initialStatus.Conditions), "Conditions should be preserved")
			}
		},
		Entry("kafkaCR is nil",
			nil,
			&translator.KafkaStatus{},
			false,
			"Not Installed",
		),
		Entry("kafkaCR has KafkaMetadataState set with ready condition true",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "KRaft",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"KRaft",
		),
		Entry("kafkaCR has KafkaMetadataState set with ready condition false",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "ZooKeeper",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"ZooKeeper",
		),
		Entry("ready condition exists and is true",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Ready",
		),
		Entry("ready condition exists and is false",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Not Ready",
		),
		Entry("ready condition is missing",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Not Ready",
		),
		Entry("state is empty string and ready is true",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Ready",
		),
		Entry("state is empty string and ready is false",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Not Ready",
		),
		Entry("state is non-empty and ready is true",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "Migrating",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Migrating",
		),
		Entry("state is non-empty and ready is false",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "Error",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Error",
		),
		Entry("different condition types (non-ready conditions) should not affect ready status",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Available",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "Progressing",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Not Ready",
		),
		Entry("ready condition with case insensitive type match (lowercase)",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Ready",
		),
		Entry("ready condition with case insensitive type match (uppercase)",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "READY",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Ready",
		),
		Entry("ready condition with case insensitive type match (mixed case)",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "ReAdY",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Ready",
		),
		Entry("multiple conditions with ready condition true",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Available",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "Progressing",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Ready",
		),
		Entry("multiple conditions with ready condition false",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Available",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
						{
							Type:   "Progressing",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Not Ready",
		),
		Entry("ready condition with unknown status (should be false)",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionUnknown,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Not Ready",
		),
		Entry("kafkaCR with empty conditions array",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "Initializing",
					Conditions:         []metav1.Condition{},
				},
			},
			&translator.KafkaStatus{},
			false,
			"Initializing",
		),
		Entry("kafkaCR with nil conditions array",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "Pending",
					Conditions:         nil,
				},
			},
			&translator.KafkaStatus{},
			false,
			"Pending",
		),
		Entry("kafka metadata state with empty string and no conditions",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "",
					Conditions:         nil,
				},
			},
			&translator.KafkaStatus{},
			false,
			"Not Ready",
		),
		Entry("kafka metadata state with special characters",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "State-With-Dashes",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"State-With-Dashes",
		),
		Entry("status with existing connection should preserve it",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{
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
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			&translator.KafkaStatus{
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
		Entry("multiple ready conditions (first one wins)",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			true,
			"Ready",
		),
		Entry("long kafka metadata state value",
			&strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "VeryLongStateNameThatRepresentsComplexInternalKafkaMetadataStateTransition",
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			&translator.KafkaStatus{},
			false,
			"VeryLongStateNameThatRepresentsComplexInternalKafkaMetadataStateTransition",
		),
	)
})
