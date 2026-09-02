package tencent

// TokenHub WAND 视频协议常量与模型列表。
// 参考 https://cloud.tencent.com/document/product/1823/135738
const (
	ChannelName     = "tokenhub-wand"
	tokenHubBaseURL = "https://tokenhub.tencentmaas.com"
)

// WAND 提交端点（按模型族与能力）。
const (
	// MiniMax：V1（扁平参数）适用于 minimax-video-v2.3/-fast；V2（content 数组）适用于 -h3/-max。
	minimaxV1GenerationEndpoint = "/v1/wand/minimax-video/generation"
	minimaxV1QueryEndpoint      = "/v1/wand/minimax-video/tasks"
	minimaxV2GenerationEndpoint = "/v1/wand/minimax-video-v2/generation"
	minimaxV2QueryEndpoint      = "/v1/wand/minimax-video-v2/tasks"

	pixverseTextToVideoEndpoint      = "/v1/wand/pixverse/text-to-video"
	pixverseImageToVideoEndpoint     = "/v1/wand/pixverse/image-to-video"
	pixverseStartEndToVideoEndpoint  = "/v1/wand/pixverse/start-end-to-video"
	pixverseReferenceToVideoEndpoint = "/v1/wand/pixverse/reference-to-video"
	pixverseQueryEndpoint            = "/v1/wand/pixverse/tasks"

	klingTextToVideoEndpoint  = "/v1/wand/kling/text-to-video"
	klingImageToVideoEndpoint = "/v1/wand/kling/image-to-video"
	klingOmniVideoEndpoint    = "/v1/wand/kling/omni-video"
	klingQueryEndpoint        = "/v1/wand/kling/tasks"

	viduTextToVideoEndpoint      = "/v1/wand/vidu/text-to-video"
	viduImageToVideoEndpoint     = "/v1/wand/vidu/image-to-video"
	viduReferenceToVideoEndpoint = "/v1/wand/vidu/reference-to-video"
	viduStartEndToVideoEndpoint  = "/v1/wand/vidu/start-end-to-video"
	viduQueryEndpoint            = "/v1/wand/vidu/tasks"

	hunyuanGenerationEndpoint = "/v1/wand/hunyuan-video/generation"
	hunyuanQueryEndpoint      = "/v1/wand/hunyuan-video/tasks"
)

// MiniMax 分辨率档位（WAND 枚举，大写下标 P）。
const (
	resolution480P  = "480P"
	resolution768P  = "768P"
	resolution1080P = "1080P"
	resolution2K    = "2K"
)

// 其它模型族的清晰度/分辨率枚举（小写）。
const (
	quality360p  = "360p"
	quality540p  = "540p"
	quality720p  = "720p"
	quality1080p = "1080p"
	quality4k    = "4k"
)

// MiniMax V1 查询任务状态。
const (
	v1StatusPreparing  = "Preparing"
	v1StatusQueueing   = "Queueing"
	v1StatusProcessing = "Processing"
	v1StatusSuccess    = "Success"
	v1StatusFail       = "Fail"
)

// MiniMax V2 查询任务状态。
const (
	v2StatusQueued    = "queued"
	v2StatusRunning   = "running"
	v2StatusSucceeded = "succeeded"
	v2StatusFailed    = "failed"
	v2StatusCancelled = "cancelled"
)

// Kling 查询任务状态。
const (
	klingStatusProcessing = "processing"
	klingStatusSucceeded  = "succeeded"
	klingStatusFailed     = "failed"
)

// Vidu 查询任务状态。
const (
	viduStateCreated    = "created"
	viduStateQueueing   = "queueing"
	viduStateProcessing = "processing"
	viduStateSuccess    = "success"
	viduStateFailed     = "failed"
)

// Hy（混元）查询任务状态。
const (
	hunyuanStatusQueued    = "queued"
	hunyuanStatusRunning   = "running"
	hunyuanStatusSucceeded = "succeeded"
	hunyuanStatusFailed    = "failed"
)

// PixVerse 查询任务状态（数字码）。
const (
	pixverseStatusSuccess    = 1
	pixverseStatusGenerating = 5
	pixverseStatusDeleted    = 6
	pixverseStatusAuditFail  = 7
	pixverseStatusFailed     = 8
)

// ModelList 为 TokenHub WAND 网关可用的全部视频模型。
var ModelList = []string{
	"minimax-video-h3",
	"minimax-video-h3-max",
	"minimax-video-v2.3",
	"minimax-video-v2.3-fast",
	"pixverse-video-v6.0",
	"pixverse-video-c1",
	"pixverse-video-v5.6",
	"kling-video-v3",
	"kling-video-v3-omni",
	"kling-video-v3-turbo",
	"kling-video-o1",
	"kling-video-v2.6",
	"kling-video-v2.5-turbo",
	"vidu-video-q3-turbo",
	"vidu-video-q3-pro",
	"vidu-video-q3",
	"vidu-video-q2",
	"vidu-video-q2-pro",
	"vidu-video-q2-turbo",
	"hy-video-v1.5",
}
