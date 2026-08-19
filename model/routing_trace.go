package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

// 结构化路由轨迹（P0-15）：请求级记录候选渠道、每次尝试的渠道与失败原因、
// 最终渠道与结果。用户可见摘要（重试次数/最终渠道）与管理员可见明细
// （候选列表、逐次失败原因）分别写入日志 Other 与 admin_info。

// RoutingAttempt 记录一次渠道尝试的结果。
type RoutingAttempt struct {
	ChannelId int    `json:"channel_id"`
	Code      string `json:"code,omitempty"`   // 稳定错误码（如 rate_limit_exceeded）
	Reason    string `json:"reason,omitempty"` // 脱敏后的失败原因
}

// RoutingTrace 是单个请求的路由轨迹。
type RoutingTrace struct {
	Candidates   []int            `json:"candidates,omitempty"`   // 候选渠道（选择前过滤后的集合）
	Attempts     []RoutingAttempt `json:"attempts,omitempty"`     // 按序尝试记录
	FinalChannel int              `json:"final_channel,omitempty"` // 最终成功/最后一次尝试渠道
	FinalSuccess bool             `json:"final_success"`          // 是否成功
}

// InitRoutingTrace 初始化请求路由轨迹（幂等）。
func InitRoutingTrace(c *gin.Context) {
	if c == nil {
		return
	}
	if _, exists := common.GetContextKey(c, constant.ContextKeyRoutingTrace); !exists {
		common.SetContextKey(c, constant.ContextKeyRoutingTrace, &RoutingTrace{})
	}
}

// RecordRoutingCandidates 记录候选渠道集合（选择前）。
func RecordRoutingCandidates(c *gin.Context, candidates []int) {
	trace := GetRoutingTrace(c)
	if trace == nil || len(candidates) == 0 {
		return
	}
	trace.Candidates = candidates
}

// RecordRoutingAttempt 追加一次渠道尝试（失败原因脱敏由调用方保证）。
func RecordRoutingAttempt(c *gin.Context, channelID int, code, reason string) {
	trace := GetRoutingTrace(c)
	if trace == nil || channelID <= 0 {
		return
	}
	trace.Attempts = append(trace.Attempts, RoutingAttempt{
		ChannelId: channelID,
		Code:      code,
		Reason:    reason,
	})
}

// MarkRoutingFinal 标记最终渠道与结果。
func MarkRoutingFinal(c *gin.Context, channelID int, success bool) {
	trace := GetRoutingTrace(c)
	if trace == nil {
		return
	}
	trace.FinalChannel = channelID
	trace.FinalSuccess = success
}

// GetRoutingTrace 返回请求路由轨迹（未初始化时为 nil）。
func GetRoutingTrace(c *gin.Context) *RoutingTrace {
	if c == nil {
		return nil
	}
	trace, _ := common.GetContextKeyType[*RoutingTrace](c, constant.ContextKeyRoutingTrace)
	return trace
}

// RoutingSummary 是用户可见的路由摘要。
type RoutingSummary struct {
	Attempts      int  `json:"attempts"`
	Retried       bool `json:"retried"`
	FinalChannel  int  `json:"final_channel,omitempty"`
	FinalSuccess  bool `json:"final_success"`
}

// RoutingSummary 从轨迹构建用户可见摘要。
func (t *RoutingTrace) RoutingSummary() RoutingSummary {
	if t == nil {
		return RoutingSummary{}
	}
	return RoutingSummary{
		Attempts:     len(t.Attempts),
		Retried:      len(t.Attempts) > 1,
		FinalChannel: t.FinalChannel,
		FinalSuccess: t.FinalSuccess,
	}
}
