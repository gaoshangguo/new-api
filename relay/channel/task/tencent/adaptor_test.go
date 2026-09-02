package tencent

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsV2Model(t *testing.T) {
	cases := []struct {
		model string
		v2    bool
	}{
		{"minimax-video-h3", true},
		{"minimax-video-h3-max", true},
		{"minimax-video-v2.3", false},
		{"minimax-video-v2.3-fast", false},
		{"", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.v2, isV2Model(tc.model), "model=%s", tc.model)
	}
}

func TestParseTaskResultV1(t *testing.T) {
	a := &TaskAdaptor{}
	cases := []struct {
		name     string
		body     string
		status   model.TaskStatus
		progress string
		url      string
		reason   string
	}{
		{
			name:     "processing",
			body:     `{"task_id":"t1","status":"Processing","base_resp":{"status_code":0,"status_msg":"success"}}`,
			status:   model.TaskStatusInProgress,
			progress: "50%",
		},
		{
			name:     "queued",
			body:     `{"task_id":"t1","status":"Queueing"}`,
			status:   model.TaskStatusInProgress,
			progress: "50%",
		},
		{
			name:     "success",
			body:     `{"task_id":"t1","status":"Success","content":{"url":"https://v.example.com/out.mp4"},"tokenhub_usage":{"total_tokens":102655}}`,
			status:   model.TaskStatusSuccess,
			progress: "100%",
			url:      "https://v.example.com/out.mp4",
		},
		{
			name:     "fail",
			body:     `{"task_id":"t1","status":"Fail","base_resp":{"status_code":0,"status_msg":"content sensitive"}}`,
			status:   model.TaskStatusFailure,
			progress: "100%",
			reason:   "content sensitive",
		},
		{
			name:     "unknown status falls back to in progress",
			body:     `{"task_id":"t1","status":"mystery"}`,
			status:   model.TaskStatusInProgress,
			progress: "30%",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := a.ParseTaskResult([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.status, model.TaskStatus(result.Status))
			assert.Equal(t, tc.progress, result.Progress)
			assert.Equal(t, tc.url, result.Url)
			assert.Equal(t, tc.reason, result.Reason)
		})
	}

	result, err := a.ParseTaskResult([]byte(`{"task_id":"t1","status":"Success","content":{"url":"u"},"tokenhub_usage":{"total_tokens":123}}`))
	require.NoError(t, err)
	assert.Equal(t, 123, result.TotalTokens)
}

func TestParseTaskResultV2(t *testing.T) {
	a := &TaskAdaptor{}
	cases := []struct {
		name     string
		body     string
		status   model.TaskStatus
		progress string
		url      string
		reason   string
	}{
		{
			name:     "queued",
			body:     `{"task":{"id":"4-WandVideo-abc","status":"queued"}}`,
			status:   model.TaskStatusQueued,
			progress: "20%",
		},
		{
			name:     "running",
			body:     `{"task":{"id":"4-WandVideo-abc","status":"running"}}`,
			status:   model.TaskStatusInProgress,
			progress: "50%",
		},
		{
			name:     "succeeded",
			body:     `{"task":{"id":"4-WandVideo-abc","status":"succeeded","content":{"url":"https://v.example.com/out.mp4"}},"tokenhub_usage":{"total_tokens":200000}}`,
			status:   model.TaskStatusSuccess,
			progress: "100%",
			url:      "https://v.example.com/out.mp4",
		},
		{
			name:     "failed with error message",
			body:     `{"task":{"id":"4-WandVideo-abc","status":"failed","error":{"message":"rate limited"}}}`,
			status:   model.TaskStatusFailure,
			progress: "100%",
			reason:   "rate limited",
		},
		{
			name:     "cancelled",
			body:     `{"task":{"id":"4-WandVideo-abc","status":"cancelled"}}`,
			status:   model.TaskStatusFailure,
			progress: "100%",
			reason:   "task failed",
		},
		{
			name:     "unknown status falls back to in progress",
			body:     `{"task":{"id":"4-WandVideo-abc","status":"mystery"}}`,
			status:   model.TaskStatusInProgress,
			progress: "30%",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := a.ParseTaskResult([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.status, model.TaskStatus(result.Status))
			assert.Equal(t, tc.progress, result.Progress)
			assert.Equal(t, tc.url, result.Url)
			assert.Equal(t, tc.reason, result.Reason)
		})
	}

	result, err := a.ParseTaskResult([]byte(`{"task":{"id":"4-WandVideo-abc","status":"succeeded","content":{"url":"u"}},"tokenhub_usage":{"total_tokens":200000}}`))
	require.NoError(t, err)
	assert.Equal(t, 200000, result.TotalTokens)
}

