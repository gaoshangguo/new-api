package tencent

import (
	"bytes"
	"encoding/json"
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

// WAND Midjourney 生图为异步任务：提交得到 id，轮询到 status=2 后取 urls。
// 文档：https://cloud.tencent.com/document/product/1823/...（md-image）
const (
	mdImageSubmitPath = "/v1/wand/md-image/generation"
	mdImageQueryPath  = "/v1/wand/md-image/tasks/"

	mdImagePollTimeout = 10 * time.Minute
)

// 测试可覆盖轮询间隔，生产默认 5 秒。
var mdImagePollInterval = 5 * time.Second

// IsMdImageModel 判断是否为 WAND Midjourney（custom-model-md-*）生图模型。
func IsMdImageModel(model string) bool {
	return strings.HasPrefix(model, "custom-model-md-")
}

// MdImageAdaptor 把 OpenAI 风格 /v1/images/generations 请求转换为
// WAND Midjourney 异步生图协议，并在请求内轮询到出图后返回 OpenAI 图片响应。
// 上游每次固定生成 4 张图片、消耗 120000 tokens。
type MdImageAdaptor struct {
	openai.Adaptor
}

func (a *MdImageAdaptor) Init(info *relaycommon.RelayInfo) {
	a.Adaptor.Init(info)
}

func (a *MdImageAdaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, mdImageSubmitPath, info.ChannelType), nil
}

func (a *MdImageAdaptor) ConvertImageRequest(_ *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return map[string]any{
		"model":  info.UpstreamModelName,
		"prompt": request.Prompt,
	}, nil
}

func (a *MdImageAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *MdImageAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info.RelayMode != relayconstant.RelayModeImagesGenerations {
		return a.Adaptor.DoResponse(c, resp, info)
	}

	responseBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var submit mdImageSubmitResponse
	if err := common.Unmarshal(responseBody, &submit); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if submit.ID == "" {
		return nil, types.NewOpenAIError(fmt.Errorf("md image task id is empty"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	urls, totalTokens, err := a.pollImageTask(c, info, submit.ID)
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

type mdImageSubmitResponse struct {
	ID string `json:"id"`
}

type mdImageUsage struct {
	TotalTokens int `json:"total_tokens"`
}

type mdImageQueryResponse struct {
	ID        string          `json:"id"`
	Status    json.RawMessage `json:"status"`
	Comment   string          `json:"comment"`
	Audits    []string        `json:"audits"`
	URLs      []string        `json:"urls"`
	Usage     *mdImageUsage   `json:"tokenhub_usage"`
	RequestID string          `json:"request_id"`
}

func (a *MdImageAdaptor) pollImageTask(c *gin.Context, info *relaycommon.RelayInfo, taskID string) ([]string, int, error) {
	endpoint := relaycommon.GetFullRequestURL(info.ChannelBaseUrl, mdImageQueryPath+taskID, info.ChannelType)
	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, 0, fmt.Errorf("new proxy http client failed: %w", err)
	}

	deadline := time.Now().Add(mdImagePollTimeout)
	for {
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+info.ApiKey)

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, fmt.Errorf("query md image task failed: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, 0, fmt.Errorf("read md image task response failed: %w", readErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, 0, fmt.Errorf("query md image task returned %d: %s", resp.StatusCode, string(body))
		}

		var query mdImageQueryResponse
		if err := common.Unmarshal(body, &query); err != nil {
			return nil, 0, err
		}
		statusCode, statusText, err := parseMdImageStatus(query.Status)
		if err != nil {
			return nil, 0, err
		}
		switch {
		case statusCode == 2:
			if len(query.URLs) == 0 {
				return nil, 0, fmt.Errorf("md image task completed without image url")
			}
			totalTokens := 0
			if query.Usage != nil {
				totalTokens = query.Usage.TotalTokens
			}
			return query.URLs, totalTokens, nil
		case statusCode == 3:
			return nil, 0, fmt.Errorf("md image task failed: %s", mdImageFailureReason(&query))
		case statusCode == 1 || statusText == "WAIT":
			// 继续轮询。
		default:
			// 未识别的中间状态按进行中处理，由超时兜底。
		}

		if time.Now().After(deadline) {
			return nil, 0, fmt.Errorf("md image task %s timed out", taskID)
		}
		select {
		case <-c.Request.Context().Done():
			return nil, 0, c.Request.Context().Err()
		case <-time.After(mdImagePollInterval):
		}
	}
}

// parseMdImageStatus 兼容 status 的两种编码：整数 1/2/3 或字符串 "WAIT"。
func parseMdImageStatus(raw json.RawMessage) (int, string, error) {
	if len(raw) == 0 {
		return 0, "", nil
	}
	var code int
	if err := common.Unmarshal(raw, &code); err == nil {
		return code, "", nil
	}
	var text string
	if err := common.Unmarshal(raw, &text); err == nil {
		return 0, strings.ToUpper(strings.TrimSpace(text)), nil
	}
	return 0, "", fmt.Errorf("invalid md image status: %s", string(raw))
}

func mdImageFailureReason(query *mdImageQueryResponse) string {
	for _, audit := range query.Audits {
		if audit != "" {
			return audit
		}
	}
	if query.Comment != "" {
		return query.Comment
	}
	return "task failed"
}
