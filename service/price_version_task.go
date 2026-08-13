package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	defaultPriceVersionApplyIntervalSeconds = 60
	minPriceVersionApplyIntervalSeconds     = 30
	maxPriceVersionApplyIntervalSeconds     = 24 * 3600
)

// priceVersionApplyTaskHandler applies pending price versions whose effective
// time has arrived. It is a scheduled system task so the DB-lease dedups
// concurrent master nodes; the optimistic row transition inside
// model.ApplyPendingPriceVersions also tolerates a manual apply-now racing the
// scheduled pass.
type priceVersionApplyTaskHandler struct{}

func init() {
	RegisterSystemTaskHandler(priceVersionApplyTaskHandler{})
}

func (priceVersionApplyTaskHandler) Type() string {
	return model.SystemTaskTypePriceVersionApply
}

func (priceVersionApplyTaskHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("PRICE_VERSION_TASK_ENABLED", true)
}

func (priceVersionApplyTaskHandler) Interval() time.Duration {
	seconds := common.GetEnvOrDefault("PRICE_VERSION_TASK_INTERVAL_SECONDS", defaultPriceVersionApplyIntervalSeconds)
	if seconds < minPriceVersionApplyIntervalSeconds {
		seconds = minPriceVersionApplyIntervalSeconds
	}
	if seconds > maxPriceVersionApplyIntervalSeconds {
		seconds = maxPriceVersionApplyIntervalSeconds
	}
	return time.Duration(seconds) * time.Second
}

func (priceVersionApplyTaskHandler) NewPayload() any {
	return nil
}

func (priceVersionApplyTaskHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	result, err := model.ApplyPendingPriceVersions()
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("price version apply task %s failed to persist result: %v", task.TaskID, err))
	}
}
