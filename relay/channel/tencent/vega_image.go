package tencent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// WAND-Vega 生图为异步任务：提交得到 task_id，轮询到 completed 后取 data[].url。
// 文档：https://cloud.tencent.com/document/product/1823/137203
const (
	vegaImageSubmitPath = "/v1/wand/vega-images/generations"
	vegaImageQueryPath  = "/v1/wand/vega-images/tasks/"

	vegaImagePollInterval = 3 * time.Second
	vegaImagePollTimeout  = 5 * time.Minute
)

// IsVegaImageModel 判断是否为 WAND-Vega 生图模型。
func IsVegaImageModel(model string) bool {
	return strings.HasPrefix(model, "wand-vega-image")
}

// VegaImageAdaptor 把 OpenAI 风格 /v1/images/generations 请求转换为
// WAND-Vega 异步生图协议，并在请求内轮询到出图后返回 OpenAI 图片响应。
type VegaImageAdaptor struct {
	openai.Adaptor
}

func (a *VegaImageAdaptor) Init(info *relaycommon.RelayInfo) {
	a.Adaptor.Init(info)
}

func (a *VegaImageAdaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, vegaImageSubmitPath, info.ChannelType), nil
}

func (a *VegaImageAdaptor) ConvertImageRequest(_ *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if request.N != nil && *request.N > 1 {
		return nil, fmt.Errorf("vega image generation supports n=1 only")
	}
	payload := map[string]any{
		"model":  info.UpstreamModelName,
		"prompt": request.Prompt,
	}
	if request.Size != "" {
		payload["size"] = request.Size
	}
	// 参考图由调用方以 Vega 的 input 结构透传（OpenAI 生图接口没有该字段）。
	if raw, ok := request.Extra["input"]; ok && len(raw) > 0 {
		payload["input"] = raw
	}
	return payload, nil
}

func (a *VegaImageAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *VegaImageAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info.RelayMode != relayconstant.RelayModeImagesGenerations {
		return a.Adaptor.DoResponse(c, resp, info)
	}

	responseBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var submit vegaImageSubmitResponse
	if err := common.Unmarshal(responseBody, &submit); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if submit.TaskID == "" {
		return nil, types.NewOpenAIError(fmt.Errorf("vega image task_id is empty"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	urls, totalTokens, err := a.pollImageTask(c, info, submit.TaskID)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusBadGateway)
	}

	data := make([]map[string]string, 0, len(urls))
	for _, url := range urls {
		data = append(data, map[string]string{"url": url})
	}
	openAIResponse, err := common.Marshal(map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
		"usage": map[string]int{
			"prompt_tokens":     totalTokens,
			"completion_tokens": 0,
			"total_tokens":      totalTokens,
		},
	})
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}

	return openai.OpenaiImageHandler(c, info, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(openAIResponse)),
	})
}

type vegaImageSubmitResponse struct {
	TaskID string `json:"task_id"`
}

type vegaImageUsage struct {
	TotalTokens int `json:"total_tokens"`
}

type vegaImageQueryResponse struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Data   []struct {
		URL string `json:"url"`
	} `json:"data"`
	Usage *vegaImageUsage `json:"usage"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ErrorMessage string `json:"error_message"`
}

func (a *VegaImageAdaptor) pollImageTask(c *gin.Context, info *relaycommon.RelayInfo, taskID string) ([]string, int, error) {
	endpoint := relaycommon.GetFullRequestURL(info.ChannelBaseUrl, vegaImageQueryPath+taskID, info.ChannelType)
	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, 0, fmt.Errorf("new proxy http client failed: %w", err)
	}

	deadline := time.Now().Add(vegaImagePollTimeout)
	for {
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+info.ApiKey)

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("query vega image task failed: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, 0, fmt.Errorf("read vega image task response failed: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, 0, fmt.Errorf("query vega image task returned %d: %s", resp.StatusCode, string(body))
		}

		var query vegaImageQueryResponse
		if err := common.Unmarshal(body, &query); err != nil {
			return nil, 0, err
		}
		switch query.Status {
		case "completed":
			urls := make([]string, 0, len(query.Data))
			for _, item := range query.Data {
				if item.URL != "" {
					urls = append(urls, item.URL)
				}
			}
			if len(urls) == 0 {
				return nil, 0, fmt.Errorf("vega image task completed without image url")
			}
			totalTokens := 0
			if query.Usage != nil {
				totalTokens = query.Usage.TotalTokens
			}
			return urls, totalTokens, nil
		case "failed", "cancelled", "incomplete":
			return nil, 0, fmt.Errorf("vega image task %s: %s", query.Status, vegaImageErrorReason(&query))
		}

		if time.Now().After(deadline) {
			return nil, 0, fmt.Errorf("vega image task %s timed out", taskID)
		}
		select {
		case <-c.Request.Context().Done():
			return nil, 0, c.Request.Context().Err()
		case <-time.After(vegaImagePollInterval):
		}
	}
}

func vegaImageErrorReason(query *vegaImageQueryResponse) string {
	if query.Error != nil && query.Error.Message != "" {
		return query.Error.Message
	}
	if query.ErrorMessage != "" {
		return query.ErrorMessage
	}
	if query.Error != nil && query.Error.Code != "" {
		return query.Error.Code
	}
	return "unknown error"
}
