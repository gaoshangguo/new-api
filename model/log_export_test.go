package model

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogExportDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB, oldCache := DB, LOG_DB, common.MemoryCacheEnabled
	oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &Channel{}))
	DB, LOG_DB, common.MemoryCacheEnabled = db, db, false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()
	t.Cleanup(func() {
		DB, LOG_DB, common.MemoryCacheEnabled = oldDB, oldLogDB, oldCache
		common.SetDatabaseTypes(oldMainType, oldLogType)
		initCol()
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func TestUsageLogExportMatchesFilteredList(t *testing.T) {
	db := setupLogExportDB(t)
	require.NoError(t, db.Create(&Channel{Id: 1, Name: "primary"}).Error)
	seed := []Log{
		{Id: 1, UserId: 1, CreatedAt: 100, Type: LogTypeConsume, ModelName: "gpt_mini", Username: "alice", TokenName: "personal", ChannelId: 1, Group: "default", RequestId: "request", UpstreamRequestId: "upstream"},
		{Id: 2, UserId: 1, CreatedAt: 200, Type: LogTypeConsume, ModelName: "gpt_mini", Username: "alice", TokenName: "personal", ChannelId: 1, Group: "default", RequestId: "request", UpstreamRequestId: "upstream"},
		{Id: 3, UserId: 2, CreatedAt: 300, Type: LogTypeError, ModelName: "gptXmini", Username: "bob", TokenName: "other", ChannelId: 2, Group: "other", RequestId: "other", UpstreamRequestId: "other"},
	}
	require.NoError(t, db.Create(&seed).Error)
	cases := []struct {
		name   string
		filter LogFilter
		ids    []int
	}{
		{"all types", LogFilter{}, []int{3, 2, 1}},
		{"type", LogFilter{Type: LogTypeConsume}, []int{2, 1}},
		{"start inclusive", LogFilter{StartTimestamp: 200}, []int{3, 2}},
		{"end inclusive", LogFilter{EndTimestamp: 200}, []int{2, 1}},
		{"model exact", LogFilter{ModelName: "gpt_mini"}, []int{2, 1}},
		{"model wildcard treats underscore literally", LogFilter{ModelName: "gpt_%"}, []int{2, 1}},
		{"username exact", LogFilter{Username: "alice"}, []int{2, 1}},
		{"username wildcard", LogFilter{Username: "ali%"}, []int{2, 1}},
		{"token", LogFilter{TokenName: "personal"}, []int{2, 1}},
		{"channel", LogFilter{Channel: 1}, []int{2, 1}},
		{"group", LogFilter{Group: "default"}, []int{2, 1}},
		{"request", LogFilter{RequestID: "request"}, []int{2, 1}},
		{"upstream request", LogFilter{UpstreamRequestID: "upstream"}, []int{2, 1}},
		{"combined", LogFilter{StartTimestamp: 200, EndTimestamp: 200, Type: LogTypeConsume, ModelName: "gpt_mini", Username: "alice", TokenName: "personal", Channel: 1, Group: "default", RequestID: "request", UpstreamRequestID: "upstream"}, []int{2}},
		{"empty result", LogFilter{ModelName: "missing"}, []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := GetLogsForExport(context.Background(), tc.filter)
			require.NoError(t, err)
			ids := make([]int, 0, len(logs))
			for _, log := range logs {
				ids = append(ids, log.Id)
				if log.ChannelId == 1 {
					assert.Equal(t, "primary", log.ChannelName)
				}
			}
			assert.Equal(t, tc.ids, ids)
			f := tc.filter
			page, total, err := GetAllLogs(f.Type, f.StartTimestamp, f.EndTimestamp, f.ModelName, f.Username, f.TokenName, 0, 20, f.Channel, f.Group, f.RequestID, f.UpstreamRequestID)
			require.NoError(t, err)
			assert.EqualValues(t, len(logs), total)
			assert.Equal(t, page, logs)
		})
	}
}

func TestUsageLogExportEnforcesOwnerAndRedactsAdminDetails(t *testing.T) {
	db := setupLogExportDB(t)
	other := common.MapToJsonStr(map[string]interface{}{"model_price": 0.25, "admin_info": map[string]interface{}{"secret": "admin"}, "audit_info": map[string]interface{}{"operator": "root"}})
	seed := []Log{{UserId: 1, Other: other}, {UserId: 2, Other: other}}
	require.NoError(t, db.Create(&seed).Error)
	owner := 1
	logs, err := GetLogsForExport(context.Background(), LogFilter{UserID: &owner})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, owner, logs[0].UserId)
	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, parsed, "admin_info")
	assert.NotContains(t, parsed, "audit_info")
	assert.Equal(t, 0.25, parsed["model_price"])
	page, _, err := GetUserLogs(owner, 0, 0, 0, "", "", 0, 20, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, page, logs)
}

func TestUsageLogExportRejectsOverLimitWithoutPartialData(t *testing.T) {
	db := setupLogExportDB(t)
	seed := make([]Log, MaxUsageLogExportRows+1)
	for i := range seed {
		seed[i].UserId = 1
	}
	require.NoError(t, db.CreateInBatches(&seed, 100).Error)
	_, err := GetLogsForExport(context.Background(), LogFilter{})
	assert.ErrorIs(t, err, ErrUsageLogExportLimit)
	require.NoError(t, db.Delete(&seed[len(seed)-1]).Error)
	logs, err := GetLogsForExport(context.Background(), LogFilter{})
	require.NoError(t, err)
	assert.Len(t, logs, MaxUsageLogExportRows)
}

func TestUsageLogExportRejectsOversizedPayload(t *testing.T) {
	db := setupLogExportDB(t)
	require.NoError(t, db.Create(&Log{Content: strings.Repeat("a", maxUsageLogExportBytes+1)}).Error)
	logs, err := GetLogsForExport(context.Background(), LogFilter{})
	assert.ErrorIs(t, err, ErrUsageLogExportSize)
	assert.Nil(t, logs)
}

func TestUsageLogExportCancellationStopsQuery(t *testing.T) {
	setupLogExportDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := GetLogsForExport(ctx, LogFilter{})
	assert.ErrorIs(t, err, context.Canceled)
}
