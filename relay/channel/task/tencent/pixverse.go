package tencent

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// pixverseFamily 实现 PixVerse 视频的 WAND 协议。
type pixverseFamily struct{}

func (f *pixverseFamily) defaultDuration() int {
	return 5
}

func (f *pixverseFamily) buildSubmitURL(req relaycommon.TaskSubmitReq, _ *relaycommon.RelayInfo) string {
	switch len(req.Images) {
	case 0:
		return pixverseTextToVideoEndpoint
	case 1:
		return pixverseImageToVideoEndpoint
	case 2:
		return pixverseStartEndToVideoEndpoint
	default:
		return pixverseReferenceToVideoEndpoint
	}
}

func (f *pixverseFamily) queryEndpoint(_ string) string {
	return pixverseQueryEndpoint
}

// ---------------------------------------------------------------------------
// 提交请求
// ---------------------------------------------------------------------------

type pixverseTextToVideoRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Duration    *int   `json:"duration"`
	Quality     string `json:"quality"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
}

type pixverseImageToVideoRequest struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	ImgID    string `json:"img_id"`
	Duration *int   `json:"duration"`
	Quality  string `json:"quality"`
}

type pixverseStartEndToVideoRequest struct {
	Model      string `json:"model"`
	Prompt     string `json:"prompt"`
	FirstFrame string `json:"first_frame_img"`
	LastFrame  string `json:"last_frame_img"`
	Duration   *int   `json:"duration"`
	Quality    string `json:"quality"`
}

type pixverseReference struct {
	ImgID string `json:"img_id"`
	Type  string `json:"type,omitempty"`
}

type pixverseReferenceToVideoRequest struct {
	Model       string              `json:"model"`
	Prompt      string              `json:"prompt"`
	References  []pixverseReference `json:"image_references"`
	Duration    *int                `json:"duration"`
	Quality     string              `json:"quality"`
	AspectRatio string              `json:"aspect_ratio,omitempty"`
}

func (f *pixverseFamily) buildSubmitBody(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (any, error) {
	duration := resolveDurationSeconds(req, f.defaultDuration())
	quality := pixverseQuality(req)
	switch len(req.Images) {
	case 0:
		return &pixverseTextToVideoRequest{
			Model:       info.UpstreamModelName,
			Prompt:      req.Prompt,
			Duration:    &duration,
			Quality:     quality,
			AspectRatio: aspectRatioFromRequest(req, "16:9"),
		}, nil
	case 1:
		return &pixverseImageToVideoRequest{
			Model:    info.UpstreamModelName,
			Prompt:   req.Prompt,
			ImgID:    req.Images[0],
			Duration: &duration,
			Quality:  quality,
		}, nil
	case 2:
		return &pixverseStartEndToVideoRequest{
			Model:      info.UpstreamModelName,
			Prompt:     req.Prompt,
			FirstFrame: req.Images[0],
			LastFrame:  req.Images[1],
			Duration:   &duration,
			Quality:    quality,
		}, nil
	default:
		refs := make([]pixverseReference, 0, len(req.Images))
		for _, img := range req.Images {
			refs = append(refs, pixverseReference{ImgID: img, Type: "subject"})
		}
		return &pixverseReferenceToVideoRequest{
			Model:       info.UpstreamModelName,
			Prompt:      req.Prompt,
			References:  refs,
			Duration:    &duration,
			Quality:     quality,
			AspectRatio: aspectRatioFromRequest(req, "16:9"),
		}, nil
	}
}

func pixverseQuality(req relaycommon.TaskSubmitReq) string {
	if r := metadataString(req.Metadata, "resolution"); r != "" {
		switch strings.ToLower(r) {
		case quality360p, quality540p, quality720p, quality1080p:
			return strings.ToLower(r)
		}
	}
	size := strings.ToLower(req.Size)
	switch {
	case strings.Contains(size, "1080"), strings.Contains(size, "1920"):
		return quality1080p
	case strings.Contains(size, "540"):
		return quality540p
	case strings.Contains(size, "360"):
		return quality360p
	case strings.Contains(size, "720"), strings.Contains(size, "1280"):
		return quality720p
	}
	return quality720p
}

// ---------------------------------------------------------------------------
// 提交响应
// ---------------------------------------------------------------------------

type pixverseSubmitResponse struct {
	ErrCode int    `json:"ErrCode"`
	ErrMsg  string `json:"ErrMsg"`
	Resp    struct {
		VideoID string `json:"video_id"`
	} `json:"Resp"`
}

func (f *pixverseFamily) parseSubmit(respBody []byte, _ string) (string, error) {
	var r pixverseSubmitResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return "", err
	}
	if r.ErrCode != 0 {
		return "", fmt.Errorf("pixverse api error: %s", r.ErrMsg)
	}
	if r.Resp.VideoID == "" {
		return "", fmt.Errorf("video_id is empty")
	}
	return r.Resp.VideoID, nil
}

// ---------------------------------------------------------------------------
// 查询响应
// ---------------------------------------------------------------------------

type pixverseQueryResponse struct {
	ErrCode int    `json:"ErrCode"`
	ErrMsg  string `json:"ErrMsg"`
	Resp    struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
		URL    string `json:"url"`
	} `json:"Resp"`
	Usage *wandUsage `json:"tokenhub_usage"`
}

func (f *pixverseFamily) parseQuery(respBody []byte) (*relaycommon.TaskInfo, error) {
	var r pixverseQueryResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return nil, err
	}
	taskResult := &relaycommon.TaskInfo{Code: 0}
	if r.Usage != nil {
		taskResult.TotalTokens = r.Usage.TotalTokens
	}
	switch r.Resp.Status {
	case pixverseStatusSuccess:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = r.Resp.URL
	case pixverseStatusGenerating:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case pixverseStatusDeleted, pixverseStatusAuditFail, pixverseStatusFailed:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = r.ErrMsg
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}
	return taskResult, nil
}
