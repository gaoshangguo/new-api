package service

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPersonalLowBalanceServiceTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousSMTP := model.DB, common.SMTPServer
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserNotification{}, &model.UserLowBalanceState{}, &model.SystemTask{}, &model.SystemTaskLock{}))
	model.DB, common.SMTPServer = db, "smtp.example.com"
	t.Cleanup(func() { model.DB, common.SMTPServer = previousDB, previousSMTP })
	return db
}

func TestUserLowBalanceMailFailurePreservesInboxAndRetries(t *testing.T) {
	db := setupPersonalLowBalanceServiceTest(t)
	enabled := true
	user := model.User{Id: 1, Username: "mail-user", AffCode: "mail-user", Quota: 19, Email: "account@example.com"}
	user.SetSetting(dto.UserSetting{QuotaWarningThreshold: 20, NotifyType: "webhook", LowBalanceEmailEnabled: &enabled, NotificationEmail: "notify@example.com"})
	require.NoError(t, db.Create(&user).Error)
	attempts := 0
	send := func(id int, email string, settings dto.UserSetting, notification dto.Notify) error {
		attempts++
		assert.Equal(t, 1, id)
		assert.Equal(t, "notify@example.com", email)
		assert.Equal(t, dto.NotifyTypeEmail, settings.NotifyType)
		assert.Equal(t, dto.NotifyTypeQuotaExceed, notification.Type)
		if attempts == 1 {
			return errors.New("SMTP unavailable")
		}
		return nil
	}
	ctx := context.Background()
	require.Error(t, checkUserLowBalance(ctx, 1, send))
	inbox, err := model.ListUserNotifications(ctx, 1, 0)
	require.NoError(t, err)
	require.Len(t, inbox.Items, 1)
	assert.EqualValues(t, 1, inbox.UnreadCount)
	require.NoError(t, checkUserLowBalance(ctx, 1, send))
	assert.Equal(t, 1, attempts)
	require.NoError(t, db.Model(&model.UserLowBalanceState{}).Where("user_id = ?", 1).Update("email_retry_at", 0).Error)
	require.NoError(t, checkUserLowBalance(ctx, 1, send))
	require.NoError(t, checkUserLowBalance(ctx, 1, send))
	assert.Equal(t, 2, attempts)
	inbox, err = model.ListUserNotifications(ctx, 1, 0)
	require.NoError(t, err)
	assert.Len(t, inbox.Items, 1)
	// No SMTP config must leave the next episode retryable, never "sent".
	common.SMTPServer = ""
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("quota", 20).Error)
	model.RearmRecoveredUserLowBalance(1)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("quota", 19).Error)
	require.Error(t, checkUserLowBalance(ctx, 1, send))
	var state model.UserLowBalanceState
	require.NoError(t, db.First(&state, "user_id = ?", 1).Error)
	assert.Zero(t, state.EmailSentAt)
	assert.Equal(t, 2, attempts)
}

func TestUserLowBalanceScheduledScanFindsIdleAccountsAndHonorsCancellation(t *testing.T) {
	db := setupPersonalLowBalanceServiceTest(t)
	user := model.User{Id: 1, Username: "idle-user", AffCode: "idle-user", Quota: 19}
	user.SetSetting(dto.UserSetting{QuotaWarningThreshold: 20, LowBalanceEmailEnabled: new(bool)})
	require.NoError(t, db.Create(&user).Error)
	disabled := model.User{Id: 2, Username: "disabled-user", AffCode: "disabled-user", Status: common.UserStatusDisabled, Quota: 1, Setting: user.Setting}
	require.NoError(t, db.Create(&disabled).Error)
	task, err := model.CreateSystemTask(model.SystemTaskTypeUserLowBalanceScan, nil, nil)
	require.NoError(t, err)
	claimed, acquired, err := model.ClaimSystemTask(task.ID, task.Type, "test-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, acquired)
	userLowBalanceTaskHandler{}.Run(context.Background(), claimed, "test-runner")
	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
	inbox, err := model.ListUserNotifications(context.Background(), 1, 0)
	require.NoError(t, err)
	assert.Len(t, inbox.Items, 1)
	inbox, err = model.ListUserNotifications(context.Background(), 2, 0)
	require.NoError(t, err)
	assert.Empty(t, inbox.Items)
	// A canceled task must terminate without generating a new episode.
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("quota", 20).Error)
	model.RearmRecoveredUserLowBalance(1)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("quota", 19).Error)
	task, err = model.CreateSystemTask(model.SystemTaskTypeUserLowBalanceScan, nil, nil)
	require.NoError(t, err)
	claimed, acquired, err = model.ClaimSystemTask(task.ID, task.Type, "test-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, acquired)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	userLowBalanceTaskHandler{}.Run(ctx, claimed, "test-runner")
	finished, err = model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	assert.Equal(t, model.SystemTaskStatusFailed, finished.Status)
	inbox, err = model.ListUserNotifications(context.Background(), 1, 0)
	require.NoError(t, err)
	assert.Len(t, inbox.Items, 1)
}
