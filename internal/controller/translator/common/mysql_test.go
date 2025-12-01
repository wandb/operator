package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMySQLCondition_RoundTrip(t *testing.T) {
	fixtures := []struct {
		name     string
		connInfo MySQLConnInfo
	}{
		{
			name: "basic connection",
			connInfo: MySQLConnInfo{
				Host: "mysql-service",
				Port: "3306",
				User: "root",
			},
		},
		{
			name: "custom user and port",
			connInfo: MySQLConnInfo{
				Host: "pxc-cluster-haproxy",
				Port: "3307",
				User: "admin",
			},
		},
		{
			name: "empty user",
			connInfo: MySQLConnInfo{
				Host: "localhost",
				Port: "3306",
				User: "",
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewMySQLConnCondition(tc.connInfo)

			assert.Equal(t, MySQLConnectionCode, cond.code)
			assert.Equal(t, "MySQL connection info", cond.Message())

			connCond, ok := cond.ToMySQLConnCondition()
			require.True(t, ok)

			assert.Equal(t, tc.connInfo.Host, connCond.connInfo.Host)
			assert.Equal(t, tc.connInfo.Port, connCond.connInfo.Port)
			assert.Equal(t, tc.connInfo.User, connCond.connInfo.User)
		})
	}
}

func TestMySQLCondition_ToMySQLConnCondition(t *testing.T) {
	fixtures := []struct {
		name        string
		condition   MySQLCondition
		expectOk    bool
		expectValid bool
	}{
		{
			name: "valid connection condition",
			condition: NewMySQLConnCondition(MySQLConnInfo{
				Host: "mysql-svc",
				Port: "3306",
				User: "root",
			}),
			expectOk:    true,
			expectValid: true,
		},
		{
			name:        "non-connection condition",
			condition:   NewMySQLCondition(MySQLCreatedCode, "created"),
			expectOk:    false,
			expectValid: false,
		},
		{
			name: "connection condition with invalid hidden data",
			condition: MySQLCondition{
				code:    MySQLConnectionCode,
				message: "test",
				hidden:  map[string]string{"invalid": "data"},
			},
			expectOk:    true,
			expectValid: false,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			connCond, ok := tc.condition.ToMySQLConnCondition()
			assert.Equal(t, tc.expectOk, ok)

			if tc.expectValid {
				assert.NotEmpty(t, connCond.connInfo.Host)
			}
		})
	}
}

func TestExtractMySQLStatus(t *testing.T) {
	ctx := context.Background()

	fixtures := []struct {
		name       string
		conditions []MySQLCondition
		expected   MySQLStatus
	}{
		{
			name: "single connection condition",
			conditions: []MySQLCondition{
				NewMySQLConnCondition(MySQLConnInfo{
					Host: "pxc-cluster-haproxy",
					Port: "3306",
					User: "root",
				}),
			},
			expected: MySQLStatus{
				Ready: true,
				Connection: MySQLConnection{
					Host: "pxc-cluster-haproxy",
					Port: "3306",
					User: "root",
				},
				Conditions: nil,
			},
		},
		{
			name: "connection with other conditions preserved",
			conditions: []MySQLCondition{
				NewMySQLCondition(MySQLCreatedCode, "created"),
				NewMySQLConnCondition(MySQLConnInfo{
					Host: "mysql-cluster",
					Port: "3307",
					User: "admin",
				}),
				NewMySQLCondition(MySQLUpdatedCode, "updated"),
			},
			expected: MySQLStatus{
				Ready: true,
				Connection: MySQLConnection{
					Host: "mysql-cluster",
					Port: "3307",
					User: "admin",
				},
				Conditions: []MySQLCondition{
					NewMySQLCondition(MySQLCreatedCode, "created"),
					NewMySQLCondition(MySQLUpdatedCode, "updated"),
				},
			},
		},
		{
			name:       "empty conditions",
			conditions: []MySQLCondition{},
			expected: MySQLStatus{
				Ready:      false,
				Connection: MySQLConnection{},
				Conditions: nil,
			},
		},
		{
			name: "no connection condition",
			conditions: []MySQLCondition{
				NewMySQLCondition(MySQLCreatedCode, "created"),
				NewMySQLCondition(MySQLDeletedCode, "deleted"),
			},
			expected: MySQLStatus{
				Ready:      false,
				Connection: MySQLConnection{},
				Conditions: []MySQLCondition{
					NewMySQLCondition(MySQLCreatedCode, "created"),
					NewMySQLCondition(MySQLDeletedCode, "deleted"),
				},
			},
		},
		{
			name: "multiple connection conditions (last wins)",
			conditions: []MySQLCondition{
				NewMySQLConnCondition(MySQLConnInfo{
					Host: "first-mysql",
					Port: "3306",
					User: "user1",
				}),
				NewMySQLConnCondition(MySQLConnInfo{
					Host: "second-mysql",
					Port: "3307",
					User: "user2",
				}),
			},
			expected: MySQLStatus{
				Ready: true,
				Connection: MySQLConnection{
					Host: "second-mysql",
					Port: "3307",
					User: "user2",
				},
				Conditions: nil,
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractMySQLStatus(ctx, tc.conditions)

			assert.Equal(t, tc.expected.Ready, result.Ready)
			assert.Equal(t, tc.expected.Connection.Host, result.Connection.Host)
			assert.Equal(t, tc.expected.Connection.Port, result.Connection.Port)
			assert.Equal(t, tc.expected.Connection.User, result.Connection.User)

			assert.Equal(t, len(tc.expected.Conditions), len(result.Conditions))
			for i, expectedCond := range tc.expected.Conditions {
				assert.Equal(t, expectedCond.Code(), result.Conditions[i].Code())
				assert.Equal(t, expectedCond.Message(), result.Conditions[i].Message())
			}
		})
	}
}

func TestNewMySQLCondition(t *testing.T) {
	fixtures := []struct {
		name    string
		code    MySQLInfraCode
		message string
	}{
		{
			name:    "created condition",
			code:    MySQLCreatedCode,
			message: "MySQL cluster created",
		},
		{
			name:    "updated condition",
			code:    MySQLUpdatedCode,
			message: "MySQL cluster updated",
		},
		{
			name:    "deleted condition",
			code:    MySQLDeletedCode,
			message: "MySQL cluster deleted",
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewMySQLCondition(tc.code, tc.message)

			assert.Equal(t, string(tc.code), cond.Code())
			assert.Equal(t, tc.message, cond.Message())
		})
	}
}
