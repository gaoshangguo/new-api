package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	s, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(s.Close)
	return redis.NewClient(&redis.Options{Addr: s.Addr()})
}

func TestCheckScopeRateLimitRPM(t *testing.T) {
	rdb := newTestRedis(t)
	old := common.RDB
	common.RDB = rdb
	defer func() { common.RDB = old }()

	cfg := ScopeRateLimit{RPM: 2, TPM: 0}
	// 前 2 次允许
	allowed, err := CheckScopeRateLimit(context.Background(), "test", "k1", cfg, 0)
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = CheckScopeRateLimit(context.Background(), "test", "k1", cfg, 0)
	require.NoError(t, err)
	assert.True(t, allowed)
	// 第 3 次拒绝
	allowed, err = CheckScopeRateLimit(context.Background(), "test", "k1", cfg, 0)
	require.NoError(t, err)
	assert.False(t, allowed)
	// 不同 id 不受影响
	allowed, err = CheckScopeRateLimit(context.Background(), "test", "k2", cfg, 0)
	require.NoError(t, err)
	assert.True(t, allowed)
	// 0 = 不限
	allowed, err = CheckScopeRateLimit(context.Background(), "test", "k3", ScopeRateLimit{}, 0)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestCheckScopeRateLimitTPMEstimated(t *testing.T) {
	rdb := newTestRedis(t)
	old := common.RDB
	common.RDB = rdb
	defer func() { common.RDB = old }()

	cfg := ScopeRateLimit{RPM: 0, TPM: 100}
	// 60 + 40 = 100 允许
	allowed, err := CheckScopeRateLimit(context.Background(), "test", "t1", cfg, 60)
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = CheckScopeRateLimit(context.Background(), "test", "t1", cfg, 40)
	require.NoError(t, err)
	assert.True(t, allowed)
	// 再加 1 超限
	allowed, err = CheckScopeRateLimit(context.Background(), "test", "t1", cfg, 1)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestCheckScopeRateLimitMemoryFallback(t *testing.T) {
	// RDB 为 nil 时走内存计数（单节点语义）
	old := common.RDB
	common.RDB = nil
	defer func() { common.RDB = old }()

	cfg := ScopeRateLimit{RPM: 1, TPM: 0}
	allowed, err := CheckScopeRateLimit(context.Background(), "test", "m1", cfg, 0)
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = CheckScopeRateLimit(context.Background(), "test", "m1", cfg, 0)
	require.NoError(t, err)
	assert.False(t, allowed)
}
