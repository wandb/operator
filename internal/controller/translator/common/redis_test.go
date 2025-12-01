package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisSentinelCondition_RoundTrip(t *testing.T) {
	fixtures := []struct {
		name     string
		connInfo RedisSentinelConnInfo
	}{
		{
			name: "basic sentinel connection",
			connInfo: RedisSentinelConnInfo{
				SentinelHost: "redis-sentinel",
				SentinelPort: "26379",
				MasterName:   "mymaster",
			},
		},
		{
			name: "custom master name",
			connInfo: RedisSentinelConnInfo{
				SentinelHost: "sentinel.example.com",
				SentinelPort: "26380",
				MasterName:   "primary",
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewRedisSentinelConnCondition(tc.connInfo)

			assert.Equal(t, RedisSentinelConnectionCode, cond.code)
			expectedMsg := "redis://" + tc.connInfo.SentinelHost + ":" + tc.connInfo.SentinelPort + "?master=" + tc.connInfo.MasterName
			assert.Equal(t, expectedMsg, cond.Message())

			connCond, ok := cond.ToRedisSentinelConnCondition()
			require.True(t, ok)

			assert.Equal(t, tc.connInfo.SentinelHost, connCond.connInfo.SentinelHost)
			assert.Equal(t, tc.connInfo.SentinelPort, connCond.connInfo.SentinelPort)
			assert.Equal(t, tc.connInfo.MasterName, connCond.connInfo.MasterName)
		})
	}
}

func TestRedisStandaloneCondition_RoundTrip(t *testing.T) {
	fixtures := []struct {
		name     string
		connInfo RedisStandaloneConnInfo
	}{
		{
			name: "basic standalone connection",
			connInfo: RedisStandaloneConnInfo{
				Host: "redis-standalone",
				Port: "6379",
			},
		},
		{
			name: "custom port",
			connInfo: RedisStandaloneConnInfo{
				Host: "redis.example.com",
				Port: "6380",
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewRedisStandaloneConnCondition(tc.connInfo)

			assert.Equal(t, RedisStandaloneConnectionCode, cond.code)
			expectedMsg := "redis://" + tc.connInfo.Host + ":" + tc.connInfo.Port
			assert.Equal(t, expectedMsg, cond.Message())

			connCond, ok := cond.ToRedisStandaloneConnCondition()
			require.True(t, ok)

			assert.Equal(t, tc.connInfo.Host, connCond.connInfo.Host)
			assert.Equal(t, tc.connInfo.Port, connCond.connInfo.Port)
		})
	}
}

func TestRedisCondition_ToRedisSentinelConnCondition(t *testing.T) {
	fixtures := []struct {
		name        string
		condition   RedisCondition
		expectOk    bool
		expectValid bool
	}{
		{
			name: "valid sentinel connection condition",
			condition: NewRedisSentinelConnCondition(RedisSentinelConnInfo{
				SentinelHost: "sentinel-svc",
				SentinelPort: "26379",
				MasterName:   "mymaster",
			}),
			expectOk:    true,
			expectValid: true,
		},
		{
			name: "standalone connection (not sentinel)",
			condition: NewRedisStandaloneConnCondition(RedisStandaloneConnInfo{
				Host: "redis-standalone",
				Port: "6379",
			}),
			expectOk:    false,
			expectValid: false,
		},
		{
			name:        "non-connection condition",
			condition:   NewRedisCondition(RedisSentinelCreatedCode, "created"),
			expectOk:    false,
			expectValid: false,
		},
		{
			name: "sentinel connection with invalid hidden data",
			condition: RedisCondition{
				code:    RedisSentinelConnectionCode,
				message: "test",
				hidden:  "invalid-type",
			},
			expectOk:    true,
			expectValid: false,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			connCond, ok := tc.condition.ToRedisSentinelConnCondition()
			assert.Equal(t, tc.expectOk, ok)

			if tc.expectValid {
				assert.NotEmpty(t, connCond.connInfo.SentinelHost)
			}
		})
	}
}

func TestRedisCondition_ToRedisStandaloneConnCondition(t *testing.T) {
	fixtures := []struct {
		name        string
		condition   RedisCondition
		expectOk    bool
		expectValid bool
	}{
		{
			name: "valid standalone connection condition",
			condition: NewRedisStandaloneConnCondition(RedisStandaloneConnInfo{
				Host: "redis-standalone",
				Port: "6379",
			}),
			expectOk:    true,
			expectValid: true,
		},
		{
			name: "sentinel connection (not standalone)",
			condition: NewRedisSentinelConnCondition(RedisSentinelConnInfo{
				SentinelHost: "sentinel-svc",
				SentinelPort: "26379",
				MasterName:   "mymaster",
			}),
			expectOk:    false,
			expectValid: false,
		},
		{
			name:        "non-connection condition",
			condition:   NewRedisCondition(RedisStandaloneCreatedCode, "created"),
			expectOk:    false,
			expectValid: false,
		},
		{
			name: "standalone connection with invalid hidden data",
			condition: RedisCondition{
				code:    RedisStandaloneConnectionCode,
				message: "test",
				hidden:  123,
			},
			expectOk:    true,
			expectValid: false,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			connCond, ok := tc.condition.ToRedisStandaloneConnCondition()
			assert.Equal(t, tc.expectOk, ok)

			if tc.expectValid {
				assert.NotEmpty(t, connCond.connInfo.Host)
			}
		})
	}
}

