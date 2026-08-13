package channel

import (
	"github.com/QuantumNous/new-api/constant"
)

// 适配器能力声明（P0-13）：每个渠道类型显式声明工具、视觉、思考、流式、
// 超时与计费能力，管理端与公开接口可查询。能力位是渠道家族级默认值；
// 个别模型可能具备超出渠道默认的能力，以模型目录/上游为准。

const (
	BillingModeTokenBased = "token_based"
	BillingModePerCall    = "per_call"
)

// ChannelCapabilities 是渠道类型的能力位声明。
type ChannelCapabilities struct {
	SupportsTools     bool   `json:"supports_tools"`
	SupportsVision    bool   `json:"supports_vision"`
	SupportsThinking  bool   `json:"supports_thinking"`
	SupportsStreaming bool   `json:"supports_streaming"`
	SupportsTimeout   bool   `json:"supports_timeout"`
	BillingMode       string `json:"billing_mode"`
}

// chat 家族默认：工具 + 视觉 + 流式 + 超时，按 token 计费。
func chatCapabilities(tools, vision, thinking bool) ChannelCapabilities {
	return ChannelCapabilities{
		SupportsTools:     tools,
		SupportsVision:    vision,
		SupportsThinking:  thinking,
		SupportsStreaming: true,
		SupportsTimeout:   true,
		BillingMode:       BillingModeTokenBased,
	}
}

// 任务/按次计费家族：无工具/视觉/思考/流式，按次计费。
func perCallCapabilities() ChannelCapabilities {
	return ChannelCapabilities{
		SupportsTimeout: true,
		BillingMode:     BillingModePerCall,
	}
}

// 最小能力（Embedding/Rerank 等非对话用途）。
func minimalCapabilities() ChannelCapabilities {
	return ChannelCapabilities{
		SupportsTimeout: true,
		BillingMode:     BillingModeTokenBased,
	}
}

// fullChat 通用对话渠道：工具+视觉+流式+超时（思考视厂商而定）。
var fullChat = chatCapabilities(true, true, false)

// thinkingChat 支持思考链的对话渠道。
var thinkingChat = chatCapabilities(true, true, true)

// 渠道类型 → 能力声明。未列出渠道回退 minimalCapabilities；
// 任务平台（视频/MJ/Suno）使用 perCallCapabilities。
var channelCapabilityTable = map[int]ChannelCapabilities{
	constant.ChannelTypeOpenAI:         thinkingChat,
	constant.ChannelTypeAzure:          thinkingChat,
	constant.ChannelTypeOllama:         chatCapabilities(true, true, false),
	constant.ChannelTypeOpenAIMax:      fullChat,
	constant.ChannelTypeOhMyGPT:        fullChat,
	constant.ChannelTypeCustom:         fullChat,
	constant.ChannelTypeAILS:           fullChat,
	constant.ChannelTypeAIProxy:        fullChat,
	constant.ChannelTypePaLM:           chatCapabilities(false, false, false),
	constant.ChannelTypeAPI2GPT:        fullChat,
	constant.ChannelTypeAIGC2D:         fullChat,
	constant.ChannelTypeAnthropic:      thinkingChat,
	constant.ChannelTypeBaidu:          chatCapabilities(true, true, false),
	constant.ChannelTypeZhipu:          thinkingChat,
	constant.ChannelTypeAli:            chatCapabilities(true, true, false),
	constant.ChannelTypeXunfei:         chatCapabilities(true, false, false),
	constant.ChannelType360:            chatCapabilities(true, false, false),
	constant.ChannelTypeOpenRouter:     thinkingChat,
	constant.ChannelTypeAIProxyLibrary: fullChat,
	constant.ChannelTypeFastGPT:        chatCapabilities(true, false, false),
	constant.ChannelTypeTencent:        chatCapabilities(true, true, false),
	constant.ChannelTypeGemini:         thinkingChat,
	constant.ChannelTypeMoonshot:       thinkingChat,
	constant.ChannelTypeZhipu_v4:       thinkingChat,
	constant.ChannelTypePerplexity:     chatCapabilities(true, false, false),
	constant.ChannelTypeLingYiWanWu:    chatCapabilities(true, true, false),
	constant.ChannelTypeAws:            chatCapabilities(true, true, false),
	constant.ChannelTypeCohere:         chatCapabilities(true, false, false),
	constant.ChannelTypeMiniMax:        thinkingChat,
	constant.ChannelTypeDify:           chatCapabilities(true, false, false),
	constant.ChannelTypeJina:           minimalCapabilities(),
	constant.ChannelCloudflare:         chatCapabilities(true, true, false),
	constant.ChannelTypeSiliconFlow:    chatCapabilities(true, true, false),
	constant.ChannelTypeVertexAi:       thinkingChat,
	constant.ChannelTypeMistral:        chatCapabilities(true, true, false),
	constant.ChannelTypeDeepSeek:       thinkingChat,
	constant.ChannelTypeMokaAI:         chatCapabilities(true, false, false),
	constant.ChannelTypeVolcEngine:     thinkingChat,
	constant.ChannelTypeBaiduV2:        chatCapabilities(true, true, false),
	constant.ChannelTypeXinference:     chatCapabilities(true, true, false),
	constant.ChannelTypeXai:            thinkingChat,
	constant.ChannelTypeCoze:           chatCapabilities(true, false, false),
	constant.ChannelTypeSubmodel:       thinkingChat,
	constant.ChannelTypeCodex:          chatCapabilities(true, false, false),
	constant.ChannelTypeAdvancedCustom: fullChat,
	constant.ChannelTypeSub2API:        fullChat,
	constant.ChannelTypeNewAPI:         fullChat,

	constant.ChannelTypeMidjourney:     perCallCapabilities(),
	constant.ChannelTypeMidjourneyPlus: perCallCapabilities(),
	constant.ChannelTypeSunoAPI:        perCallCapabilities(),
	constant.ChannelTypeKling:          perCallCapabilities(),
	constant.ChannelTypeJimeng:         perCallCapabilities(),
	constant.ChannelTypeVidu:           perCallCapabilities(),
	constant.ChannelTypeDoubaoVideo:    perCallCapabilities(),
	constant.ChannelTypeSora:           perCallCapabilities(),
	constant.ChannelTypeReplicate:      perCallCapabilities(),
}

// GetChannelCapabilities 返回渠道类型的能力声明。
func GetChannelCapabilities(channelType int) ChannelCapabilities {
	if caps, ok := channelCapabilityTable[channelType]; ok {
		return caps
	}
	return minimalCapabilities()
}

// GetChannelCapabilitiesMap 返回全部渠道类型的能力声明（管理端/公开查询）。
func GetChannelCapabilitiesMap() map[string]ChannelCapabilities {
	result := make(map[string]ChannelCapabilities, len(channelCapabilityTable))
	for channelType, caps := range channelCapabilityTable {
		result[constant.GetChannelTypeName(channelType)] = caps
	}
	return result
}
