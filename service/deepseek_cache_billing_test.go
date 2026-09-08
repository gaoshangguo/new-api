package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// DeepSeek-style tiered expression: cache-hit (read) tokens priced separately
// at a lower rate via `cr`; `p` then auto-excludes the cached portion.
const deepseekCacheExpr = `tier("default", p * 2 + c * 10 + cr * 0.2)`

func deepseekCacheUsage(prompt, cached, completion int) *dto.Usage {
	return &dto.Usage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: cached,
		},
	}
}

// TestTieredSettleDeepSeekCacheHitDiscounted guards the headline behavior:
// a Tencent-tokenhub-style DeepSeek usage (cached_tokens close to prompt_tokens)
// must be charged at the discounted cache-hit rate through the tiered
// expression, and the all-miss pre-consume estimate must never be exceeded.
func TestTieredSettleDeepSeekCacheHitDiscounted(t *testing.T) {
	info := makeRelayInfo(deepseekCacheExpr, 1.0, 1000, 500)
	// pre-consume treats every input token as a cache miss: p*2 + c*10 = 7000
	// -> quota 3500.
	require.Equal(t, 3500, info.FinalPreConsumedQuota)

	usedVars := billingexpr.UsedVars(deepseekCacheExpr)
	params := BuildTieredTokenParams(deepseekCacheUsage(1000, 700, 500), false, usedVars)
	require.Equal(t, 300.0, params.P, "p must exclude the cached portion when cr is used")
	require.Equal(t, 700.0, params.CR, "cr must carry the cache-hit count")

	ok, quota, _ := TryTieredSettle(info, params)
	require.True(t, ok)
	// 300*2 + 500*10 + 700*0.2 = 5740 -> quota 2870.
	require.Equal(t, 2870, quota, "cache hits must be billed at the discounted cr rate")
	require.Less(t, quota, info.FinalPreConsumedQuota,
		"settlement with cache hits must never exceed the all-miss pre-consume")
}

func TestTieredSettleDeepSeekFullCacheHit(t *testing.T) {
	info := makeRelayInfo(deepseekCacheExpr, 1.0, 1000, 100)
	usedVars := billingexpr.UsedVars(deepseekCacheExpr)
	params := BuildTieredTokenParams(deepseekCacheUsage(1000, 1000, 100), false, usedVars)
	require.Equal(t, 0.0, params.P)
	require.Equal(t, 1000.0, params.CR)

	ok, quota, _ := TryTieredSettle(info, params)
	require.True(t, ok)
	// 0*2 + 100*10 + 1000*0.2 = 1200 -> quota 600.
	require.Equal(t, 600, quota)
}

func TestTieredSettleDeepSeekZeroCacheHit(t *testing.T) {
	info := makeRelayInfo(deepseekCacheExpr, 1.0, 1000, 500)
	usedVars := billingexpr.UsedVars(deepseekCacheExpr)
	params := BuildTieredTokenParams(deepseekCacheUsage(1000, 0, 500), false, usedVars)
	require.Equal(t, 1000.0, params.P, "no hits means every input token is billed as a miss")
	require.Equal(t, 0.0, params.CR)

	ok, quota, _ := TryTieredSettle(info, params)
	require.True(t, ok)
	require.Equal(t, 3500, quota, "zero hits must equal the all-miss charge")
}

// TestTieredSettleDeepSeekClampedCachedOverPrompt guards the billing-safety
// invariant: a malformed upstream cached_tokens that exceeds prompt_tokens
// must be clamped to the prompt size so it can neither produce a negative
// miss remainder nor inflate the charge beyond the actual prompt.
func TestTieredSettleDeepSeekClampedCachedOverPrompt(t *testing.T) {
	info := makeRelayInfo(deepseekCacheExpr, 1.0, 100, 10)
	usedVars := billingexpr.UsedVars(deepseekCacheExpr)
	params := BuildTieredTokenParams(deepseekCacheUsage(100, 999, 10), false, usedVars)
	require.Equal(t, 0.0, params.P)
	require.Equal(t, 100.0, params.CR, "cached_tokens must be clamped to prompt_tokens")

	ok, quota, _ := TryTieredSettle(info, params)
	require.True(t, ok)
	// 0*2 + 10*10 + 100*0.2 = 120 -> quota 60 (not 999*0.2 = ~150).
	require.Equal(t, 60, quota)
	require.GreaterOrEqual(t, quota, 0, "quota must never go negative")
}

