package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupBusinessReminderFixture(t *testing.T, customerQuota int) (BusinessActor, *User, *BusinessProject) {
	t.Helper()
	entryActor, _, customer, _, project := setupBusinessFixture(t, customerQuota)
	require.NoError(t, DB.AutoMigrate(&BusinessProjectReminder{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&BusinessProjectReminder{}).Error)
	return entryActor, customer, project
}

func TestBusinessProjectReminderScanDeduplicatesAndResolves(t *testing.T) {
	entryActor, customer, project := setupBusinessReminderFixture(t, 40)
	now := time.Now().Unix()
	project.BudgetQuota = 100
	project.LowBalanceQuota = 50
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Updates(map[string]interface{}{
		"budget_quota":      project.BudgetQuota,
		"low_balance_quota": project.LowBalanceQuota,
	}).Error)

	token := &Token{
		UserId: customer.Id,
		Name:   "reminder-key",
		Key:    "business-reminder-key-0001",
		Status: common.TokenStatusEnabled,
	}
	require.NoError(t, DB.Create(token).Error)
	require.NoError(t, SetBusinessProjectToken(project.Id, token.Id, "Associate reminder test key", entryActor))
	require.NoError(t, DB.Create(&BusinessConsumption{
		CompanyId: project.CompanyId,
		ProjectId: project.Id,
		UserId:    customer.Id,
		TokenId:   token.Id,
		RequestId: "business-reminder-consumption",
		Quota:     100,
		CreatedAt: now,
	}).Error)
	for index := 0; index < 9; index++ {
		require.NoError(t, LOG_DB.Create(&Log{
			UserId:    customer.Id,
			TokenId:   token.Id,
			Type:      LogTypeConsume,
			CreatedAt: now,
		}).Error)
	}
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    customer.Id,
		TokenId:   token.Id,
		Type:      LogTypeError,
		CreatedAt: now,
	}).Error)

	options := BusinessReminderScanOptions{
		Now:                  now,
		InactiveAfterSeconds: int64((30 * 24 * time.Hour).Seconds()),
		ErrorWindowSeconds:   int64((24 * time.Hour).Seconds()),
		ErrorRatePercent:     10,
		ErrorMinRequestCount: 10,
	}
	first, candidates, err := ScanBusinessProjectReminders(context.Background(), options)
	require.NoError(t, err)
	assert.Equal(t, 3, first.ActiveReminderCount)
	assert.Equal(t, 3, first.CreatedReminderCount)
	require.Len(t, candidates, 3)

	for _, candidate := range candidates {
		require.NotNil(t, candidate.Reminder)
		require.NoError(t, MarkBusinessProjectReminderNotified(context.Background(), candidate.Reminder.Id, now))
	}
	second, secondCandidates, err := ScanBusinessProjectReminders(context.Background(), options)
	require.NoError(t, err)
	assert.Equal(t, 3, second.ActiveReminderCount)
	assert.Zero(t, second.CreatedReminderCount)
	assert.Empty(t, secondCandidates)

	var reminders []BusinessProjectReminder
	require.NoError(t, DB.Where("project_id = ?", project.Id).Order("reminder_type asc").Find(&reminders).Error)
	require.Len(t, reminders, 3)
	assert.Equal(t, []string{
		BusinessReminderTypeBudgetExceeded,
		BusinessReminderTypeHighFailureRate,
		BusinessReminderTypeLowBalance,
	}, []string{reminders[0].ReminderType, reminders[1].ReminderType, reminders[2].ReminderType})
	for _, reminder := range reminders {
		assert.Equal(t, BusinessReminderStateActive, reminder.State)
		assert.Equal(t, now, reminder.LastNotifiedAt)
	}

	require.NoError(t, DB.Model(&User{}).Where("id = ?", customer.Id).Update("quota", 1000).Error)
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("budget_quota", 1000).Error)
	require.NoError(t, LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Log{}).Error)
	resolved, candidates, err := ScanBusinessProjectReminders(context.Background(), options)
	require.NoError(t, err)
	assert.Zero(t, resolved.ActiveReminderCount)
	assert.Equal(t, 3, resolved.ResolvedReminderCount)
	assert.Empty(t, candidates)

	for _, reminder := range reminders {
		var persisted BusinessProjectReminder
		require.NoError(t, DB.First(&persisted, reminder.Id).Error)
		assert.Equal(t, BusinessReminderStateResolved, persisted.State)
		assert.Equal(t, now, persisted.ResolvedAt)
	}
}

func TestBusinessProjectReminderScanDetectsLongUnusedProjectWithoutDuplicates(t *testing.T) {
	_, customer, project := setupBusinessReminderFixture(t, 1000)
	now := time.Now().Unix()
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("created_at", now-int64((31*24*time.Hour).Seconds())).Error)

	options := BusinessReminderScanOptions{
		Now:                  now,
		InactiveAfterSeconds: int64((30 * 24 * time.Hour).Seconds()),
		ErrorWindowSeconds:   int64((24 * time.Hour).Seconds()),
		ErrorRatePercent:     10,
		ErrorMinRequestCount: 10,
	}
	first, candidates, err := ScanBusinessProjectReminders(context.Background(), options)
	require.NoError(t, err)
	assert.Equal(t, 1, first.ActiveReminderCount)
	require.Len(t, candidates, 1)
	assert.Equal(t, customer.Id, candidates[0].Owner.Id)
	assert.Equal(t, BusinessReminderTypeInactiveProject, candidates[0].Reminder.ReminderType)
	require.NoError(t, MarkBusinessProjectReminderNotified(context.Background(), candidates[0].Reminder.Id, now))

	second, candidates, err := ScanBusinessProjectReminders(context.Background(), options)
	require.NoError(t, err)
	assert.Equal(t, 1, second.ActiveReminderCount)
	assert.Zero(t, second.CreatedReminderCount)
	assert.Empty(t, candidates)

	reminders, total, err := ListBusinessProjectReminders(customer.Id, project.Id, true, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, reminders, 1)
	assert.Equal(t, BusinessReminderStateActive, reminders[0].State)
}
