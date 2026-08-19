package model

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetRoutingSettingRefresh() {
	channelRoutingSettingRefresh = time.Time{}
}

func TestFilterChannelsByRegion(t *testing.T) {
	channels := []int{1, 2, 3}
	lookup := func(id int) *Channel {
		switch id {
		case 1:
			region := "cn-east"
			return &Channel{Id: 1, Region: &region}
		case 2:
			return &Channel{Id: 2} // 无地区
		case 3:
			region := "US-WEST"
			return &Channel{Id: 3, Region: &region}
		}
		return nil
	}

	// 大小写不敏感匹配
	assert.Equal(t, []int{1}, FilterChannelsByRegion(channels, "CN-EAST", lookup))
	assert.Equal(t, []int{3}, FilterChannelsByRegion(channels, "us-west", lookup))
	// 空地区不过滤
	assert.Equal(t, channels, FilterChannelsByRegion(channels, "", lookup))
	// 无匹配回退全部
	assert.Equal(t, channels, FilterChannelsByRegion(channels, "eu-north", lookup))
}

func TestGetChannelRoutingSettingParsesAndFallsBack(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	InitOptionMap()
	previous, hadPrevious := common.OptionMap[channelRoutingSettingKey]
	t.Cleanup(func() {
		if hadPrevious {
			common.OptionMap[channelRoutingSettingKey] = previous
		} else {
			delete(common.OptionMap, channelRoutingSettingKey)
		}
		resetRoutingSettingRefresh()
	})

	common.OptionMap[channelRoutingSettingKey] = `{"enabled":true,"use_latency":false,"latency_weight":0.2}`
	resetRoutingSettingRefresh()
	setting := GetChannelRoutingSetting()
	assert.True(t, setting.Enabled)
	assert.False(t, setting.UseLatency)
	assert.Equal(t, 0.2, setting.LatencyWeight)
	assert.True(t, setting.UseRegion, "defaults must apply for unset fields")

	// 非法 JSON 回退默认
	common.OptionMap[channelRoutingSettingKey] = `{broken`
	resetRoutingSettingRefresh()
	setting = GetChannelRoutingSetting()
	assert.False(t, setting.Enabled)

	// 越界值回退默认
	common.OptionMap[channelRoutingSettingKey] = `{"enabled":true,"latency_weight":5}`
	resetRoutingSettingRefresh()
	setting = GetChannelRoutingSetting()
	assert.Equal(t, defaultChannelRoutingSetting.LatencyWeight, setting.LatencyWeight)
}

func TestRecordChannelOutcomeAndSuccessRate(t *testing.T) {
	// 清空健康表
	channelHealthMap.Range(func(key, value any) bool {
		channelHealthMap.Delete(key)
		return true
	})

	RecordChannelOutcome(11, true)
	RecordChannelOutcome(11, true)
	RecordChannelOutcome(11, false)
	rate, samples, ok := GetChannelSuccessRate(11)
	require.True(t, ok)
	assert.Equal(t, 3, samples)
	assert.InDelta(t, 2.0/3.0, rate, 0.001)

	_, _, ok = GetChannelSuccessRate(99)
	assert.False(t, ok, "unknown channel must report no data")
}

func TestRoutingTraceRecordsCandidatesAndAttempts(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`))

	InitRoutingTrace(ctx)
	RecordRoutingCandidates(ctx, []int{1, 2, 3})
	RecordRoutingAttempt(ctx, 1, "channel:response_time_exceeded", "timeout")
	RecordRoutingAttempt(ctx, 2, "bad_response_status_code", "502")
	MarkRoutingFinal(ctx, 2, false)

	trace := GetRoutingTrace(ctx)
	require.NotNil(t, trace)
	assert.Equal(t, []int{1, 2, 3}, trace.Candidates)
	require.Len(t, trace.Attempts, 2)
	assert.Equal(t, 1, trace.Attempts[0].ChannelId)
	assert.Equal(t, "channel:response_time_exceeded", trace.Attempts[0].Code)
	assert.Equal(t, 2, trace.FinalChannel)
	assert.False(t, trace.FinalSuccess)

	summary := trace.RoutingSummary()
	assert.Equal(t, 2, summary.Attempts)
	assert.True(t, summary.Retried)
	assert.Equal(t, 2, summary.FinalChannel)
}

// P0-14：多维路由因子放大高分渠道权重时，加权随机选择必须始终命中一个渠道。
// 回归：totalWeight 曾按基础权重计算而逐渠道扣减按「基础权重×因子」，
// 因子 >1 时有效权重总和超过 totalWeight，导致随机值到循环结束仍 ≥0，
// 间歇性返回 "channel not found"。
func TestRandomSatisfiedChannelSelectionTerminatesWithRoutingFactors(t *testing.T) {
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	channelHealthMap.Range(func(key, value any) bool {
		channelHealthMap.Delete(key)
		return true
	})
	previous, hadPrevious := common.OptionMap[channelRoutingSettingKey]
	t.Cleanup(func() {
		if hadPrevious {
			common.OptionMap[channelRoutingSettingKey] = previous
		} else {
			delete(common.OptionMap, channelRoutingSettingKey)
		}
		resetRoutingSettingRefresh()
		require.NoError(t, DB.Exec("DELETE FROM channels").Error)
		InitChannelCache()
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		channelHealthMap.Range(func(key, value any) bool {
			channelHealthMap.Delete(key)
			return true
		})
	})

	priority := int64(0)
	highWeight := uint(300)
	lowWeight := uint(100)
	channels := []*Channel{
		{Id: 601, Type: 1, Key: "k-601", Status: common.ChannelStatusEnabled, Name: "high-weight", Group: "default", Models: "gpt-test", Weight: &highWeight, Priority: &priority},
		{Id: 602, Type: 1, Key: "k-602", Status: common.ChannelStatusEnabled, Name: "low-weight", Group: "default", Models: "gpt-test", Weight: &lowWeight, Priority: &priority},
	}
	for _, channel := range channels {
		require.NoError(t, DB.Create(channel).Error)
	}
	InitChannelCache()

	// 高权重渠道成功率 100%（因子 1.1），低权重渠道 80%（因子 0.9）：
	// 有效权重总和 300*1.1+100*0.9=420 > 基础总和 400，旧实现会间歇失败。
	common.OptionMap[channelRoutingSettingKey] = `{"enabled":true,"use_latency":false,"use_success_rate":true,"success_rate_target":0.9,"success_rate_weight":1.0,"success_rate_min_requests":5}`
	resetRoutingSettingRefresh()
	for i := 0; i < 20; i++ {
		RecordChannelOutcome(601, true)
	}
	for i := 0; i < 16; i++ {
		RecordChannelOutcome(602, true)
	}
	for i := 0; i < 4; i++ {
		RecordChannelOutcome(602, false)
	}

	seen := map[int]bool{}
	for i := 0; i < 500; i++ {
		channel, err := GetRandomSatisfiedChannel("default", "gpt-test", 0, "", nil)
		require.NoError(t, err)
		require.NotNil(t, channel)
		seen[channel.Id] = true
	}
	assert.True(t, seen[601], "high-weight channel must be selected at least once")
	assert.True(t, seen[602], "low-weight channel must be selected at least once")
}

