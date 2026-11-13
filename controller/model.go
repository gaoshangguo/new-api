package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/ai360"
	"github.com/QuantumNous/new-api/relay/channel/lingyiwanwu"
	"github.com/QuantumNous/new-api/relay/channel/minimax"
	"github.com/QuantumNous/new-api/relay/channel/moonshot"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// https://platform.openai.com/docs/api-reference/models/list

var openAIModels []dto.OpenAIModels
var openAIModelsMap map[string]dto.OpenAIModels
var channelId2Models map[int][]string

func init() {
	// https://platform.openai.com/docs/models/model-endpoint-compatibility
	for i := 0; i < constant.APITypeDummy; i++ {
		if i == constant.APITypeAIProxyLibrary {
			continue
		}
		adaptor := relay.GetAdaptor(i)
		channelName := adaptor.GetChannelName()
		modelNames := adaptor.GetModelList()
		for _, modelName := range modelNames {
			openAIModels = append(openAIModels, dto.OpenAIModels{
				Id:      modelName,
				Object:  "model",
				Created: 1626777600,
				OwnedBy: channelName,
			})
		}
	}
	for _, modelName := range ai360.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: ai360.ChannelName,
		})
	}
	for _, modelName := range moonshot.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: moonshot.ChannelName,
		})
	}
	for _, modelName := range lingyiwanwu.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: lingyiwanwu.ChannelName,
		})
	}
	for _, modelName := range minimax.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: minimax.ChannelName,
		})
	}
	for modelName, _ := range constant.MidjourneyModel2Action {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: "midjourney",
		})
	}
	openAIModelsMap = make(map[string]dto.OpenAIModels)
	for _, aiModel := range openAIModels {
		openAIModelsMap[aiModel.Id] = aiModel
	}
	channelId2Models = make(map[int][]string)
	for i := 1; i <= constant.ChannelTypeDummy; i++ {
		apiType, success := common.ChannelType2APIType(i)
		if !success || apiType == constant.APITypeAIProxyLibrary {
			continue
		}
		meta := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: i,
		}}
		adaptor := relay.GetAdaptor(apiType)
		adaptor.Init(meta)
		channelId2Models[i] = adaptor.GetModelList()
	}
	openAIModels = lo.UniqBy(openAIModels, func(m dto.OpenAIModels) string {
		return m.Id
	})
}

func channelOwnerName(channelType int) string {
	apiType, success := common.ChannelType2APIType(channelType)
	if !success {
		return strings.ToLower(constant.GetChannelTypeName(channelType))
	}
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return strings.ToLower(constant.GetChannelTypeName(channelType))
	}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: channelType,
	}})
	if name := strings.TrimSpace(adaptor.GetChannelName()); name != "" {
		return name
	}
	return strings.ToLower(constant.GetChannelTypeName(channelType))
}

func getPreferredModelOwners(modelNames []string, groups []string) map[string]string {
	channelTypes, err := model.GetPreferredModelOwnerChannelTypes(modelNames, groups)
	if err != nil {
		common.SysLog(fmt.Sprintf("GetPreferredModelOwnerChannelTypes error: %v", err))
		return map[string]string{}
	}

	ownerByChannelType := make(map[int]string)
	owners := make(map[string]string, len(channelTypes))
	for modelName, channelType := range channelTypes {
		owner, ok := ownerByChannelType[channelType]
		if !ok {
			owner = channelOwnerName(channelType)
			ownerByChannelType[channelType] = owner
		}
		if owner != "" {
			owners[modelName] = owner
		}
	}
	return owners
}

func buildOpenAIModel(modelName string, ownerByModel map[string]string, aliasByTarget map[string][]string) dto.OpenAIModels {
	var oaiModel dto.OpenAIModels
	if staticModel, ok := openAIModelsMap[modelName]; ok {
		oaiModel = staticModel
	} else {
		oaiModel = dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: "custom",
		}
	}
	if owner, ok := ownerByModel[modelName]; ok && owner != "" {
		oaiModel.OwnedBy = owner
	}
	oaiModel.SupportedEndpointTypes = model.GetModelSupportEndpointTypes(modelName)
	oaiModel.Aliases = aliasByTarget[modelName]
	return oaiModel
}

type modelListGroups struct {
	userGroup   string
	tokenGroup  string
	ownerGroups []string
}

func getModelListGroups(c *gin.Context) (modelListGroups, error) {
	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	if userGroup == "" && (tokenGroup == "" || tokenGroup == "auto") {
		var err error
		userGroup, err = model.GetUserGroup(c.GetInt("id"), false)
		if err != nil {
			return modelListGroups{}, err
		}
	}

	if tokenGroup == "auto" {
		return modelListGroups{
			userGroup:   userGroup,
			tokenGroup:  tokenGroup,
			ownerGroups: service.GetRequestAutoGroups(c, userGroup),
		}, nil
	}

	group := userGroup
	if tokenGroup != "" {
		group = tokenGroup
	}
	return modelListGroups{
		userGroup:   userGroup,
		tokenGroup:  tokenGroup,
		ownerGroups: []string{group},
	}, nil
}

