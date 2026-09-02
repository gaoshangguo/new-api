package tencent

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// wandFamily 定义 TokenHub WAND 各模型族的差异点（端点/请求/响应解析）。
type wandFamily interface {
	// defaultDuration 返回未指定时长时的默认值（秒）。
	defaultDuration() int
	// buildSubmitURL 返回提交端点完整路径（不含 base URL）。
	buildSubmitURL(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) string
	// buildSubmitBody 构建提交请求载荷。
	buildSubmitBody(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (any, error)
	// parseSubmit 从提交响应提取上游任务 ID。
	parseSubmit(respBody []byte, model string) (taskID string, err error)
	// queryEndpoint 返回查询路径前缀 "/v1/wand/<family>/tasks"（按模型可选变体）。
	queryEndpoint(model string) string
	// parseQuery 解析查询响应为任务状态。
	parseQuery(respBody []byte) (*relaycommon.TaskInfo, error)
}

// familyForModel 按上游模型名选择对应的 WAND 模型族。
func familyForModel(model string) wandFamily {
	switch {
	case strings.HasPrefix(model, "minimax-video"):
		return &minimaxFamily{}
	case strings.HasPrefix(model, "pixverse-video"):
		return &pixverseFamily{}
	case strings.HasPrefix(model, "kling-video"):
		return &klingFamily{}
	case strings.HasPrefix(model, "vidu-video"):
		return &viduFamily{}
	case strings.HasPrefix(model, "hy-video"):
		return &hunyuanFamily{}
	}
	return nil
}

// familyForResponse 按查询响应结构识别模型族。
// 各族的查询响应均有独有标记：MiniMax V2 嵌套 task、PixVerse 有 ErrCode、
// Kling 有 data 数组、Vidu 有 state、Hy 有 videos；Hy 与 MiniMax V1 的顶层
// status 枚举互斥（queued/running/succeeded/failed vs Preparing/Queueing/...），
// 依此区分，兜底视为 MiniMax V1。
func familyForResponse(respBody []byte) wandFamily {
	var probe map[string]any
	if err := common.Unmarshal(respBody, &probe); err != nil {
		return nil
	}
	if _, ok := probe["task"].(map[string]any); ok {
		return &minimaxFamily{}
	}
	if _, ok := probe["ErrCode"]; ok {
		return &pixverseFamily{}
	}
	if _, ok := probe["data"].([]any); ok {
		return &klingFamily{}
	}
	if state, ok := probe["state"].(string); ok && state != "" {
		return &viduFamily{}
	}
	if _, ok := probe["videos"].([]any); ok {
		return &hunyuanFamily{}
	}
	if _, ok := probe["content"].(map[string]any); ok {
		return &minimaxFamily{}
	}
	if _, ok := probe["base_resp"].(map[string]any); ok {
		return &minimaxFamily{}
	}
	if status, ok := probe["status"].(string); ok {
		switch status {
		case hunyuanStatusQueued, hunyuanStatusRunning, hunyuanStatusSucceeded, hunyuanStatusFailed:
			return &hunyuanFamily{}
		}
	}
	return &minimaxFamily{}
}
