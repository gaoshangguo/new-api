package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPersonalNotificationController(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousRedis := model.DB, common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserNotification{}, &model.UserLowBalanceState{}))
	model.DB, common.RedisEnabled = db, false
	t.Cleanup(func() { model.DB, common.RedisEnabled = previousDB, previousRedis })
	user := model.User{Id: 1, Username: "inbox-user", AffCode: "inbox-user"}
	user.SetSetting(dto.UserSetting{Language: "en", SidebarModules: "sidebar", BillingPreference: "wallet_only", UpstreamModelUpdateNotifyEnabled: true})
	require.NoError(t, db.Create(&user).Error)
	return db
}

func TestPersonalNotificationAPICannotReadAnotherUsersMessage(t *testing.T) {
	db := setupPersonalNotificationController(t)
	foreign := model.UserNotification{UserId: 2, Type: "low_balance", CreatedAt: 100}
	own := model.UserNotification{UserId: 1, Type: "low_balance", CreatedAt: 100}
	require.NoError(t, db.Create(&foreign).Error)
	require.NoError(t, db.Create(&own).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", 1) })
	router.GET("/notifications", GetMyNotifications)
	router.POST("/notifications/:id/read", ReadMyNotification)
	listed := httptest.NewRecorder()
	router.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/notifications", nil))
	var response struct {
		Success bool
		Data    model.UserNotificationInbox
	}
	require.NoError(t, common.Unmarshal(listed.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, own.Id, response.Data.Items[0].Id)
	assert.EqualValues(t, 1, response.Data.UnreadCount)
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/notifications/"+strconv.Itoa(foreign.Id)+"/read", nil))
	assert.Equal(t, http.StatusNotFound, denied.Code)
	require.NoError(t, db.First(&foreign, foreign.Id).Error)
	assert.Zero(t, foreign.ReadAt)
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/notifications/"+strconv.Itoa(own.Id)+"/read", nil))
		assert.Equal(t, http.StatusOK, rec.Code)
	}
	inbox, err := model.ListUserNotifications(context.Background(), 1, 0)
	require.NoError(t, err)
	assert.Zero(t, inbox.UnreadCount)
}

func TestPersonalNotificationCursorKeepsUnreadCountAndOwnership(t *testing.T) {
	db := setupPersonalNotificationController(t)
	items := make([]model.UserNotification, 51)
	for i := range items {
		items[i] = model.UserNotification{UserId: 1, Type: "low_balance"}
	}
	require.NoError(t, db.Create(&items).Error)
	first, err := model.ListUserNotifications(context.Background(), 1, 0)
	require.NoError(t, err)
	require.Len(t, first.Items, 50)
	next, err := model.ListUserNotifications(context.Background(), 1, first.Items[49].Id)
	require.NoError(t, err)
	require.Len(t, next.Items, 1)
	assert.Equal(t, items[0].Id, next.Items[0].Id)
	assert.EqualValues(t, 51, next.UnreadCount)
}

func TestPersonalNotificationSettingsPreserveOtherPreferencesAndAllowIndependentEmail(t *testing.T) {
	db := setupPersonalNotificationController(t)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set("id", 1)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/setting", strings.NewReader(`{"notify_type":"webhook","webhook_url":"https://example.com/hook","quota_warning_threshold":10000000,"low_balance_in_app_enabled":false,"low_balance_email_enabled":true,"notification_email":"notify@example.com","upstream_model_update_notify_enabled":false}`))
	UpdateUserSetting(ctx)
	var response struct{ Success bool }
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success, rec.Body.String())
	var user model.User
	require.NoError(t, db.First(&user, 1).Error)
	settings := user.GetSetting()
	require.NotNil(t, settings.LowBalanceInAppEnabled)
	require.NotNil(t, settings.LowBalanceEmailEnabled)
	assert.False(t, *settings.LowBalanceInAppEnabled)
	assert.True(t, *settings.LowBalanceEmailEnabled)
	assert.Equal(t, "notify@example.com", settings.NotificationEmail)
	assert.Equal(t, "en", settings.Language)
	assert.Equal(t, "sidebar", settings.SidebarModules)
	assert.Equal(t, "wallet_only", settings.BillingPreference)
	assert.True(t, settings.UpstreamModelUpdateNotifyEnabled, "ordinary users cannot change admin notification settings")
	// Older clients that omit the new switches must not reset channel choices.
	legacyRec := httptest.NewRecorder()
	legacyCtx, _ := gin.CreateTestContext(legacyRec)
	legacyCtx.Set("id", 1)
	legacyCtx.Request = httptest.NewRequest(http.MethodPut, "/setting", strings.NewReader(`{"notify_type":"email","quota_warning_threshold":20}`))
	UpdateUserSetting(legacyCtx)
	require.NoError(t, db.First(&user, 1).Error)
	settings = user.GetSetting()
	require.NotNil(t, settings.LowBalanceInAppEnabled)
	require.NotNil(t, settings.LowBalanceEmailEnabled)
	assert.False(t, *settings.LowBalanceInAppEnabled)
	assert.True(t, *settings.LowBalanceEmailEnabled)
}

func TestPersonalNotificationSettingsRejectInvalidThresholdsAndEmail(t *testing.T) {
	setupPersonalNotificationController(t)
	for _, body := range []string{
		`{"notify_type":"email","quota_warning_threshold":0}`,
		`{"notify_type":"email","quota_warning_threshold":0.5}`,
		`{"notify_type":"email","quota_warning_threshold":2147483648}`,
		`{"notify_type":"email","quota_warning_threshold":20,"notification_email":"bad@example.com\r\nBcc: other@example.com"}`,
		`{"notify_type":"email","quota_warning_threshold":20,"notification_email":"a@example.com;b@example.com"}`,
	} {
		t.Run(body, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Set("id", 1)
			ctx.Request = httptest.NewRequest(http.MethodPut, "/setting", strings.NewReader(body))
			UpdateUserSetting(ctx)
			var response struct{ Success bool }
			require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
			assert.False(t, response.Success)
		})
	}
}
