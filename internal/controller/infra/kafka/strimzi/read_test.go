package strimzi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wandb/operator/internal/controller/translator"
	strimziv1 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputeStatusSummary(t *testing.T) {
	tests := []struct {
		name          string
		kafkaCR       *strimziv1.Kafka
		initialStatus *translator.KafkaStatus
		expectedReady bool
		expectedState string
	}{
		{
			name:          "kafkaCR is nil",
			kafkaCR:       nil,
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotInstalled",
		},
		{
			name: "kafkaCR has KafkaMetadataState set with ready condition true",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "kafkaCR has KafkaMetadataState set with ready condition false",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "ready condition exists and is true",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "ready condition exists and is false",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "ready condition is missing",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "state is empty string and ready is true",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "state is empty string and ready is false",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "state is non-empty and ready is true",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "state is non-empty and ready is false",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "different condition types (non-ready conditions) should not affect ready status",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "ready condition with case insensitive type match (lowercase)",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "ready condition with case insensitive type match (uppercase)",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "READY",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "ready condition with case insensitive type match (mixed case)",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "ReAdY",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "multiple conditions with ready condition true",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "multiple conditions with ready condition false",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "ready condition with unknown status (should be false)",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionUnknown,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "kafkaCR with empty conditions array",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "Initializing",
					Conditions:         []metav1.Condition{},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "kafkaCR with nil conditions array",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "Pending",
					Conditions:         nil,
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "kafka metadata state with empty string and no conditions",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					KafkaMetadataState: "",
					Conditions:         nil,
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
		{
			name: "kafka metadata state with special characters",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "status with existing connection should preserve it",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{
				Connection: translator.InfraConnection{
					URL: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "existing-secret",
						},
						Key: "url",
					},
				},
			},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "status with existing conditions should preserve them",
			kafkaCR: &strimziv1.Kafka{
				Status: strimziv1.KafkaStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "Ready",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			initialStatus: &translator.KafkaStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "Custom",
						Status: metav1.ConditionTrue,
					},
				},
			},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "multiple ready conditions (first one wins)",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: true,
			expectedState: "Ready",
		},
		{
			name: "long kafka metadata state value",
			kafkaCR: &strimziv1.Kafka{
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
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "NotReady",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			status := tt.initialStatus

			computeStatusSummary(ctx, tt.kafkaCR, status)

			assert.Equal(t, tt.expectedReady, status.Ready, "Ready status mismatch")
			assert.Equal(t, tt.expectedState, status.State, "State mismatch")

			if tt.initialStatus.Connection.URL.Name != "" {
				assert.Equal(t, tt.initialStatus.Connection.URL.Name, status.Connection.URL.Name, "Connection should be preserved")
			}

			if len(tt.initialStatus.Conditions) > 0 {
				assert.Equal(t, tt.initialStatus.Conditions, status.Conditions, "Conditions should be preserved")
			}
		})
	}
}
