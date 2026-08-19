package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupBusinessProjectRuntimeMiddlewareTest(t *testing.T, projectStatus int, modelLimits string, channelLimits string, role int) (*model.Token, *model.BusinessProject) {
	t.Helper()
	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Company{},
		&model.BusinessProject{},
		&model.BusinessConsumption{},
		&model.BusinessProjectBudgetReservation{},
		&model.Channel{},
	))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.RedisEnabled = previousRedisEnabled
	})

	user := &model.User{
		Username:    "runtime-middleware-user",
		Password:    "password",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Quota:       common.MaxQuota,
		AffCode:     "runtime-middleware-aff",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(user).Error)
	company := &model.Company{Name: "Runtime Middleware Company", OwnerUserId: user.Id}
	require.NoError(t, db.Create(company).Error)
	project := &model.BusinessProject{
		CompanyId:     company.Id,
		Name:          "Runtime Middleware Project",
		OwnerUserId:   user.Id,
		ModelLimits:   modelLimits,
		ChannelLimits: channelLimits,
		Status:        projectStatus,
	}
	require.NoError(t, db.Create(project).Error)
	token := &model.Token{
		UserId:      user.Id,
		ProjectId:   project.Id,
		Name:        "runtime middleware key",
		Key:         "runtimekey",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: common.MaxQuota,
	}
	require.NoError(t, db.Create(token).Error)
	return token, project
}

func TestTokenAuthRejectsDisabledBusinessProjectBeforeRelay(t *testing.T) {
	token, _ := setupBusinessProjectRuntimeMiddlewareTest(t, model.BusinessProjectStatusDisabled, "", "", common.RoleCommonUser)
	router := gin.New()
	router.GET("/v1/test", TokenAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	request.Header.Set("Authorization", "Bearer "+token.Key)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Contains(t, response.Body.String(), "access_denied")
}

func TestDistributeRejectsProjectModelAndSpecificChannelOutsideAllowLists(t *testing.T) {
	t.Run("model", func(t *testing.T) {
		token, _ := setupBusinessProjectRuntimeMiddlewareTest(t, model.BusinessProjectStatusEnabled, `["gpt-4o"]`, "", common.RoleCommonUser)
		router := gin.New()
		router.POST("/v1/chat/completions", TokenAuth(), Distribute(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4.1"}`))
		request.Header.Set("Authorization", "Bearer "+token.Key)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		assert.Equal(t, http.StatusForbidden, response.Code)
		assert.Contains(t, response.Body.String(), "access_denied")
	})

	t.Run("specific channel", func(t *testing.T) {
		token, project := setupBusinessProjectRuntimeMiddlewareTest(t, model.BusinessProjectStatusEnabled, `["gpt-4o"]`, `[]`, common.RoleAdminUser)
		allowedChannel := &model.Channel{Name: "allowed project channel", Status: common.ChannelStatusEnabled}
		disallowedChannel := &model.Channel{Name: "disallowed project channel", Status: common.ChannelStatusEnabled}
		require.NoError(t, model.DB.Create(allowedChannel).Error)
		require.NoError(t, model.DB.Create(disallowedChannel).Error)
		require.NoError(t, model.DB.Model(&model.BusinessProject{}).Where("id = ?", project.Id).Update("channel_limits", fmt.Sprintf("[%d]", allowedChannel.Id)).Error)

		router := gin.New()
		router.POST("/v1/chat/completions", TokenAuth(), Distribute(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4o"}`))
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s-%d", token.Key, disallowedChannel.Id))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		assert.Equal(t, http.StatusForbidden, response.Code)
		assert.Contains(t, response.Body.String(), "access_denied")
	})
}
