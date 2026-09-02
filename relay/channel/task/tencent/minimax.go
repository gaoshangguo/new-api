package tencent

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/pkg/errors"
)

// minimaxFamily 实现 MiniMax 视频的 WAND 协议。
// V1（扁平参数 /v1/wand/minimax-video/…）：minimax-video-v2.3、minimax-video-v2.3-fast；
// V2（content 数组 /v1/wand/minimax-video-v2/…）：minimax-video-h3、minimax-video-h3-max。
type minimaxFamily struct{}

func (f *minimaxFamily) defaultDuration() int {
	return 6
}

func (f *minimaxFamily) buildSubmitURL(_ relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) string {
	if isV2Model(info.UpstreamModelName) {
		return minimaxV2GenerationEndpoint
	}
	return minimaxV1GenerationEndpoint
}

func (f *minimaxFamily) queryEndpoint(model string) string {
	if isV2Model(model) {
		return minimaxV2QueryEndpoint
	}
	return minimaxV1QueryEndpoint
}

func isV2Model(model string) bool {
	return strings.HasPrefix(model, "minimax-video-h3")
}

// ---------------------------------------------------------------------------
// 提交请求
// ---------------------------------------------------------------------------

type minimaxV1Request struct {
	Model           string `json:"model"`
	Prompt          string `json:"prompt"`
	FirstFrameImage string `json:"first_frame_image,omitempty"`
	Duration        *int   `json:"duration,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
}

type minimaxMediaURL struct {
	URL string `json:"url"`
}

type minimaxContentItem struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	ImageURL *minimaxMediaURL `json:"image_url,omitempty"`
	Role     string           `json:"role,omitempty"`
}

type minimaxV2Request struct {
	Model      string               `json:"model"`
	Content    []minimaxContentItem `json:"content"`
	Resolution string               `json:"resolution"`
	Duration   *int                 `json:"duration"`
	Ratio      string               `json:"ratio,omitempty"`
}

func (f *minimaxFamily) buildSubmitBody(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (any, error) {
	if isV2Model(info.UpstreamModelName) {
		return f.buildV2Payload(&req, info), nil
	}
	return f.buildV1Payload(&req, info), nil
}

func (f *minimaxFamily) buildV1Payload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) *minimaxV1Request {
	payload := &minimaxV1Request{
		Model:      info.UpstreamModelName,
		Prompt:     req.Prompt,
		Resolution: f.resolveResolution(*req, info),
	}
	if duration := resolveDurationSeconds(*req, f.defaultDuration()); duration > 0 {
		payload.Duration = &duration
	}
	if len(req.Images) > 0 {
		payload.FirstFrameImage = req.Images[0]
	}
	return payload
}

func (f *minimaxFamily) buildV2Payload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) *minimaxV2Request {
	content := make([]minimaxContentItem, 0, 1+len(req.Images))
	content = append(content, minimaxContentItem{Type: "text", Text: req.Prompt})
	for i, image := range req.Images {
		role := "first_frame"
		if i > 0 {
			role = "last_frame"
		}
		content = append(content, minimaxContentItem{
			Type:     "image_url",
			ImageURL: &minimaxMediaURL{URL: image},
			Role:     role,
		})
	}

	payload := &minimaxV2Request{
		Model:      info.UpstreamModelName,
		Content:    content,
		Resolution: f.resolveResolution(*req, info),
	}
	if duration := resolveDurationSeconds(*req, f.defaultDuration()); duration > 0 {
		payload.Duration = &duration
	}
	// 文生视频 ratio 必填（不能为 adaptive）；图生视频默认 adaptive，可不传。
	if len(req.Images) == 0 {
		payload.Ratio = aspectRatioFromRequest(*req, "16:9")
	} else if ratio := aspectRatioFromRequest(*req, ""); ratio != "" {
		payload.Ratio = ratio
	}
	return payload
}

// resolveResolution 解析 MiniMax 分辨率档位（768P / 2K / 480P）。
func (f *minimaxFamily) resolveResolution(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) string {
	if v, ok := req.Metadata["resolution"].(string); ok {
		if r := normalizeWandResolution(v, info.UpstreamModelName); r != "" {
			return r
		}
	}
	size := strings.ToLower(req.Size)
	switch {
	case strings.Contains(size, "2k"), strings.Contains(size, "1440"):
		if isV2Model(info.UpstreamModelName) {
			return resolution2K
		}
		return resolution768P
	case strings.Contains(size, "1080"), strings.Contains(size, "1920"):
		if isV2Model(info.UpstreamModelName) {
			return resolution768P
		}
		return resolution1080P
	case strings.Contains(size, "768"), strings.Contains(size, "720"), strings.Contains(size, "1280"):
		return resolution768P
	case strings.Contains(size, "480"):
		return resolution480P
	}
	return resolution768P
}

func normalizeWandResolution(raw, model string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(s, "2k"), strings.Contains(s, "1440"):
		if isV2Model(model) {
			return resolution2K
		}
		return resolution768P
	case strings.Contains(s, "1080"), strings.Contains(s, "1920"):
		if isV2Model(model) {
			return resolution768P
		}
		return resolution1080P
	case strings.Contains(s, "768"), strings.Contains(s, "720"), strings.Contains(s, "1280"):
		return resolution768P
	case strings.Contains(s, "480"):
		return resolution480P
	}
	return ""
}

// ---------------------------------------------------------------------------
// 提交响应
// ---------------------------------------------------------------------------

type minimaxSubmitBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type minimaxSubmitResponse struct {
	TaskID    string                 `json:"task_id"`
	RequestID string                 `json:"request_id"`
	BaseResp  *minimaxSubmitBaseResp `json:"base_resp,omitempty"`
}

func (f *minimaxFamily) parseSubmit(respBody []byte, model string) (string, error) {
	var sResp minimaxSubmitResponse
	if err := common.Unmarshal(respBody, &sResp); err != nil {
		return "", err
	}
	// V1 提交响应带 base_resp；V2 不带，以 task_id 是否返回为准。
	if sResp.BaseResp != nil && sResp.BaseResp.StatusCode != 0 {
		return "", fmt.Errorf("minimax wand api error: %s", sResp.BaseResp.StatusMsg)
	}
	if sResp.TaskID == "" {
		return "", fmt.Errorf("task_id is empty")
	}
	return sResp.TaskID, nil
}

// ---------------------------------------------------------------------------
// 查询响应
// ---------------------------------------------------------------------------

// minimaxV1QueryResponse：状态在顶层，结果在 content.url。
type minimaxV1QueryResponse struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Content *struct {
		URL string `json:"url"`
	} `json:"content"`
	BaseResp  minimaxSubmitBaseResp `json:"base_resp"`
	Usage     *wandUsage            `json:"tokenhub_usage"`
	RequestID string                `json:"request_id"`
}

// minimaxV2QueryResponse：状态嵌套在 task.status，结果在 task.content.url。
type minimaxV2QueryResponse struct {
	Task struct {
		ID         string `json:"id"`
		Model      string `json:"model"`
		TaskType   string `json:"task_type"`
		Status     string `json:"status"`
		CreatedAt  int64  `json:"created_at"`
		UpdatedAt  int64  `json:"updated_at"`
		Resolution string `json:"resolution"`
		Duration   int    `json:"duration"`
		Ratio      string `json:"ratio"`
		Content    *struct {
			URL string `json:"url"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"task"`
	Usage     *wandUsage `json:"tokenhub_usage"`
	RequestID string     `json:"request_id"`
}

func (f *minimaxFamily) parseQuery(respBody []byte) (*relaycommon.TaskInfo, error) {
	var v2Resp minimaxV2QueryResponse
	if err := common.Unmarshal(respBody, &v2Resp); err == nil && v2Resp.Task.Status != "" {
		return parseMinimaxV2Result(&v2Resp), nil
	}

	var v1Resp minimaxV1QueryResponse
	if err := common.Unmarshal(respBody, &v1Resp); err == nil {
		return parseMinimaxV1Result(&v1Resp), nil
	}

	return nil, errors.New(fmt.Sprintf("unmarshal minimax wand task result failed: %s", string(respBody)))
}

func parseMinimaxV1Result(resTask *minimaxV1QueryResponse) *relaycommon.TaskInfo {
	taskResult := &relaycommon.TaskInfo{Code: 0}
	if resTask.Usage != nil {
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	}
	switch resTask.Status {
	case v1StatusPreparing, v1StatusQueueing, v1StatusProcessing:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case v1StatusSuccess:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if resTask.Content != nil {
			taskResult.Url = resTask.Content.URL
		}
	case v1StatusFail:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.BaseResp.StatusMsg
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}
	return taskResult
}

func parseMinimaxV2Result(resTask *minimaxV2QueryResponse) *relaycommon.TaskInfo {
	taskResult := &relaycommon.TaskInfo{Code: 0, TaskID: resTask.Task.ID}
	if resTask.Usage != nil {
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	}
	switch resTask.Task.Status {
	case v2StatusQueued:
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "20%"
	case v2StatusRunning:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case v2StatusSucceeded:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if resTask.Task.Content != nil {
			taskResult.Url = resTask.Task.Content.URL
		}
	case v2StatusFailed, v2StatusCancelled:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if resTask.Task.Error != nil && resTask.Task.Error.Message != "" {
			taskResult.Reason = resTask.Task.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}
	return taskResult
}