// publicModelEntry 是一个对外模型条目及其真实目标模型名。
type publicModelEntry struct {
	Model  dto.OpenAIModels
	Target string
}

// listPublicModelEntries 计算当前请求可见的对外模型列表：
// 分组启用模型 → token 模型限制/计费配置过滤 → 别名展开。
// 过滤阶段始终按真实模型名判断，保证可见性与计费配置不变。
func listPublicModelEntries(c *gin.Context) ([]publicModelEntry, error) {
	acceptUnsetRatioModel := operation_setting.SelfUseModeEnabled
	if !acceptUnsetRatioModel {
		userId := c.GetInt("id")
		if userId > 0 {
			userSettings, _ := model.GetUserSetting(userId, false)
			if userSettings.AcceptUnsetRatioModel {
				acceptUnsetRatioModel = true
			}
		}
	}

	groups, err := getModelListGroups(c)
	if err != nil {
		return nil, err
	}
	ownerGroups := groups.ownerGroups

	modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
	var tokenModelLimit map[string]bool
	if modelLimitEnable {
		s, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
		if ok {
			tokenModelLimit, _ = s.(map[string]bool)
		}
		if tokenModelLimit == nil {
			tokenModelLimit = map[string]bool{}
		}
	}

	exposeAliases := model_setting.ShouldExposeModelAliases()
	var usableAliasesByTarget map[string][]model.ModelAliasEntry
	if exposeAliases {
		usableAliasesByTarget = model.GetUsableAliasEntriesByTarget(ownerGroups)
	}

	models := service.GetGroupsEnabledModels(ownerGroups)
	visibleModelNames := make([]string, 0, len(models))
	for _, modelName := range models {
		if modelLimitEnable {
			matchingName := ratio_setting.FormatMatchingModelName(modelName)
			allowedByAlias := false
			for _, entry := range usableAliasesByTarget[modelName] {
				if tokenModelLimit[entry.AliasName] {
					allowedByAlias = true
					break
				}
			}
			if !tokenModelLimit[modelName] && !tokenModelLimit[matchingName] && !allowedByAlias {
				continue
			}
		}
		if !acceptUnsetRatioModel && !helper.HasModelBillingConfig(modelName) {
			continue
		}
		visibleModelNames = append(visibleModelNames, modelName)
	}

	ownerByModel := map[string]string{}
	if len(ownerGroups) > 0 {
		ownerByModel = getPreferredModelOwners(visibleModelNames, ownerGroups)
	}

	if !exposeAliases {
		aliasByTarget := model.GetModelAliasesByTarget()
		entries := make([]publicModelEntry, 0, len(visibleModelNames))
		for _, modelName := range visibleModelNames {
			entries = append(entries, publicModelEntry{
				Model:  buildOpenAIModel(modelName, ownerByModel, aliasByTarget),
				Target: modelName,
			})
		}
		return entries, nil
	}

	// alias 模式：有 active 别名的模型只暴露别名；即使别名因渠道限制暂时不可用，
	// 也不回退展示真实名，避免通过列表反推上游模型。
	allAliasesByTarget := model.GetActiveAliasEntriesByTarget()
	shadowedNames := make(map[string]struct{}, len(visibleModelNames))
	for _, modelName := range visibleModelNames {
		for _, entry := range allAliasesByTarget[modelName] {
			shadowedNames[entry.AliasName] = struct{}{}
		}
	}

	entries := make([]publicModelEntry, 0, len(visibleModelNames))
	seen := make(map[string]struct{}, len(visibleModelNames))
	for _, modelName := range visibleModelNames {
		if len(allAliasesByTarget[modelName]) > 0 {
			for _, entry := range usableAliasesByTarget[modelName] {
				if _, exists := seen[entry.AliasName]; exists {
					continue
				}
				seen[entry.AliasName] = struct{}{}
				entries = append(entries, publicModelEntry{
					Model:  buildAliasModel(entry.AliasName, modelName, ownerByModel),
					Target: modelName,
				})
			}
			continue
		}
		if _, shadowed := shadowedNames[modelName]; shadowed {
			continue
		}
		if _, exists := seen[modelName]; exists {
			continue
		}
		seen[modelName] = struct{}{}
		entries = append(entries, publicModelEntry{
			Model:  buildOpenAIModel(modelName, ownerByModel, nil),
			Target: modelName,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Model.Id < entries[j].Model.Id
	})
	return entries, nil
}

// buildAliasModel 构造对外只暴露别名的模型条目。
// owned_by 沿用该目标模型的渠道 owner，不额外暴露目标模型名。
func buildAliasModel(aliasName, targetName string, ownerByModel map[string]string) dto.OpenAIModels {
	entry := dto.OpenAIModels{
		Id:      aliasName,
		Object:  "model",
		Created: 1626777600,
		OwnedBy: "custom",
	}
	if owner, ok := ownerByModel[targetName]; ok && owner != "" {
		entry.OwnedBy = owner
	}
	entry.SupportedEndpointTypes = model.GetModelSupportEndpointTypes(targetName)
	return entry
}

// publicModelNamesForGroups 把分组启用的真实模型名展开为对外模型名：
// alias 模式下有可用别名的模型返回别名，无别名的模型保持原名；
// 别名存在但当前分组不可用时，不返回该模型（不回退真实名）。
func publicModelNamesForGroups(models []string, groups []string) []string {
	if !model_setting.ShouldExposeModelAliases() {
		return models
	}
	allAliasesByTarget := model.GetActiveAliasEntriesByTarget()
	usableAliasesByTarget := model.GetUsableAliasEntriesByTarget(groups)
	names := make([]string, 0, len(models))
	for _, modelName := range models {
		if len(allAliasesByTarget[modelName]) > 0 {
			for _, entry := range usableAliasesByTarget[modelName] {
				names = append(names, entry.AliasName)
			}
			continue
		}
		names = append(names, modelName)
	}
	return names
}

func ListModels(c *gin.Context, modelType int) {
	entries, err := listPublicModelEntries(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "get user group failed",
		})
		return
	}

	userOpenAiModels := make([]dto.OpenAIModels, 0, len(entries))
	for _, entry := range entries {
		userOpenAiModels = append(userOpenAiModels, entry.Model)
	}

	switch modelType {
	case constant.ChannelTypeAnthropic:
		useranthropicModels := make([]dto.AnthropicModel, len(userOpenAiModels))
		for i, model := range userOpenAiModels {
			useranthropicModels[i] = dto.AnthropicModel{
				ID:          model.Id,
				CreatedAt:   time.Unix(int64(model.Created), 0).UTC().Format(time.RFC3339),
				DisplayName: model.Id,
				Type:        "model",
			}
		}
		firstID := ""
		lastID := ""
		if len(useranthropicModels) > 0 {
			firstID = useranthropicModels[0].ID
			lastID = useranthropicModels[len(useranthropicModels)-1].ID
		}
		c.JSON(200, gin.H{
			"data":     useranthropicModels,
			"first_id": firstID,
			"has_more": false,
			"last_id":  lastID,
		})
	case constant.ChannelTypeGemini:
		userGeminiModels := make([]dto.GeminiModel, len(userOpenAiModels))
		for i, model := range userOpenAiModels {
			userGeminiModels[i] = dto.GeminiModel{
				Name:        model.Id,
				DisplayName: model.Id,
			}
		}
		c.JSON(200, gin.H{
			"models":        userGeminiModels,
			"nextPageToken": nil,
		})
	default:
		c.JSON(200, gin.H{
			"success": true,
			"data":    userOpenAiModels,
			"object":  "list",
		})
	}
}

func ChannelListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    openAIModels,
	})
}

func DashboardListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    channelId2Models,
	})
}

func EnabledListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    model.GetEnabledModels(),
	})
}

func respondSingleModel(c *gin.Context, modelType int, aiModel dto.OpenAIModels) {
	switch modelType {
	case constant.ChannelTypeAnthropic:
		c.JSON(200, dto.AnthropicModel{
			ID:          aiModel.Id,
			CreatedAt:   time.Unix(int64(aiModel.Created), 0).UTC().Format(time.RFC3339),
			DisplayName: aiModel.Id,
			Type:        "model",
		})
	default:
		c.JSON(200, aiModel)
	}
}

func respondModelNotFound(c *gin.Context, modelId string) {
	openAIError := types.OpenAIError{
		Message: fmt.Sprintf("The model '%s' does not exist", modelId),
		Type:    "invalid_request_error",
		Param:   "model",
		Code:    "model_not_found",
	}
	c.JSON(200, gin.H{
		"error": openAIError,
	})
}

func RetrieveModel(c *gin.Context, modelType int) {
	modelId := c.Param("model")

	// 先按当前请求可见的对外模型列表查找，保证单模型查询与列表使用同一套
	// 分组/计费/别名可见性规则。
	if entries, err := listPublicModelEntries(c); err == nil {
		for _, entry := range entries {
			if entry.Model.Id == modelId {
				respondSingleModel(c, modelType, entry.Model)
				return
			}
		}
		if model_setting.ShouldExposeModelAliases() {
			// 别名模式下的隐藏面：别名本身、以及被别名替代的真实模型名，
			// 不通过单模型查询接口回显，避免反推真实上游模型名。
			if _, isAlias := model.GetModelAliasEntry(modelId); isAlias {
				respondModelNotFound(c, modelId)
				return
			}
			if aliases := model.GetActiveAliasEntriesByTarget()[modelId]; len(aliases) > 0 {
				respondModelNotFound(c, modelId)
				return
			}
		}
	}

	if aiModel, ok := openAIModelsMap[modelId]; ok {
		respondSingleModel(c, modelType, aiModel)
		return
	}
	respondModelNotFound(c, modelId)
}
