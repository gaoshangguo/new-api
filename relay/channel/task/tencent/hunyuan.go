package tencent

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// hunyuanFamily 实现腾讯混元（Hy）视频的 WAND 协议。
type hunyuanFamily struct{}

func (f *hunyuanFamily) defaultDuration() int {
	return 5
}

func (f *hunyuanFamily) buildSubmitURL(_ relaycommon.TaskSubmitReq, _ *relaycommon.RelayInfo) string {
	return hunyuanGenerationEndpoint
}

func (f *hunyuanFamily) queryEndpoint(_ string) string {
	return hunyuanQueryEndpoint
}

// ---------------------------------------------------------------------------
// 提交请求
// ---------------------------------------------------------------------------

type hunyuanVideoRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	Image       string `json:"image,omitempty"`
	Duration    *int   `json:"duration,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
}

func (f *hunyuanFamily) buildSubmitBody(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (any, error) {
	payload := &hunyuanVideoRequest{
		Model:  info.UpstreamModelName,
		Prompt: req.Prompt,
	}
	if duration := resolveDurationSeconds(req, f.defaultDuration()); duration > 0 {
		payload.Duration = &duration
	}
	if len(req.Images) > 0 {
		if strings.HasPrefix(req.Images[0], "data:") {
			payload.Image = req.Images[0]
		} else {
			payload.ImageURL = req.Images[0]
		}
	} else {
		// 文生视频可指定画幅；图生视频画幅由输入图片决定。
		payload.AspectRatio = aspectRatioFromRequest(req, "16:9")
	}
	return payload, nil
}

// ---------------------------------------------------------------------------
// 提交响应
// ---------------------------------------------------------------------------

type hunyuanSubmitResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (f *hunyuanFamily) parseSubmit(respBody []byte, _ string) (string, error) {
	var r hunyuanSubmitResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return "", err
	}
	if r.TaskID == "" {
		if r.Error != nil && r.Error.Message != "" {
			return "", fmt.Errorf("hunyuan api error: %s", r.Error.Message)
		}
		return "", fmt.Errorf("task_id is empty")
	}
	return r.TaskID, nil
}

// ---------------------------------------------------------------------------
// 查询响应
// ---------------------------------------------------------------------------

type hunyuanQueryResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Videos []struct {
		URL string `json:"url"`
	} `json:"videos"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Usage *wandUsage `json:"tokenhub_usage"`
}

func (f *hunyuanFamily) parseQuery(respBody []byte) (*relaycommon.TaskInfo, error) {
	var r hunyuanQueryResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	taskResult := &relaycommon.TaskInfo{Code: 0}
	if r.Usage != nil {
		taskResult.TotalTokens = r.Usage.TotalTokens
	}
	switch r.Status {
	case hunyuanStatusQueued:
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "20%"
	case hunyuanStatusRunning:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case hunyuanStatusSucceeded:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if len(r.Videos) > 0 {
			taskResult.Url = r.Videos[0].URL
		}
	case hunyuanStatusFailed:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if r.Error != nil && r.Error.Message != "" {
			taskResult.Reason = r.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}
	return taskResult, nil
}
