package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMinioCondition_RoundTrip(t *testing.T) {
	fixtures := []struct {
		name     string
		connInfo MinioConnInfo
	}{
		{
			name: "basic connection",
			connInfo: MinioConnInfo{
				Host:      "minio-service",
				Port:      "9000",
				AccessKey: "minioadmin",
			},
		},
		{
			name: "custom access key",
			connInfo: MinioConnInfo{
				Host:      "minio.example.com",
				Port:      "443",
				AccessKey: "custom-key",
			},
		},
		{
			name: "empty access key",
			connInfo: MinioConnInfo{
				Host:      "localhost",
				Port:      "9000",
				AccessKey: "",
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewMinioConnCondition(tc.connInfo)

			assert.Equal(t, MinioConnectionCode, cond.code)
			assert.Equal(t, "Minio connection info", cond.Message())

			connCond, ok := cond.ToMinioConnCondition()
			require.True(t, ok)

			assert.Equal(t, tc.connInfo.Host, connCond.connInfo.Host)
			assert.Equal(t, tc.connInfo.Port, connCond.connInfo.Port)
			assert.Equal(t, tc.connInfo.AccessKey, connCond.connInfo.AccessKey)
		})
	}
}

func TestMinioCondition_ToMinioConnCondition(t *testing.T) {
	fixtures := []struct {
		name        string
		condition   MinioCondition
		expectOk    bool
		expectValid bool
	}{
		{
			name: "valid connection condition",
			condition: NewMinioConnCondition(MinioConnInfo{
				Host:      "minio-svc",
				Port:      "9000",
				AccessKey: "admin",
			}),
			expectOk:    true,
			expectValid: true,
		},
		{
			name:        "non-connection condition",
			condition:   NewMinioCondition(MinioCreatedCode, "created"),
			expectOk:    false,
			expectValid: false,
		},
		{
			name: "connection condition with invalid hidden data",
			condition: MinioCondition{
				code:    MinioConnectionCode,
				message: "test",
				hidden:  []string{"invalid"},
			},
			expectOk:    true,
			expectValid: false,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			connCond, ok := tc.condition.ToMinioConnCondition()
			assert.Equal(t, tc.expectOk, ok)

			if tc.expectValid {
				assert.NotEmpty(t, connCond.connInfo.Host)
			}
		})
	}
}

func TestExtractMinioStatus(t *testing.T) {
	ctx := context.Background()

	fixtures := []struct {
		name       string
		conditions []MinioCondition
		expected   MinioStatus
	}{
		{
			name: "single connection condition",
			conditions: []MinioCondition{
				NewMinioConnCondition(MinioConnInfo{
					Host:      "minio-svc",
					Port:      "9000",
					AccessKey: "minioadmin",
				}),
			},
			expected: MinioStatus{
				Ready: true,
				Connection: MinioConnection{
					Host:      "minio-svc",
					Port:      "9000",
					AccessKey: "minioadmin",
				},
				Conditions: nil,
			},
		},
		{
			name: "connection with other conditions preserved",
			conditions: []MinioCondition{
				NewMinioCondition(MinioCreatedCode, "created"),
				NewMinioConnCondition(MinioConnInfo{
					Host:      "minio-tenant",
					Port:      "443",
					AccessKey: "admin-key",
				}),
				NewMinioCondition(MinioUpdatedCode, "updated"),
			},
			expected: MinioStatus{
				Ready: true,
				Connection: MinioConnection{
					Host:      "minio-tenant",
					Port:      "443",
					AccessKey: "admin-key",
				},
				Conditions: []MinioCondition{
					NewMinioCondition(MinioCreatedCode, "created"),
					NewMinioCondition(MinioUpdatedCode, "updated"),
				},
			},
		},
		{
			name:       "empty conditions",
			conditions: []MinioCondition{},
			expected: MinioStatus{
				Ready:      false,
				Connection: MinioConnection{},
				Conditions: nil,
			},
		},
		{
			name: "no connection condition",
			conditions: []MinioCondition{
				NewMinioCondition(MinioCreatedCode, "created"),
				NewMinioCondition(MinioDeletedCode, "deleted"),
			},
			expected: MinioStatus{
				Ready:      false,
				Connection: MinioConnection{},
				Conditions: []MinioCondition{
					NewMinioCondition(MinioCreatedCode, "created"),
					NewMinioCondition(MinioDeletedCode, "deleted"),
				},
			},
		},
		{
			name: "multiple connection conditions (last wins)",
			conditions: []MinioCondition{
				NewMinioConnCondition(MinioConnInfo{
					Host:      "first-minio",
					Port:      "9000",
					AccessKey: "key1",
				}),
				NewMinioConnCondition(MinioConnInfo{
					Host:      "second-minio",
					Port:      "9001",
					AccessKey: "key2",
				}),
			},
			expected: MinioStatus{
				Ready: true,
				Connection: MinioConnection{
					Host:      "second-minio",
					Port:      "9001",
					AccessKey: "key2",
				},
				Conditions: nil,
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractMinioStatus(ctx, tc.conditions)

			assert.Equal(t, tc.expected.Ready, result.Ready)
			assert.Equal(t, tc.expected.Connection.Host, result.Connection.Host)
			assert.Equal(t, tc.expected.Connection.Port, result.Connection.Port)
			assert.Equal(t, tc.expected.Connection.AccessKey, result.Connection.AccessKey)

			assert.Equal(t, len(tc.expected.Conditions), len(result.Conditions))
			for i, expectedCond := range tc.expected.Conditions {
				assert.Equal(t, expectedCond.Code(), result.Conditions[i].Code())
				assert.Equal(t, expectedCond.Message(), result.Conditions[i].Message())
			}
		})
	}
}

func TestNewMinioCondition(t *testing.T) {
	fixtures := []struct {
		name    string
		code    MinioInfraCode
		message string
	}{
		{
			name:    "created condition",
			code:    MinioCreatedCode,
			message: "Minio tenant created",
		},
		{
			name:    "updated condition",
			code:    MinioUpdatedCode,
			message: "Minio tenant updated",
		},
		{
			name:    "deleted condition",
			code:    MinioDeletedCode,
			message: "Minio tenant deleted",
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewMinioCondition(tc.code, tc.message)

			assert.Equal(t, string(tc.code), cond.Code())
			assert.Equal(t, tc.message, cond.Message())
		})
	}
}
