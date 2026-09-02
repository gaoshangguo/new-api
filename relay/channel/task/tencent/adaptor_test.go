package tencent

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func infoFor(model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: model}}
}

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

func TestFamilyForModel(t *testing.T) {
	cases := []struct {
		model  string
		family string
	}{
		{"minimax-video-h3", "minimax"},
		{"minimax-video-v2.3", "minimax"},
		{"pixverse-video-v6.0", "pixverse"},
		{"pixverse-video-c1", "pixverse"},
		{"kling-video-v3", "kling"},
		{"kling-video-v3-omni", "kling"},
		{"vidu-video-q3-turbo", "vidu"},
		{"vidu-video-q2-pro", "vidu"},
		{"hy-video-v1.5", "hunyuan"},
		{"unknown-model", ""},
	}
	for _, tc := range cases {
		family := familyForModel(tc.model)
		if tc.family == "" {
			assert.Nil(t, family, "model=%s", tc.model)
			continue
		}
		require.NotNil(t, family, "model=%s", tc.model)
		assert.IsType(t, familyType(tc.family), family, "model=%s", tc.model)
	}
}

func familyType(name string) any {
	switch name {
	case "minimax":
		return &minimaxFamily{}
	case "pixverse":
		return &pixverseFamily{}
	case "kling":
		return &klingFamily{}
	case "vidu":
		return &viduFamily{}
	default:
		return &hunyuanFamily{}
	}
}

func TestFamilyForResponse(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		family string
	}{
		{"minimax v2", `{"task":{"id":"t","status":"queued"}}`, "minimax"},
		{"minimax v1 processing", `{"task_id":"t","status":"Processing"}`, "minimax"},
		{"minimax v1 success", `{"task_id":"t","status":"Success","content":{"url":"u"}}`, "minimax"},
		{"pixverse", `{"ErrCode":0,"Resp":{"id":"t","status":5}}`, "pixverse"},
		{"kling", `{"code":0,"data":[{"id":"t","status":"succeeded"}]}`, "kling"},
		{"vidu", `{"task_id":"t","state":"processing"}`, "vidu"},
		{"hunyuan success", `{"task_id":"t","status":"succeeded","videos":[]}`, "hunyuan"},
		{"hunyuan failed no videos", `{"task_id":"t","status":"failed","error":{"message":"x"}}`, "hunyuan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			family := familyForResponse([]byte(tc.body))
			require.NotNil(t, family)
			assert.IsType(t, familyType(tc.family), family)
		})
	}
}

// ---------------------------------------------------------------------------
// MiniMax
// ---------------------------------------------------------------------------

func TestParseTaskResultMinimaxV1(t *testing.T) {
	a := &TaskAdaptor{}
	cases := []struct {
		name     string
		body     string
		status   string
		progress string
		url      string
		reason   string
	}{
		{"processing", `{"task_id":"t1","status":"Processing"}`, string(model.TaskStatusInProgress), "50%", "", ""},
		{"queueing", `{"task_id":"t1","status":"Queueing"}`, string(model.TaskStatusInProgress), "50%", "", ""},
		{"success", `{"task_id":"t1","status":"Success","content":{"url":"https://v.example.com/out.mp4"}}`, string(model.TaskStatusSuccess), "100%", "https://v.example.com/out.mp4", ""},
		{"fail", `{"task_id":"t1","status":"Fail","base_resp":{"status_msg":"content sensitive"}}`, string(model.TaskStatusFailure), "100%", "", "content sensitive"},
		{"unknown", `{"task_id":"t1","status":"mystery"}`, string(model.TaskStatusInProgress), "30%", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := a.ParseTaskResult([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.status, result.Status)
			assert.Equal(t, tc.progress, result.Progress)
			assert.Equal(t, tc.url, result.Url)
			assert.Equal(t, tc.reason, result.Reason)
		})
	}
}

func TestParseTaskResultMinimaxV2(t *testing.T) {
	a := &TaskAdaptor{}
	cases := []struct {
		name     string
		body     string
		status   string
		progress string
		url      string
		reason   string
	}{
		{"queued", `{"task":{"id":"t","status":"queued"}}`, string(model.TaskStatusQueued), "20%", "", ""},
		{"running", `{"task":{"id":"t","status":"running"}}`, string(model.TaskStatusInProgress), "50%", "", ""},
		{"succeeded", `{"task":{"id":"t","status":"succeeded","content":{"url":"u"}}}`, string(model.TaskStatusSuccess), "100%", "u", ""},
		{"failed", `{"task":{"id":"t","status":"failed","error":{"message":"boom"}}}`, string(model.TaskStatusFailure), "100%", "", "boom"},
		{"cancelled", `{"task":{"id":"t","status":"cancelled"}}`, string(model.TaskStatusFailure), "100%", "", "task failed"},
		{"unknown", `{"task":{"id":"t","status":"mystery"}}`, string(model.TaskStatusInProgress), "30%", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := a.ParseTaskResult([]byte(tc.body))
			require.NoError(t, err)
			assert.Equal(t, tc.status, result.Status)
			assert.Equal(t, tc.progress, result.Progress)
			assert.Equal(t, tc.url, result.Url)
			assert.Equal(t, tc.reason, result.Reason)
		})
	}
}

