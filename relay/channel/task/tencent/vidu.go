package tencent

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// viduFamily 实现 Vidu 视频的 WAND 协议。
type viduFamily struct{}

func (f *viduFamily) defaultDuration() int {
	return 5
}

func (f *viduFamily) buildSubmitURL(req relaycommon.TaskSubmitReq, _ *relaycommon.RelayInfo) string {
	switch len(req.Images) {
	case 0:
		return viduTextToVideoEndpoint
	case 1:
		return viduImageToVideoEndpoint
	case 2:
		return viduStartEndToVideoEndpoint
	default:
		return viduReferenceToVideoEndpoint
	}
}

func (f *viduFamily) queryEndpoint(_ string) string {
	return viduQueryEndpoint
}

// ---------------------------------------------------------------------------
// 提交请求
// ---------------------------------------------------------------------------

type viduTextToVideoRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Duration    *int   `json:"duration,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
}

type viduImagesToVideoRequest struct {
	Model      string   `json:"model"`
	Images     []string `json:"images"`
	Prompt     string   `json:"prompt,omitempty"`
	Duration   *int     `json:"duration,omitempty"`
	Resolution string   `json:"resolution,omitempty"`
}

func (f *viduFamily) buildSubmitBody(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (any, error) {
	duration := resolveDurationSeconds(req, f.defaultDuration())
	if len(req.Images) == 0 {
		return &viduTextToVideoRequest{
			Model:       info.UpstreamModelName,
			Prompt:      req.Prompt,
			Duration:    &duration,
			AspectRatio: aspectRatioFromRequest(req, "16:9"),
			Resolution:  viduResolution(req),
		}, nil
	}
	// 图生 / 首尾帧 / 多图参考均使用 images 数组传参。
	return &viduImagesToVideoRequest{
		Model:      info.UpstreamModelName,
		Images:     req.Images,
		Prompt:     req.Prompt,
		Duration:   &duration,
		Resolution: viduResolution(req),
	}, nil
}

func viduResolution(req relaycommon.TaskSubmitReq) string {
	if r := metadataString(req.Metadata, "resolution"); r != "" {
		switch strings.ToLower(r) {
		case quality540p, quality720p, quality1080p:
			return strings.ToLower(r)
		}
	}
	size := strings.ToLower(req.Size)
	switch {
	case strings.Contains(size, "1080"), strings.Contains(size, "1920"):
		return quality1080p
	case strings.Contains(size, "540"):
		return quality540p
	case strings.Contains(size, "720"), strings.Contains(size, "1280"):
		return quality720p
	}
	return quality720p
}

// ---------------------------------------------------------------------------
// 提交响应
// ---------------------------------------------------------------------------

type viduSubmitResponse struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
}

func (f *viduFamily) parseSubmit(respBody []byte, _ string) (string, error) {
	var r viduSubmitResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return "", err
	}
	if r.TaskID == "" {
		return "", fmt.Errorf("task_id is empty")
	}
	return r.TaskID, nil
}

// ---------------------------------------------------------------------------
// 查询响应
// ---------------------------------------------------------------------------

type viduQueryResponse struct {
	TaskID    string `json:"task_id"`
	State     string `json:"state"`
	Creations []struct {
		URL string `json:"url"`
	} `json:"creations"`
	Usage *wandUsage `json:"tokenhub_usage"`
}

func (f *viduFamily) parseQuery(respBody []byte) (*relaycommon.TaskInfo, error) {
	var r viduQueryResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	taskResult := &relaycommon.TaskInfo{Code: 0}
	if r.Usage != nil {
		taskResult.TotalTokens = r.Usage.TotalTokens
	}
	switch r.State {
	case viduStateCreated, viduStateQueueing:
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "20%"
	case viduStateProcessing:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case viduStateSuccess:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if len(r.Creations) > 0 {
			taskResult.Url = r.Creations[0].URL
		}
	case viduStateFailed:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = "task failed"
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}
	return taskResult, nil
}
