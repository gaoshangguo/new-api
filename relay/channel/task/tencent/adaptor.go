package tencent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
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
// 覆盖的模型族（端点/请求/响应各不相同，见各 family 文件）：
//   - MiniMax（minimax-video-*）：V1 扁平参数 / V2 content 数组
//   - PixVerse（pixverse-video-*）
//   - Kling（kling-video-*）
//   - Vidu（vidu-video-*）
//   - Hy 混元（hy-video-*）
//
// 鉴权为 Authorization: Bearer <TokenHub API Key>。
// 参考 https://cloud.tencent.com/document/product/1823/135738
type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
	// req 由 ValidateRequestAndSetAction 缓存，供 BuildRequestURL 选择提交端点
	//（不同能力对应不同子端点，BuildRequestURL 拿不到 gin context）。
	req *relaycommon.TaskSubmitReq
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
	if taskErr = relaycommon.ValidateMultipartDirect(c, info); taskErr != nil {
		return taskErr
	}
	if req, err := relaycommon.GetTaskRequest(c); err == nil {
		a.req = &req
	}
	return nil
}

// EstimateBilling 返回时长（秒）作为计费倍率；按时长计费模型叠加分辨率倍率。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	family := familyForModel(info.UpstreamModelName)
	fallbackSeconds := 6
	if family != nil {
		fallbackSeconds = family.defaultDuration()
	}
	ratios := map[string]float64{
		"seconds": float64(resolveDurationSeconds(req, fallbackSeconds)),
	}
	if billing_setting.GetBillingMode(info.OriginModelName) == billing_setting.BillingModePerDuration {
		size := req.Size
		if size == "" {
			size = "720x1280"
		}
		if ratio, ok := billing_setting.GetBillingDurationResolutionRatio(info.OriginModelName, size); ok && ratio != 1.0 {
			ratios["resolution"] = ratio
		}
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	family := familyForModel(info.UpstreamModelName)
	if family == nil {
		return "", fmt.Errorf("unsupported wand model: %s", info.UpstreamModelName)
	}
	req := relaycommon.TaskSubmitReq{}
	if a.req != nil {
		req = *a.req
	}
	return a.baseURL + family.buildSubmitURL(req, info), nil
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

	family := familyForModel(info.UpstreamModelName)
	if family == nil {
		return nil, fmt.Errorf("unsupported wand model: %s", info.UpstreamModelName)
	}
	payload, err := family.buildSubmitBody(req, info)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}

	data, err := common.Marshal(payload)
	if err != nil {
		return nil, err
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

	family := familyForModel(info.UpstreamModelName)
	if family == nil {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("unsupported wand model: %s", info.UpstreamModelName),
			"invalid_model",
			http.StatusBadRequest,
		)
		return
	}

	upstreamID, err := family.parseSubmit(responseBody, info.UpstreamModelName)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "submit_failed", http.StatusBadRequest)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return upstreamID, responseBody, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// FetchTask 按模型族选择查询端点。
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	model, _ := body["model"].(string)

	family := familyForModel(model)
	if family == nil {
		return nil, fmt.Errorf("unsupported wand model: %s", model)
	}

	uri := fmt.Sprintf("%s%s/%s", baseUrl, family.queryEndpoint(model), taskID)
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

// ParseTaskResult 通过响应结构识别模型族后委托对应解析器。
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	family := familyForResponse(respBody)
	if family == nil {
		return nil, errors.New(fmt.Sprintf("unmarshal wand task result failed: %s", string(respBody)))
	}
	return family.parseQuery(respBody)
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
