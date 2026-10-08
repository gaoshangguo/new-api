package model

import (
	"context"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserNotificationTest(t *testing.T, settings dto.UserSetting) *gorm.DB {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}, &UserNotification{}, &UserLowBalanceState{}))
	DB = db
	user := User{Id: 1, Username: "notification-user", AffCode: "notification-user", Quota: 19, Status: common.UserStatusEnabled}
	user.SetSetting(settings)
	require.NoError(t, db.Create(&user).Error)
	t.Cleanup(func() { DB = previousDB; _ = sqlDB.Close() })
	return db
}

func TestUserLowBalanceDeduplicatesConcurrentChecksAndInboxReads(t *testing.T) {
	setupUserNotificationTest(t, dto.UserSetting{QuotaWarningThreshold: 20})
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan *UserLowBalanceDelivery, 2)
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := PrepareUserLowBalance(ctx, 1, 100)
			results <- result
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	claims := 0
	for result := range results {
		if result != nil {
			claims++
		}
	}
	assert.Equal(t, 1, claims)
	inbox, err := ListUserNotifications(ctx, 1, 0)
	require.NoError(t, err)
	require.Len(t, inbox.Items, 1)
	assert.EqualValues(t, 1, inbox.UnreadCount)
	assert.Equal(t, 19, inbox.Items[0].Quota)
	require.NoError(t, MarkUserNotificationRead(ctx, 1, inbox.Items[0].Id, 200))
	require.NoError(t, MarkUserNotificationRead(ctx, 1, inbox.Items[0].Id, 300))
	_, err = PrepareUserLowBalance(ctx, 1, 400)
	require.NoError(t, err)
	inbox, err = ListUserNotifications(ctx, 1, 0)
	require.NoError(t, err)
	require.Len(t, inbox.Items, 1)
	assert.Zero(t, inbox.UnreadCount)
	assert.EqualValues(t, 200, inbox.Items[0].ReadAt)
}

func TestUserLowBalanceRetriesEmailAndRejectsStaleLeaseAfterRecovery(t *testing.T) {
	db := setupUserNotificationTest(t, dto.UserSetting{QuotaWarningThreshold: 20})
	ctx := context.Background()
	first, err := PrepareUserLowBalance(ctx, 1, 100)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NoError(t, FinishUserLowBalanceEmail(ctx, 1, first.Lease, 101, false))
	waiting, err := PrepareUserLowBalance(ctx, 1, 400)
	require.NoError(t, err)
	assert.Nil(t, waiting)
	retry, err := PrepareUserLowBalance(ctx, 1, 401)
	require.NoError(t, err)
	require.NotNil(t, retry)
	require.NoError(t, FinishUserLowBalanceEmail(ctx, 1, retry.Lease, 402, true))
	sent, err := PrepareUserLowBalance(ctx, 1, 2000)
	require.NoError(t, err)
	assert.Nil(t, sent)
	// Recharging to exactly the threshold rearms without needing a scan.
	require.NoError(t, db.Model(&User{}).Where("id = ?", 1).Update("quota", 20).Error)
	RearmRecoveredUserLowBalance(1)
	require.NoError(t, db.Model(&User{}).Where("id = ?", 1).Update("quota", 18).Error)
	next, err := PrepareUserLowBalance(ctx, 1, 2001)
	require.NoError(t, err)
	require.NotNil(t, next)
	require.NoError(t, FinishUserLowBalanceEmail(ctx, 1, retry.Lease, 2002, true))
	var state UserLowBalanceState
	require.NoError(t, db.First(&state, "user_id = ?", 1).Error)
	assert.Zero(t, state.EmailSentAt)
	assert.Equal(t, next.Lease, state.EmailLease)
	inbox, err := ListUserNotifications(ctx, 1, 0)
	require.NoError(t, err)
	assert.Len(t, inbox.Items, 2)
}

func TestUserLowBalanceChannelPreferencesAreIndependent(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings dto.UserSetting
		inbox    bool
		email    bool
	}{
		{"legacy email", dto.UserSetting{}, true, true},
		{"legacy webhook", dto.UserSetting{NotifyType: "webhook"}, true, false},
		{"both off", dto.UserSetting{LowBalanceInAppEnabled: new(bool), LowBalanceEmailEnabled: new(bool)}, false, false},
		{"in-app only", dto.UserSetting{LowBalanceEmailEnabled: new(bool)}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.settings.QuotaWarningThreshold = 20
			setupUserNotificationTest(t, tc.settings)
			result, err := PrepareUserLowBalance(context.Background(), 1, 100)
			require.NoError(t, err)
			assert.Equal(t, tc.email, result != nil)
			inbox, err := ListUserNotifications(context.Background(), 1, 0)
			require.NoError(t, err)
			assert.Equal(t, tc.inbox, len(inbox.Items) == 1)
		})
	}
}

func TestUserLowBalanceIgnoresDisabledAndHealthyAccounts(t *testing.T) {
	db := setupUserNotificationTest(t, dto.UserSetting{QuotaWarningThreshold: 20})
	for _, update := range []map[string]any{{"quota": 20}, {"quota": 19, "status": common.UserStatusDisabled}} {
		require.NoError(t, db.Model(&User{}).Where("id = ?", 1).Updates(update).Error)
		delivery, err := PrepareUserLowBalance(context.Background(), 1, 100)
		require.NoError(t, err)
		assert.Nil(t, delivery)
	}
	inbox, err := ListUserNotifications(context.Background(), 1, 0)
	require.NoError(t, err)
	assert.Empty(t, inbox.Items)
}
