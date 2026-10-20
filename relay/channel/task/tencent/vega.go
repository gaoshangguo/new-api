package tencent

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// vegaFamily 实现 WAND-Vega 视频协议。
// 提交：POST /v1/wand/vega-videos/tasks（返回 201 + id/status=queued）
// 查询：GET  /v1/wand/vega-videos/tasks/{id}
// 文档：https://cloud.tencent.com/document/product/1823/138195
type vegaFamily struct{}

func (f *vegaFamily) defaultDuration() int {
	return 5
}

func (f *vegaFamily) buildSubmitURL(_ relaycommon.TaskSubmitReq, _ *relaycommon.RelayInfo) string {
	return vegaVideoSubmitEndpoint
}

func (f *vegaFamily) queryEndpoint(_ string) string {
	return vegaVideoQueryEndpoint
}

// ---------------------------------------------------------------------------
// 提交请求
// ---------------------------------------------------------------------------

type vegaMediaURL struct {
	URL string `json:"url"`
}

type vegaContentItem struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *vegaMediaURL `json:"image_url,omitempty"`
	Role     string        `json:"role,omitempty"`
}

type vegaVideoRequest struct {
	Model      string            `json:"model"`
	Content    []vegaContentItem `json:"content"`
	Resolution string            `json:"resolution,omitempty"`
	Ratio      string            `json:"ratio,omitempty"`
	Duration   *int              `json:"duration,omitempty"`
}

func (f *vegaFamily) buildSubmitBody(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (any, error) {
	duration := resolveDurationSeconds(req, f.defaultDuration())
	if duration < 4 || duration > 15 {
		return nil, fmt.Errorf("duration must be between 4 and 15 seconds")
	}

	content := make([]vegaContentItem, 0, 1+len(req.Images))
	content = append(content, vegaContentItem{Type: "text", Text: req.Prompt})
	content = append(content, buildVegaImageContent(req.Images)...)

	payload := &vegaVideoRequest{
		Model:      info.UpstreamModelName,
		Content:    content,
		Resolution: vegaResolution(req, info.UpstreamModelName),
		Duration:   &duration,
	}
	// 文生视频可指定画幅；首尾帧/参考图场景由素材决定画幅，仅在显式传参时发送。
	if len(req.Images) == 0 {
		payload.Ratio = aspectRatioFromRequest(req, "16:9")
	} else if ratio := aspectRatioFromRequest(req, ""); ratio != "" {
		payload.Ratio = ratio
	}
	return payload, nil
}

func buildVegaImageContent(images []string) []vegaContentItem {
	if len(images) == 0 {
		return nil
	}
	content := make([]vegaContentItem, 0, len(images))
	if len(images) > 2 {
		// 3 张及以上视为参考图；首帧/尾帧协议最多各 1 张。
		for _, image := range images {
			content = append(content, vegaContentItem{
				Type:     "image_url",
				ImageURL: &vegaMediaURL{URL: image},
				Role:     "reference_image",
			})
		}
		return content
	}
	roles := []string{"first_frame", "last_frame"}
	for i, image := range images {
		content = append(content, vegaContentItem{
			Type:     "image_url",
			ImageURL: &vegaMediaURL{URL: image},
			Role:     roles[i],
		})
	}
	return content
}

// vegaResolution 解析分辨率档位。Vega Pro 档（1.0-pro/1.5-pro）最低 1080P，
// 公开的 1.0-lite 才支持 768P。
func vegaResolution(req relaycommon.TaskSubmitReq, model string) string {
	fallback := "768P"
	if strings.Contains(model, "-pro") {
		fallback = "1080P"
	}

	raw := metadataString(req.Metadata, "resolution")
	if raw == "" {
		raw = req.Size
	}
	resolution := normalizeVegaResolution(raw)
	if resolution == "" {
		return fallback
	}
	if strings.Contains(model, "-pro") && resolution == "768P" {
		return "1080P"
	}
	return resolution
}

func normalizeVegaResolution(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(s, "4k"), strings.Contains(s, "2160"):
		return "4K"
	case strings.Contains(s, "2k"), strings.Contains(s, "1440"):
		return "2K"
	case strings.Contains(s, "1080"), strings.Contains(s, "1920"):
		return "1080P"
	case strings.Contains(s, "768"), strings.Contains(s, "720"), strings.Contains(s, "1280"):
		return "768P"
	}
	return ""
}

// ---------------------------------------------------------------------------
// 提交响应
// ---------------------------------------------------------------------------

type vegaSubmitResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Status string `json:"status"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (f *vegaFamily) parseSubmit(respBody []byte, _ string) (string, error) {
	var r vegaSubmitResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return "", err
	}
	if r.ID == "" {
		if r.Error != nil && r.Error.Message != "" {
			return "", fmt.Errorf("vega api error: %s", r.Error.Message)
		}
		return "", fmt.Errorf("task id is empty")
	}
	return r.ID, nil
}

// ---------------------------------------------------------------------------
// 查询响应
// ---------------------------------------------------------------------------

type vegaQueryResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content *struct {
		VideoURL     string `json:"video_url"`
		LastFrameURL string `json:"last_frame_url"`
	} `json:"content"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Usage *wandUsage `json:"tokenhub_usage"`
}

func (f *vegaFamily) parseQuery(respBody []byte) (*relaycommon.TaskInfo, error) {
	var r vegaQueryResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	taskResult := &relaycommon.TaskInfo{Code: 0, TaskID: r.ID}
	if r.Usage != nil {
		taskResult.TotalTokens = r.Usage.TotalTokens
	}
	switch r.Status {
	case vegaStatusQueued:
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "20%"
	case vegaStatusRunning:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case vegaStatusSucceeded:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if r.Content != nil {
			taskResult.Url = r.Content.VideoURL
		}
	case vegaStatusFailed:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if r.Error != nil && r.Error.Message != "" {
			taskResult.Reason = r.Error.Message
		} else if r.Error != nil && r.Error.Code != "" {
			taskResult.Reason = r.Error.Code
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}
	return taskResult, nil
}
