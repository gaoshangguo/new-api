package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var (
	errTokenDailyQuotaExceeded   = errors.New("token daily quota exceeded")
	errTokenMonthlyQuotaExceeded = errors.New("token monthly quota exceeded")
)

const (
	tokenDailyQuotaCacheTTL   = 25 * time.Hour
	tokenMonthlyQuotaCacheTTL = 35 * 24 * time.Hour
	// tokenBudgetMemoryCacheTTL bounds how long an aggregated used-quota value
	// stays cached in-process when Redis is unavailable (single-node semantics,
	// mirroring scope_rate_limit's memory mode). Without it every request would
	// re-aggregate the consume-log table.
	tokenBudgetMemoryCacheTTL = 60 * time.Second
)

// CheckTokenDailyMonthlyQuota rejects requests that would exceed the token's
// calendar-day / calendar-month budget. Used quota is aggregated from consume
// logs and cached in Redis (daily/monthly keys) so the hot pre-consume path
// only reads cached counters. 0 quota means unlimited. Exceeding returns
// errTokenDailyQuotaExceeded / errTokenMonthlyQuotaExceeded for the caller to
// translate into HTTP 429.
func CheckTokenDailyMonthlyQuota(ctx context.Context, tokenID int, dailyQuota, monthlyQuota int64) error {
	if dailyQuota <= 0 && monthlyQuota <= 0 {
		return nil
	}
	now := time.Now()
	if dailyQuota > 0 {
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
		used, err := getCachedTokenUsedQuota(ctx, tokenID, "daily", dayStart, tokenDailyQuotaCacheTTL, now, func() (int64, error) {
			return getTokenUsedQuotaSince(tokenID, dayStart)
		})
		if err != nil {
			// 聚合失败不阻断请求（尽力拦截语义），记录告警
			common.SysError(fmt.Sprintf("token %d daily quota lookup failed: %v", tokenID, err))
			return nil
		}
		if used >= dailyQuota {
			return errTokenDailyQuotaExceeded
		}
	}
	if monthlyQuota > 0 {
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
		used, err := getCachedTokenUsedQuota(ctx, tokenID, "monthly", monthStart, tokenMonthlyQuotaCacheTTL, now, func() (int64, error) {
			return getTokenUsedQuotaSince(tokenID, monthStart)
		})
		if err != nil {
			common.SysError(fmt.Sprintf("token %d monthly quota lookup failed: %v", tokenID, err))
			return nil
		}
		if used >= monthlyQuota {
			return errTokenMonthlyQuotaExceeded
		}
	}
	return nil
}

// getTokenUsedQuotaSince aggregates consume-log quota for a token since the
// given unix timestamp (inclusive), querying the log database.
func getTokenUsedQuotaSince(tokenID int, since int64) (int64, error) {
	var used int64
	err := model.LOG_DB.Model(&model.Log{}).
		Where("token_id = ? AND type = ? AND created_at >= ?", tokenID, model.LogTypeConsume, since).
		Select("COALESCE(SUM(quota), 0)").
		Scan(&used).Error
	return used, err
}

// tokenBudgetCacheKey builds the counter key for a token budget window. Shared
// by the aggregation read and the settle increment so both always address the
// same key (token_daily_quota:<id>:<YYYYMMDD> / token_monthly_quota:<id>:<YYYYMM>).
func tokenBudgetCacheKey(scope string, tokenID int, now time.Time) string {
	if scope == "daily" {
		return fmt.Sprintf("token_daily_quota:%d:%s", tokenID, dayKey(now))
	}
	return fmt.Sprintf("token_monthly_quota:%d:%s", tokenID, monthKey(now))
}

// memoryTokenBudget is the Redis-unavailable fallback for cached used-quota
// values: per-key 60s TTL guarded by a mutex, single-node semantics.
var memoryTokenBudget = struct {
	mu      sync.Mutex
	value   map[string]int64
	expires map[string]time.Time
}{
	value:   map[string]int64{},
	expires: map[string]time.Time{},
}

