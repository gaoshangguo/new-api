package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/relay/channel"

	"github.com/gin-gonic/gin"
)

// GetAdaptorCapabilities 返回全部渠道类型的能力声明（P0-13）。
// 能力位为渠道家族级默认值：工具、视觉、思考、流式、超时与计费模式。
func GetAdaptorCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel.GetChannelCapabilitiesMap(),
	})
}
