package billing_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setDurationPrices 直接写入包级配置并返回清理函数，避免测试互相污染。
func setDurationPrices(model string, prices map[string]float64) func() {
	billingSetting.BillingDurationPrice[model] = prices
	return func() {
		delete(billingSetting.BillingDurationPrice, model)
	}
}

// setTokenPrices 直接写入包级 token 单价配置并返回清理函数。
func setTokenPrices(model string, prices map[string]float64) func() {
	billingSetting.BillingTokenPrice[model] = prices
	return func() {
		delete(billingSetting.BillingTokenPrice, model)
	}
}

func TestNormalizeResolution(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"720p", DurationPrice720p},
		{"1080p", DurationPrice1080p},
		{"4k", DurationPrice4k},
		{"4K", DurationPrice4k},
		{"1080P", DurationPrice1080p},
		{"3840x2160", DurationPrice4k},
		{"1920x1080", DurationPrice1080p},
		{"1280x720", DurationPrice720p},
		{"fullhd", DurationPrice1080p},
		{"", ""},
		{"1792x1024", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, NormalizeResolution(tt.raw), "NormalizeResolution(%q)", tt.raw)
	}
}

func TestGetBillingDurationPriceBasePriority(t *testing.T) {
	cleanup := setDurationPrices("seedance-test", map[string]float64{
		"1080p": 0.041,
		"4k":    0.081,
	})
	defer cleanup()

	// 无 default/720p 时，按档位顺序取 1080p
	price, ok := GetBillingDurationPrice("seedance-test")
	require.True(t, ok)
	assert.Equal(t, 0.041, price)

	// default 优先
	cleanup2 := setDurationPrices("seedance-test", map[string]float64{
		"default": 0.02,
		"720p":    0.027,
	})
	defer cleanup2()
	price, ok = GetBillingDurationPrice("seedance-test")
	require.True(t, ok)
	assert.Equal(t, 0.02, price)

	// 未配置返回 false
	_, ok = GetBillingDurationPrice("unset-model")
	assert.False(t, ok)
}

func TestGetBillingDurationResolutionRatio(t *testing.T) {
	// default 0.027 作为基准；1080p=0.041，4k=0.081
	cleanup := setDurationPrices("seedance-test", map[string]float64{
		"default": 0.027,
		"1080p":   0.041,
		"4k":      0.081,
	})
	defer cleanup()

	// 720p（未单独配置）→ 兜底 default，倍率 1.0
	ratio, ok := GetBillingDurationResolutionRatio("seedance-test", "720p")
	require.True(t, ok)
	assert.Equal(t, 1.0, ratio)

	// 1080p → 0.041/0.027
	ratio, ok = GetBillingDurationResolutionRatio("seedance-test", "1080p")
	require.True(t, ok)
	assert.InDelta(t, 0.041/0.027, ratio, 1e-9)

	// 4k → 0.081/0.027
	ratio, ok = GetBillingDurationResolutionRatio("seedance-test", "4K")
	require.True(t, ok)
	assert.InDelta(t, 0.081/0.027, ratio, 1e-9)

	// 分辨率不在档位列表 → 兜底，倍率 1.0
	ratio, ok = GetBillingDurationResolutionRatio("seedance-test", "1792x1024")
	require.True(t, ok)
	assert.Equal(t, 1.0, ratio)

	// 未配置时长单价 → (0, false)
	_, ok = GetBillingDurationResolutionRatio("unset-model", "1080p")
	assert.False(t, ok)
}

func TestGetBillingTokenPriceTierPriority(t *testing.T) {
	cleanup := setTokenPrices("video-token-test", map[string]float64{
		"720p":  1.2,
		"1080p": 1.8,
		"4k":    4.5,
	})
	defer cleanup()

	// 精确档位命中
	price, tier, ok := GetBillingTokenPrice("video-token-test", "1080p")
	require.True(t, ok)
	assert.Equal(t, "1080p", tier)
	assert.Equal(t, 1.8, price)

	// 原始尺寸字符串归一化后命中
	price, tier, ok = GetBillingTokenPrice("video-token-test", "3840x2160")
	require.True(t, ok)
	assert.Equal(t, "4k", tier)
	assert.Equal(t, 4.5, price)

	// 未知分辨率且无 default → 取已配置档位的最小值，避免算出 0 元
	price, tier, ok = GetBillingTokenPrice("video-token-test", "1792x1024")
	require.True(t, ok)
	assert.Equal(t, "720p", tier)
	assert.Equal(t, 1.2, price)

	// default 优先于最小档位
	cleanup2 := setTokenPrices("video-token-test", map[string]float64{
		"720p":    1.2,
		"1080p":   1.8,
		"default": 1.5,
	})
	defer cleanup2()
	price, tier, ok = GetBillingTokenPrice("video-token-test", "1792x1024")
	require.True(t, ok)
	assert.Equal(t, "default", tier)
	assert.Equal(t, 1.5, price)

	// 未配置 → false
	_, _, ok = GetBillingTokenPrice("unset-model", "1080p")
	assert.False(t, ok)
}

func TestGetBillingTokenPricesReturnsCopy(t *testing.T) {
	cleanup := setTokenPrices("video-token-copy", map[string]float64{"1080p": 2})
	defer cleanup()

	prices := GetBillingTokenPrices("video-token-copy")
	require.Equal(t, map[string]float64{"1080p": 2}, prices)

	// 修改返回值不得影响包级配置
	prices["1080p"] = 99
	price, _, ok := GetBillingTokenPrice("video-token-copy", "1080p")
	require.True(t, ok)
	assert.Equal(t, 2.0, price)
}
