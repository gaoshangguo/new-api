package service

import (
	"context"
	"testing"
	"time"

	_ "github.com/QuantumNous/new-api/common" // 简报要求 imports 含 common；测试代码未直接引用，故空导入
	"github.com/QuantumNous/new-api/model"
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
