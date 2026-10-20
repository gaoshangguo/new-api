package tencent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsMdImageModel(t *testing.T) {
	assert.True(t, IsMdImageModel("custom-model-md-v7"))
	assert.True(t, IsMdImageModel("custom-model-md-v8.1"))
	assert.True(t, IsMdImageModel("custom-model-md-v8.2"))
	assert.False(t, IsMdImageModel("wand-vega-image-1.5-pro"))
	assert.False(t, IsMdImageModel("wand-vega-video-1.5-pro"))
}

func TestMdImageConvertImageRequest(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "custom-model-md-v8.2"}}
	converted, err := (&MdImageAdaptor{}).ConvertImageRequest(nil, info, dto.ImageRequest{
		Model:  "custom-model-md-v8.2",
		Prompt: "小狗跳舞 --ar 1:1",
		Size:   "1024x1024",
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"model":  "custom-model-md-v8.2",
		"prompt": "小狗跳舞 --ar 1:1",
	}, converted)
}

func TestDispatchAdaptorSelectsMdImage(t *testing.T) {
	makeInfo := func(model string) *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			RelayMode: relayconstant.RelayModeImagesGenerations,
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: model,
				ChannelBaseUrl:    constant.ChannelBaseURLs[constant.ChannelTypeTencent],
				ApiKey:            "single-key",
			},
		}
	}

	for _, model := range []string{"custom-model-md-v7", "custom-model-md-v8.1", "custom-model-md-v8.2"} {
		dispatch := &DispatchAdaptor{}
		info := makeInfo(model)
		dispatch.Init(info)
		assert.IsType(t, &MdImageAdaptor{}, dispatch.Adaptor, "model=%s", model)
		assert.Equal(t, tokenHubBaseURL, info.ChannelBaseUrl)
	}

	dispatch := &DispatchAdaptor{}
	dispatch.Init(makeInfo("wand-vega-image-1.5-pro"))
	_, isMd := dispatch.Adaptor.(*MdImageAdaptor)
	assert.False(t, isMd)
}

func TestParseMdImageStatus(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantCode int
		wantText string
	}{
		{"integer running", `1`, 1, ""},
		{"integer success", `2`, 2, ""},
		{"integer failure", `3`, 3, ""},
		{"string wait", `"WAIT"`, 0, "WAIT"},
		{"lowercase wait", `"wait"`, 0, "WAIT"},
		{"empty", ``, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, text, err := parseMdImageStatus(json.RawMessage(tc.raw))
			require.NoError(t, err)
			assert.Equal(t, tc.wantCode, code)
			assert.Equal(t, tc.wantText, text)
		})
	}
}

func TestMdImageDoResponsePollsAndReturnsFourImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousInterval := mdImagePollInterval
	mdImagePollInterval = time.Millisecond
	t.Cleanup(func() { mdImagePollInterval = previousInterval })

	urls := []string{"https://img/0.png", "https://img/1.png", "https://img/2.png", "https://img/3.png"}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/wand/md-image/tasks/task-md-1", r.URL.Path)
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			_, _ = io.WriteString(w, `{"id":"","status":"WAIT"}`)
			return
		}
		_, _ = io.WriteString(w, `{"audits":["","","",""],"comment":"成功","id":"task-md-1","status":2,"tokenhub_usage":{"total_tokens":120000},"urls":["https://img/0.png","https://img/1.png","https://img/2.png","https://img/3.png"]}`)
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "custom-model-md-v8.2",
			ChannelBaseUrl:    server.URL,
			ApiKey:            "key",
		},
	}

	usage, apiErr := (&MdImageAdaptor{}).DoResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"task-md-1"}`)),
	}, info)

	require.Nil(t, apiErr)
	parsedUsage, ok := usage.(*dto.Usage)
	require.True(t, ok)
	assert.Equal(t, 120000, parsedUsage.TotalTokens)
	assert.Equal(t, 120000, parsedUsage.PromptTokens)
	assert.Equal(t, int32(2), calls.Load())
	body := w.Body.String()
	for _, url := range urls {
		assert.Contains(t, body, url)
	}
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMdImageFailureReason(t *testing.T) {
	assert.Equal(t, "content rejected", mdImageFailureReason(&mdImageQueryResponse{
		Comment: "失败",
		Audits:  []string{"", "content rejected"},
	}))
	assert.Equal(t, "失败", mdImageFailureReason(&mdImageQueryResponse{Comment: "失败"}))
	assert.Equal(t, "task failed", mdImageFailureReason(&mdImageQueryResponse{}))
}
