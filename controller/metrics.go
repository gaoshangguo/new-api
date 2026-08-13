package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetPrometheusMetrics 输出 Prometheus 文本格式的基础平台指标（P0-29）。
// 不包含任何敏感数据（不输出密钥/请求正文），可安全挂到抓取端点。
func GetPrometheusMetrics(c *gin.Context) {
	var builder strings.Builder
	builder.WriteString("# HELP newapi_up 1 if the gateway is up.\n")
	builder.WriteString("# TYPE newapi_up gauge\n")
	builder.WriteString("newapi_up 1\n")

	var userCount, channelCount, tokenCount int64
	_ = model.DB.Model(&model.User{}).Count(&userCount).Error
	_ = model.DB.Model(&model.Channel{}).Count(&channelCount).Error
	_ = model.DB.Model(&model.Token{}).Count(&tokenCount).Error
	writeGauge(&builder, "newapi_users_total", userCount, "registered users")
	writeGauge(&builder, "newapi_channels_total", channelCount, "configured channels")
	writeGauge(&builder, "newapi_tokens_total", tokenCount, "API keys")

	var enabledChannels, degradedChannels int64
	_ = model.DB.Model(&model.Channel{}).Where("status = ?", common.ChannelStatusEnabled).Count(&enabledChannels).Error
	_ = model.DB.Model(&model.Channel{}).Where("status = ?", common.ChannelStatusAutoDisabled).Count(&degradedChannels).Error
	writeGauge(&builder, "newapi_channels_enabled", enabledChannels, "enabled channels")
	writeGauge(&builder, "newapi_channels_auto_disabled", degradedChannels, "auto-disabled channels")

	// 近 1 小时请求与错误（消费日志口径，LOG_DB）。
	hourAgo := common.GetTimestamp() - 3600
	var requests, failed int64
	_ = model.LOG_DB.Model(&model.Log{}).Where("type = ? AND created_at >= ?", model.LogTypeConsume, hourAgo).Count(&requests).Error
	_ = model.LOG_DB.Model(&model.Log{}).Where("type = ? AND quota = 0 AND created_at >= ?", model.LogTypeConsume, hourAgo).Count(&failed).Error
	writeGauge(&builder, "newapi_requests_total_1h", requests, "consume-logged requests in the last hour")
	writeGauge(&builder, "newapi_failed_requests_total_1h", failed, "zero-settlement requests in the last hour")

	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.String(http.StatusOK, builder.String())
}

func writeGauge(builder *strings.Builder, name string, value int64, help string) {
	builder.WriteString(fmt.Sprintf("# HELP %s %s.\n", name, help))
	builder.WriteString(fmt.Sprintf("# TYPE %s gauge\n", name))
	builder.WriteString(fmt.Sprintf("%s %d\n", name, value))
}
