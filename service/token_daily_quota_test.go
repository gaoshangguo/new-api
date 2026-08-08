package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedConsumeLogs(t *testing.T, tokenID int, baseTime int64, quotas ...int64) {
	t.Helper()
	for i, q := range quotas {
		require.NoError(t, model.DB.Create(&model.Log{
			UserId: 1, TokenId: tokenID, Type: model.LogTypeConsume, ModelName: "gpt-test",
			Quota: int(q), CreatedAt: baseTime + int64(i),
		}).Error)
	}
}

// 与实现一致的自然日/自然月窗口起点（time.Date 计算，服务器本地时区）
func calendarWindowStarts(t *testing.T) (dayStart int64, monthStart int64) {
	t.Helper()
	nowTime := time.Now()
	dayStart = time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 0, 0, 0, 0, nowTime.Location()).Unix()
	monthStart = time.Date(nowTime.Year(), nowTime.Month(), 1, 0, 0, 0, 0, nowTime.Location()).Unix()
	return dayStart, monthStart
}

func TestCheckTokenDailyMonthlyQuota(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}, &model.Token{}))
	dayStart, monthStart := calendarWindowStarts(t)

	tokenID := 9001
	// 今日已用 800（500+300），日预算 1000；本月（含今日）已用 5000（1000+4000），月预算 20000
	seedConsumeLogs(t, tokenID, dayStart, 500, 300)
	seedConsumeLogs(t, tokenID, monthStart, 1000, 4000)

	// 日预算未超（已用 800 < 1000）
	require.NoError(t, CheckTokenDailyMonthlyQuota(context.Background(), tokenID, 1000, 20000))
	// 0 = 不限
	require.NoError(t, CheckTokenDailyMonthlyQuota(context.Background(), tokenID, 0, 0))
	// 日预算超限（已用 800 >= 800）
	err := CheckTokenDailyMonthlyQuota(context.Background(), tokenID, 800, 20000)
	require.Error(t, err)
	assert.ErrorIs(t, err, errTokenDailyQuotaExceeded)
}

func TestCheckTokenMonthlyQuotaExceeded(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}))
	_, monthStart := calendarWindowStarts(t)
	tokenID := 9002
	seedConsumeLogs(t, tokenID, monthStart, 25000) // 本月已用 25000
	err := CheckTokenDailyMonthlyQuota(context.Background(), tokenID, 100000, 24000)
	require.Error(t, err)
	assert.ErrorIs(t, err, errTokenMonthlyQuotaExceeded)
}

func TestGetTokenUsedQuotaSinceWindowBoundary(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}))
	tokenID := 9003
	now := time.Now()
	// 昨日（窗口外）日志不计入
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix() - 3600
	seedConsumeLogs(t, tokenID, yesterday, 999)
	dayStart, _ := calendarWindowStarts(t)
	used, err := getTokenUsedQuotaSince(tokenID, dayStart)
	require.NoError(t, err)
	assert.Equal(t, int64(0), used)
}

// TestPreConsumeBillingTokenDailyQuotaExceededMapsTo429 verifies the
// pre-consume hot path translates a token daily-budget exceedance into
// HTTP 429 (rate-limit semantics) instead of the default 403.
func TestPreConsumeBillingTokenDailyQuotaExceededMapsTo429(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}, &model.Token{}, &model.User{}))
	dayStart, _ := calendarWindowStarts(t)

	user := &model.User{Id: 9088, Username: "budget-429-user", Quota: 1000000, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		Id: 9089, UserId: user.Id, Key: "budget-429-key", Name: "budget-429-token",
		Status: common.TokenStatusEnabled, RemainQuota: 100000, DailyQuota: 1,
	}
	require.NoError(t, model.DB.Create(token).Error)
	seedConsumeLogs(t, token.Id, dayStart, 100) // 今日已用 100 >= 日预算 1

	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: false,
	}

	apiErr := PreConsumeBilling(c, 2000, info)

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.ErrorIs(t, apiErr, errTokenDailyQuotaExceeded)
	require.Nil(t, info.Billing)
}

// TestPreConsumeBillingTrustedUnlimitedTokenDailyQuotaStill429 verifies that
// the trust bypass only waives the total-quota pre-charge: an unlimited-quota
// token owned by a user above the trust threshold (10 * QuotaPerUnit) whose
// daily budget is exceeded must still be rejected with 429. B2 design:
// 无限额只豁免总预算，日/月预算是独立防护维度（配置了就该执行）。
func TestPreConsumeBillingTrustedUnlimitedTokenDailyQuotaStill429(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}, &model.Token{}, &model.User{}))
	dayStart, _ := calendarWindowStarts(t)

	// 用户额度 6,000,000 > 信任阈值 5,000,000（10 * QuotaPerUnit），钱包侧触发信任旁路
	user := &model.User{Id: 9086, Username: "budget-429-trusted-user", Quota: 6000000, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	// 无限额令牌：总预算豁免，但日预算仍必须执行
	token := &model.Token{
		Id: 9087, UserId: user.Id, Key: "budget-429-trusted-key", Name: "budget-429-trusted-token",
		Status: common.TokenStatusEnabled, RemainQuota: 100000, UnlimitedQuota: true, DailyQuota: 1,
	}
	require.NoError(t, model.DB.Create(token).Error)
	seedConsumeLogs(t, token.Id, dayStart, 100) // 今日已用 100 >= 日预算 1

	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: true,
	}

	apiErr := PreConsumeBilling(c, 2000, info)

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
	require.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	require.ErrorIs(t, apiErr, errTokenDailyQuotaExceeded)
	require.Nil(t, info.Billing)
}

// TestPreConsumeBillingTrustedUnlimitedTokenNoBudgetSucceeds verifies the
// trusted path still succeeds when no daily/monthly budget is configured
// (0 = unlimited), guarding the restructured pre-consume against over-rejecting
// trusted requests.
func TestPreConsumeBillingTrustedUnlimitedTokenNoBudgetSucceeds(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}, &model.Token{}, &model.User{}))

	user := &model.User{Id: 9084, Username: "budget-trusted-ok-user", Quota: 6000000, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		Id: 9085, UserId: user.Id, Key: "budget-trusted-ok-key", Name: "budget-trusted-ok-token",
		Status: common.TokenStatusEnabled, RemainQuota: 100000, UnlimitedQuota: true, // 日/月预算 0 = 不限
	}
	require.NoError(t, model.DB.Create(token).Error)

	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: true,
	}

	apiErr := PreConsumeBilling(c, 2000, info)

	require.Nil(t, apiErr)
	require.NotNil(t, info.Billing)
}

// TestPreConsumeBillingInsufficientTokenQuotaStays403 verifies that errors
// other than budget exceedance keep the original 403 mapping, so the 429
// branch does not over-match unrelated pre-consume failures.
func TestPreConsumeBillingInsufficientTokenQuotaStays403(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}, &model.Token{}, &model.User{}))

	user := &model.User{Id: 9098, Username: "budget-403-user", Quota: 1000000, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		Id: 9099, UserId: user.Id, Key: "budget-403-key", Name: "budget-403-token",
		Status: common.TokenStatusEnabled, RemainQuota: 100, // 日/月预算不限（0），但余额不足
	}
	require.NoError(t, model.DB.Create(token).Error)

	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: false,
	}

	apiErr := PreConsumeBilling(c, 2000, info)

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, apiErr.GetErrorCode())
	require.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}
