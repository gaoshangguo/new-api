package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allowPrivateCallbackURLs(t *testing.T) {
	t.Helper()
	InitHttpClient() // initialize the SSRF-protected client used by delivery
	fetchSetting := system_setting.GetFetchSetting()
	previousPrivate := fetchSetting.AllowPrivateIp
	previousPorts := fetchSetting.AllowedPorts
	fetchSetting.AllowPrivateIp = true
	fetchSetting.AllowedPorts = nil // allow any port (test servers use ephemeral ports)
	t.Cleanup(func() {
		fetchSetting.AllowPrivateIp = previousPrivate
		fetchSetting.AllowedPorts = previousPorts
	})
}

func TestParseTaskCallbackHeadersValidatesURL(t *testing.T) {
	allowPrivateCallbackURLs(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	// Missing callback URL -> empty, no error.
	url, secret, err := ParseTaskCallbackHeaders(ctx)
	require.NoError(t, err)
	assert.Empty(t, url)
	assert.Empty(t, secret)

	// Valid http(s) callback URL passes.
	ctx.Request.Header.Set(taskCallbackHeaderUrl, "http://127.0.0.1:8080/hook")
	ctx.Request.Header.Set(taskCallbackHeaderSecret, "s3cret")
	url, secret, err = ParseTaskCallbackHeaders(ctx)
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:8080/hook", url)
	assert.Equal(t, "s3cret", secret)
}

func TestParseTaskCallbackHeadersRejectsUnsafeURL(t *testing.T) {
	// SSRF protection defaults on: private IP callbacks must be rejected.
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	ctx.Request.Header.Set(taskCallbackHeaderUrl, "http://169.254.169.254/latest/meta-data/")

	_, _, err := ParseTaskCallbackHeaders(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSRF")
}

func TestParseTaskCallbackHeadersRejectsOversizedValues(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	ctx.Request.Header.Set(taskCallbackHeaderUrl, "http://127.0.0.1:8080/"+strings.Repeat("a", 600))

	_, _, err := ParseTaskCallbackHeaders(ctx)
	require.Error(t, err)
}

func TestDeliverTaskCallbackSignsAndRetries(t *testing.T) {
	allowPrivateCallbackURLs(t)

	var received atomic.Int64
	var payloadBody []byte
	var signatureHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payloadBody = body
		signatureHeader = r.Header.Get(taskCallbackSignatureKey)
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	payload := dto.TaskCallbackPayload{
		Type:      "task",
		TaskId:    "task_abc",
		Status:    "SUCCESS",
		ModelName: "sora",
		Quota:     100,
		Timestamp: common.GetTimestamp(),
	}
	err := DeliverTaskCallback(context.Background(), server.URL, "secret-key", payload)
	require.NoError(t, err)
	assert.Equal(t, int64(1), received.Load())

	// The body is the exact JSON payload and the signature verifies against it.
	expected := common.HmacSha256(string(payloadBody), "secret-key")
	assert.Equal(t, expected, signatureHeader)
	var roundTrip dto.TaskCallbackPayload
	require.NoError(t, common.Unmarshal(payloadBody, &roundTrip))
	assert.Equal(t, payload.TaskId, roundTrip.TaskId)
}

func TestDeliverTaskCallbackRetriesTransientFailures(t *testing.T) {
	allowPrivateCallbackURLs(t)

	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	payload := dto.TaskCallbackPayload{Type: "task", TaskId: "task_retry", Status: "FAILURE"}
	err := DeliverTaskCallback(context.Background(), server.URL, "", payload)
	require.Error(t, err)
	assert.Equal(t, int64(taskCallbackAttempts), attempts.Load(), "transient failures must be retried")
}

func TestBuildTaskCallbackPayload(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_payload",
		Status:   model.TaskStatusFailure,
		Quota:    0,
		FailReason: "upstream exploded",
	}
	task.PrivateData = model.TaskPrivateData{
		ResultURL:      "https://cdn.example.com/result.mp4",
		CallbackUrl:    "https://customer.example.com/hook",
		CallbackSecret: "s",
	}
	task.Properties = model.Properties{OriginModelName: "sora"}

	payload := BuildTaskCallbackPayload(task)
	assert.Equal(t, "task", payload.Type)
	assert.Equal(t, "task_payload", payload.TaskId)
	assert.Equal(t, "FAILURE", payload.Status)
	assert.Equal(t, "sora", payload.ModelName)
	assert.Equal(t, "upstream exploded", payload.Error)
	assert.Equal(t, "https://cdn.example.com/result.mp4", payload.ResultUrl)
	assert.Zero(t, payload.Quota)
}

func TestDeliverTaskCallbackSkipsEmptyURL(t *testing.T) {
	err := DeliverTaskCallback(context.Background(), "  ", "k", dto.TaskCallbackPayload{TaskId: "x"})
	require.NoError(t, err)
}
