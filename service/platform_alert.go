package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// 平台级告警评估（P0-29）：定时任务按窗口聚合指标评估所有启用的规则，
// 触发时创建 active 事件（去重：同规则已有 active 事件则仅更新数值），
// 恢复时把 active 事件标记 resolved；可选的站内/邮件通知。
const (
	defaultPlatformAlertEvalIntervalMinutes = 10
	minPlatformAlertEvalIntervalMinutes     = 1
	maxPlatformAlertEvalIntervalMinutes     = 24 * 60
)

type platformAlertEvalHandler struct{}

func init() {
	RegisterSystemTaskHandler(platformAlertEvalHandler{})
}

func (platformAlertEvalHandler) Type() string {
	return model.SystemTaskTypePlatformAlertEval
}

func (platformAlertEvalHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("PLATFORM_ALERT_EVAL_ENABLED", true)
}

func (platformAlertEvalHandler) Interval() time.Duration {
	minutes := common.GetEnvOrDefault("PLATFORM_ALERT_EVAL_INTERVAL_MINUTES", defaultPlatformAlertEvalIntervalMinutes)
	if minutes < minPlatformAlertEvalIntervalMinutes {
		minutes = minPlatformAlertEvalIntervalMinutes
	}
	if minutes > maxPlatformAlertEvalIntervalMinutes {
		minutes = maxPlatformAlertEvalIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (platformAlertEvalHandler) NewPayload() any {
	return nil
}

func (platformAlertEvalHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := RunPlatformAlertEvaluation(ctx)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("platform alert eval task %s failed to persist result: %v", task.TaskID, err))
	}
}

// PlatformAlertEvalResult 单次评估结果。
type PlatformAlertEvalResult struct {
	Evaluated  int `json:"evaluated"`
	Triggered  int `json:"triggered"`
	Resolved   int `json:"resolved"`
}

// RunPlatformAlertEvaluation 评估全部启用的告警规则。
func RunPlatformAlertEvaluation(ctx context.Context) (*PlatformAlertEvalResult, error) {
	result := &PlatformAlertEvalResult{}
	rules, err := model.ListPlatformAlertRules()
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if ctx != nil && ctx.Err() != nil {
			return result, ctx.Err()
		}
		if !rule.Enabled {
			continue
		}
		result.Evaluated++
		value, err := model.PlatformAlertMetricValue(rule.Metric, rule.WindowMinutes)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("platform alert metric %s evaluation failed: %v", rule.Metric, err))
			continue
		}
		triggered := model.CompareAlertThreshold(value, rule.Threshold, rule.Operator)
		if triggered {
			active, err := model.GetActivePlatformAlertEvent(rule.Id)
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("platform alert active lookup failed: %v", err))
				continue
			}
			if active == nil {
				event, err := model.CreatePlatformAlertEvent(rule, value)
				if err != nil {
					logger.LogWarn(ctx, fmt.Sprintf("platform alert event create failed: %v", err))
					continue
				}
				result.Triggered++
				if rule.NotifyEnabled {
					notifyPlatformAlert(event)
				}
			} else {
				if err := model.UpdatePlatformAlertEventValue(active.Id, value); err != nil {
					logger.LogWarn(ctx, fmt.Sprintf("platform alert event value update failed: %v", err))
				}
			}
		} else {
			resolved, err := model.ResolveActivePlatformAlertEvent(rule.Id)
			if err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("platform alert event resolve failed: %v", err))
				continue
			}
			if resolved {
				result.Resolved++
			}
		}
	}
	if result.Triggered > 0 || result.Resolved > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("platform alert evaluation: %d evaluated, %d triggered, %d resolved", result.Evaluated, result.Triggered, result.Resolved))
	}
	return result, nil
}

// notifyPlatformAlert 站内/邮件通知：通知根管理员（邮件走其通知设置）。
func notifyPlatformAlert(event *model.PlatformAlertEvent) {
	NotifyRootUser(dto.NotifyTypePlatformAlert, "Platform alert: "+event.RuleName, event.Message)
}
