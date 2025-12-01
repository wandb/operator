package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKafkaCondition_RoundTrip(t *testing.T) {
	fixtures := []struct {
		name     string
		connInfo KafkaConnInfo
	}{
		{
			name: "basic connection",
			connInfo: KafkaConnInfo{
				Host: "kafka-bootstrap",
				Port: "9092",
			},
		},
		{
			name: "custom port",
			connInfo: KafkaConnInfo{
				Host: "kafka.example.com",
				Port: "9093",
			},
		},
		{
			name: "localhost connection",
			connInfo: KafkaConnInfo{
				Host: "localhost",
				Port: "29092",
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewKafkaConnCondition(tc.connInfo)

			assert.Equal(t, KafkaConnectionCode, cond.code)
			expectedMsg := "kafka://" + tc.connInfo.Host + ":" + tc.connInfo.Port
			assert.Equal(t, expectedMsg, cond.Message())

			connCond, ok := cond.ToKafkaConnCondition()
			require.True(t, ok)

			assert.Equal(t, tc.connInfo.Host, connCond.connInfo.Host)
			assert.Equal(t, tc.connInfo.Port, connCond.connInfo.Port)
		})
	}
}

func TestKafkaCondition_ToKafkaConnCondition(t *testing.T) {
	fixtures := []struct {
		name        string
		condition   KafkaCondition
		expectOk    bool
		expectValid bool
	}{
		{
			name: "valid connection condition",
			condition: NewKafkaConnCondition(KafkaConnInfo{
				Host: "kafka-svc",
				Port: "9092",
			}),
			expectOk:    true,
			expectValid: true,
		},
		{
			name:        "non-connection condition",
			condition:   NewKafkaCondition(KafkaCreatedCode, "created"),
			expectOk:    false,
			expectValid: false,
		},
		{
			name: "connection condition with invalid hidden data",
			condition: KafkaCondition{
				code:    KafkaConnectionCode,
				message: "test",
				hidden:  12345,
			},
			expectOk:    true,
			expectValid: false,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			connCond, ok := tc.condition.ToKafkaConnCondition()
			assert.Equal(t, tc.expectOk, ok)

			if tc.expectValid {
				assert.NotEmpty(t, connCond.connInfo.Host)
			}
		})
	}
}

func TestExtractKafkaStatus(t *testing.T) {
	ctx := context.Background()

	fixtures := []struct {
		name       string
		conditions []KafkaCondition
		expected   KafkaStatus
	}{
		{
			name: "single connection condition",
			conditions: []KafkaCondition{
				NewKafkaConnCondition(KafkaConnInfo{
					Host: "kafka-bootstrap",
					Port: "9092",
				}),
			},
			expected: KafkaStatus{
				Ready: true,
				Connection: KafkaConnection{
					Host: "kafka-bootstrap",
					Port: "9092",
				},
				Conditions: nil,
			},
		},
		{
			name: "connection with other conditions preserved",
			conditions: []KafkaCondition{
				NewKafkaCondition(KafkaCreatedCode, "created"),
				NewKafkaConnCondition(KafkaConnInfo{
					Host: "kafka-svc",
					Port: "9093",
				}),
				NewKafkaCondition(KafkaNodePoolCreatedCode, "node pool created"),
			},
			expected: KafkaStatus{
				Ready: true,
				Connection: KafkaConnection{
					Host: "kafka-svc",
					Port: "9093",
				},
				Conditions: []KafkaCondition{
					NewKafkaCondition(KafkaCreatedCode, "created"),
					NewKafkaCondition(KafkaNodePoolCreatedCode, "node pool created"),
				},
			},
		},
		{
			name:       "empty conditions",
			conditions: []KafkaCondition{},
			expected: KafkaStatus{
				Ready:      false,
				Connection: KafkaConnection{},
				Conditions: nil,
			},
		},
		{
			name: "no connection condition",
			conditions: []KafkaCondition{
				NewKafkaCondition(KafkaCreatedCode, "created"),
				NewKafkaCondition(KafkaUpdatedCode, "updated"),
			},
			expected: KafkaStatus{
				Ready:      false,
				Connection: KafkaConnection{},
				Conditions: []KafkaCondition{
					NewKafkaCondition(KafkaCreatedCode, "created"),
					NewKafkaCondition(KafkaUpdatedCode, "updated"),
				},
			},
		},
		{
			name: "multiple connection conditions (last wins)",
			conditions: []KafkaCondition{
				NewKafkaConnCondition(KafkaConnInfo{
					Host: "first-kafka",
					Port: "9092",
				}),
				NewKafkaConnCondition(KafkaConnInfo{
					Host: "second-kafka",
					Port: "9093",
				}),
			},
			expected: KafkaStatus{
				Ready: true,
				Connection: KafkaConnection{
					Host: "second-kafka",
					Port: "9093",
				},
				Conditions: nil,
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractKafkaStatus(ctx, tc.conditions)

			assert.Equal(t, tc.expected.Ready, result.Ready)
			assert.Equal(t, tc.expected.Connection.Host, result.Connection.Host)
			assert.Equal(t, tc.expected.Connection.Port, result.Connection.Port)

			assert.Equal(t, len(tc.expected.Conditions), len(result.Conditions))
			for i, expectedCond := range tc.expected.Conditions {
				assert.Equal(t, expectedCond.Code(), result.Conditions[i].Code())
				assert.Equal(t, expectedCond.Message(), result.Conditions[i].Message())
			}
		})
	}
}

func TestNewKafkaCondition(t *testing.T) {
	fixtures := []struct {
		name    string
		code    KafkaInfraCode
		message string
	}{
		{
			name:    "created condition",
			code:    KafkaCreatedCode,
			message: "Kafka cluster created",
		},
		{
			name:    "node pool updated",
			code:    KafkaNodePoolUpdatedCode,
			message: "Node pool updated",
		},
		{
			name:    "deleted condition",
			code:    KafkaDeletedCode,
			message: "Kafka cluster deleted",
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewKafkaCondition(tc.code, tc.message)

			assert.Equal(t, string(tc.code), cond.Code())
			assert.Equal(t, tc.message, cond.Message())
		})
	}
}
