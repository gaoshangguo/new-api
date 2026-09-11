package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSalesAccountPromotionTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.SalesAccountProfile{},
		&model.CasbinRule{},
		&model.BusinessAuditEvent{},
		&model.Log{},
	))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedis
	})
	return db
}

func createSalesPromotionTestUser(t *testing.T, db *gorm.DB, id int, username string, role int, roleKey string) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		Id:       id,
		Username: username,
		Password: "password",
		Role:     role,
		Status:   common.UserStatusEnabled,
		AffCode:  username + "-aff",
	}).Error)
	if roleKey != "" {
		require.NoError(t, db.Create(&model.CasbinRule{
			Ptype: "g",
			V0:    authz.UserSubject(id),
			V1:    authz.RoleSubject(roleKey),
		}).Error)
	}
}

func callPromoteUserToSalesAccount(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/business/sales/accounts/promote", strings.NewReader(body))
	ctx.Set("id", 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)
	PromoteUserToSalesAccount(ctx)
	return rec
}

// 用户管理“设为销售账号”：普通用户获得销售监管角色并保存销售资料，原有业务角色保留。
func TestPromoteUserToSalesAccountGrantsRoleAndProfile(t *testing.T) {
	db := setupSalesAccountPromotionTest(t)
	createSalesPromotionTestUser(t, db, 50, "seller", common.RoleCommonUser, authz.BusinessRolePlatformManager)

	rec := callPromoteUserToSalesAccount(t, `{"user_id":50,"department":"华东销售部","region":"上海","note":"demo"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"success":true`)

	roles, err := authz.UserBusinessRoles(db, 50)
	require.NoError(t, err)
	assert.Contains(t, roles, authz.BusinessRoleSalesSupervisor)
	assert.Contains(t, roles, authz.BusinessRolePlatformManager)

	var profile model.SalesAccountProfile
	require.NoError(t, db.Where("user_id = ?", 50).First(&profile).Error)
	assert.Equal(t, "华东销售部", profile.Department)
	assert.Equal(t, "上海", profile.Region)
	assert.Equal(t, "demo", profile.Note)

	var event model.BusinessAuditEvent
	require.NoError(t, db.Where("action = ?", "sales.account.promote").First(&event).Error)
	assert.Equal(t, 50, event.ResourceId)
	assert.Equal(t, 1, event.ActorUserId)
}

// 管理员等非普通用户不能通过该操作获得业务角色。
func TestPromoteUserToSalesAccountRejectsNonOrdinaryUser(t *testing.T) {
	db := setupSalesAccountPromotionTest(t)
	createSalesPromotionTestUser(t, db, 60, "admin-user", common.RoleAdminUser, "")

	rec := callPromoteUserToSalesAccount(t, `{"user_id":60}`)
	assert.Contains(t, rec.Body.String(), "sales accounts must be ordinary users")

	var ruleCount int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Count(&ruleCount).Error)
	assert.Zero(t, ruleCount)
	var profileCount int64
	require.NoError(t, db.Model(&model.SalesAccountProfile{}).Count(&profileCount).Error)
	assert.Zero(t, profileCount)
}

// 已是销售账号的用户不能重复设置，避免覆盖既有销售资料。
func TestPromoteUserToSalesAccountRejectsExistingSalesAccount(t *testing.T) {
	db := setupSalesAccountPromotionTest(t)
	createSalesPromotionTestUser(t, db, 70, "existing-seller", common.RoleCommonUser, authz.BusinessRoleSalesSupervisor)

	rec := callPromoteUserToSalesAccount(t, `{"user_id":70,"department":"覆盖尝试"}`)
	assert.Contains(t, rec.Body.String(), "user is already a sales account")

	var profileCount int64
	require.NoError(t, db.Model(&model.SalesAccountProfile{}).Count(&profileCount).Error)
	assert.Zero(t, profileCount)
}
