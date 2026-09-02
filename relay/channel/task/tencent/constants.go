package tencent

// TokenHub WAND 视频协议常量与模型列表。
// 参考 https://cloud.tencent.com/document/product/1823/135737
const (
	ChannelName = "tokenhub-wand"

	tokenHubBaseURL = "https://tokenhub.tencentmaas.com"

	v1GenerationEndpoint = "/v1/wand/minimax-video/generation"
	v1QueryEndpoint      = "/v1/wand/minimax-video/tasks"
	v2GenerationEndpoint = "/v1/wand/minimax-video-v2/generation"
	v2QueryEndpoint      = "/v1/wand/minimax-video-v2/tasks"

	// 分辨率档位（WAND 枚举）。
	resolution480P  = "480P"
	resolution768P  = "768P"
	resolution1080P = "1080P"
	resolution2K    = "2K"
)

// V1（minimax-video-v2.3 / v2.3-fast）查询任务状态。
const (
	v1StatusPreparing  = "Preparing"
	v1StatusQueueing   = "Queueing"
	v1StatusProcessing = "Processing"
	v1StatusSuccess    = "Success"
	v1StatusFail       = "Fail"
)

// V2（minimax-video-h3 / h3-max）查询任务状态。
const (
	v2StatusQueued    = "queued"
	v2StatusRunning   = "running"
	v2StatusSucceeded = "succeeded"
	v2StatusFailed    = "failed"
	v2StatusCancelled = "cancelled"
)

// ModelList 为 TokenHub 上可用的 MiniMax 视频模型。
var ModelList = []string{
	"minimax-video-h3",
	"minimax-video-h3-max",
	"minimax-video-v2.3",
	"minimax-video-v2.3-fast",
}
