package controller

import (
	"fmt"
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

func setupPriceVersionControllerTest(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PriceVersion{}, &model.BusinessAuditEvent{}, &model.Option{}, &model.Log{}, &model.User{}))
	model.DB = db
	model.LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	ratio_setting.InitRatioSettings()
	model.InitOptionMap()
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
}

func TestUpdateOptionModelRatioCreatesAuditedPriceVersion(t *testing.T) {
	setupPriceVersionControllerTest(t)

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"ModelRatio","value":"{\"gpt-version-hook\": 4}"}`),
	)
	context.Set("id", 1)
	context.Set("username", "root")
	context.Set("role", 1)

	UpdateOption(context)

	assert.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)

	var version model.PriceVersion
	require.NoError(t, model.DB.Order("id desc").First(&version).Error)
	assert.Equal(t, "active", version.Status)
	assert.Contains(t, version.ModelRatio, "gpt-version-hook")
	assert.Equal(t, 1, version.CreatedBy)

	var audit model.BusinessAuditEvent
	require.NoError(t, model.DB.Where("action = ?", "price_version.create").First(&audit).Error)
	assert.Equal(t, "root", audit.ActorName)
	assert.Contains(t, audit.Reason, "ModelRatio")
	assert.Contains(t, audit.AfterValue, fmt.Sprintf(`"version_id":%d`, version.Id))

	assert.Equal(t, 4.0, ratio_setting.GetModelRatioCopy()["gpt-version-hook"])
	assert.Equal(t, version.Id, model.GetCurrentPriceVersionId())
}

func TestUpdateOptionNonPriceKeySkipsVersion(t *testing.T) {
	setupPriceVersionControllerTest(t)

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"QuotaForNewUser","value":"5000"}`),
	)
	context.Set("id", 1)
	context.Set("username", "root")
	context.Set("role", 1)

	UpdateOption(context)

	assert.Equal(t, http.StatusOK, response.Code)
	var count int64
	require.NoError(t, model.DB.Model(&model.PriceVersion{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}
