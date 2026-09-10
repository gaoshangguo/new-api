package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRegistrationCompanyTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Company{},
		&model.CustomerAssignment{},
		&model.BusinessAuditEvent{},
		&model.CasbinRule{},
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

func createRegistrationTestUser(t *testing.T, db *gorm.DB, id int, username string, roleKey string) *model.User {
	t.Helper()
	user := &model.User{
		Id:       id,
		Username: username,
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  username + "-aff",
	}
	require.NoError(t, db.Create(user).Error)
	if roleKey != "" {
		require.NoError(t, db.Create(&model.CasbinRule{
			Ptype: "g",
			V0:    authz.UserSubject(id),
			V1:    authz.RoleSubject(roleKey),
		}).Error)
	}
	return user
}

// 注册后自动生成企业，默认归属平台管理账户。
func TestEnsureCompanyForNewUserAssignsPlatformManagerByDefault(t *testing.T) {
	db := setupRegistrationCompanyTest(t)
	manager := createRegistrationTestUser(t, db, 7, "platform-manager", authz.BusinessRolePlatformManager)
	newUser := createRegistrationTestUser(t, db, 100, "fresh-user", "")

	ensureCompanyForNewUser(newUser, 0)

	var company model.Company
	require.NoError(t, db.Where("owner_user_id = ?", newUser.Id).First(&company).Error)
	assert.Equal(t, "fresh-user的企业", company.Name)
	var assignment model.CustomerAssignment
	require.NoError(t, db.Where("company_id = ? AND active = ?", company.Id, true).First(&assignment).Error)
	assert.Equal(t, manager.Id, assignment.SalesUserId)
	assert.Equal(t, "auto company on registration", assignment.Reason)
}

// 邀请注册优先归属邀请人（销售监管账户），而不是平台管理账户。
func TestEnsureCompanyForNewUserPrefersInvitingSalesSupervisor(t *testing.T) {
	db := setupRegistrationCompanyTest(t)
	createRegistrationTestUser(t, db, 7, "platform-manager", authz.BusinessRolePlatformManager)
	inviter := createRegistrationTestUser(t, db, 9, "inviting-sales", authz.BusinessRoleSalesSupervisor)
	newUser := createRegistrationTestUser(t, db, 100, "invited-user", "")

	ensureCompanyForNewUser(newUser, inviter.Id)

	var assignments []model.CustomerAssignment
	require.NoError(t, db.Where("active = ?", true).Find(&assignments).Error)
	require.Len(t, assignments, 1)
	assert.Equal(t, inviter.Id, assignments[0].SalesUserId)
}

// 没有可用归属人时跳过企业创建，注册流程本身不受影响。
func TestEnsureCompanyForNewUserSkipsWithoutEligibleOwner(t *testing.T) {
	db := setupRegistrationCompanyTest(t)
	newUser := createRegistrationTestUser(t, db, 100, "fresh-user", "")

	ensureCompanyForNewUser(newUser, 0)

	var count int64
	require.NoError(t, db.Model(&model.Company{}).Count(&count).Error)
	assert.Zero(t, count)
}

// 关闭开关后不再自动生成企业。
func TestEnsureCompanyForNewUserHonorsDisableSwitch(t *testing.T) {
	db := setupRegistrationCompanyTest(t)
	t.Setenv("BUSINESS_AUTO_COMPANY_ON_REGISTER", "false")
	createRegistrationTestUser(t, db, 7, "platform-manager", authz.BusinessRolePlatformManager)
	newUser := createRegistrationTestUser(t, db, 100, "fresh-user", "")

	ensureCompanyForNewUser(newUser, 0)

	var count int64
	require.NoError(t, db.Model(&model.Company{}).Count(&count).Error)
	assert.Zero(t, count)
}
