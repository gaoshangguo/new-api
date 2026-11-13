package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateVideoTokens(t *testing.T) {
	tests := []struct {
		name    string
		width   int64
		height  int64
		fps     int64
		seconds int64
		want    int64
	}{
		{"1080p 5s 24fps", 1920, 1080, 24, 5, 243000},
		{"720p 5s 24fps", 1280, 720, 24, 5, 108000},
		{"4k 5s 24fps", 3840, 2160, 24, 5, 972000},
		{"invalid width returns zero", 0, 1080, 24, 5, 0},
		{"invalid seconds returns zero", 1920, 1080, 24, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EstimateVideoTokens(tt.width, tt.height, tt.fps, tt.seconds))
		})
	}
}

func TestEstimateVideoTokensSaturatesOversizedInput(t *testing.T) {
	// 尺寸、帧率、时长都超出上限时必须先钳制，再饱和到 int32 上限，不能回绕成负数。
	tokens := EstimateVideoTokens(20000, 20000, 500, 7200)
	assert.Equal(t, int64(maxVideoTokenEstimate), tokens)
}

func TestResolveVideoBillingSpecFromSize(t *testing.T) {
	spec := ResolveVideoBillingSpec(TaskSubmitReq{
		Size:     "1920x1080",
		Duration: 5,
	})
	assert.Equal(t, "1080p", spec.Resolution)
	assert.Equal(t, int64(1920), spec.Width)
	assert.Equal(t, int64(1080), spec.Height)
	assert.Equal(t, int64(24), spec.FPS)
	assert.Equal(t, int64(5), spec.Seconds)
	assert.Equal(t, int64(243000), spec.EstimatedTokens)
}

func TestResolveVideoBillingSpecFromSecondsString(t *testing.T) {
	spec := ResolveVideoBillingSpec(TaskSubmitReq{
		Size:    "1280*720",
		Seconds: "5",
	})
	assert.Equal(t, "720p", spec.Resolution)
	assert.Equal(t, int64(108000), spec.EstimatedTokens)
}

func TestResolveVideoBillingSpecFromMetadataResolution(t *testing.T) {
	spec := ResolveVideoBillingSpec(TaskSubmitReq{
		Metadata: map[string]interface{}{
			"resolution":      "4k",
			"durationSeconds": float64(5),
		},
	})
	assert.Equal(t, "4k", spec.Resolution)
	assert.Equal(t, int64(3840), spec.Width)
	assert.Equal(t, int64(2160), spec.Height)
	assert.Equal(t, int64(5), spec.Seconds)
	assert.Equal(t, int64(972000), spec.EstimatedTokens)
}

func TestResolveVideoBillingSpecDefaultsWhenSizeMissing(t *testing.T) {
	// 尺寸未知时按 720p 估算，时长未知时用保守默认时长。
	spec := ResolveVideoBillingSpec(TaskSubmitReq{})
	assert.Equal(t, "720p", spec.Resolution)
	assert.Equal(t, int64(1280), spec.Width)
	assert.Equal(t, int64(720), spec.Height)
	assert.Equal(t, int64(DefaultVideoSeconds), spec.Seconds)
	assert.Equal(t, EstimateVideoTokens(1280, 720, DefaultVideoFPS, DefaultVideoSeconds), spec.EstimatedTokens)
}

func TestResolveVideoBillingSpecInfersTierFromDimensions(t *testing.T) {
	// 1792x1024 不在标准档位字符串里，但按最长边可归入 720p 档；像素仍按实际值估算。
	spec := ResolveVideoBillingSpec(TaskSubmitReq{Size: "1792x1024", Duration: 5})
	assert.Equal(t, "720p", spec.Resolution)
	assert.Equal(t, int64(1792), spec.Width)
	assert.Equal(t, int64(1024), spec.Height)
	assert.Equal(t, EstimateVideoTokens(1792, 1024, 24, 5), spec.EstimatedTokens)
}

func TestResolveVideoBillingSpecClampsMetadataFPS(t *testing.T) {
	spec := ResolveVideoBillingSpec(TaskSubmitReq{
		Size:     "1280x720",
		Duration: 5,
		Metadata: map[string]interface{}{"fps": float64(500)},
	})
	assert.Equal(t, int64(MaxVideoFPS), spec.FPS)
}

func TestVideoBillingSpecSetSecondsRecomputesTokens(t *testing.T) {
	spec := ResolveVideoBillingSpec(TaskSubmitReq{Size: "1920x1080"})
	spec.SetSeconds(10)
	assert.Equal(t, int64(10), spec.Seconds)
	assert.Equal(t, int64(486000), spec.EstimatedTokens)
}
