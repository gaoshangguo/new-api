package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// 客户级任务回调（P0-09）：异步任务到达终态后，网关把统一签名的
// TaskCallbackPayload 投递到客户提交任务时提供的 callback_url。
// 投递全程经 SSRF 防护（非 worker 模式走 GetSSRFProtectedHTTPClient），
// 签名密钥优先用客户提供的 callback_secret，缺省回退平台级环境变量。

const (
	taskCallbackHeaderUrl    = "X-New-Api-Callback-Url"
	taskCallbackHeaderSecret = "X-New-Api-Callback-Secret"
	taskCallbackSignatureKey = "X-New-Api-Signature"
	taskCallbackMaxUrlLength  = 512
	taskCallbackMaxSecretLen  = 128
	// taskCallbackAttempts 单次投递重试次数（含首次），失败按指数退避。
	taskCallbackAttempts = 3
)

// TaskCallbackSigningSecret 返回平台级回调签名密钥（客户未提供 callback_secret 时使用）。
func TaskCallbackSigningSecret() string {
	return common.GetEnvOrDefaultString("TASK_CALLBACK_SIGNING_SECRET", "")
}

// ParseTaskCallbackHeaders 从提交请求头解析客户回调配置。未提供 callback_url
// 时返回空值（任务不投递回调）。URL 必须通过 SSRF 校验并满足长度限制，
// 失败返回错误（提交方应收到 400）。
func ParseTaskCallbackHeaders(c *gin.Context) (url string, secret string, err error) {
	url = strings.TrimSpace(c.GetHeader(taskCallbackHeaderUrl))
	if url == "" {
		return "", "", nil
	}
	secret = strings.TrimSpace(c.GetHeader(taskCallbackHeaderSecret))
	if len(url) > taskCallbackMaxUrlLength {
		return "", "", errors.New("callback url exceeds the 512 byte limit")
	}
	if len(secret) > taskCallbackMaxSecretLen {
		return "", "", errors.New("callback secret exceeds the 128 byte limit")
	}
	if err := ValidateSSRFProtectedFetchURL(url); err != nil {
		return "", "", fmt.Errorf("callback url blocked by SSRF protection: %w", err)
	}
	return url, secret, nil
}

// NormalizeMidjourneyCallback 校验 MJ 协议 notifyHook 形式的回调 URL。
func NormalizeMidjourneyCallback(url, secret string) (string, string, error) {
	url = strings.TrimSpace(url)
	secret = strings.TrimSpace(secret)
	if url == "" {
		return "", "", nil
	}
	if len(url) > taskCallbackMaxUrlLength {
		return "", "", errors.New("callback url exceeds the 512 byte limit")
	}
	if len(secret) > taskCallbackMaxSecretLen {
		return "", "", errors.New("callback secret exceeds the 128 byte limit")
	}
	if err := ValidateSSRFProtectedFetchURL(url); err != nil {
		return "", "", fmt.Errorf("callback url blocked by SSRF protection: %w", err)
	}
	return url, secret, nil
}

// BuildTaskCallbackPayload 组装通用任务（视频/音频等）的回调负载。
func BuildTaskCallbackPayload(task *model.Task) dto.TaskCallbackPayload {
	payload := dto.TaskCallbackPayload{
		Type:      "task",
		TaskId:    task.TaskID,
		Status:    string(task.Status),
		ModelName: taskModelName(task),
		Quota:     task.Quota,
		Timestamp: common.GetTimestamp(),
	}
	if task.Status == model.TaskStatusFailure {
		payload.Error = task.FailReason
	}
	payload.ResultUrl = task.PrivateData.ResultURL
	return payload
}

// BuildMidjourneyCallbackPayload 组装 Midjourney 任务回调负载。
func BuildMidjourneyCallbackPayload(task *model.Midjourney) dto.TaskCallbackPayload {
	payload := dto.TaskCallbackPayload{
		Type:      "midjourney",
		TaskId:    task.MjId,
		Status:    task.Status,
		ModelName: CovertMjpActionToModelName(task.Action),
		Quota:     task.Quota,
		Timestamp: common.GetTimestamp(),
	}
	if task.Status == "FAILURE" {
		payload.Error = task.FailReason
	}
	if task.ImageUrl != "" {
		payload.ResultUrl = task.ImageUrl
	} else if task.VideoUrl != "" {
		payload.ResultUrl = task.VideoUrl
	}
	return payload
}

// DeliverTaskCallback 带签名与重试地投递任务回调。投递失败在日志中记录，
// 不影响任务计费结果（回调是尽力而为的通知）。
func DeliverTaskCallback(ctx context.Context, callbackURL, secret string, payload dto.TaskCallbackPayload) error {
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return nil
	}
	payloadBytes, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task callback payload: %w", err)
	}
	signingSecret := secret
	if signingSecret == "" {
		signingSecret = TaskCallbackSigningSecret()
	}
	var lastErr error
	for attempt := 1; attempt <= taskCallbackAttempts; attempt++ {
		lastErr = deliverTaskCallbackOnce(ctx, callbackURL, signingSecret, payloadBytes)
		if lastErr == nil {
			return nil
		}
		if attempt < taskCallbackAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
	}
	logger.LogError(ctx, fmt.Sprintf("task callback delivery failed after %d attempts: url=%s err=%v", taskCallbackAttempts, callbackURL, lastErr))
	return lastErr
}

// DeliverTaskCallbackAsync 在后台 goroutine 投递回调，不阻塞任务轮询循环。
func DeliverTaskCallbackAsync(ctx context.Context, callbackURL, secret string, payload dto.TaskCallbackPayload) {
	if strings.TrimSpace(callbackURL) == "" {
		return
	}
	gopool.Go(func() {
		deliveryCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := DeliverTaskCallback(deliveryCtx, callbackURL, secret, payload); err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("task callback async delivery failed: task_id=%s url=%s err=%v", payload.TaskId, callbackURL, err))
		}
	})
}

func deliverTaskCallbackOnce(ctx context.Context, callbackURL, secret string, payloadBytes []byte) error {
	if err := ValidateSSRFProtectedFetchURL(callbackURL); err != nil {
		return fmt.Errorf("callback url blocked by SSRF protection: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set(taskCallbackSignatureKey, common.HmacSha256(string(payloadBytes), secret))
	}
	resp, err := GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send callback: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callback returned status code: %d", resp.StatusCode)
	}
	return nil
}
