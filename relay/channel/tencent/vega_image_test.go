package tencent

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsVegaImageModel(t *testing.T) {
	assert.True(t, IsVegaImageModel("wand-vega-image-1.5-pro"))
	assert.True(t, IsVegaImageModel("wand-vega-image-lite"))
	assert.False(t, IsVegaImageModel("wand-vega-video-1.5-pro"))
	assert.False(t, IsVegaImageModel("gpt-image-2"))
}

func TestVegaImageConvertImageRequest(t *testing.T) {
	adaptor := &VegaImageAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "wand-vega-image-1.5-pro"}}

	n1 := uint(1)
	converted, err := adaptor.ConvertImageRequest(nil, info, dto.ImageRequest{
		Model:  "wand-vega-image-1.5-pro",
		Prompt: "a red apple",
		N:      &n1,
		Size:   "1024x1024",
	})
	require.NoError(t, err)
	payload, ok := converted.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "wand-vega-image-1.5-pro", payload["model"])
	assert.Equal(t, "a red apple", payload["prompt"])
	assert.Equal(t, "1024x1024", payload["size"])

	n2 := uint(2)
	_, err = adaptor.ConvertImageRequest(nil, info, dto.ImageRequest{
		Prompt: "a red apple",
		N:      &n2,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "n=1")
}

func TestDispatchAdaptorSelectsVegaImage(t *testing.T) {
	dispatch := &DispatchAdaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "wand-vega-image-1.5-pro",
			ChannelBaseUrl:    constant.ChannelBaseURLs[constant.ChannelTypeTencent],
			ApiKey:            "single-key",
		},
	}
	dispatch.Init(info)
	assert.IsType(t, &VegaImageAdaptor{}, dispatch.Adaptor)
	assert.Equal(t, tokenHubBaseURL, info.ChannelBaseUrl)

	videoInfo := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "wand-vega-video-1.5-pro",
			ApiKey:            "single-key",
		},
	}
	dispatch = &DispatchAdaptor{}
	dispatch.Init(videoInfo)
	_, isVegaImage := dispatch.Adaptor.(*VegaImageAdaptor)
	assert.False(t, isVegaImage)
}

func TestVegaImageDoResponsePollsAndReturnsOpenAIImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/wand/vega-images/tasks/task-1", r.URL.Path)
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"task_id":"task-1","status":"completed","data":[{"url":"https://img/out.png"}],"usage":{"total_tokens":50000}}`)
	}))
	defer server.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "wand-vega-image-1.5-pro",
			ChannelBaseUrl:    server.URL,
			ApiKey:            "key",
		},
	}

	usage, apiErr := (&VegaImageAdaptor{}).DoResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"task_id":"task-1","request_id":"req-1"}`)),
	}, info)

	require.Nil(t, apiErr)
	parsedUsage, ok := usage.(*dto.Usage)
	require.True(t, ok)
	assert.Equal(t, 50000, parsedUsage.TotalTokens)
	assert.Equal(t, 50000, parsedUsage.PromptTokens)
	assert.Contains(t, w.Body.String(), "https://img/out.png")
	assert.Equal(t, http.StatusOK, w.Code)
}
