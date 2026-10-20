package tencent

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVegaBuildSubmitBody(t *testing.T) {
	cases := []struct {
		name       string
		model      string
		req        relaycommon.TaskSubmitReq
		wantRes    string
		wantRatio  string
		wantDur    int
		wantImages []vegaContentItem
	}{
		{
			name:      "pro defaults",
			model:     "wand-vega-video-1.5-pro",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat"},
			wantRes:   "1080P",
			wantRatio: "16:9",
			wantDur:   5,
		},
		{
			name:      "pro 720p is raised to 1080p",
			model:     "wand-vega-video-1.0-pro",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat", Size: "1280x720"},
			wantRes:   "1080P",
			wantRatio: "16:9",
			wantDur:   5,
		},
		{
			name:      "lite 720p stays 768p",
			model:     "wand-vega-video-1.0-lite",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat", Size: "1280x720"},
			wantRes:   "768P",
			wantRatio: "16:9",
			wantDur:   5,
		},
		{
			name:      "metadata resolution wins",
			model:     "wand-vega-video-1.5-pro",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat", Metadata: map[string]any{"resolution": "4k"}},
			wantRes:   "4K",
			wantRatio: "16:9",
			wantDur:   5,
		},
		{
			name:      "explicit duration",
			model:     "wand-vega-video-1.5-pro",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat", Seconds: "8"},
			wantRes:   "1080P",
			wantRatio: "16:9",
			wantDur:   8,
		},
		{
			name:      "single image is first frame",
			model:     "wand-vega-video-1.5-pro",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat", Images: []string{"https://img/1.png"}},
			wantRes:   "1080P",
			wantRatio: "",
			wantDur:   5,
			wantImages: []vegaContentItem{
				{Type: "image_url", ImageURL: &vegaMediaURL{URL: "https://img/1.png"}, Role: "first_frame"},
			},
		},
		{
			name:      "two images are first and last frame",
			model:     "wand-vega-video-1.5-pro",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat", Images: []string{"https://img/1.png", "https://img/2.png"}},
			wantRes:   "1080P",
			wantRatio: "",
			wantDur:   5,
			wantImages: []vegaContentItem{
				{Type: "image_url", ImageURL: &vegaMediaURL{URL: "https://img/1.png"}, Role: "first_frame"},
				{Type: "image_url", ImageURL: &vegaMediaURL{URL: "https://img/2.png"}, Role: "last_frame"},
			},
		},
		{
			name:      "three images are references",
			model:     "wand-vega-video-1.5-pro",
			req:       relaycommon.TaskSubmitReq{Prompt: "cat", Images: []string{"a", "b", "c"}},
			wantRes:   "1080P",
			wantRatio: "",
			wantDur:   5,
			wantImages: []vegaContentItem{
				{Type: "image_url", ImageURL: &vegaMediaURL{URL: "a"}, Role: "reference_image"},
				{Type: "image_url", ImageURL: &vegaMediaURL{URL: "b"}, Role: "reference_image"},
				{Type: "image_url", ImageURL: &vegaMediaURL{URL: "c"}, Role: "reference_image"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := (&vegaFamily{}).buildSubmitBody(tc.req, infoFor(tc.model))
			require.NoError(t, err)
			payload, ok := body.(*vegaVideoRequest)
			require.True(t, ok)
			assert.Equal(t, tc.model, payload.Model)
			assert.Equal(t, tc.wantRes, payload.Resolution)
			assert.Equal(t, tc.wantRatio, payload.Ratio)
			require.NotNil(t, payload.Duration)
			assert.Equal(t, tc.wantDur, *payload.Duration)
			require.NotEmpty(t, payload.Content)
			assert.Equal(t, vegaContentItem{Type: "text", Text: tc.req.Prompt}, payload.Content[0])
			if tc.wantImages == nil {
				assert.Empty(t, payload.Content[1:])
			} else {
				assert.Equal(t, tc.wantImages, payload.Content[1:])
			}
		})
	}
}

func TestVegaBuildSubmitBodyRejectsDurationOutOfRange(t *testing.T) {
	for _, seconds := range []int{3, 16} {
		_, err := (&vegaFamily{}).buildSubmitBody(
			relaycommon.TaskSubmitReq{Prompt: "cat", Duration: seconds},
			infoFor("wand-vega-video-1.5-pro"),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duration must be between 4 and 15 seconds")
	}
}

func TestVegaParseSubmit(t *testing.T) {
	id, err := (&vegaFamily{}).parseSubmit([]byte(`{"id":"1410905459-video-x","model":"wand-vega-video-1.5-pro","status":"queued"}`), "")
	require.NoError(t, err)
	assert.Equal(t, "1410905459-video-x", id)

	_, err = (&vegaFamily{}).parseSubmit([]byte(`{"error":{"code":"invalid_request_error","message":"bad"}}`), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad")

	_, err = (&vegaFamily{}).parseSubmit([]byte(`{}`), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task id is empty")
}

func TestVegaParseQuery(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		status     model.TaskStatus
		url        string
		reason     string
		totalToken int
	}{
		{
			name:   "queued",
			body:   `{"id":"t","status":"queued"}`,
			status: model.TaskStatusQueued,
		},
		{
			name:   "running",
			body:   `{"id":"t","status":"running"}`,
			status: model.TaskStatusInProgress,
		},
		{
			name:       "succeeded",
			body:       `{"id":"t","status":"succeeded","content":{"video_url":"https://v/out.mp4"},"tokenhub_usage":{"total_tokens":1034800}}`,
			status:     model.TaskStatusSuccess,
			url:        "https://v/out.mp4",
			totalToken: 1034800,
		},
		{
			name:   "failed",
			body:   `{"id":"t","status":"failed","error":{"code":"TaskTimeout","message":"task timeout"}}`,
			status: model.TaskStatusFailure,
			reason: "task timeout",
		},
		{
			name:   "failed without message uses code",
			body:   `{"id":"t","status":"failed","error":{"code":"TaskTimeout"}}`,
			status: model.TaskStatusFailure,
			reason: "TaskTimeout",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := (&vegaFamily{}).parseQuery([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.status, model.TaskStatus(info.Status))
			assert.Equal(t, tc.url, info.Url)
			assert.Equal(t, tc.reason, info.Reason)
			assert.Equal(t, tc.totalToken, info.TotalTokens)
			assert.Equal(t, "t", info.TaskID)
		})
	}
}
