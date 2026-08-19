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
	defaultChannelKeyExpiryIntervalMinutes     = 360
	minChannelKeyExpiryIntervalMinutes         = 60
	maxChannelKeyExpiryIntervalMinutes         = 7 * 24 * 60
	defaultChannelKeyExpiryAdvanceDays         = 7
	defaultChannelKeyExpiryNotifyIntervalHours = 24
)

// channelKeyExpiryTaskHandler periodically scans channels whose key expires
// within the advance window and notifies root. State keeps per-channel last
// notify timestamps so the same channel is not alerted on every run.
type channelKeyExpiryTaskHandler struct{}

func init() {
	RegisterSystemTaskHandler(channelKeyExpiryTaskHandler{})
}

func (channelKeyExpiryTaskHandler) Type() string { return model.SystemTaskTypeChannelKeyExpiry }

func (channelKeyExpiryTaskHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("CHANNEL_KEY_EXPIRY_TASK_ENABLED", true)
}

func (channelKeyExpiryTaskHandler) Interval() time.Duration {
	minutes := common.GetEnvOrDefault("CHANNEL_KEY_EXPIRY_TASK_INTERVAL_MINUTES", defaultChannelKeyExpiryIntervalMinutes)
	if minutes < minChannelKeyExpiryIntervalMinutes {
		minutes = minChannelKeyExpiryIntervalMinutes
	}
	if minutes > maxChannelKeyExpiryIntervalMinutes {
		minutes = maxChannelKeyExpiryIntervalMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (channelKeyExpiryTaskHandler) NewPayload() any { return nil }

func (channelKeyExpiryTaskHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	var state ChannelKeyExpiryState
	if err := task.DecodeState(&state); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	opts := ChannelKeyExpiryOptions{
		AdvanceSeconds:       int64(normalizedExpiryEnv("CHANNEL_KEY_EXPIRY_ADVANCE_DAYS", defaultChannelKeyExpiryAdvanceDays, 1, 90)) * 24 * 60 * 60,
		MinNotifyIntervalSec: int64(defaultChannelKeyExpiryNotifyIntervalHours) * 60 * 60,
	}
	result, err := ScanChannelKeyExpiry(ctx, opts, &state)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	// 检测与外发分离：命中提醒默认只写系统任务结果与日志，须显式开启
	// CHANNEL_KEY_EXPIRY_NOTIFY_ENABLED 才调用 NotifyRootUser 外部通知。
	notifyEnabled := common.GetEnvOrDefaultBool("CHANNEL_KEY_EXPIRY_NOTIFY_ENABLED", false)
	for _, reminder := range result.Reminders {
		if !notifyEnabled {
			logger.LogInfo(ctx, fmt.Sprintf("channel key expiry detected, notify disabled: %s(#%d) expires %s", reminder.Name, reminder.Id, reminder.ExpiresAtText))
			continue
		}
		subject := fmt.Sprintf("渠道密钥即将到期：%s（#%d）", reminder.Name, reminder.Id)
		content := fmt.Sprintf("渠道「%s」（#%d）密钥将于 %s 到期，请及时轮换。", reminder.Name, reminder.Id, reminder.ExpiresAtText)
		NotifyRootUser(fmt.Sprintf("channel_key_expiry_%d", reminder.Id), subject, content)
	}
	if err := model.UpdateSystemTaskState(task.TaskID, runnerID, state); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("channel key expiry task %s state update failed: %v", task.TaskID, err))
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("channel key expiry task %s failed to persist result: %v", task.TaskID, err))
	}
}

type ChannelKeyExpiryOptions struct {
	AdvanceSeconds       int64 // 提前提醒窗口（秒）
	MinNotifyIntervalSec int64 // 同一渠道两次通知的最小间隔（秒）
}

type ChannelKeyExpiryState struct {
	NotifiedAt map[int64]int64 `json:"notified_at"` // channel id -> 上次通知时间（unix 秒）
}

type ChannelKeyExpiryReminder struct {
	Id            int64  `json:"id"`
	Name          string `json:"name"`
	ExpiresAt     int64  `json:"expires_at"`
	ExpiresAtText string `json:"expires_at_text"`
}

type ChannelKeyExpiryResult struct {
	Checked   int                        `json:"checked"`
	Notified  int                        `json:"notified"`
	Reminders []ChannelKeyExpiryReminder `json:"reminders"`
}

// ScanChannelKeyExpiry computes which channels need a key-expiry reminder.
// Pure calculation: it never sends notifications, so it is safe to unit test.
func ScanChannelKeyExpiry(_ context.Context, opts ChannelKeyExpiryOptions, state *ChannelKeyExpiryState) (ChannelKeyExpiryResult, error) {
	now := common.GetTimestamp()
	if state.NotifiedAt == nil {
		state.NotifiedAt = map[int64]int64{}
	}
	var channels []*model.Channel
	if err := model.DB.Where("key_expires_at > 0").Find(&channels).Error; err != nil {
		return ChannelKeyExpiryResult{}, err
	}
	result := ChannelKeyExpiryResult{Checked: len(channels)}
	for _, ch := range channels {
		if ch.KeyExpiresAt > now+opts.AdvanceSeconds {
			continue // 提前提醒窗口之外：计入 Checked，但不通知
		}
		last, ok := state.NotifiedAt[int64(ch.Id)]
		if ok && now-last < opts.MinNotifyIntervalSec {
			continue
		}
		state.NotifiedAt[int64(ch.Id)] = now
		result.Notified++
		result.Reminders = append(result.Reminders, ChannelKeyExpiryReminder{
			Id:            int64(ch.Id),
			Name:          ch.Name,
			ExpiresAt:     ch.KeyExpiresAt,
			ExpiresAtText: time.Unix(ch.KeyExpiresAt, 0).Format("2006-01-02 15:04"),
		})
	}
	return result, nil
}

func normalizedExpiryEnv(name string, def, min, max int) int {
	value := common.GetEnvOrDefault(name, def)
	if value < min || value > max {
		value = def
	}
	return value
}