func TestMinimaxBuildV1Payload(t *testing.T) {
	f := &minimaxFamily{}
	req := relaycommon.TaskSubmitReq{
		Prompt:   "a cat",
		Images:   []string{"https://example.com/first.jpg"},
		Duration: 6,
		Size:     "768x1024",
	}
	payload := f.buildV1Payload(&req, infoFor("minimax-video-v2.3"))
	assert.Equal(t, "minimax-video-v2.3", payload.Model)
	assert.Equal(t, "a cat", payload.Prompt)
	assert.Equal(t, "https://example.com/first.jpg", payload.FirstFrameImage)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, 6, *payload.Duration)
	assert.Equal(t, "768P", payload.Resolution)
}

func TestMinimaxBuildV2Payload(t *testing.T) {
	f := &minimaxFamily{}
	req := relaycommon.TaskSubmitReq{
		Prompt:   "transition",
		Images:   []string{"https://example.com/start.jpg", "https://example.com/end.jpg"},
		Metadata: map[string]interface{}{"ratio": "16:9", "resolution": "2K"},
	}
	payload := f.buildV2Payload(&req, infoFor("minimax-video-h3"))
	require.Len(t, payload.Content, 3)
	assert.Equal(t, "text", payload.Content[0].Type)
	assert.Equal(t, "image_url", payload.Content[1].Type)
	assert.Equal(t, "first_frame", payload.Content[1].Role)
	assert.Equal(t, "last_frame", payload.Content[2].Role)
	assert.Equal(t, "16:9", payload.Ratio)
	assert.Equal(t, "2K", payload.Resolution)

	textOnly := relaycommon.TaskSubmitReq{Prompt: "hello"}
	payload2 := f.buildV2Payload(&textOnly, infoFor("minimax-video-h3"))
	assert.Equal(t, "16:9", payload2.Ratio)
	assert.Equal(t, "768P", payload2.Resolution)
}

// ---------------------------------------------------------------------------
// PixVerse
// ---------------------------------------------------------------------------

func TestPixVerseBuildSubmitBodyAndURL(t *testing.T) {
	f := &pixverseFamily{}
	info := infoFor("pixverse-video-v6.0")

	text := relaycommon.TaskSubmitReq{Prompt: "a cat", Duration: 5}
	assert.Equal(t, pixverseTextToVideoEndpoint, f.buildSubmitURL(text, info))
	body, err := f.buildSubmitBody(text, info)
	require.NoError(t, err)
	assert.Equal(t, "16:9", body.(*pixverseTextToVideoRequest).AspectRatio)
	assert.Equal(t, quality720p, body.(*pixverseTextToVideoRequest).Quality)

	image := relaycommon.TaskSubmitReq{Prompt: "move", Images: []string{"https://example.com/a.jpg"}}
	assert.Equal(t, pixverseImageToVideoEndpoint, f.buildSubmitURL(image, info))
	ib, err := f.buildSubmitBody(image, info)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/a.jpg", ib.(*pixverseImageToVideoRequest).ImgID)

	se := relaycommon.TaskSubmitReq{Prompt: "transition", Images: []string{"a.jpg", "b.jpg"}}
	assert.Equal(t, pixverseStartEndToVideoEndpoint, f.buildSubmitURL(se, info))

	ref := relaycommon.TaskSubmitReq{Prompt: "subject", Images: []string{"a.jpg", "b.jpg", "c.jpg"}}
	assert.Equal(t, pixverseReferenceToVideoEndpoint, f.buildSubmitURL(ref, info))
	rb, err := f.buildSubmitBody(ref, info)
	require.NoError(t, err)
	assert.Len(t, rb.(*pixverseReferenceToVideoRequest).References, 3)
}

