package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// ListPlatformAlertRules 告警规则列表（Root）。
func ListPlatformAlertRules(c *gin.Context) {
	rules, err := model.ListPlatformAlertRules()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rules})
}

// CreatePlatformAlertRule 创建告警规则（Root，审计）。
func CreatePlatformAlertRule(c *gin.Context) {
	var rule model.PlatformAlertRule
	if err := common.DecodeJson(c.Request.Body, &rule); err != nil {
		common.ApiError(c, err)
		return
	}
	actor := businessAuditActor(c)
	if err := model.CreatePlatformAlertRule(&rule, &actor); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "platform_alert.rule.create", map[string]interface{}{"id": rule.Id, "name": rule.Name, "metric": rule.Metric})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rule})
}

// UpdatePlatformAlertRule 更新告警规则（Root，审计前后值）。
func UpdatePlatformAlertRule(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var rule model.PlatformAlertRule
	if err := common.DecodeJson(c.Request.Body, &rule); err != nil {
		common.ApiError(c, err)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = common.DecodeJson(c.Request.Body, &req)
	actor := businessAuditActor(c)
	if err := model.UpdatePlatformAlertRule(id, &rule, req.Reason, &actor); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "platform_alert.rule.update", map[string]interface{}{"id": id, "name": rule.Name})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// DeletePlatformAlertRule 删除告警规则（Root，审计）。
func DeletePlatformAlertRule(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = common.DecodeJson(c.Request.Body, &req)
	actor := businessAuditActor(c)
	if err := model.DeletePlatformAlertRule(id, &actor, req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "platform_alert.rule.delete", map[string]interface{}{"id": id})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// ListPlatformAlertEvents 告警事件列表（Root）。
func ListPlatformAlertEvents(c *gin.Context) {
	activeOnly := c.Query("active_only") == "true"
	pageInfo := common.GetPageQuery(c)
	events, total, err := model.ListPlatformAlertEvents(activeOnly, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": events, "total": total})
}

// TriggerPlatformAlertEvaluation 手动触发一次告警评估（Root）。
func TriggerPlatformAlertEvaluation(c *gin.Context) {
	result, err := service.RunPlatformAlertEvaluation(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "platform_alert.eval", map[string]interface{}{"evaluated": result.Evaluated, "triggered": result.Triggered, "resolved": result.Resolved})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
