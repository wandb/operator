package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClickHouseCondition_RoundTrip(t *testing.T) {
	fixtures := []struct {
		name     string
		connInfo ClickHouseConnInfo
	}{
		{
			name: "basic connection",
			connInfo: ClickHouseConnInfo{
				Host: "clickhouse-service",
				Port: "9000",
				User: "default",
			},
		},
		{
			name: "custom user",
			connInfo: ClickHouseConnInfo{
				Host: "ch-cluster",
				Port: "8123",
				User: "admin",
			},
		},
		{
			name: "empty user",
			connInfo: ClickHouseConnInfo{
				Host: "localhost",
				Port: "9000",
				User: "",
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewClickHouseConnCondition(tc.connInfo)

			assert.Equal(t, ClickHouseConnectionCode, cond.code)
			assert.Equal(t, "ClickHouse connection info", cond.Message())

			connCond, ok := cond.ToClickHouseConnCondition()
			require.True(t, ok)

			assert.Equal(t, tc.connInfo.Host, connCond.connInfo.Host)
			assert.Equal(t, tc.connInfo.Port, connCond.connInfo.Port)
			assert.Equal(t, tc.connInfo.User, connCond.connInfo.User)
		})
	}
}

func TestClickHouseCondition_ToClickHouseConnCondition(t *testing.T) {
	fixtures := []struct {
		name        string
		condition   ClickHouseCondition
		expectOk    bool
		expectValid bool
	}{
		{
			name: "valid connection condition",
			condition: NewClickHouseConnCondition(ClickHouseConnInfo{
				Host: "test-host",
				Port: "9000",
				User: "test-user",
			}),
			expectOk:    true,
			expectValid: true,
		},
		{
			name:        "non-connection condition",
			condition:   NewClickHouseCondition(ClickHouseCreatedCode, "created"),
			expectOk:    false,
			expectValid: false,
		},
		{
			name: "connection condition with invalid hidden data",
			condition: ClickHouseCondition{
				code:    ClickHouseConnectionCode,
				message: "test",
				hidden:  "invalid-type",
			},
			expectOk:    true,
			expectValid: false,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			connCond, ok := tc.condition.ToClickHouseConnCondition()
			assert.Equal(t, tc.expectOk, ok)

			if tc.expectValid {
				assert.NotEmpty(t, connCond.connInfo.Host)
			}
		})
	}
}

func TestExtractClickHouseStatus(t *testing.T) {
	ctx := context.Background()

	fixtures := []struct {
		name       string
		conditions []ClickHouseCondition
		expected   ClickHouseStatus
	}{
		{
			name: "single connection condition",
			conditions: []ClickHouseCondition{
				NewClickHouseConnCondition(ClickHouseConnInfo{
					Host: "clickhouse-svc",
					Port: "9000",
					User: "default",
				}),
			},
			expected: ClickHouseStatus{
				Ready: true,
				Connection: ClickHouseConnection{
					Host: "clickhouse-svc",
					Port: "9000",
					User: "default",
				},
				Conditions: nil,
			},
		},
		{
			name: "connection with other conditions",
			conditions: []ClickHouseCondition{
				NewClickHouseCondition(ClickHouseCreatedCode, "created"),
				NewClickHouseConnCondition(ClickHouseConnInfo{
					Host: "ch-host",
					Port: "8123",
					User: "admin",
				}),
				NewClickHouseCondition(ClickHouseUpdatedCode, "updated"),
			},
			expected: ClickHouseStatus{
				Ready: true,
				Connection: ClickHouseConnection{
					Host: "ch-host",
					Port: "8123",
					User: "admin",
				},
				Conditions: nil,
			},
		},
		{
			name:       "empty conditions",
			conditions: []ClickHouseCondition{},
			expected: ClickHouseStatus{
				Ready:      false,
				Connection: ClickHouseConnection{},
				Conditions: nil,
			},
		},
		{
			name: "no connection condition",
			conditions: []ClickHouseCondition{
				NewClickHouseCondition(ClickHouseCreatedCode, "created"),
				NewClickHouseCondition(ClickHouseDeletedCode, "deleted"),
			},
			expected: ClickHouseStatus{
				Ready:      false,
				Connection: ClickHouseConnection{},
				Conditions: nil,
			},
		},
		{
			name: "multiple connection conditions (last wins)",
			conditions: []ClickHouseCondition{
				NewClickHouseConnCondition(ClickHouseConnInfo{
					Host: "first-host",
					Port: "9000",
					User: "user1",
				}),
				NewClickHouseConnCondition(ClickHouseConnInfo{
					Host: "second-host",
					Port: "8123",
					User: "user2",
				}),
			},
			expected: ClickHouseStatus{
				Ready: true,
				Connection: ClickHouseConnection{
					Host: "second-host",
					Port: "8123",
					User: "user2",
				},
				Conditions: nil,
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractClickHouseStatus(ctx, tc.conditions)

			assert.Equal(t, tc.expected.Ready, result.Ready)
			assert.Equal(t, tc.expected.Connection.Host, result.Connection.Host)
			assert.Equal(t, tc.expected.Connection.Port, result.Connection.Port)
			assert.Equal(t, tc.expected.Connection.User, result.Connection.User)
		})
	}
}

func TestNewClickHouseCondition(t *testing.T) {
	fixtures := []struct {
		name    string
		code    ClickHouseInfraCode
		message string
	}{
		{
			name:    "created condition",
			code:    ClickHouseCreatedCode,
			message: "ClickHouse instance created",
		},
		{
			name:    "updated condition",
			code:    ClickHouseUpdatedCode,
			message: "ClickHouse instance updated",
		},
		{
			name:    "deleted condition",
			code:    ClickHouseDeletedCode,
			message: "ClickHouse instance deleted",
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewClickHouseCondition(tc.code, tc.message)

			assert.Equal(t, string(tc.code), cond.Code())
			assert.Equal(t, tc.message, cond.Message())
		})
	}
}