func TestPixVerseParse(t *testing.T) {
	f := &pixverseFamily{}
	a := &TaskAdaptor{}

	id, err := f.parseSubmit([]byte(`{"ErrCode":0,"Resp":{"video_id":"v1"}}`), "")
	require.NoError(t, err)
	assert.Equal(t, "v1", id)

	_, err = f.parseSubmit([]byte(`{"ErrCode":1300,"ErrMsg":"sensitive"}`), "")
	require.Error(t, err)

	res, err := a.ParseTaskResult([]byte(`{"ErrCode":0,"Resp":{"id":"v1","status":1,"url":"https://v.mp4"},"tokenhub_usage":{"total_tokens":100}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), res.Status)
	assert.Equal(t, "https://v.mp4", res.Url)
	assert.Equal(t, 100, res.TotalTokens)

	res, err = a.ParseTaskResult([]byte(`{"ErrCode":0,"Resp":{"id":"v1","status":5}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), res.Status)

	res, err = a.ParseTaskResult([]byte(`{"ErrCode":0,"Resp":{"id":"v1","status":8}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), res.Status)
}

// ---------------------------------------------------------------------------
// Kling
// ---------------------------------------------------------------------------

func TestKlingBuildSubmitBodyAndURL(t *testing.T) {
	f := &klingFamily{}

	text := relaycommon.TaskSubmitReq{Prompt: "a girl", Duration: 5}
	assert.Equal(t, klingTextToVideoEndpoint, f.buildSubmitURL(text, infoFor("kling-video-v3")))
	tb, err := f.buildSubmitBody(text, infoFor("kling-video-v3"))
	require.NoError(t, err)
	assert.Equal(t, "a girl", tb.(*klingTextToVideoRequest).Prompt)
	require.NotNil(t, tb.(*klingTextToVideoRequest).Settings)
	assert.Equal(t, "16:9", tb.(*klingTextToVideoRequest).Settings.AspectRatio)

	image := relaycommon.TaskSubmitReq{Prompt: "move", Images: []string{"https://example.com/a.jpg"}}
	assert.Equal(t, klingImageToVideoEndpoint, f.buildSubmitURL(image, infoFor("kling-video-v3")))
	ib, err := f.buildSubmitBody(image, infoFor("kling-video-v3"))
	require.NoError(t, err)
	contents := ib.(*klingContentsToVideoRequest).Contents
	require.Len(t, contents, 2)
	assert.Equal(t, "prompt", contents[0].Type)
	assert.Equal(t, "first_frame", contents[1].Type)
	assert.Equal(t, "https://example.com/a.jpg", contents[1].URL)

	// Omni 模型走 omni-video 端点。
	assert.Equal(t, klingOmniVideoEndpoint, f.buildSubmitURL(text, infoFor("kling-video-v3-omni")))
	assert.Equal(t, klingOmniVideoEndpoint, f.buildSubmitURL(text, infoFor("kling-video-o1")))
}

func TestKlingParse(t *testing.T) {
	f := &klingFamily{}
	a := &TaskAdaptor{}

	id, err := f.parseSubmit([]byte(`{"code":0,"data":{"id":"task_1"}}`), "")
	require.NoError(t, err)
	assert.Equal(t, "task_1", id)

	_, err = f.parseSubmit([]byte(`{"code":1000,"message":"auth failed"}`), "")
	require.Error(t, err)

	res, err := a.ParseTaskResult([]byte(`{"code":0,"data":[{"id":"task_1","status":"succeeded","outputs":[{"url":"https://v.mp4"}]}],"tokenhub_usage":{"total_tokens":600000}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), res.Status)
	assert.Equal(t, "https://v.mp4", res.Url)
	assert.Equal(t, 600000, res.TotalTokens)

	res, err = a.ParseTaskResult([]byte(`{"code":0,"data":[{"id":"task_1","status":"failed","message":"boom"}]}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), res.Status)
	assert.Equal(t, "boom", res.Reason)

	res, err = a.ParseTaskResult([]byte(`{"code":0,"data":[{"id":"task_1","status":"processing"}]}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), res.Status)
}

// ---------------------------------------------------------------------------
// Vidu
// ---------------------------------------------------------------------------

func TestViduBuildSubmitBodyAndURL(t *testing.T) {
	f := &viduFamily{}
	info := infoFor("vidu-video-q3-turbo")

	text := relaycommon.TaskSubmitReq{Prompt: "a cat", Duration: 5}
	assert.Equal(t, viduTextToVideoEndpoint, f.buildSubmitURL(text, info))
	tb, err := f.buildSubmitBody(text, info)
	require.NoError(t, err)
	assert.Equal(t, "16:9", tb.(*viduTextToVideoRequest).AspectRatio)

	image := relaycommon.TaskSubmitReq{Prompt: "move", Images: []string{"https://example.com/a.jpg"}}
	assert.Equal(t, viduImageToVideoEndpoint, f.buildSubmitURL(image, info))
	ib, err := f.buildSubmitBody(image, info)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/a.jpg"}, ib.(*viduImagesToVideoRequest).Images)

	se := relaycommon.TaskSubmitReq{Prompt: "transition", Images: []string{"a.jpg", "b.jpg"}}
	assert.Equal(t, viduStartEndToVideoEndpoint, f.buildSubmitURL(se, info))

	ref := relaycommon.TaskSubmitReq{Prompt: "multi", Images: []string{"a.jpg", "b.jpg", "c.jpg"}}
	assert.Equal(t, viduReferenceToVideoEndpoint, f.buildSubmitURL(ref, info))
}

func TestViduParse(t *testing.T) {
	f := &viduFamily{}
	a := &TaskAdaptor{}

	id, err := f.parseSubmit([]byte(`{"task_id":"t1","state":"created"}`), "")
	require.NoError(t, err)
	assert.Equal(t, "t1", id)

	_, err = f.parseSubmit([]byte(`{"state":"created"}`), "")
	require.Error(t, err)

	res, err := a.ParseTaskResult([]byte(`{"task_id":"t1","state":"success","creations":[{"url":"https://v.mp4"}],"tokenhub_usage":{"total_tokens":102655}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), res.Status)
	assert.Equal(t, "https://v.mp4", res.Url)
	assert.Equal(t, 102655, res.TotalTokens)

	res, err = a.ParseTaskResult([]byte(`{"task_id":"t1","state":"processing"}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), res.Status)

	res, err = a.ParseTaskResult([]byte(`{"task_id":"t1","state":"failed"}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), res.Status)
}

// ---------------------------------------------------------------------------
// Hy（混元）
// ---------------------------------------------------------------------------

func TestHunyuanBuildSubmitBody(t *testing.T) {
	f := &hunyuanFamily{}
	info := infoFor("hy-video-v1.5")

	text := relaycommon.TaskSubmitReq{Prompt: "a dog"}
	body, err := f.buildSubmitBody(text, info)
	require.NoError(t, err)
	req := body.(*hunyuanVideoRequest)
	assert.Equal(t, "16:9", req.AspectRatio)
	assert.Equal(t, "", req.ImageURL)

	imageURL := relaycommon.TaskSubmitReq{Prompt: "move", Images: []string{"https://example.com/a.jpg"}}
	body, err = f.buildSubmitBody(imageURL, info)
	require.NoError(t, err)
	req = body.(*hunyuanVideoRequest)
	assert.Equal(t, "https://example.com/a.jpg", req.ImageURL)
	assert.Equal(t, "", req.Image)
	assert.Equal(t, "", req.AspectRatio)

	base64Image := relaycommon.TaskSubmitReq{Prompt: "move", Images: []string{"data:image/jpeg;base64,xxx"}}
	body, err = f.buildSubmitBody(base64Image, info)
	require.NoError(t, err)
	assert.Equal(t, "data:image/jpeg;base64,xxx", body.(*hunyuanVideoRequest).Image)
}

func TestHunyuanParse(t *testing.T) {
	f := &hunyuanFamily{}
	a := &TaskAdaptor{}

	id, err := f.parseSubmit([]byte(`{"task_id":"t1","created":1,"request_id":"r1"}`), "")
	require.NoError(t, err)
	assert.Equal(t, "t1", id)

	_, err = f.parseSubmit([]byte(`{"task_id":"","status":"failed","error":{"code":"1300","message":"sensitive"}}`), "")
	require.Error(t, err)

	res, err := a.ParseTaskResult([]byte(`{"task_id":"t1","status":"succeeded","videos":[{"url":"https://v.mp4"}],"tokenhub_usage":{"total_tokens":150000}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusSuccess), res.Status)
	assert.Equal(t, "https://v.mp4", res.Url)
	assert.Equal(t, 150000, res.TotalTokens)

	res, err = a.ParseTaskResult([]byte(`{"task_id":"t1","status":"failed","error":{"message":"generate failed"}}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusFailure), res.Status)
	assert.Equal(t, "generate failed", res.Reason)

	res, err = a.ParseTaskResult([]byte(`{"task_id":"t1","status":"running"}`))
	require.NoError(t, err)
	assert.Equal(t, string(model.TaskStatusInProgress), res.Status)
}

// ---------------------------------------------------------------------------
// 公共辅助
// ---------------------------------------------------------------------------

func TestResolveResolutionMinimax(t *testing.T) {
	f := &minimaxFamily{}
	h3 := infoFor("minimax-video-h3")
	v23 := infoFor("minimax-video-v2.3")

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
		assert.Equal(t, tc.expected, f.resolveResolution(req, h3), "h3 size=%s", tc.size)
	}

	// v2.3 系列支持 1080P。
	req := relaycommon.TaskSubmitReq{Size: "1080x1920"}
	assert.Equal(t, "1080P", f.resolveResolution(req, v23))
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
