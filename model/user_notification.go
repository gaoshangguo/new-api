package model

import (
	"context"
	"errors"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// UserNotification contains structured, locale-independent personal inbox data.
type UserNotification struct {
	Id        int    `json:"id" gorm:"index:idx_user_notification_inbox,priority:2"`
	UserId    int    `json:"-" gorm:"index:idx_user_notification_inbox,priority:1;index:idx_user_notification_unread,priority:1"`
	Type      string `json:"type" gorm:"size:32"`
	Quota     int    `json:"quota"`
	Threshold int    `json:"threshold"`
	CreatedAt int64  `json:"created_at"`
	ReadAt    int64  `json:"read_at" gorm:"index:idx_user_notification_unread,priority:2"`
}

// UserLowBalanceState deduplicates each channel for a continuous low-balance
// episode. Delivery leases survive restarts and are independent of inbox reads.
type UserLowBalanceState struct {
	UserId         int `gorm:"primaryKey;autoIncrement:false"`
	Active         bool
	NotificationId int
	Threshold      int
	EmailSentAt    int64
	EmailLease     string `gorm:"size:32"`
	EmailRetryAt   int64
}

type UserLowBalanceDelivery struct {
	User      User
	Threshold int
	Lease     string
}

// PrepareUserLowBalance uses the authoritative balance/settings and serializes
// episode changes with the user row, not with process-local notification caches.
func PrepareUserLowBalance(ctx context.Context, userId int, now int64) (*UserLowBalanceDelivery, error) {
	var delivery *UserLowBalanceDelivery
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota", "status", "email", "setting").First(&user, userId).Error; err != nil {
			return err
		}
		settings := user.GetSetting()
		threshold := common.QuotaRemindThreshold
		if settings.QuotaWarningThreshold != 0 {
			value := settings.QuotaWarningThreshold
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 1 || value > float64(common.MaxQuota) {
				return nil // Invalid legacy settings must not overflow or trigger mail.
			}
			threshold = int(value)
		}
		if user.Status != common.UserStatusEnabled || user.Quota >= threshold {
			return tx.Model(&UserLowBalanceState{}).Where("user_id = ? AND active = ?", userId, true).
				Updates(map[string]any{"active": false, "notification_id": 0, "email_sent_at": 0, "email_lease": "", "email_retry_at": 0}).Error
		}
		var state UserLowBalanceState
		err := tx.First(&state, "user_id = ?", userId).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			state.UserId = userId
			if err := tx.Create(&state).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		state.Active = true
		state.Threshold = threshold
		if (settings.LowBalanceInAppEnabled == nil || *settings.LowBalanceInAppEnabled) && state.NotificationId == 0 {
			notification := UserNotification{UserId: userId, Type: "low_balance", Quota: user.Quota, Threshold: threshold, CreatedAt: now}
			if err := tx.Create(&notification).Error; err != nil {
				return err
			}
			state.NotificationId = notification.Id
		}
		// Preserve legacy email opt-in: webhook/Bark/Gotify users do not suddenly
		// receive email unless they explicitly enable the independent switch.
		emailEnabled := settings.NotifyType == "" || settings.NotifyType == "email"
		if settings.LowBalanceEmailEnabled != nil {
			emailEnabled = *settings.LowBalanceEmailEnabled
		}
		if emailEnabled && state.EmailSentAt == 0 && state.EmailRetryAt <= now {
			state.EmailLease = common.GetRandomString(32)
			state.EmailRetryAt = now + 600
			delivery = &UserLowBalanceDelivery{User: user, Threshold: threshold, Lease: state.EmailLease}
		}
		return tx.Save(&state).Error
	})
	return delivery, err
}

func FinishUserLowBalanceEmail(ctx context.Context, userId int, lease string, now int64, delivered bool) error {
	updates := map[string]any{"email_lease": "", "email_retry_at": now + 300}
	if delivered {
		updates["email_sent_at"] = now
		updates["email_retry_at"] = 0
	}
	return DB.WithContext(ctx).Model(&UserLowBalanceState{}).
		Where("user_id = ? AND active = ? AND email_lease = ?", userId, true, lease).Updates(updates).Error
}

// RearmRecoveredUserLowBalance observes credits synchronously after commit,
// even when a recovered balance is consumed again before the periodic scan.
// Notification failures must never roll back a successful wallet credit.
func RearmRecoveredUserLowBalance(userId int) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Use the same user -> state lock order as reminder evaluation, avoiding
		// opposite-order locks from an UPDATE containing a wallet subquery.
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").First(&user, userId).Error; err != nil {
			return err
		}
		return tx.Model(&UserLowBalanceState{}).
			Where("user_id = ? AND active = ? AND threshold <= ?", userId, true, user.Quota).
			Updates(map[string]any{"active": false, "notification_id": 0, "email_sent_at": 0, "email_lease": "", "email_retry_at": 0}).Error
	})
	if err != nil {
		common.SysError("failed to rearm personal low balance reminder: " + err.Error())
	}
}

type UserNotificationInbox struct {
	Items       []UserNotification `json:"items"`
	UnreadCount int64              `json:"unread_count"`
}

func ListUserNotifications(ctx context.Context, userId, beforeId int) (UserNotificationInbox, error) {
	inbox := UserNotificationInbox{Items: []UserNotification{}}
	query := DB.WithContext(ctx).Model(&UserNotification{}).Where("user_id = ?", userId)
	if err := query.Where("read_at = ?", 0).Count(&inbox.UnreadCount).Error; err != nil {
		return inbox, err
	}
	query = DB.WithContext(ctx).Where("user_id = ?", userId)
	if beforeId > 0 {
		query = query.Where("id < ?", beforeId)
	}
	err := query.Order("id DESC").Limit(50).Find(&inbox.Items).Error
	return inbox, err
}

func MarkUserNotificationRead(ctx context.Context, userId, id int, now int64) error {
	// Scope the lookup as well as the update. Foreign IDs are indistinguishable
	// from missing IDs, and repeated reads preserve the original read timestamp.
	var notification UserNotification
	if err := DB.WithContext(ctx).Where("user_id = ? AND id = ?", userId, id).First(&notification).Error; err != nil {
		return err
	}
	return DB.WithContext(ctx).Model(&UserNotification{}).
		Where("user_id = ? AND id = ? AND read_at = ?", userId, id, 0).Update("read_at", now).Error
}
