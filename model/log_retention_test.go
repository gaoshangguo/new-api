package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestOldLogCleanupPreservesAuditLogs 验证数据保留（P0-31）的审计轨迹豁免：
// 保留期清理只删除消费/错误等业务日志，管理审计（LogTypeManage）与
// 登录日志（LogTypeLogin）不受影响，最近窗口内的业务日志也不受影响。
func TestOldLogCleanupPreservesAuditLogs(t *testing.T) {
	require.NoError(t, LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&Log{}).Error)
	old := common.GetTimestamp() - 200*24*3600
	recent := common.GetTimestamp()

	seed := []Log{
		{UserId: 1, Type: LogTypeConsume, CreatedAt: old},
		{UserId: 1, Type: LogTypeError, CreatedAt: old},
		{UserId: 1, Type: LogTypeConsume, CreatedAt: recent},
		{UserId: 1, Type: LogTypeManage, CreatedAt: old},
		{UserId: 1, Type: LogTypeLogin, CreatedAt: old},
	}
	for _, log := range seed {
		require.NoError(t, LOG_DB.Create(&log).Error)
	}
	t.Cleanup(func() {
		_ = LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&Log{}).Error
	})

	remaining, err := CountOldLogExcludingAudit(context.Background(), old+1)
	require.NoError(t, err)
	assert.EqualValues(t, 2, remaining)

	deleted, err := DeleteOldLogBatchExcludingAudit(context.Background(), old+1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted)

	var left []Log
	require.NoError(t, LOG_DB.Find(&left).Error)
	require.Len(t, left, 3)

	types := map[int]int{}
	for _, log := range left {
		types[log.Type]++
	}
	assert.Equal(t, 1, types[LogTypeConsume])
	assert.Equal(t, 1, types[LogTypeManage], "audit logs must survive retention cleanup")
	assert.Equal(t, 1, types[LogTypeLogin], "login logs must survive retention cleanup")
}

// TestOldLogCleanupAllTypesWhenRequested 验证显式清理（log_cleanup 手动任务）仍删除全部旧日志。
func TestOldLogCleanupAllTypesWhenRequested(t *testing.T) {
	require.NoError(t, LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&Log{}).Error)
	old := common.GetTimestamp() - 200*24*3600

	seed := []Log{
		{UserId: 1, Type: LogTypeConsume, CreatedAt: old},
		{UserId: 1, Type: LogTypeManage, CreatedAt: old},
		{UserId: 1, Type: LogTypeLogin, CreatedAt: old},
	}
	for _, log := range seed {
		require.NoError(t, LOG_DB.Create(&log).Error)
	}
	t.Cleanup(func() {
		_ = LOG_DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&Log{}).Error
	})

	deleted, err := DeleteOldLogBatch(context.Background(), old+1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 3, deleted)

	var count int64
	require.NoError(t, LOG_DB.Model(&Log{}).Count(&count).Error)
	assert.Zero(t, count)
}
