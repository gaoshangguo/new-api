package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// GetPriceVersions lists price version metadata (snapshots served on detail).
func GetPriceVersions(c *gin.Context) {
	startIdx, err := strconv.Atoi(c.DefaultQuery("page", "0"))
	if err != nil || startIdx < 0 {
		startIdx = 0
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if err != nil || pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	items, total, err := model.ListPriceVersions(startIdx, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
		"total":   total,
	})
}

// GetPriceVersion returns one version including its full snapshot payload.
func GetPriceVersion(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	version, err := model.GetPriceVersion(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    version,
	})
}

type createPriceVersionRequest struct {
	Name        string            `json:"name"`
	EffectiveAt int64             `json:"effective_at"`
	Overlay     map[string]string `json:"overlay"`
	Reason      string            `json:"reason"`
}

// CreatePriceVersion creates a new price version. Without an overlay, the
// snapshot freezes the current live prices (useful for an auditable price
// "record point" or a scheduled no-op). With an overlay, the provided maps
// replace the live values inside the new version. When effective_at has
// already passed, the version is applied immediately; otherwise it stays
// pending and the scheduled task applies it at the effective time.
func CreatePriceVersion(c *gin.Context) {
	var req createPriceVersionRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.EffectiveAt <= 0 {
		req.EffectiveAt = common.GetTimestamp()
	}
	if len(req.Overlay) > 0 {
		for key := range req.Overlay {
			if !ratio_setting.IsPriceSnapshotKey(key) {
				common.ApiErrorMsg(c, "overlay key "+key+" is not part of a price snapshot")
				return
			}
		}
	}
	actor := businessAuditActor(c)
	version, err := model.CreatePriceVersion(model.PriceVersionCreateParams{
		Name:        req.Name,
		EffectiveAt: req.EffectiveAt,
		Overlay:     req.Overlay,
		CreatedBy:   c.GetInt("id"),
		Reason:      req.Reason,
		Actor:       &actor,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "price_version.create", map[string]interface{}{
		"version_id": version.Id,
		"name":       version.Name,
		"status":     version.Status,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    version,
	})
}

// ApplyPriceVersionNow forces a pending price version active immediately.
func ApplyPriceVersionNow(c *gin.Context) {
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
	err = model.ApplyPriceVersionNow(id, &actor, req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "price_version.apply", map[string]interface{}{
		"version_id": id,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
