package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOptionAuditTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Log{}, &model.User{}))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	ratio_setting.InitRatioSettings()
	model.InitOptionMap()
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedis
	})
	return db
}

// P0-26：非敏感配置项的两次修改必须留下包含 before_value/after_value 的审计日志。
func TestUpdateOptionRecordsAuditDiffForNonSensitiveKey(t *testing.T) {
	db := setupOptionAuditTest(t)

	update := func(value string) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		body := "{\"key\":\"SystemName\",\"value\":\"" + value + "\"}"
		ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(body))
		ctx.Set("id", 1)
		ctx.Set("username", "root")
		ctx.Set("role", 100)
		UpdateOption(ctx)
		assert.Equal(t, http.StatusOK, rec.Code)
	}
	update("First")
	update("Second")

	var entries []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).Find(&entries).Error)
	require.Len(t, entries, 2)

	// 第二次修改的日志必须包含 before_value=First 与 after_value=Second。
	for _, entry := range entries {
		if strings.Contains(entry.Other, "audit_info") &&
			strings.Contains(entry.Other, "before_value") &&
			strings.Contains(entry.Other, "First") &&
			strings.Contains(entry.Other, "Second") {
			return
		}
	}
	t.Fatalf("no audit log contains the before->after transition: %v", entries)
}

// P0-26：敏感配置项（如 TelegramBotToken）不得记录配置值。
func TestUpdateOptionSkipsValuesForSensitiveKey(t *testing.T) {
	db := setupOptionAuditTest(t)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/",
		strings.NewReader(`{"key":"TelegramBotToken","value":"SECRET123"}`))
	ctx.Set("id", 1)
	ctx.Set("username", "root")
	ctx.Set("role", 100)
	UpdateOption(ctx)

	var entries []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.NotContains(t, entries[0].Other, "SECRET123", "sensitive option values must never be audited")
	assert.NotContains(t, entries[0].Other, "audit_info")
}

func TestRecordManageAuditForWithDiffWritesAuditInfo(t *testing.T) {
	db := setupOptionAuditTest(t)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader("{}"))
	c.Set("id", 2)
	c.Set("username", "root")
	c.Set("role", 100)
	recordManageAuditForWithDiff(c, 2, "option.update", map[string]interface{}{"key": "X"},
		map[string]string{"X": "old"}, map[string]string{"X": "new"})

	var entries []model.Log
	require.NoError(t, db.Where("type = ?", model.LogTypeManage).Find(&entries).Error)
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].Other, "audit_info")
	assert.Contains(t, entries[0].Other, "before_value")
	assert.Contains(t, entries[0].Other, "old")
	assert.Contains(t, entries[0].Other, "new")
}
