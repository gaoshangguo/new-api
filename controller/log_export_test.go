package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func setupUsageLogExportController(t *testing.T) *gin.Engine {
	t.Helper()
	oldDB, oldLogs, oldCache := model.DB, model.LOG_DB, common.MemoryCacheEnabled
	mainType, logType := common.MainDatabaseType(), common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.Channel{}))
	model.DB, model.LOG_DB, common.MemoryCacheEnabled = db, db, false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB, model.LOG_DB, common.MemoryCacheEnabled = oldDB, oldLogs, oldCache
		common.SetDatabaseTypes(mainType, logType)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	other := common.MapToJsonStr(map[string]interface{}{"model_price": 0.25, "admin_info": map[string]interface{}{"secret": "private"}, "audit_info": map[string]interface{}{"operator": "root"}})
	seed := []model.Log{
		{UserId: 1, Username: "alice", ModelName: "gpt", CreatedAt: 100, Other: other},
		{UserId: 1, Username: "alice", ModelName: "gpt", CreatedAt: 200, Other: other},
		{UserId: 2, Username: "bob", ModelName: "gpt", CreatedAt: 300, Other: other},
	}
	require.NoError(t, db.Create(&seed).Error)
	r := gin.New()
	r.GET("/self", func(c *gin.Context) { c.Set("id", 1) }, ExportUserLogs)
	r.GET("/admin", ExportAllLogs)
	r.GET("/unauthenticated", ExportUserLogs)
	return r
}

func TestUsageLogExportSelfCannotOverrideOwnerAndExportsAllPages(t *testing.T) {
	r := setupUsageLogExportController(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/self?user_id=2&UserID=2&username=bob&channel=9&p=100&page_size=1&model_name=gpt", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), ".xlsx")
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", w.Header().Get("Content-Type"))
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	defer f.Close()
	rows, err := f.GetRows("Usage Logs")
	require.NoError(t, err)
	require.Len(t, rows, 3, "all filtered rows, not just the current page")
	for _, row := range rows[1:] {
		parsed, err := common.StrToMap(row[14])
		require.NoError(t, err)
		assert.NotContains(t, parsed, "admin_info")
		assert.NotContains(t, parsed, "audit_info")
		assert.Equal(t, 0.25, parsed["model_price"])
		assert.NotContains(t, row, "bob")
	}
	assert.Equal(t, "1970-01-01 00:03:20", rows[1][0])
	assert.Equal(t, "1970-01-01 00:01:40", rows[2][0])
}

func TestUsageLogExportAdminRespectsFiltersAndRetainsAdminDetails(t *testing.T) {
	r := setupUsageLogExportController(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin?username=alice&start_timestamp=200&end_timestamp=200", nil))
	require.Equal(t, http.StatusOK, w.Code)
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	defer f.Close()
	rows, err := f.GetRows("Usage Logs")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "alice", rows[1][16])
	assert.Contains(t, rows[1][14], "admin_info")
}

func TestUsageLogExportInvalidFiltersReturnErrorNotAttachment(t *testing.T) {
	r := setupUsageLogExportController(t)
	for _, query := range []string{"type=-1", "type=99", "type=invalid", "start_timestamp=-1", "start_timestamp=200&end_timestamp=100", "end_timestamp=NaN", "channel=-1"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/self?"+query, nil))
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Empty(t, w.Header().Get("Content-Disposition"))
			assert.Contains(t, w.Body.String(), "LOG_EXPORT_INVALID_FILTER")
		})
	}
}

func TestUsageLogExportMissingAuthenticatedOwnerCannotExport(t *testing.T) {
	r := setupUsageLogExportController(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/unauthenticated", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, w.Header().Get("Content-Disposition"))
}

func TestUsageLogExportDatabaseErrorDoesNotDiscloseInternalDetails(t *testing.T) {
	r := setupUsageLogExportController(t)
	require.NoError(t, model.LOG_DB.Migrator().DropTable(&model.Log{}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/self", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "LOG_EXPORT_FAILED")
	assert.NotContains(t, w.Body.String(), "no such table")
	assert.Empty(t, w.Header().Get("Content-Disposition"))
}
