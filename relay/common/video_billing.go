package common

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
)

// 视频 token 计费（billing_mode = video_token）在提交阶段需要把请求规格
// （分辨率、像素、帧率、时长）换算成 token 估算值，作为预扣额度；
// 任务完成后按上游返回的 total_tokens 结算差额。
//
// 估算口径与主流视频生成模型一致：token = ceil(宽 × 高 × 帧率 × 时长 / 1024)。
// 例如 1920×1080、24fps、5 秒 = 243000 token，1280×720、24fps、5 秒 = 108000 token。
const (
	// MaxVideoDimension 限制计费用的单边像素，防止用户传入超大尺寸溢出成负扣费。
	MaxVideoDimension = 8192
	// MaxVideoFPS 限制计费用的帧率，理由同上。
	MaxVideoFPS = 120
	// DefaultVideoFPS 是请求未声明帧率时的估算帧率。
	DefaultVideoFPS = 24
	// DefaultVideoSeconds 是请求未声明时长时的保守估算时长（取各上游默认时长的上限）。
	DefaultVideoSeconds = 8
	// videoTokenDivisor 是视频 token 估算的除数，与上游 token 口径对齐。
	videoTokenDivisor = 1024
	// maxVideoTokenEstimate 与任务 token 字段的类型宽度对齐；
	// 超出时饱和，避免 32 位平台上的 int 回绕。
	maxVideoTokenEstimate = math.MaxInt32
)

// VideoBillingSpec 是视频 token 计费所需的请求规格快照。
type VideoBillingSpec struct {
	// Resolution 是归一化后的分辨率档位（720p/1080p/4k），无法识别时为空。
	Resolution string
	Width      int64
	Height     int64
	FPS        int64
	Seconds    int64
	// EstimatedTokens 是按当前规格估算的计费 token 数（提交阶段）。
	EstimatedTokens int64
	// TokenPrice 是每百万 token 的单价（美元），由定价层在预扣时写入并冻结。
	TokenPrice float64
}

// SetSeconds 用适配器解析出的时长校正规格，并按校正后的时长重算 token 估算。
func (s *VideoBillingSpec) SetSeconds(seconds int64) {
	if s == nil || seconds <= 0 {
		return
	}
	s.Seconds = min(seconds, MaxTaskDurationSeconds)
	s.EstimatedTokens = EstimateVideoTokens(s.Width, s.Height, s.FPS, s.Seconds)
}

// ResolveVideoBillingSpec 从任务提交请求中解析计费规格。
// 尺寸优先取 size（如 "1920x1080"），其次按分辨率字符串（如 "1080p"）取默认尺寸；
// 帧率与时长可来自 metadata，未声明时使用保守默认值并做上限钳制。
func ResolveVideoBillingSpec(req TaskSubmitReq) VideoBillingSpec {
	sizeRaw := strings.TrimSpace(req.Size)
	resolutionRaw := metadataString(req.Metadata, "resolution", "size")

	width, height, hasSize := parseVideoSize(sizeRaw)
	tier := billing_setting.NormalizeResolution(resolutionRaw)
	if tier == "" {
		tier = billing_setting.NormalizeResolution(sizeRaw)
	}
	if tier == "" && hasSize {
		tier = videoResolutionFromDimensions(width, height)
	}
	if !hasSize {
		if width, height = videoResolutionDimensions(tier); width <= 0 || height <= 0 {
			// 尺寸未知时按 720p 估算，档位与像素保持一致，避免估算值与档位价脱节。
			tier = billing_setting.DurationPrice720p
			width, height = videoResolutionDimensions(tier)
		}
	}

	fps := metadataInt64(req.Metadata, "fps", "framespersecond", "frame_rate", "frameRate")
	if fps <= 0 {
		fps = DefaultVideoFPS
	}

	seconds := int64(req.Duration)
	if seconds <= 0 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(req.Seconds)); err == nil {
			seconds = int64(parsed)
		}
	}
	if seconds <= 0 {
		seconds = metadataInt64(req.Metadata, "durationSeconds", "duration", "seconds")
	}
	if seconds <= 0 {
		seconds = DefaultVideoSeconds
	}

	spec := VideoBillingSpec{
		Resolution: tier,
		Width:      min(width, MaxVideoDimension),
		Height:     min(height, MaxVideoDimension),
		FPS:        min(fps, MaxVideoFPS),
		Seconds:    min(seconds, MaxTaskDurationSeconds),
	}
	spec.EstimatedTokens = EstimateVideoTokens(spec.Width, spec.Height, spec.FPS, spec.Seconds)
	return spec
}

// EstimateVideoTokens 按 宽 × 高 × 帧率 × 时长 / 1024 估算视频 token 数。
// 任一分量为非正数时返回 0；输入会被钳制到计费上限内，结果饱和到 int32 上限。
func EstimateVideoTokens(width, height, fps, seconds int64) int64 {
	if width <= 0 || height <= 0 || fps <= 0 || seconds <= 0 {
		return 0
	}
	width = min(width, MaxVideoDimension)
	height = min(height, MaxVideoDimension)
	fps = min(fps, MaxVideoFPS)
	seconds = min(seconds, MaxTaskDurationSeconds)

	tokens := (width*height*fps*seconds + videoTokenDivisor - 1) / videoTokenDivisor
	if tokens <= 0 {
		return 0
	}
	if tokens > maxVideoTokenEstimate {
		common.SysError(fmt.Sprintf("video token estimate saturated: width=%d height=%d fps=%d seconds=%d tokens=%d", width, height, fps, seconds, tokens))
		return maxVideoTokenEstimate
	}
	return tokens
}

// parseVideoSize 解析 "1920x1080" / "1080*1920" / "1920×1080" 形式的尺寸。
func parseVideoSize(size string) (int64, int64, bool) {
	s := strings.ToLower(strings.TrimSpace(size))
	if s == "" {
		return 0, 0, false
	}
	s = strings.NewReplacer("×", "x", "*", "x").Replace(s)
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

// videoResolutionDimensions 返回分辨率档位对应的标准像素，未知档位返回 0。
func videoResolutionDimensions(tier string) (int64, int64) {
	switch tier {
	case billing_setting.DurationPrice720p:
		return 1280, 720
	case billing_setting.DurationPrice1080p:
		return 1920, 1080
	case billing_setting.DurationPrice4k:
		return 3840, 2160
	default:
		return 0, 0
	}
}

// videoResolutionFromDimensions 按最长边反推分辨率档位，无法归类时返回空串。
func videoResolutionFromDimensions(width, height int64) string {
	longest := max(width, height)
	switch {
	case longest >= 3840:
		return billing_setting.DurationPrice4k
	case longest >= 1920:
		return billing_setting.DurationPrice1080p
	case longest >= 1280:
		return billing_setting.DurationPrice720p
	default:
		return ""
	}
}

func metadataString(metadata map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func metadataInt64(metadata map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int64(typed)
		case float32:
			return int64(typed)
		case int:
			return int64(typed)
		case int64:
			return typed
		case string:
			if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}
