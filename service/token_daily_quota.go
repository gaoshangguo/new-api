package service

import (
	"context"
	"errors"
	"fmt"
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
		used, err := getCachedTokenUsedQuota(ctx, tokenID, "daily", dayStart, tokenDailyQuotaCacheTTL, dayKey(now), func() (int64, error) {
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
		used, err := getCachedTokenUsedQuota(ctx, tokenID, "monthly", monthStart, tokenMonthlyQuotaCacheTTL, monthKey(now), func() (int64, error) {
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

func getCachedTokenUsedQuota(ctx context.Context, tokenID int, scope string, since int64, ttl time.Duration, key string, aggregate func() (int64, error)) (int64, error) {
	fullKey := fmt.Sprintf("token_%s_quota:%d:%s", scope, tokenID, key)
	rdb := common.RDB
	if rdb != nil {
		if v, err := rdb.Get(ctx, fullKey).Int64(); err == nil {
			return v, nil
		}
	}
	used, err := aggregate()
	if err != nil {
		return 0, err
	}
	if rdb != nil {
		if err := rdb.Set(ctx, fullKey, used, ttl).Err(); err != nil {
			common.SysError(fmt.Sprintf("token %d %s quota cache set failed: %v", tokenID, scope, err))
		}
	}
	return used, nil
}

func dayKey(t time.Time) string  { return t.Format("20060102") }
func monthKey(t time.Time) string { return t.Format("200601") }
