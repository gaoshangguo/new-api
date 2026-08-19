package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetModelAliases 管理端列表（含 deprecated），Root 权限。
func GetModelAliases(c *gin.Context) {
	aliases, err := model.ListModelAliases(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    aliases,
	})
}

// GetPublicModelAliases 公开列表：仅 active 别名（对外名 → 内部模型），
// 供模型目录/价格页展示别名关系。
func GetPublicModelAliases(c *gin.Context) {
	aliases, err := model.ListModelAliases(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	type publicAlias struct {
		AliasName string `json:"alias_name"`
		ModelName string `json:"model_name"`
		Note      string `json:"note,omitempty"`
	}
	items := make([]publicAlias, 0, len(aliases))
	for _, alias := range aliases {
		items = append(items, publicAlias{AliasName: alias.AliasName, ModelName: alias.ModelName, Note: alias.Note})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}

type modelAliasRequest struct {
	AliasName   string `json:"alias_name"`
	ModelName   string `json:"model_name"`
	ChannelIds  string `json:"channel_ids"`
	Status      string `json:"status"`
	Replacement string `json:"replacement"`
	Note        string `json:"note"`
	Reason      string `json:"reason"`
}

// CreateModelAlias 创建模型别名（Root 权限，审计）。
func CreateModelAlias(c *gin.Context) {
	var req modelAliasRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.ChannelIds != "" {
		var ids []int
		if err := common.UnmarshalJsonStr(req.ChannelIds, &ids); err != nil {
			common.ApiErrorMsg(c, "channel_ids must be a JSON array of channel ids")
			return
		}
	}
	actor := businessAuditActor(c)
	alias, err := model.CreateModelAlias(model.ModelAliasCreateParams{
		AliasName:   req.AliasName,
		ModelName:   req.ModelName,
		ChannelIds:  req.ChannelIds,
		Status:      req.Status,
		Replacement: req.Replacement,
		Note:        req.Note,
		Actor:       &actor,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "model_alias.create", map[string]interface{}{
		"alias_name": alias.AliasName,
		"id":         alias.Id,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    alias,
	})
}

// UpdateModelAlias 更新别名（目标变化时版本自增，审计前后值）。
func UpdateModelAlias(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req modelAliasRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.ChannelIds != "" {
		var ids []int
		if err := common.UnmarshalJsonStr(req.ChannelIds, &ids); err != nil {
			common.ApiErrorMsg(c, "channel_ids must be a JSON array of channel ids")
			return
		}
	}
	actor := businessAuditActor(c)
	alias, err := model.UpdateModelAlias(id, model.ModelAliasCreateParams{
		ModelName:   req.ModelName,
		ChannelIds:  req.ChannelIds,
		Status:      req.Status,
		Replacement: req.Replacement,
		Note:        req.Note,
		Actor:       &actor,
	}, req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "model_alias.update", map[string]interface{}{
		"alias_name": alias.AliasName,
		"version":    alias.Version,
		"id":         alias.Id,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    alias,
	})
}

// DeleteModelAlias 删除别名（审计）。
func DeleteModelAlias(c *gin.Context) {
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
	if err := model.DeleteModelAlias(id, &actor, req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "model_alias.delete", map[string]interface{}{
		"id": id,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