func TestParseTaskResultDetectsFamily(t *testing.T) {
	a := &TaskAdaptor{}
	// V1 顶层 status 与 V2 task.status 应互不误判。
	v1, err := a.ParseTaskResult([]byte(`{"task_id":"t","status":"Processing"}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), v1.Status)

	v2, err := a.ParseTaskResult([]byte(`{"task":{"id":"t","status":"queued"}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusQueued), v2.Status)
}

func TestBuildV1Payload(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "minimax-video-v2.3"}}
	req := &relaycommon.TaskSubmitReq{
		Prompt:   "a cat on the windowsill",
		Images:   []string{"https://example.com/first.jpg"},
		Duration: 6,
		Size:     "768x1024",
	}

	payload := a.buildV1Payload(req, info)
	assert.Equal(t, "minimax-video-v2.3", payload.Model)
	assert.Equal(t, "a cat on the windowsill", payload.Prompt)
	assert.Equal(t, "https://example.com/first.jpg", payload.FirstFrameImage)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, 6, *payload.Duration)
	assert.Equal(t, "768P", payload.Resolution)
}

func TestBuildV2PayloadTextOnly(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "minimax-video-h3"}}
	req := &relaycommon.TaskSubmitReq{
		Prompt: "a cat on the windowsill",
	}

	payload := a.buildV2Payload(req, info)
	assert.Equal(t, "minimax-video-h3", payload.Model)
	require.Len(t, payload.Content, 1)
	assert.Equal(t, "text", payload.Content[0].Type)
	assert.Equal(t, "a cat on the windowsill", payload.Content[0].Text)
	// 文生视频 ratio 必填，默�?16:9�?	assert.Equal(t, "16:9", payload.Ratio)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, 6, *payload.Duration)
	assert.Equal(t, "768P", payload.Resolution)
}

func TestBuildV2PayloadWithImages(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "minimax-video-h3"}}
	req := &relaycommon.TaskSubmitReq{
		Prompt: "transition naturally",
		Images: []string{
			"https://example.com/start.jpg",
			"https://example.com/end.jpg",
		},
		Metadata: map[string]interface{}{
			"ratio":      "16:9",
			"resolution": "2K",
		},
	}

	payload := a.buildV2Payload(req, info)
	require.Len(t, payload.Content, 3)
	assert.Equal(t, "text", payload.Content[0].Type)
	assert.Equal(t, "image_url", payload.Content[1].Type)
	assert.Equal(t, "first_frame", payload.Content[1].Role)
	require.NotNil(t, payload.Content[1].ImageURL)
	assert.Equal(t, "https://example.com/start.jpg", payload.Content[1].ImageURL.URL)
	assert.Equal(t, "last_frame", payload.Content[2].Role)
	// 图生视频 ratio 显式提供时透传�?	assert.Equal(t, "16:9", payload.Ratio)
	assert.Equal(t, "2K", payload.Resolution)
}

func TestResolveResolution(t *testing.T) {
	a := &TaskAdaptor{}
	h3 := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "minimax-video-h3"}}
	v23 := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "minimax-video-v2.3"}}

	h3Cases := []struct {
		size     string
		expected string
	}{
		{"2K", "2K"},
		{"1440x2560", "2K"},
		{"1080x1920", "768P"},
		{"768x1024", "768P"},
		{"720x1280", "768P"},
		{"480x854", "480P"},
		{"", "768P"},
	}
	for _, tc := range h3Cases {
		req := relaycommon.TaskSubmitReq{Size: tc.size}
		assert.Equal(t, tc.expected, a.resolveResolution(req, h3), "h3 size=%s", tc.size)
	}

	// v2.3 系列支持 1080P，不应被降级为 768P。
	req := relaycommon.TaskSubmitReq{Size: "1080x1920"}
	assert.Equal(t, "1080P", a.resolveResolution(req, v23))
}

func TestRatioFromSize(t *testing.T) {
	cases := []struct {
		size     string
		expected string
	}{
		{"1024x1024", "1:1"},
		{"1344x768", "16:9"},
		{"768x1344", "9:16"},
		{"1024x768", "4:3"},
		{"768x1024", "3:4"},
		{"2100x900", "21:9"},
		{"", ""},
		{"garbage", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.expected, ratioFromSize(tc.size), "size=%s", tc.size)
	}
}
