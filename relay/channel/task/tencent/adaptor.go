package tencent

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// TaskAdaptor 实现腾讯云 TokenHub 的 WAND 视频生成协议。
// MiniMax 系列在 TokenHub 上有两套端点（提交与查询必须用同一套）：
//   - V1（扁平参数 /v1/wand/minimax-video/…）：minimax-video-v2.3、minimax-video-v2.3-fast
//   - V2（content 数组 /v1/wand/minimax-video-v2/…）：minimax-video-h3、minimax-video-h3-max
//
// 鉴权为 Authorization: Bearer <TokenHub API Key>。
type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	if a.baseURL == "" || a.baseURL == constant.ChannelBaseURLs[constant.ChannelTypeTencent] {
		// TokenHub key 走 OpenAI 兼容网关，默认 base URL 指向 TokenHub。
		a.baseURL = tokenHubBaseURL
	}
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *taskdto.TaskError) {
	return relaycommon.ValidateMultipartDirect(c, info)
}

// EstimateBilling 返回时长（秒）作为计费倍率；按时长计费模型叠加分辨率倍率。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	ratios := map[string]float64{
		"seconds": float64(a.resolveDuration(req, info)),
	}
	if billing_setting.GetBillingMode(info.OriginModelName) == billing_setting.BillingModePerDuration {
		if ratio, ok := billing_setting.GetBillingDurationResolutionRatio(
			info.OriginModelName, a.resolveResolution(req, info),
		); ok && ratio != 1.0 {
			ratios["resolution"] = ratio
		}
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if isV2Model(info.UpstreamModelName) {
		return a.baseURL + v2GenerationEndpoint, nil
	}
	return a.baseURL + v1GenerationEndpoint, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	var payload any
	if isV2Model(info.UpstreamModelName) {
		payload = a.buildV2Payload(&req, info)
	} else {
		payload = a.buildV1Payload(&req, info)
	}

	data, err := common.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "marshal wand request payload failed")
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var sResp submitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	// V1 提交响应带 base_resp；V2 不带，以 task_id 是否返回为准。
	if sResp.BaseResp != nil && sResp.BaseResp.StatusCode != 0 {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("tokenhub wand api error: %s", sResp.BaseResp.StatusMsg),
			strconv.Itoa(sResp.BaseResp.StatusCode),
			http.StatusBadRequest,
		)
		return
	}
	if sResp.TaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return sResp.TaskID, responseBody, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// FetchTask 按模型族选择 V1/V2 查询端点。
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	model, _ := body["model"].(string)

	queryPath := v2QueryEndpoint
	if !isV2Model(model) {
		queryPath = v1QueryEndpoint
	}

	uri := fmt.Sprintf("%s%s/%s", baseUrl, queryPath, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

// ParseTaskResult 通过响应结构区分 V1（顶层 status）与 V2（task.status 嵌套）。
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var v2Resp v2QueryResponse
	if err := common.Unmarshal(respBody, &v2Resp); err == nil && v2Resp.Task.Status != "" {
		return a.parseV2Result(&v2Resp), nil
	}

	var v1Resp v1QueryResponse
	if err := common.Unmarshal(respBody, &v1Resp); err == nil {
		return a.parseV1Result(&v1Resp), nil
	}

	return nil, errors.New(fmt.Sprintf("unmarshal wand task result failed: %s", string(respBody)))
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	openAIVideo := originTask.ToOpenAIVideo()
	if originTask.Status == model.TaskStatusFailure {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: originTask.FailReason,
			Code:    "1",
		}
	}
	jsonData, err := common.Marshal(openAIVideo)
	if err != nil {
		return nil, errors.Wrap(err, "marshal openai video failed")
	}
	return jsonData, nil
}

// ---------------------------------------------------------------------------
// 请求载荷
// ---------------------------------------------------------------------------

