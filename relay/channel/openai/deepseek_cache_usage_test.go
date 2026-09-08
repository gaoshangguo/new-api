package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

// TestApplyUsagePostProcessingDeepSeekCacheDerivation covers the DeepSeek KV
// cache derivation in applyUsagePostProcessing: given only
// prompt_tokens_details.cached_tokens from the upstream, the relay must expose
// DeepSeek's prompt_cache_hit_tokens / prompt_cache_miss_tokens to the client.
func TestApplyUsagePostProcessingDeepSeekCacheDerivation(t *testing.T) {
	newUsage := func(prompt, cached int) *dto.Usage {
		return &dto.Usage{
			PromptTokens: prompt,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: cached,
			},
		}
	}

	tests := []struct {
		name        string
		prompt      int
		cached      int
		preHit      int // pre-existing prompt_cache_hit_tokens from upstream
		preMiss     int // pre-existing prompt_cache_miss_tokens from upstream
		wantHit     int
		wantMiss    int
		wantDerived bool // whether the derivation branch should run and set fields
	}{
		{
			name:        "upstream only reports cached_tokens",
			prompt:      100,
			cached:      30,
			wantHit:     30,
			wantMiss:    70,
			wantDerived: true,
		},
		{
			name:        "full cache hit",
			prompt:      100,
			cached:      100,
			wantHit:     100,
			wantMiss:    0,
			wantDerived: true,
		},
		{
			name:        "no cache hit",
			prompt:      100,
			cached:      0,
			wantHit:     0,
			wantMiss:    100,
			wantDerived: true,
		},
		{
			name:        "prompt tokens zero skips derivation",
			prompt:      0,
			cached:      50,
			wantHit:     0,
			wantMiss:    0,
			wantDerived: false,
		},
		{
			name:        "cached exceeds prompt tokens is clamped",
			prompt:      100,
			cached:      999,
			wantHit:     100,
			wantMiss:    0,
			wantDerived: true,
		},
		{
			name:        "upstream already provides miss tokens is preserved",
			prompt:      100,
			cached:      30,
			preMiss:     25,
			wantHit:     0,
			wantMiss:    25,
			wantDerived: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := newUsage(tt.prompt, tt.cached)
			usage.PromptCacheHitTokens = tt.preHit
			usage.PromptCacheMissTokens = tt.preMiss

			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType: constant.ChannelTypeDeepSeek,
				},
			}
			applyUsagePostProcessing(info, usage, nil)

			require.Equal(t, tt.wantHit, usage.PromptCacheHitTokens, "prompt_cache_hit_tokens")
			require.Equal(t, tt.wantMiss, usage.PromptCacheMissTokens, "prompt_cache_miss_tokens")
			require.GreaterOrEqual(t, usage.PromptCacheMissTokens, 0, "miss tokens must never be negative")
		})
	}
}
