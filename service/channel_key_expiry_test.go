package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupExpiryTestChannel(t *testing.T, name string, expiresInSeconds int64) *model.Channel {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))
	channel := &model.Channel{
		Type:          1,
		Name:          name,
		Key:           "sk-expiry-" + name,
		Status:        1,
		Group:         "default",
		KeyExpiresAt:  common.GetTimestamp() + expiresInSeconds,
	}
	require.NoError(t, channel.Insert())
	return channel
}

func TestScanChannelKeyExpiryNotifiesOncePerWindow(t *testing.T) {
	due := setupExpiryTestChannel(t, "due-1", 3*24*60*60)    // 3 天后到期
	_ = setupExpiryTestChannel(t, "overdue-1", -3600)        // 已过期
	_ = setupExpiryTestChannel(t, "far-1", 30*24*60*60)      // 30 天后到期（窗口外）

	state := &ChannelKeyExpiryState{}
	opts := ChannelKeyExpiryOptions{
		AdvanceSeconds:       7 * 24 * 60 * 60,
		MinNotifyIntervalSec: 24 * 60 * 60,
	}

	result, err := ScanChannelKeyExpiry(context.Background(), opts, state)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Checked)
	assert.Equal(t, 2, result.Notified) // due + overdue；far 不在窗口
	names := make([]string, 0, len(result.Reminders))
	for _, r := range result.Reminders {
		names = append(names, r.Name)
	}
	assert.Contains(t, names, "due-1")
	assert.Contains(t, names, "overdue-1")

	// 同一通知窗口内重复扫描不重复通知
	result2, err := ScanChannelKeyExpiry(context.Background(), opts, state)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.Notified)

	// 超过最小间隔后再次通知
	state.NotifiedAt[int64(due.Id)] -= 25 * 60 * 60
	result3, err := ScanChannelKeyExpiry(context.Background(), opts, state)
	require.NoError(t, err)
	assert.Equal(t, 1, result3.Notified)
	assert.Equal(t, "due-1", result3.Reminders[0].Name)
}