func TestExtractRedisStatus(t *testing.T) {
	ctx := context.Background()

	fixtures := []struct {
		name       string
		conditions []RedisCondition
		expected   RedisStatus
	}{
		{
			name: "standalone connection only",
			conditions: []RedisCondition{
				NewRedisStandaloneConnCondition(RedisStandaloneConnInfo{
					Host: "redis-standalone",
					Port: "6379",
				}),
			},
			expected: RedisStatus{
				Ready: true,
				Connection: RedisConnection{
					RedisHost: "redis-standalone",
					RedisPort: "6379",
				},
				Conditions: nil,
			},
		},
		{
			name: "sentinel connection only",
			conditions: []RedisCondition{
				NewRedisSentinelConnCondition(RedisSentinelConnInfo{
					SentinelHost: "sentinel-svc",
					SentinelPort: "26379",
					MasterName:   "mymaster",
				}),
			},
			expected: RedisStatus{
				Ready: true,
				Connection: RedisConnection{
					SentinelHost:   "sentinel-svc",
					SentinelPort:   "26379",
					SentinelMaster: "mymaster",
				},
				Conditions: nil,
			},
		},
		{
			name: "both sentinel and standalone (both populated)",
			conditions: []RedisCondition{
				NewRedisStandaloneConnCondition(RedisStandaloneConnInfo{
					Host: "redis-standalone",
					Port: "6379",
				}),
				NewRedisSentinelConnCondition(RedisSentinelConnInfo{
					SentinelHost: "sentinel-svc",
					SentinelPort: "26379",
					MasterName:   "mymaster",
				}),
			},
			expected: RedisStatus{
				Ready: true,
				Connection: RedisConnection{
					RedisHost:      "redis-standalone",
					RedisPort:      "6379",
					SentinelHost:   "sentinel-svc",
					SentinelPort:   "26379",
					SentinelMaster: "mymaster",
				},
				Conditions: nil,
			},
		},
		{
			name: "connection with other conditions preserved",
			conditions: []RedisCondition{
				NewRedisCondition(RedisSentinelCreatedCode, "sentinel created"),
				NewRedisSentinelConnCondition(RedisSentinelConnInfo{
					SentinelHost: "sentinel-svc",
					SentinelPort: "26379",
					MasterName:   "mymaster",
				}),
				NewRedisCondition(RedisReplicationCreatedCode, "replication created"),
			},
			expected: RedisStatus{
				Ready: true,
				Connection: RedisConnection{
					SentinelHost:   "sentinel-svc",
					SentinelPort:   "26379",
					SentinelMaster: "mymaster",
				},
				Conditions: []RedisCondition{
					NewRedisCondition(RedisSentinelCreatedCode, "sentinel created"),
					NewRedisCondition(RedisReplicationCreatedCode, "replication created"),
				},
			},
		},
		{
			name:       "empty conditions",
			conditions: []RedisCondition{},
			expected: RedisStatus{
				Ready:      false,
				Connection: RedisConnection{},
				Conditions: nil,
			},
		},
		{
			name: "no connection condition",
			conditions: []RedisCondition{
				NewRedisCondition(RedisStandaloneCreatedCode, "created"),
				NewRedisCondition(RedisStandaloneDeletedCode, "deleted"),
			},
			expected: RedisStatus{
				Ready:      false,
				Connection: RedisConnection{},
				Conditions: []RedisCondition{
					NewRedisCondition(RedisStandaloneCreatedCode, "created"),
					NewRedisCondition(RedisStandaloneDeletedCode, "deleted"),
				},
			},
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractRedisStatus(ctx, tc.conditions)

			assert.Equal(t, tc.expected.Ready, result.Ready)
			assert.Equal(t, tc.expected.Connection.RedisHost, result.Connection.RedisHost)
			assert.Equal(t, tc.expected.Connection.RedisPort, result.Connection.RedisPort)
			assert.Equal(t, tc.expected.Connection.SentinelHost, result.Connection.SentinelHost)
			assert.Equal(t, tc.expected.Connection.SentinelPort, result.Connection.SentinelPort)
			assert.Equal(t, tc.expected.Connection.SentinelMaster, result.Connection.SentinelMaster)

			assert.Equal(t, len(tc.expected.Conditions), len(result.Conditions))
			for i, expectedCond := range tc.expected.Conditions {
				assert.Equal(t, expectedCond.Code(), result.Conditions[i].Code())
				assert.Equal(t, expectedCond.Message(), result.Conditions[i].Message())
			}
		})
	}
}

func TestNewRedisCondition(t *testing.T) {
	fixtures := []struct {
		name    string
		code    RedisInfraCode
		message string
	}{
		{
			name:    "sentinel created",
			code:    RedisSentinelCreatedCode,
			message: "Redis Sentinel created",
		},
		{
			name:    "replication created",
			code:    RedisReplicationCreatedCode,
			message: "Redis Replication created",
		},
		{
			name:    "standalone created",
			code:    RedisStandaloneCreatedCode,
			message: "Redis Standalone created",
		},
		{
			name:    "sentinel deleted",
			code:    RedisSentinelDeletedCode,
			message: "Redis Sentinel deleted",
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			cond := NewRedisCondition(tc.code, tc.message)

			assert.Equal(t, string(tc.code), cond.Code())
			assert.Equal(t, tc.message, cond.Message())
		})
	}
}
