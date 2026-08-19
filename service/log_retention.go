package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

// 数据保留（P0-31）：可配置保留周期的自动清理调度。
// 环境变量：
//   - LOG_RETENTION_TASK_ENABLED（默认 true）：开关
//   - LOG_RETENTION_DAYS（默认 90，1～3650）：日志保留天数
//   - LOG_RETENTION_TASK_INTERVAL_HOURS（默认 24，1～720）：清理间隔
const (
	defaultLogRetentionDays    = 90
	minLogRetentionDays        = 1
	maxLogRetentionDays        = 3650
	defaultLogRetentionIntervalHours = 24
	minLogRetentionIntervalHours     = 1
	maxLogRetentionIntervalHours     = 720
)

type logRetentionHandler struct{}

func init() {
	RegisterSystemTaskHandler(logRetentionHandler{})
}

func (logRetentionHandler) Type() string {
	return model.SystemTaskTypeLogRetention
}

func (logRetentionHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("LOG_RETENTION_TASK_ENABLED", true)
}

func (logRetentionHandler) Interval() time.Duration {
	hours := common.GetEnvOrDefault("LOG_RETENTION_TASK_INTERVAL_HOURS", defaultLogRetentionIntervalHours)
	if hours < minLogRetentionIntervalHours {
		hours = minLogRetentionIntervalHours
	}
	if hours > maxLogRetentionIntervalHours {
		hours = maxLogRetentionIntervalHours
	}
	return time.Duration(hours) * time.Hour
}

func (logRetentionHandler) NewPayload() any {
	return nil
}

func (logRetentionHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	days := common.GetEnvOrDefault("LOG_RETENTION_DAYS", defaultLogRetentionDays)
	if days < minLogRetentionDays {
		days = minLogRetentionDays
	}
	if days > maxLogRetentionDays {
		days = maxLogRetentionDays
	}
	target := common.GetTimestamp() - int64(days)*24*3600
	result, err := runLogCleanupToTarget(ctx, target)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("log retention task %s failed to persist result: %v", task.TaskID, err))
	}
}

// runLogCleanupToTarget 删除 created_at < target 的业务日志（分批复用
// CountOldLogExcludingAudit/DeleteOldLogBatchExcludingAudit）；审计日志
// （LogTypeManage）与登录日志（LogTypeLogin）属于不可抵赖审计轨迹，
// 不计入保留期自动清理，需显式管理。返回清理统计。
func runLogCleanupToTarget(ctx context.Context, target int64) (LogCleanupResult, error) {
	summary := LogCleanupResult{}
	for {
		remaining, err := model.CountOldLogExcludingAudit(ctx, target)
		if err != nil {
			return summary, err
		}
		if remaining == 0 {
			break
		}
		rowsAffected, err := model.DeleteOldLogBatchExcludingAudit(ctx, target, logCleanupBatchSize)
		if err != nil {
			return summary, err
		}
		if rowsAffected == 0 {
			break // 残留行无法删除，停止循环避免忙等
		}
		summary.DeletedCount += rowsAffected
	}
	return summary, nil
}
