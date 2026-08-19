package service

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrencyGateAcquireRelease(t *testing.T) {
	rdb := newTestRedis(t)
	old := common.RDB
	common.RDB = rdb
	defer func() { common.RDB = old }()

	ok, err := AcquireConcurrencySlot(context.Background(), "token", "cg1", 2)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = AcquireConcurrencySlot(context.Background(), "token", "cg1", 2)
	require.NoError(t, err)
	assert.True(t, ok)
	// 第 3 个超限
	ok, err = AcquireConcurrencySlot(context.Background(), "token", "cg1", 2)
	require.NoError(t, err)
	assert.False(t, ok)
	// 释放后恢复
	require.NoError(t, ReleaseConcurrencySlot(context.Background(), "token", "cg1"))
	ok, err = AcquireConcurrencySlot(context.Background(), "token", "cg1", 2)
	require.NoError(t, err)
	assert.True(t, ok)
	// 0 = 不限
	ok, err = AcquireConcurrencySlot(context.Background(), "token", "cg2", 0)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestConcurrencyGateMemoryFallback(t *testing.T) {
	old := common.RDB
	common.RDB = nil
	defer func() { common.RDB = old }()

	var wg sync.WaitGroup
	results := make([]bool, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := AcquireConcurrencySlot(context.Background(), "token", "cgm", 2)
			if err == nil {
				results[i] = ok
			}
		}(i)
	}
	wg.Wait()
	trueCount := 0
	for _, r := range results {
		if r {
			trueCount++
		}
	}
	assert.Equal(t, 2, trueCount) // 内存计数并发安全，恰好 2 个成功
}

// TestAcquireRelayConcurrencyRollbackAndRelease protects the multi-scope
// borrow contract: a request that overruns a later scope must roll back the
// slots it already borrowed, and the returned release function must release
// every borrowed slot exactly once.
func TestAcquireRelayConcurrencyRollbackAndRelease(t *testing.T) {
	rdb := newTestRedis(t)
	old := common.RDB
	common.RDB = rdb
	defer func() { common.RDB = old }()

	// 企业并发上限 2、Key 并发上限 1：第 2 个请求在企业上成功、Key 上超限，
	// 必须回滚已经借到的企业槽位。
	company := &model.Company{Name: "cg-rollback-9001", OwnerUserId: 9001, MaxConcurrentRequests: 2}
	require.NoError(t, model.DB.Create(company).Error)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyUserId, 9001)
	common.SetContextKey(c, constant.ContextKeyToken, &model.Token{Id: 7001, MaxConcurrentRequests: 1})

	release1, err := AcquireRelayConcurrency(c)
	require.NoError(t, err)
	require.NotNil(t, release1)

	// 第 2 个请求：企业 OK（2/2），Key 超限（2>1）→ 失败并回滚企业槽位
	release2, err := AcquireRelayConcurrency(c)
	require.Error(t, err)
	require.Nil(t, release2)

	// 回滚契约主断言：释放第 1 个请求后，第 3 个请求必须完整成功
	// （回滚则企业计数 0、Key 0；未回滚则企业计数 1——仍 ≤ 上限 2，因此
	// 此断言本身无法区分回滚有无，决定性区分依赖下方 limit=1 探测）。
	release1()
	release3, err := AcquireRelayConcurrency(c)
	require.NoError(t, err)
	require.NotNil(t, release3)
	release3()

	// 决定性探测：以 limit=1 借入企业作用域。回滚则计数为 0、INCR→1 成功；
	// r2 借到的企业槽位若泄漏（未回滚）则计数为 1、INCR→2 超限失败——
	// 回滚契约被删除时该断言必红。
	ok, err := AcquireConcurrencySlot(context.Background(), "company", "9001", 1)
	require.NoError(t, err)
	require.True(t, ok) // 回滚契约：r2 借到的企业槽位已被释放
	require.NoError(t, ReleaseConcurrencySlot(context.Background(), "company", "9001"))

	// 全部释放后再次完整借入成功
	release4, err := AcquireRelayConcurrency(c)
	require.NoError(t, err)
	require.NotNil(t, release4)
	release4()
}
