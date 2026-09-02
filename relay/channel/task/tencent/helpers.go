package tencent

import (
	"math"
	"strconv"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// wandUsage 为各查询响应共用的用量字段。
type wandUsage struct {
	TotalTokens int `json:"total_tokens"`
}

func metadataString(metadata map[string]any, key string) string {
	if v, ok := metadata[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// resolveDurationSeconds 解析请求时长并钳制到系统上限；未指定时返回 fallback。
func resolveDurationSeconds(req relaycommon.TaskSubmitReq, fallback int) int {
	duration := req.Duration
	if duration <= 0 && req.Seconds != "" {
		duration, _ = strconv.Atoi(req.Seconds)
	}
	if duration <= 0 {
		return fallback
	}
	if duration > relaycommon.MaxTaskDurationSeconds {
		duration = relaycommon.MaxTaskDurationSeconds
	}
	return duration
}

// aspectRatioFromRequest 从 metadata.ratio 或 size 宽高比推导画幅。
func aspectRatioFromRequest(req relaycommon.TaskSubmitReq, fallback string) string {
	if r := metadataString(req.Metadata, "ratio"); r != "" {
		return r
	}
	if r := ratioFromSize(req.Size); r != "" {
		return r
	}
	return fallback
}

// ratioFromSize 将 "WxH" 尺寸映射到最近的画幅档位。
func ratioFromSize(size string) string {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return ""
	}
	width, errW := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	height, errH := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return ""
	}

	aspect := width / height
	best := "16:9"
	bestDiff := math.Abs(aspect - 16.0/9.0)
	for _, candidate := range []struct {
		ratio string
		value float64
	}{
		{"21:9", 21.0 / 9.0},
		{"16:9", 16.0 / 9.0},
		{"4:3", 4.0 / 3.0},
		{"1:1", 1.0},
		{"3:4", 3.0 / 4.0},
		{"9:16", 9.0 / 16.0},
	} {
		if diff := math.Abs(aspect - candidate.value); diff < bestDiff {
			best = candidate.ratio
			bestDiff = diff
		}
	}
	return best
}
