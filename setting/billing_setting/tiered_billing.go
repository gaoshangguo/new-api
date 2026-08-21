package billing_setting

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/samber/lo"
)

const (
	BillingModeRatio          = "ratio"
	BillingModeTieredExpr     = "tiered_expr"
	BillingModePerDuration    = "per_duration"
	BillingModeField          = "billing_mode"
	BillingExprField          = "billing_expr"
	BillingDurationPriceField = "billing_duration_price"
)

// billing_duration_price 的值是"分辨率档位 → 每秒单价（美元/秒）"的映射。
// "default" 为兜底档位：请求分辨率不在档位列表时按它计费。
const (
	DurationPriceDefault = "default"
	DurationPrice720p    = "720p"
	DurationPrice1080p   = "1080p"
	DurationPrice4k      = "4k"
)

// BillingSetting is managed by config.GlobalConfig.Register.
// DB keys: billing_setting.billing_mode, billing_setting.billing_expr, billing_setting.billing_duration_price
type BillingSetting struct {
	BillingMode          map[string]string             `json:"billing_mode"`
	BillingExpr          map[string]string             `json:"billing_expr"`
	BillingDurationPrice map[string]map[string]float64 `json:"billing_duration_price"`
}

var billingSetting = BillingSetting{
	BillingMode:          make(map[string]string),
	BillingExpr:          make(map[string]string),
	BillingDurationPrice: make(map[string]map[string]float64),
}

func init() {
	config.GlobalConfig.Register("billing_setting", &billingSetting)
}

// ---------------------------------------------------------------------------
// Read accessors (hot path, must be fast)
// ---------------------------------------------------------------------------

func GetBillingMode(model string) string {
	if mode, ok := billingSetting.BillingMode[model]; ok {
		return mode
	}
	return BillingModeRatio
}

func GetBillingExpr(model string) (string, bool) {
	expr, ok := billingSetting.BillingExpr[model]
	return expr, ok
}

func GetBillingModeCopy() map[string]string {
	return lo.Assign(billingSetting.BillingMode)
}

func GetBillingExprCopy() map[string]string {
	return lo.Assign(billingSetting.BillingExpr)
}

// GetBillingDurationPrices 返回模型在按时长计费模式下的"分辨率档位 → 每秒单价"
// 映射（键为 DurationPriceDefault/DurationPrice720p/DurationPrice1080p/DurationPrice4k）。
// 返回的是拷贝，调用方不应修改。
func GetBillingDurationPrices(model string) map[string]float64 {
	prices := billingSetting.BillingDurationPrice[model]
	if len(prices) == 0 {
		return nil
	}
	out := make(map[string]float64, len(prices))
	for k, v := range prices {
		out[k] = v
	}
	return out
}

// GetBillingDurationPrice 返回按时长计费模型的基准单价（美元/秒），作为
// ModelPriceHelperPerCall 的固定价基础。解析优先级：
// default > 720p > 1080p > 4k > 其余档位最小值；均未配置时返回 false。
func GetBillingDurationPrice(model string) (float64, bool) {
	prices := GetBillingDurationPrices(model)
	if len(prices) == 0 {
		return 0, false
	}
	if price := prices[DurationPriceDefault]; price > 0 {
		return price, true
	}
	for _, key := range []string{DurationPrice720p, DurationPrice1080p, DurationPrice4k} {
		if price := prices[key]; price > 0 {
			return price, true
		}
	}
	minPrice := math.MaxFloat64
	found := false
	for _, price := range prices {
		if price > 0 && price < minPrice {
			minPrice = price
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return minPrice, true
}

// NormalizeResolution 把请求中的原始分辨率/尺寸字符串归一化为计费档位键。
// 无法识别的输入返回空串，调用方按兜底单价（default）处理。
func NormalizeResolution(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(s, "4k"), strings.Contains(s, "4096"), strings.Contains(s, "3840"):
		return DurationPrice4k
	case strings.Contains(s, "1080"), strings.Contains(s, "fullhd"), strings.Contains(s, "1920"):
		return DurationPrice1080p
	case strings.Contains(s, "720"), strings.Contains(s, "1280"):
		return DurationPrice720p
	}
	return ""
}

// GetBillingDurationResolutionRatio 返回模型在指定原始分辨率下的计费倍率
// （分辨率单价 / 基准单价），供适配器作为 OtherRatio "resolution" 使用。
// 分辨率不在档位列表时返回 1.0（按基准单价计费）。
// 第二个返回值表示该模型是否配置了时长单价。
func GetBillingDurationResolutionRatio(model, rawResolution string) (float64, bool) {
	base, ok := GetBillingDurationPrice(model)
	if !ok || base <= 0 {
		return 0, false
	}
	prices := GetBillingDurationPrices(model)
	if key := NormalizeResolution(rawResolution); key != "" {
		if price := prices[key]; price > 0 {
			return price / base, true
		}
	}
	return 1.0, true
}

func GetBillingDurationPriceCopy() map[string]map[string]float64 {
	src := billingSetting.BillingDurationPrice
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]map[string]float64, len(src))
	for model, prices := range src {
		cp := make(map[string]float64, len(prices))
		for k, v := range prices {
			cp[k] = v
		}
		out[model] = cp
	}
	return out
}

func GetPricingSyncData(base map[string]any) map[string]any {
	extra := make(map[string]any, 3)
	if modes := GetBillingModeCopy(); len(modes) > 0 {
		extra[BillingModeField] = modes
	}
	if exprs := GetBillingExprCopy(); len(exprs) > 0 {
		extra[BillingExprField] = exprs
	}
	if prices := GetBillingDurationPriceCopy(); len(prices) > 0 {
		extra[BillingDurationPriceField] = prices
	}
	return lo.Assign(base, extra)
}

// ---------------------------------------------------------------------------
// Smoke test (called externally for validation before save)
// ---------------------------------------------------------------------------

func SmokeTestExpr(exprStr string) error {
	return smokeTestExpr(exprStr)
}

func smokeTestExpr(exprStr string) error {
	vectors := []billingexpr.TokenParams{
		{P: 0, C: 0, Len: 0},
		{P: 1000, C: 1000, Len: 1000},
		{P: 100000, C: 100000, Len: 100000},
		{P: 1000000, C: 1000000, Len: 1000000},
	}
	requests := []billingexpr.RequestInput{
		{},
		{
			Headers: map[string]string{
				"anthropic-beta": "fast-mode-2026-02-01",
			},
			Body: []byte(`{"service_tier":"fast","stream_options":{"include_usage":true},"messages":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`),
		},
	}

	for _, v := range vectors {
		for _, request := range requests {
			result, _, err := billingexpr.RunExprWithRequest(exprStr, v, request)
			if err != nil {
				return fmt.Errorf("vector {p=%g, c=%g}: run failed: %w", v.P, v.C, err)
			}
			if result < 0 {
				return fmt.Errorf("vector {p=%g, c=%g}: result %f < 0", v.P, v.C, result)
			}
		}
	}
	return nil
}
