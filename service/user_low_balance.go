package service

import (
	"context"
	"fmt"
	"html"
	"net/mail"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

type userLowBalanceTaskHandler struct{}

func init() { RegisterSystemTaskHandler(userLowBalanceTaskHandler{}) }

func (userLowBalanceTaskHandler) Type() string { return model.SystemTaskTypeUserLowBalanceScan }
func (userLowBalanceTaskHandler) Enabled() bool {
	return common.GetEnvOrDefaultBool("USER_LOW_BALANCE_TASK_ENABLED", true)
}
func (userLowBalanceTaskHandler) Interval() time.Duration {
	minutes := common.GetEnvOrDefault("USER_LOW_BALANCE_TASK_INTERVAL_MINUTES", 5)
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 1440 {
		minutes = 1440
	}
	return time.Duration(minutes) * time.Minute
}
func (userLowBalanceTaskHandler) NewPayload() any { return nil }
func (userLowBalanceTaskHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	processed := 0
	failed := 0
	lastID := 0
	for {
		if err := ctx.Err(); err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		var users []model.User
		if err := model.DB.WithContext(ctx).Select("id").Where("id > ?", lastID).Order("id").Limit(100).Find(&users).Error; err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		if len(users) == 0 {
			break
		}
		for _, user := range users {
			if err := ctx.Err(); err != nil {
				failSystemTask(task, runnerID, err)
				return
			}
			if err := CheckUserLowBalance(ctx, user.Id); err != nil {
				failed++
				logger.LogWarn(ctx, fmt.Sprintf("personal low balance check failed: user_id=%d err=%v", user.Id, err))
			}
			processed++
			lastID = user.Id
		}
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, map[string]int{"processed": processed, "failed": failed}, ""); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("personal low balance scan result failed: %v", err))
	}
}

// CheckUserLowBalance saves the inbox record before attempting outbound mail.
// Mail is leased per episode, and errors leave it retryable by the next scan.
func CheckUserLowBalance(ctx context.Context, userId int) error {
	return checkUserLowBalance(ctx, userId, NotifyUser)
}

func checkUserLowBalance(ctx context.Context, userId int, send func(int, string, dto.UserSetting, dto.Notify) error) error {
	delivery, err := model.PrepareUserLowBalance(ctx, userId, common.GetTimestamp())
	if err != nil || delivery == nil {
		return err
	}
	settings := delivery.User.GetSetting()
	settings.NotifyType = dto.NotifyTypeEmail
	email := settings.NotificationEmail
	if email == "" {
		email = delivery.User.Email
	}
	address, addressErr := mail.ParseAddress(email)
	err = addressErr
	if err == nil && address.Address != email {
		err = fmt.Errorf("invalid notification email")
	}
	if err == nil && common.SMTPServer == "" {
		err = fmt.Errorf("SMTP server is not configured")
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		link := html.EscapeString(PaymentReturnURL("/wallet"))
		content := "您的余额不足，当前余额为 {{value}}，提醒阈值为 {{value}}。请及时充值。<br/><a href='{{value}}'>充值</a>"
		err = send(userId, email, settings, dto.NewNotify(dto.NotifyTypeQuotaExceed, "低余额提醒", content,
			[]interface{}{logger.FormatQuota(delivery.User.Quota), logger.FormatQuota(delivery.Threshold), link}))
	}
	// Persist completion even if the runner lost its context; the lease token
	// prevents a stale delivery from acknowledging a newer episode.
	markErr := model.FinishUserLowBalanceEmail(context.Background(), userId, delivery.Lease, common.GetTimestamp(), err == nil)
	if markErr != nil {
		return markErr
	}
	return err
}
