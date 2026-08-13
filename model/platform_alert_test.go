package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPlatformAlertTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&PlatformAlertRule{}, &PlatformAlertEvent{}, &BusinessAuditEvent{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&PlatformAlertRule{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&PlatformAlertEvent{}).Error)
}

func TestValidatePlatformAlertRule(t *testing.T) {
	setupPlatformAlertTest(t)

	require.Error(t, ValidatePlatformAlertRule(&PlatformAlertRule{Name: "", Metric: "error_rate", Operator: ">", Threshold: 0.5}))
	require.Error(t, ValidatePlatformAlertRule(&PlatformAlertRule{Name: "r", Metric: "unknown", Operator: ">", Threshold: 1}))
	require.Error(t, ValidatePlatformAlertRule(&PlatformAlertRule{Name: "r", Metric: "error_rate", Operator: "==", Threshold: 0.5}))
	require.Error(t, ValidatePlatformAlertRule(&PlatformAlertRule{Name: "r", Metric: "error_rate", Operator: ">", Threshold: 1.5}))

	rule := &PlatformAlertRule{Name: "high error", Metric: "error_rate", Operator: ">", Threshold: 0.5}
	require.NoError(t, ValidatePlatformAlertRule(rule))
	assert.Equal(t, 10, rule.WindowMinutes, "default window applies")
}

func TestPlatformAlertEvaluationLifecycle(t *testing.T) {
	setupPlatformAlertTest(t)
	// 制造一个停用渠道使 disabled_channels 计数 > 0。
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	require.NoError(t, DB.Create(&Channel{Id: 9001, Type: 1, Name: "alert-disabled", Status: common.ChannelStatusAutoDisabled}).Error)
	t.Cleanup(func() {
		_ = DB.Unscoped().Delete(&Channel{}, 9001).Error
	})

	rule := &PlatformAlertRule{
		Name:          "disabled channel alert",
		Metric:        PlatformAlertMetricDisabledChannels,
		Operator:      ">",
		Threshold:     0,
		WindowMinutes: 10,
		Enabled:       true,
	}
	require.NoError(t, CreatePlatformAlertRule(rule, priceVersionTestActorPtr()))

	value, err := PlatformAlertMetricValue(rule.Metric, rule.WindowMinutes)
	require.NoError(t, err)
	assert.Greater(t, value, float64(0))

	event, err := CreatePlatformAlertEvent(rule, value)
	require.NoError(t, err)
	assert.Equal(t, PlatformAlertStatusActive, event.Status)

	// 同规则不重复创建：GetActivePlatformAlertEvent 返回现有事件。
	active, err := GetActivePlatformAlertEvent(rule.Id)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, event.Id, active.Id)

	// 指标恢复后事件被标记 resolved，且仅一次。
	resolved, err := ResolveActivePlatformAlertEvent(rule.Id)
	require.NoError(t, err)
	assert.True(t, resolved)
	resolved, err = ResolveActivePlatformAlertEvent(rule.Id)
	require.NoError(t, err)
	assert.False(t, resolved)

	var reloaded PlatformAlertEvent
	require.NoError(t, DB.First(&reloaded, event.Id).Error)
	assert.Equal(t, PlatformAlertStatusResolved, reloaded.Status)
	assert.NotZero(t, reloaded.ResolvedAt)
}

func TestCompareAlertThreshold(t *testing.T) {
	cases := []struct {
		value, threshold float64
		operator         string
		want             bool
	}{
		{0.8, 0.5, ">", true},
		{0.5, 0.5, ">=", true},
		{0.4, 0.5, "<", true},
		{0.5, 0.5, "<=", true},
		{0.3, 0.5, ">", false},
		{0.5, 0.5, ">", false},
	}
	for _, tc := range cases {
		got := CompareAlertThreshold(tc.value, tc.threshold, tc.operator)
		assert.Equal(t, tc.want, got, "value=%v threshold=%v operator=%v", tc.value, tc.threshold, tc.operator)
	}
}