type v1Request struct {
	Model           string `json:"model"`
	Prompt          string `json:"prompt"`
	FirstFrameImage string `json:"first_frame_image,omitempty"`
	Duration        *int   `json:"duration,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
}

type wandMediaURL struct {
	URL string `json:"url"`
}

type wandContentItem struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *wandMediaURL `json:"image_url,omitempty"`
	Role     string        `json:"role,omitempty"`
}

type v2Request struct {
	Model      string            `json:"model"`
	Content    []wandContentItem `json:"content"`
	Resolution string            `json:"resolution"`
	Duration   *int              `json:"duration"`
	Ratio      string            `json:"ratio,omitempty"`
}

func (a *TaskAdaptor) buildV1Payload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) *v1Request {
	payload := &v1Request{
		Model:      info.UpstreamModelName,
		Prompt:     req.Prompt,
		Resolution: a.resolveResolution(*req, info),
	}
	if duration := a.resolveDuration(*req, info); duration > 0 {
		payload.Duration = &duration
	}
	if len(req.Images) > 0 {
		payload.FirstFrameImage = req.Images[0]
	}
	return payload
}

func (a *TaskAdaptor) buildV2Payload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) *v2Request {
	content := make([]wandContentItem, 0, 1+len(req.Images))
	content = append(content, wandContentItem{Type: "text", Text: req.Prompt})
	for i, image := range req.Images {
		role := "first_frame"
		if i > 0 {
			role = "last_frame"
		}
		content = append(content, wandContentItem{
			Type:     "image_url",
			ImageURL: &wandMediaURL{URL: image},
			Role:     role,
		})
	}

	payload := &v2Request{
		Model:      info.UpstreamModelName,
		Content:    content,
		Resolution: a.resolveResolution(*req, info),
	}
	if duration := a.resolveDuration(*req, info); duration > 0 {
		payload.Duration = &duration
	}
	// 文生视频 ratio 必填（不能为 adaptive）；图生视频默认 adaptive，可不传。
	if len(req.Images) == 0 {
		payload.Ratio = a.resolveRatio(*req, "16:9")
	} else if ratio := a.resolveRatio(*req, ""); ratio != "" {
		payload.Ratio = ratio
	}
	return payload
}

// ---------------------------------------------------------------------------
// 参数解析
// ---------------------------------------------------------------------------

func isV2Model(model string) bool {
	return strings.HasPrefix(model, "minimax-video-h3")
}

// resolveDuration 解析请求时长并钳制到系统上限；未指定时返回默认值。
func (a *TaskAdaptor) resolveDuration(req relaycommon.TaskSubmitReq, _ *relaycommon.RelayInfo) int {
	duration := req.Duration
	if duration <= 0 && req.Seconds != "" {
		duration, _ = strconv.Atoi(req.Seconds)
	}
	if duration <= 0 {
		return 6
	}
	if duration > relaycommon.MaxTaskDurationSeconds {
		duration = relaycommon.MaxTaskDurationSeconds
	}
	return duration
}

// resolveResolution 从 metadata 或 size 解析 WAND 分辨率档位。
// V1（v2.3/v2.3-fast）支持 768P/1080P；V2（h3/h3-max）支持 768P/2K，
// 且 h3 传入 1080P 会静默降级为 768P，因此按模型族归一化。
func (a *TaskAdaptor) resolveResolution(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) string {
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

// resolveRatio 从 metadata 或 size 宽高比推导 WAND 画幅；fallback 为文生视频默认值。
func (a *TaskAdaptor) resolveRatio(req relaycommon.TaskSubmitReq, fallback string) string {
	if v, ok := req.Metadata["ratio"].(string); ok {
		if r := normalizeWandRatio(v); r != "" {
			return r
		}
	}
	if r := ratioFromSize(req.Size); r != "" {
		return r
	}
	return fallback
}

func normalizeWandRatio(raw string) string {
	r := strings.ToLower(strings.TrimSpace(raw))
	switch r {
	case "21:9", "16:9", "4:3", "1:1", "3:4", "9:16":
		return r
	}
	return ""
}

// ratioFromSize 将 "WxH" 尺寸映射到最近的 WAND 画幅档位。
func ratioFromSize(size string) string {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return ""
	}
	width, errW := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	height, errH := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return ""
	}

	aspect := width / height
	best := "16:9"
	bestDiff := math.Abs(aspect - 16.0/9.0)
	for _, candidate := range []struct {
		ratio string
		value float64
	}{
		{"21:9", 21.0 / 9.0},
		{"16:9", 16.0 / 9.0},
		{"4:3", 4.0 / 3.0},
		{"1:1", 1.0},
		{"3:4", 3.0 / 4.0},
		{"9:16", 9.0 / 16.0},
	} {
		if diff := math.Abs(aspect - candidate.value); diff < bestDiff {
			best = candidate.ratio
			bestDiff = diff
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// 响应结构
// ---------------------------------------------------------------------------

type submitBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type submitResponse struct {
	TaskID    string          `json:"task_id"`
	RequestID string          `json:"request_id"`
	BaseResp  *submitBaseResp `json:"base_resp,omitempty"`
}

type wandUsage struct {
	TotalTokens int `json:"total_tokens"`
}

// V1 查询响应：状态在顶层，结果在 content.url。
type v1QueryResponse struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Content *struct {
		URL string `json:"url"`
	} `json:"content"`
	BaseResp  submitBaseResp `json:"base_resp"`
	Usage     *wandUsage     `json:"tokenhub_usage"`
	RequestID string         `json:"request_id"`
}

// V2 查询响应：状态嵌套在 task.status，结果在 task.content.url。
type v2QueryResponse struct {
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

func (a *TaskAdaptor) parseV1Result(resTask *v1QueryResponse) *relaycommon.TaskInfo {
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

func (a *TaskAdaptor) parseV2Result(resTask *v2QueryResponse) *relaycommon.TaskInfo {
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
