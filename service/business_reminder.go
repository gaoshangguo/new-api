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

const (
	defaultBusinessReminderTaskIntervalMinutes = 360
	minBusinessReminderTaskIntervalMinutes     = 60
	maxBusinessReminderTaskIntervalMinutes     = 7 * 24 * 60
)

// businessReminderTaskHandler is deliberately registered in service init so it
// is available before StartSystemTaskRunner, without coupling the reminder
// domain to an HTTP controller. Scheduled runs create durable in-app rows;
// optional external delivery is controlled separately below.
type businessReminderTaskHandler struct{}

func init() {
	RegisterSystemTaskHandler(businessReminderTaskHandler{})
}

func (businessReminderTaskHandler) Type() string {
	return model.SystemTaskTypeBusinessReminderScan
}

func (businessReminderTaskHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("BUSINESS_REMINDER_TASK_ENABLED", true)
}

func (businessReminderTaskHandler) Interval() time.Duration {
	minutes := common.GetEnvOrDefault("BUSINESS_REMINDER_TASK_INTERVAL_MINUTES", defaultBusinessReminderTaskIntervalMinutes)
	if minutes < minBusinessReminderTaskIntervalMinutes {
		minutes = minBusinessReminderTaskIntervalMinutes
	}
	if minutes > maxBusinessReminderTaskIntervalMinutes {
		minutes = maxBusinessReminderTaskIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (businessReminderTaskHandler) NewPayload() any {
	return nil
}

func (businessReminderTaskHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	summary, err := RunBusinessReminderScan(ctx)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, summary, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("business reminder task %s failed to persist result: %v", task.TaskID, err))
	}
}

// EnqueueBusinessReminderScan queues an explicit scan through the same
// database-leased SystemTask path as scheduled runs. It never evaluates or
// sends notifications synchronously in the request handler.
func EnqueueBusinessReminderScan() (*model.SystemTask, bool, error) {
	return EnqueueSystemTask(model.SystemTaskTypeBusinessReminderScan, nil)
}

// RunBusinessReminderScan persists project reminder state and, only when the
// operator has explicitly enabled external delivery, hands newly-active (or
// previously undelivered) reminders to the existing NotifyUser path. The
// database record is the in-app notification and is saved before delivery, so
// mail/webhook failures cannot erase an alert.
func RunBusinessReminderScan(ctx context.Context) (model.BusinessReminderScanResult, error) {
	options := model.BusinessReminderScanOptions{
		InactiveAfterSeconds: int64(normalizedBusinessReminderEnv("BUSINESS_REMINDER_INACTIVE_DAYS", 30, 1, 3650)) * 24 * 60 * 60,
		ErrorWindowSeconds:   int64(normalizedBusinessReminderEnv("BUSINESS_REMINDER_ERROR_WINDOW_HOURS", 24, 1, 24*30)) * 60 * 60,
		ErrorRatePercent:     normalizedBusinessReminderEnv("BUSINESS_REMINDER_ERROR_RATE_PERCENT", 10, 1, 100),
		ErrorMinRequestCount: int64(normalizedBusinessReminderEnv("BUSINESS_REMINDER_ERROR_MIN_REQUESTS", 10, 1, 1000000)),
	}
	summary, candidates, err := model.ScanBusinessProjectReminders(ctx, options)
	if err != nil {
		return summary, err
	}

	// Default false is intentional. Persisted in-app reminders work out of the
	// box, while deployments opt into outbound messages only after configuring
	// their existing SMTP/notification channel and user notification settings.
	if !common.GetEnvOrDefaultBool("BUSINESS_REMINDER_NOTIFY_ENABLED", false) {
		return summary, nil
	}

	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		if candidate.Reminder == nil || candidate.Owner == nil {
			continue
		}
		notification := dto.NewNotify(
			dto.NotifyTypeBusinessReminder,
			candidate.Reminder.Title,
			candidate.Reminder.Content,
			nil,
		)
		if err := NotifyUser(candidate.Owner.Id, candidate.Owner.Email, candidate.Owner.GetSetting(), notification); err != nil {
			summary.FailedNotificationCount++
			logger.LogWarn(ctx, fmt.Sprintf("business reminder notification failed: reminder_id=%d user_id=%d err=%v", candidate.Reminder.Id, candidate.Owner.Id, err))
			continue
		}
		if err := model.MarkBusinessProjectReminderNotified(ctx, candidate.Reminder.Id, common.GetTimestamp()); err != nil {
			summary.FailedNotificationCount++
			logger.LogWarn(ctx, fmt.Sprintf("business reminder notification mark failed: reminder_id=%d err=%v", candidate.Reminder.Id, err))
			continue
		}
		summary.DeliveredNotificationCount++
	}
	return summary, nil
}

func normalizedBusinessReminderEnv(name string, defaultValue, minimum, maximum int) int {
	value := common.GetEnvOrDefault(name, defaultValue)
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
