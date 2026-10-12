package controller

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Bound workbook allocation across users on this server instance as well as
// applying the per-user rate limit at the router.
var usageLogExportSlots = make(chan struct{}, 2)

func ExportAllLogs(c *gin.Context) {
	exportUsageLogs(c, true)
}

func ExportUserLogs(c *gin.Context) {
	exportUsageLogs(c, false)
}

func exportUsageLogs(c *gin.Context, admin bool) {
	var filter model.LogFilter
	if err := c.ShouldBindQuery(&filter); err != nil ||
		(filter.EndTimestamp != 0 && filter.StartTimestamp > filter.EndTimestamp) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "LOG_EXPORT_INVALID_FILTER", "message": "Invalid log export filters"})
		return
	}
	if !admin {
		userID := c.GetInt("id")
		if userID <= 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		filter.UserID = &userID
		filter.Username, filter.Channel = "", 0
	}
	select {
	case usageLogExportSlots <- struct{}{}:
		defer func() { <-usageLogExportSlots }()
	default:
		c.Header("Retry-After", "30")
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "code": "LOG_EXPORT_BUSY", "message": "Log export is busy; try again later"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	logs, err := model.GetLogsForExport(ctx, filter)
	if err != nil {
		code, status := "LOG_EXPORT_FAILED", http.StatusInternalServerError
		if errors.Is(err, model.ErrUsageLogExportLimit) {
			code, status = "LOG_EXPORT_LIMIT", http.StatusBadRequest
		} else if errors.Is(err, model.ErrUsageLogExportSize) {
			code, status = "LOG_EXPORT_SIZE", http.StatusBadRequest
		}
		common.SysError("failed to export usage logs: " + err.Error())
		c.JSON(status, gin.H{"success": false, "code": code, "message": "Unable to export usage logs; narrow the filters or try again"})
		return
	}
	data, err := service.BuildUsageLogExcel(ctx, logs, admin)
	if err != nil {
		common.SysError("failed to build usage log workbook: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "LOG_EXPORT_FAILED", "message": "Unable to export usage logs"})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="usage-logs-`+time.Now().UTC().Format("20060102-150405")+`.xlsx"`)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}