func getCachedTokenUsedQuota(ctx context.Context, tokenID int, scope string, since int64, ttl time.Duration, now time.Time, aggregate func() (int64, error)) (int64, error) {
	fullKey := tokenBudgetCacheKey(scope, tokenID, now)
	rdb := common.RDB
	if rdb != nil {
		if v, err := rdb.Get(ctx, fullKey).Int64(); err == nil {
			return v, nil
		}
	} else {
		memoryTokenBudget.mu.Lock()
		if exp, ok := memoryTokenBudget.expires[fullKey]; ok && time.Now().Before(exp) {
			v := memoryTokenBudget.value[fullKey]
			memoryTokenBudget.mu.Unlock()
			return v, nil
		}
		memoryTokenBudget.mu.Unlock()
	}
	used, err := aggregate()
	if err != nil {
		return 0, err
	}
	if rdb != nil {
		if err := rdb.Set(ctx, fullKey, used, ttl).Err(); err != nil {
			common.SysError(fmt.Sprintf("token %d %s quota cache set failed: %v", tokenID, scope, err))
		}
	} else {
		memoryTokenBudget.mu.Lock()
		memoryTokenBudget.value[fullKey] = used
		memoryTokenBudget.expires[fullKey] = time.Now().Add(tokenBudgetMemoryCacheTTL)
		memoryTokenBudget.mu.Unlock()
	}
	return used, nil
}

// bumpTokenBudgetUsed adjusts the cached used-quota counters for the current
// day/month windows by delta (positive = charge, negative = refund). Only
// windows with a configured budget are touched, so tokens without budget
// config never grow cache keys. Redis uses INCRBY/DECRBY with the window TTL;
// without Redis it updates the same in-memory cache getCachedTokenUsedQuota
// reads, so a settle stays visible to subsequent checks inside the 60s memory
// window. Called from the settle paths right after the token quota adjustment
// succeeds; without it the window key filled by the first check never changes
// and the day/month budget only ever sees the first-request-of-window state.
func bumpTokenBudgetUsed(ctx context.Context, tokenID int, delta int64, dailyQuota, monthlyQuota int64) {
	if tokenID <= 0 || delta == 0 {
		return
	}
	now := time.Now()
	if dailyQuota > 0 {
		bumpTokenBudgetWindow(ctx, "daily", tokenID, now, delta, tokenDailyQuotaCacheTTL)
	}
	if monthlyQuota > 0 {
		bumpTokenBudgetWindow(ctx, "monthly", tokenID, now, delta, tokenMonthlyQuotaCacheTTL)
	}
}

func bumpTokenBudgetWindow(ctx context.Context, scope string, tokenID int, now time.Time, delta int64, ttl time.Duration) {
	fullKey := tokenBudgetCacheKey(scope, tokenID, now)
	rdb := common.RDB
	if rdb != nil {
		var err error
		if delta >= 0 {
			err = rdb.IncrBy(ctx, fullKey, delta).Err()
		} else {
			err = rdb.DecrBy(ctx, fullKey, -delta).Err()
		}
		if err != nil {
			common.SysError(fmt.Sprintf("token %d %s quota cache increment failed: %v", tokenID, scope, err))
			return
		}
		if err := rdb.Expire(ctx, fullKey, ttl).Err(); err != nil {
			common.SysError(fmt.Sprintf("token %d %s quota cache expire failed: %v", tokenID, scope, err))
		}
		return
	}
	memoryTokenBudget.mu.Lock()
	defer memoryTokenBudget.mu.Unlock()
	if _, ok := memoryTokenBudget.value[fullKey]; !ok && delta < 0 {
		// 尚无聚合结果时退款无从抵消（计数将由下一次聚合从日志重建）
		return
	}
	memoryTokenBudget.value[fullKey] += delta
	// 结算递增后重置窗口，避免刚递增的键立即过期触发重聚合
	memoryTokenBudget.expires[fullKey] = time.Now().Add(tokenBudgetMemoryCacheTTL)
}

func dayKey(t time.Time) string  { return t.Format("20060102") }
func monthKey(t time.Time) string { return t.Format("200601") }
