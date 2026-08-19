package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIsPhoneTaken(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Unscoped().Where("username LIKE ?", "phone-test-%").Delete(&User{}).Error)
	require.NoError(t, DB.Create(&User{Username: "phone-test-1", Password: "x", Status: common.UserStatusEnabled, Phone: "13800000001"}).Error)
	t.Cleanup(func() {
		_ = DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Unscoped().Where("username LIKE ?", "phone-test-%").Delete(&User{}).Error
	})

	taken, err := IsPhoneTaken("13800000001")
	require.NoError(t, err)
	assert.True(t, taken)
	taken, err = IsPhoneTaken("13800000002")
	require.NoError(t, err)
	assert.False(t, taken)
	taken, err = IsPhoneTaken("  ")
	require.NoError(t, err)
	assert.False(t, taken)
}

func TestRecentLoginIPs(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&UserSession{}))
	session := &UserSession{SID: "sess-keep", UserID: 4242, Status: UserSessionStatusActive, Version: 1, UserAuthVersion: 1, IP: "1.2.3.4"}
	require.NoError(t, DB.Create(session).Error)
	require.NoError(t, DB.Create(&UserSession{SID: "sess-prev", UserID: 4242, Status: UserSessionStatusActive, Version: 1, UserAuthVersion: 1, IP: "5.6.7.8"}).Error)
	require.NoError(t, DB.Create(&UserSession{SID: "sess-dup", UserID: 4242, Status: UserSessionStatusActive, Version: 1, UserAuthVersion: 1, IP: "5.6.7.8"}).Error)
	t.Cleanup(func() {
		_ = DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).
			Where("user_id = ?", 4242).Delete(&UserSession{}).Error
	})

	ips, err := RecentLoginIPs(4242, "sess-keep", 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"5.6.7.8"}, ips, "current session excluded and duplicates deduped")

	ips, err = RecentLoginIPs(4242, "not-existing", 10)
	require.NoError(t, err)
	assert.Len(t, ips, 2)
}

func TestGetUserConsumeErrorRate(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Log{}))
	require.NoError(t, LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Where("user_id = ?", 5151).Delete(&Log{}).Error)
	now := common.GetTimestamp()
	// 6 条消费日志、2 条零结算（错误率 1/3），满足最少样本数。
	for i := 0; i < 4; i++ {
		require.NoError(t, LOG_DB.Create(&Log{UserId: 5151, Type: LogTypeConsume, Quota: 100 + i, CreatedAt: now}).Error)
	}
	require.NoError(t, LOG_DB.Create(&Log{UserId: 5151, Type: LogTypeConsume, Quota: 0, CreatedAt: now}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 5151, Type: LogTypeConsume, Quota: 0, CreatedAt: now}).Error)
	t.Cleanup(func() {
		_ = LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Where("user_id = ?", 5151).Delete(&Log{}).Error
	})

	rate := GetUserConsumeErrorRate(5151, 24)
	assert.InDelta(t, 1.0/3.0, rate, 0.001)
	// 窗口外日志不计入：调小窗口至 1 小时仍覆盖 now，样本足够。
	assert.InDelta(t, 1.0/3.0, GetUserConsumeErrorRate(5151, 1), 0.001)
	// 无效用户返回 0
	assert.Equal(t, 0.0, GetUserConsumeErrorRate(0, 24))
}
