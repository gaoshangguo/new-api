package model

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// 多维渠道路由（P0-14）：地区、延迟、成功率作为可配置、可回退的选择维度。
// 无数据或无匹配时保持既有策略（权重随机/优先级）。

const channelRoutingSettingKey = "channel_routing_setting"

// ChannelRoutingSetting 是渠道选择的多维路由配置（JSON，经通用设置接口修改）。
type ChannelRoutingSetting struct {
	Enabled               bool    `json:"enabled"`                  // 总开关
	UseRegion             bool    `json:"use_region"`               // 启用地区维度
	UseLatency            bool    `json:"use_latency"`              // 启用延迟维度
	UseSuccessRate        bool    `json:"use_success_rate"`         // 启用成功率维度
	SuccessRateMinRequests int    `json:"success_rate_min_requests"` // 成功率生效所需最少样本数
	SuccessRateTarget     float64 `json:"success_rate_target"`      // 成功率基准（低于则降权）
	LatencyWeight         float64 `json:"latency_weight"`           // 延迟权重（0~1）
	SuccessRateWeight     float64 `json:"success_rate_weight"`      // 成功率权重
}

var defaultChannelRoutingSetting = ChannelRoutingSetting{
	Enabled:                false,
	UseRegion:              true,
	UseLatency:             true,
	UseSuccessRate:         true,
	SuccessRateMinRequests: 20,
	SuccessRateTarget:      0.9,
	LatencyWeight:          0.5,
	SuccessRateWeight:      1.0,
}

var (
	channelRoutingSettingMu      sync.Mutex
	channelRoutingSettingValue   ChannelRoutingSetting
	channelRoutingSettingRefresh time.Time
)

// GetChannelRoutingSetting 返回渠道路由设置（TTL 缓存 + 解析失败回退默认值）。
func GetChannelRoutingSetting() ChannelRoutingSetting {
	channelRoutingSettingMu.Lock()
	defer channelRoutingSettingMu.Unlock()
	if time.Since(channelRoutingSettingRefresh) < 30*time.Second && channelRoutingSettingRefresh.Unix() > 0 {
		return channelRoutingSettingValue
	}
	value := defaultChannelRoutingSetting
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[channelRoutingSettingKey]
	common.OptionMapRWMutex.RUnlock()
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			common.SysError("failed to parse channel_routing_setting: " + err.Error())
			value = defaultChannelRoutingSetting
		}
		// 非法配置值回退默认
		if value.LatencyWeight < 0 || value.LatencyWeight > 1 {
			value.LatencyWeight = defaultChannelRoutingSetting.LatencyWeight
		}
		if value.SuccessRateWeight < 0 {
			value.SuccessRateWeight = defaultChannelRoutingSetting.SuccessRateWeight
		}
		if value.SuccessRateTarget <= 0 || value.SuccessRateTarget > 1 {
			value.SuccessRateTarget = defaultChannelRoutingSetting.SuccessRateTarget
		}
		if value.SuccessRateMinRequests < 1 {
			value.SuccessRateMinRequests = defaultChannelRoutingSetting.SuccessRateMinRequests
		}
	}
	channelRoutingSettingValue = value
	channelRoutingSettingRefresh = time.Now()
	return value
}

// channelHealth 记录每个渠道近期的成功/失败次数（进程内窗口）。
type channelHealth struct {
	success atomic.Int64
	failed  atomic.Int64
}

var channelHealthMap sync.Map // channelID -> *channelHealth

// RecordChannelOutcome 记录一次渠道调用结果（成功/失败），供成功率维度使用。
func RecordChannelOutcome(channelID int, success bool) {
	if channelID <= 0 {
		return
	}
	raw, _ := channelHealthMap.LoadOrStore(channelID, &channelHealth{})
	health := raw.(*channelHealth)
	if success {
		health.success.Add(1)
	} else {
		health.failed.Add(1)
	}
}

// GetChannelSuccessRate 返回渠道近期成功率与样本数；样本不足时 ok=false。
func GetChannelSuccessRate(channelID int) (rate float64, samples int, ok bool) {
	raw, found := channelHealthMap.Load(channelID)
	if !found {
		return 0, 0, false
	}
	health := raw.(*channelHealth)
	success := health.success.Load()
	failed := health.failed.Load()
	total := success + failed
	if total == 0 {
		return 0, 0, false
	}
	return float64(success) / float64(total), int(total), true
}

// FilterChannelsByRegion 按地区过滤渠道候选。无匹配时返回原列表（可回退）。
func FilterChannelsByRegion(channels []int, region string, lookup func(int) *Channel) []int {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		return channels
	}
	filtered := make([]int, 0, len(channels))
	for _, channelID := range channels {
		channel := lookup(channelID)
		if channel != nil && channel.Region != nil && strings.ToLower(strings.TrimSpace(*channel.Region)) == region {
			filtered = append(filtered, channelID)
		}
	}
	if len(filtered) == 0 {
		return channels
	}
	return filtered
}

// AdjustRoutingWeights 计算目标优先级渠道内多维路由的权重因子。
// 无数据（延迟未知/成功率样本不足）的渠道因子为 1.0（保持既有策略）；
// 返回与 targetChannels 一一对应的因子切片。
func AdjustRoutingWeights(targetChannels []*Channel) []float64 {
	setting := GetChannelRoutingSetting()
	factors := make([]float64, len(targetChannels))
	for i := range factors {
		factors[i] = 1.0
	}
	if !setting.Enabled || len(targetChannels) == 0 {
		return factors
	}

	// 延迟维度：以同层渠道的响应时间相对排序归一化。
	if setting.UseLatency {
		minRT := math.MaxInt32
		maxRT := 0
		hasRT := false
		for _, channel := range targetChannels {
			if channel.ResponseTime > 0 {
				hasRT = true
				if channel.ResponseTime < minRT {
					minRT = channel.ResponseTime
				}
				if channel.ResponseTime > maxRT {
					maxRT = channel.ResponseTime
				}
			}
		}
		if hasRT && maxRT > minRT {
			for i, channel := range targetChannels {
				if channel.ResponseTime <= 0 {
					continue // 无延迟数据：保持既有权重
				}
				norm := float64(channel.ResponseTime-minRT) / float64(maxRT-minRT)
				factor := 1 + (0.5-norm)*2*setting.LatencyWeight
				factors[i] *= clampRoutingFactor(factor)
			}
		}
	}

	// 成功率维度：低于基准降权、高于基准升权。
	if setting.UseSuccessRate {
		for i, channel := range targetChannels {
			rate, samples, ok := GetChannelSuccessRate(channel.Id)
			if !ok || samples < setting.SuccessRateMinRequests {
				continue // 样本不足：保持既有策略
			}
			factor := 1 + (rate-setting.SuccessRateTarget)*setting.SuccessRateWeight
			factors[i] *= clampRoutingFactor(factor)
		}
	}
	return factors
}

func clampRoutingFactor(factor float64) float64 {
	if factor < 0.1 {
		return 0.1
	}
	if factor > 2 {
		return 2
	}
	return factor
}