// TestTieredSettleDeepSeekCanonicalHitSource guards requirement 3: whichever
// upstream field carried the hit count (cached_tokens in prompt_tokens_details
// vs input_tokens_details vs prompt_cache_hit_tokens), settlement must derive
// the same cr and the same charge.
func TestTieredSettleDeepSeekCanonicalHitSource(t *testing.T) {
	info := makeRelayInfo(deepseekCacheExpr, 1.0, 1000, 500)
	usedVars := billingexpr.UsedVars(deepseekCacheExpr)

	cases := []struct {
		name  string
		usage *dto.Usage
	}{
		{
			name:  "prompt_tokens_details.cached_tokens",
			usage: deepseekCacheUsage(1000, 700, 500),
		},
		{
			name: "input_tokens_details.cached_tokens",
			usage: &dto.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
				TotalTokens:      1500,
				InputTokensDetails: &dto.InputTokenDetails{
					CachedTokens: 700,
				},
			},
		},
		{
			name: "prompt_cache_hit_tokens",
			usage: &dto.Usage{
				PromptTokens:          1000,
				CompletionTokens:      500,
				TotalTokens:           1500,
				PromptCacheHitTokens:  700,
				PromptCacheMissTokens: 300,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := BuildTieredTokenParams(tc.usage, false, usedVars)
			require.Equal(t, 300.0, params.P)
			require.Equal(t, 700.0, params.CR)

			ok, quota, _ := TryTieredSettle(info, params)
			require.True(t, ok)
			require.Equal(t, 2870, quota, "all hit sources must settle to the same discounted charge")
		})
	}
}

// The legacy (non-expression) text quota path must apply the same canonical
// cache-hit source and CacheRatio discount so logs and charges stay consistent
// with the tiered path.
func TestCalculateTextQuotaSummaryDeepSeekCacheHitDiscounted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "deepseek-chat",
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			CacheRatio:      0.25,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, deepseekCacheUsage(1000, 700, 500))
	require.False(t, summary.IsClaudeUsageSemantic)
	require.Equal(t, 700, summary.CacheTokens)
	// (1000-700) + 700*0.25 + 500 = 975.
	require.Equal(t, 975, summary.Quota)
}

func TestCalculateTextQuotaSummaryDeepSeekZeroHitPaysFullPrice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "deepseek-chat",
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			CacheRatio:      0.25,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, deepseekCacheUsage(1000, 0, 500))
	require.Equal(t, 0, summary.CacheTokens)
	// 1000 + 500 = 1500 (no cache discount).
	require.Equal(t, 1500, summary.Quota)
}

func TestCalculateTextQuotaSummaryDeepSeekClampedCachedOverPrompt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "deepseek-chat",
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			CacheRatio:      0.25,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, deepseekCacheUsage(100, 999, 0))
	require.Equal(t, 100, summary.CacheTokens, "cached_tokens must be clamped to prompt_tokens")
	// 0 + 100*0.25 = 25 (not 999*0.25).
	require.Equal(t, 25, summary.Quota)
}

func TestCalculateTextQuotaSummaryDeepSeekFallbackToPromptCacheHitTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAI,
		OriginModelName: "deepseek-chat",
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			CacheRatio:      0.25,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
		StartTime: time.Now(),
	}

	usage := &dto.Usage{
		PromptTokens:          1000,
		CompletionTokens:      500,
		TotalTokens:           1500,
		PromptCacheHitTokens:  700,
		PromptCacheMissTokens: 300,
	}
	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	require.Equal(t, 700, summary.CacheTokens, "hit count must fall back to prompt_cache_hit_tokens")
	require.Equal(t, 975, summary.Quota)
}
