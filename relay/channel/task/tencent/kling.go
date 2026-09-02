package tencent

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// klingFamily 实现 Kling（可灵）视频的 WAND 协议。
type klingFamily struct{}

func (f *klingFamily) defaultDuration() int {
	return 5
}

func (f *klingFamily) buildSubmitURL(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) string {
	if isOmniModel(info.UpstreamModelName) {
		return klingOmniVideoEndpoint
	}
	if len(req.Images) > 0 {
		return klingImageToVideoEndpoint
	}
	return klingTextToVideoEndpoint
}

func (f *klingFamily) queryEndpoint(_ string) string {
	return klingQueryEndpoint
}

func isOmniModel(model string) bool {
	return model == "kling-video-v3-omni" || model == "kling-video-o1"
}

// ---------------------------------------------------------------------------
// 提交请求
// ---------------------------------------------------------------------------

type klingSettings struct {
	Resolution  string `json:"resolution,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	Duration    *int   `json:"duration,omitempty"`
}

type klingTextToVideoRequest struct {
	Model    string         `json:"model"`
	Prompt   string         `json:"prompt"`
	Settings *klingSettings `json:"settings,omitempty"`
}

type klingContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

type klingContentsToVideoRequest struct {
	Model    string             `json:"model"`
	Contents []klingContentItem `json:"contents"`
	Settings *klingSettings     `json:"settings,omitempty"`
}

func (f *klingFamily) buildSubmitBody(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (any, error) {
	duration := resolveDurationSeconds(req, f.defaultDuration())
	settings := &klingSettings{
		Resolution: klingResolution(req),
		Duration:   &duration,
	}
	if isOmniModel(info.UpstreamModelName) || len(req.Images) == 0 {
		settings.AspectRatio = aspectRatioFromRequest(req, "16:9")
	}

	if isOmniModel(info.UpstreamModelName) || len(req.Images) > 0 {
		return &klingContentsToVideoRequest{
			Model:    info.UpstreamModelName,
			Contents: buildKlingContents(req),
			Settings: settings,
		}, nil
	}
	return &klingTextToVideoRequest{
		Model:    info.UpstreamModelName,
		Prompt:   req.Prompt,
		Settings: settings,
	}, nil
}

func buildKlingContents(req relaycommon.TaskSubmitReq) []klingContentItem {
	contents := make([]klingContentItem, 0, 1+len(req.Images))
	contents = append(contents, klingContentItem{Type: "prompt", Text: req.Prompt})
	for i, image := range req.Images {
		contentType := "first_frame"
		if i > 0 {
			contentType = "last_frame"
		}
		contents = append(contents, klingContentItem{Type: contentType, URL: image})
	}
	return contents
}

func klingResolution(req relaycommon.TaskSubmitReq) string {
	if r := metadataString(req.Metadata, "resolution"); r != "" {
		switch strings.ToLower(r) {
		case quality720p, quality1080p, quality4k:
			return strings.ToLower(r)
		}
	}
	size := strings.ToLower(req.Size)
	switch {
	case strings.Contains(size, "4k"), strings.Contains(size, "3840"):
		return quality4k
	case strings.Contains(size, "1080"), strings.Contains(size, "1920"):
		return quality1080p
	case strings.Contains(size, "720"), strings.Contains(size, "1280"):
		return quality720p
	}
	return quality720p
}

// ---------------------------------------------------------------------------
// 提交响应
// ---------------------------------------------------------------------------

type klingSubmitResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (f *klingFamily) parseSubmit(respBody []byte, _ string) (string, error) {
	var r klingSubmitResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return "", err
	}
	if r.Code != 0 {
		return "", fmt.Errorf("kling api error: %s", r.Message)
	}
	if r.Data.ID == "" {
		return "", fmt.Errorf("task id is empty")
	}
	return r.Data.ID, nil
}

// ---------------------------------------------------------------------------
// 查询响应
// ---------------------------------------------------------------------------

type klingQueryResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Outputs []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			URL  string `json:"url"`
		} `json:"outputs"`
	} `json:"data"`
	Usage *wandUsage `json:"tokenhub_usage"`
}

func (f *klingFamily) parseQuery(respBody []byte) (*relaycommon.TaskInfo, error) {
	var r klingQueryResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	taskResult := &relaycommon.TaskInfo{Code: 0}
	if r.Usage != nil {
		taskResult.TotalTokens = r.Usage.TotalTokens
	}

	var status, reason string
	var outputs []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		URL  string `json:"url"`
	}
	if len(r.Data) > 0 {
		status = r.Data[0].Status
		reason = r.Data[0].Message
		outputs = r.Data[0].Outputs
	}

	switch status {
	case klingStatusProcessing:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case klingStatusSucceeded:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if len(outputs) > 0 {
			taskResult.Url = outputs[0].URL
		}
	case klingStatusFailed:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = reason
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}
	return taskResult, nil
}
