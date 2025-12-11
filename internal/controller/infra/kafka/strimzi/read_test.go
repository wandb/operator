package strimzi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wandb/operator/internal/controller/translator"
	v1beta3 "github.com/wandb/operator/internal/vendored/strimzi-kafka/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputeStatusSummary(t *testing.T) {
	tests := []struct {
		name          string
		kafkaCR       *v1beta3.Kafka
		initialStatus *translator.KafkaStatus
		expectedReady bool
		expectedState string
	}{
		{
			name:          "kafkaCR is nil",
			kafkaCR:       nil,
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "Not Installed",
		},
		{
			name: "kafkaCR has KafkaMetadataState set with ready condition true",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "KRaft",
		},
		{
			name: "kafkaCR has KafkaMetadataState set with ready condition false",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "ZooKeeper",
		},
		{
			name: "ready condition exists and is true",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "Not Ready",
		},
		{
			name: "ready condition is missing",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
					Conditions: []metav1.Condition{},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "Not Ready",
		},
		{
			name: "state is empty string and ready is true",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "Not Ready",
		},
		{
			name: "state is non-empty and ready is true",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "Migrating",
		},
		{
			name: "state is non-empty and ready is false",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "Error",
		},
		{
			name: "different condition types (non-ready conditions) should not affect ready status",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "Not Ready",
		},
		{
			name: "ready condition with case insensitive type match (lowercase)",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "Not Ready",
		},
		{
			name: "ready condition with unknown status (should be false)",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "Not Ready",
		},
		{
			name: "kafkaCR with empty conditions array",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
					KafkaMetadataState: "Initializing",
					Conditions:         []metav1.Condition{},
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "Initializing",
		},
		{
			name: "kafkaCR with nil conditions array",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
					KafkaMetadataState: "Pending",
					Conditions:         nil,
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "Pending",
		},
		{
			name: "kafka metadata state with empty string and no conditions",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
					KafkaMetadataState: "",
					Conditions:         nil,
				},
			},
			initialStatus: &translator.KafkaStatus{},
			expectedReady: false,
			expectedState: "Not Ready",
		},
		{
			name: "kafka metadata state with special characters",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "State-With-Dashes",
		},
		{
			name: "status with existing connection should preserve it",
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			kafkaCR: &v1beta3.Kafka{
				Status: v1beta3.KafkaStatus{
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
			expectedState: "VeryLongStateNameThatRepresentsComplexInternalKafkaMetadataStateTransition",
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
