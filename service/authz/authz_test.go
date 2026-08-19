package authz

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAuthzTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	wasMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() {
		common.IsMasterNode = wasMaster
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.CasbinRule{}, &model.AuthzRole{}))
	return db
}

func TestInitSeedsBuiltInRolesAndPoliciesOnce(t *testing.T) {
	db := newAuthzTestDB(t)

	require.NoError(t, Init(db))
	require.NoError(t, Init(db))

	// Root is a superuser role and is granted everything implicitly. Every
	// non-superuser built-in role receives only its baseline policy rows.
	var count int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Count(&count).Error)
	expectedPolicies := 0
	for _, role := range builtInRoles {
		if !role.Superuser {
			expectedPolicies += len(PermissionsForRole(role.Key))
		}
	}
	assert.Equal(t, int64(expectedPolicies), count)

	var roles []model.AuthzRole
	require.NoError(t, db.Order("sort asc").Find(&roles).Error)
	require.Len(t, roles, len(builtInRoles))
	assert.Equal(t, BuiltInRoleRoot, roles[0].Key)
	assert.Equal(t, BuiltInRoleAdmin, roles[1].Key)

	assert.True(t, Can(1, common.RoleRootUser, ChannelSensitiveWrite))
	assert.True(t, Can(2, common.RoleAdminUser, ChannelRead))
	assert.True(t, Can(2, common.RoleAdminUser, ChannelOperate))
	assert.True(t, Can(2, common.RoleAdminUser, ChannelWrite))
	assert.False(t, Can(2, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(3, common.RoleCommonUser, ChannelRead))
}

func TestInitOnSlaveOnlyLoadsPolicies(t *testing.T) {
	wasMaster := common.IsMasterNode
	common.IsMasterNode = false
	t.Cleanup(func() {
		common.IsMasterNode = wasMaster
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.CasbinRule{}, &model.AuthzRole{}))

	require.NoError(t, Init(db))

	var roleCount int64
	require.NoError(t, db.Model(&model.AuthzRole{}).Count(&roleCount).Error)
	assert.Equal(t, int64(0), roleCount)
	var policyCount int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Count(&policyCount).Error)
	assert.Equal(t, int64(0), policyCount)
	assert.False(t, Can(2, common.RoleAdminUser, ChannelRead))
}

func TestSetUserPermissionsStoresOnlyOverrides(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	require.NoError(t, SetUserPermissions(42, PermissionsMap{
		ResourceChannel: {
			ActionRead:           true,
			ActionOperate:        true,
			ActionWrite:          false,
			ActionSensitiveWrite: true,
			ActionSecretView:     false,
			"unknown":            true,
		},
		"unknown": {
			ActionRead: true,
		},
	}))

	assert.True(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(42, common.RoleAdminUser, ChannelWrite))
	assert.Equal(t, map[string]bool{
		ActionRead:           true,
		ActionOperate:        true,
		ActionWrite:          false,
		ActionSensitiveWrite: true,
		ActionSecretView:     false,
	}, ExplicitUserPermissions(42)[ResourceChannel])
	assert.Equal(t, PermissionsMap{
		ResourceChannel: {
			ActionSensitiveWrite: true,
			ActionWrite:          false,
		},
	}, ExplicitUserOverrides(42))

	var userPolicyCount int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Where("v0 = ?", UserSubject(42)).Count(&userPolicyCount).Error)
	assert.Equal(t, int64(2), userPolicyCount)

	require.NoError(t, SetUserPermissions(42, PermissionsMap{ResourceChannel: {
		ActionRead:           true,
		ActionOperate:        true,
		ActionWrite:          true,
		ActionSensitiveWrite: false,
		ActionSecretView:     false,
	}}))
	assert.False(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.Equal(t, map[string]bool{
		ActionRead:           true,
		ActionOperate:        true,
		ActionWrite:          true,
		ActionSensitiveWrite: false,
		ActionSecretView:     false,
	}, ExplicitUserPermissions(42)[ResourceChannel])
	assert.Empty(t, ExplicitUserOverrides(42))
}

func TestClearUserAuthorizationRemovesOverrides(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	require.NoError(t, SetUserPermissions(90, PermissionsMap{ResourceChannel: {
		ActionWrite:          false,
		ActionSensitiveWrite: true,
	}}))

	assert.True(t, Can(90, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(90, common.RoleAdminUser, ChannelWrite))

	require.NoError(t, ClearUserAuthorization(90))

	assert.Empty(t, ExplicitUserOverrides(90))
	assert.True(t, Can(90, common.RoleAdminUser, ChannelRead))
	assert.True(t, Can(90, common.RoleAdminUser, ChannelWrite))
	assert.False(t, Can(90, common.RoleAdminUser, ChannelSensitiveWrite))
	assert.False(t, Can(90, common.RoleCommonUser, ChannelRead))
}

func TestSetUserPermissionsInTxDoesNotMutateEnforcerBeforeReload(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return SetUserPermissionsInTx(tx, 42, PermissionsMap{ResourceChannel: {
			ActionRead:           true,
			ActionOperate:        true,
			ActionWrite:          true,
			ActionSensitiveWrite: true,
			ActionSecretView:     false,
		}})
	}))

	assert.False(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
	require.NoError(t, ReloadPolicy())
	assert.True(t, Can(42, common.RoleAdminUser, ChannelSensitiveWrite))
}

func TestSetUserPermissionsInTxRollbackLeavesNoPolicy(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	tx := db.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, SetUserPermissionsInTx(tx, 43, PermissionsMap{ResourceChannel: {
		ActionSensitiveWrite: true,
	}}))
	require.NoError(t, tx.Rollback().Error)
	require.NoError(t, ReloadPolicy())

	assert.False(t, Can(43, common.RoleAdminUser, ChannelSensitiveWrite))
	var count int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Where("v0 = ?", UserSubject(43)).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestAdapterAddPolicyIsIdempotent(t *testing.T) {
	db := newAuthzTestDB(t)
	adapter := newGormAdapter(db)
	rule := []string{UserSubject(55), ResourceChannel, ActionSensitiveWrite, EffectAllow}

	require.NoError(t, adapter.AddPolicy("p", "p", rule))
	require.NoError(t, adapter.AddPolicy("p", "p", rule))

	var count int64
	require.NoError(t, db.Model(&model.CasbinRule{}).Where(
		"ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ?",
		"p",
		UserSubject(55),
		ResourceChannel,
		ActionSensitiveWrite,
		EffectAllow,
	).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCapabilitiesUseCatalogShape(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	capabilities := Capabilities(7, common.RoleAdminUser)

	assert.True(t, capabilities[ResourceChannel][ActionRead])
	assert.True(t, capabilities[ResourceChannel][ActionOperate])
	assert.True(t, capabilities[ResourceChannel][ActionWrite])
	assert.False(t, capabilities[ResourceChannel][ActionSensitiveWrite])
	assert.False(t, capabilities[ResourceChannel][ActionSecretView])
}

func TestBusinessRoleAssignmentsUsePersistedCasbinGroups(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	financeEntry := model.User{
		Username: "finance-entry",
		Password: "password",
		AffCode:  "finance-entry-aff",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&financeEntry).Error)

	require.NoError(t, SetUserBusinessRoles(db, financeEntry.Id, []string{BusinessRoleFinanceEntry}))
	assert.Equal(t, []string{BusinessRoleFinanceEntry}, mustUserBusinessRoles(t, db, financeEntry.Id))
	enforced, err := currentEnforcer().Enforce(UserSubject(financeEntry.Id), ResourceBusinessManualCredit, ActionCreate)
	require.NoError(t, err)
	assert.True(t, enforced)
	assert.True(t, Can(financeEntry.Id, common.RoleCommonUser, BusinessManualCreditCreate))
	assert.True(t, Can(financeEntry.Id, common.RoleCommonUser, BusinessManualCreditRead))
	assert.False(t, Can(financeEntry.Id, common.RoleCommonUser, BusinessManualCreditApprove))
	assert.False(t, Can(financeEntry.Id, common.RoleCommonUser, ChannelRead))

	var assignmentRule model.CasbinRule
	require.NoError(t, db.Where("ptype = ? AND v0 = ? AND v1 = ?", "g", UserSubject(financeEntry.Id), RoleSubject(BusinessRoleFinanceEntry)).First(&assignmentRule).Error)
}

func TestBusinessRoleBaselinesAndRootAccess(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	tests := []struct {
		name    string
		roleKey string
		allowed Permission
		denied  []Permission
	}{
		{
			name:    "platform manager",
			roleKey: BusinessRolePlatformManager,
			allowed: BusinessProjectUpdate,
			denied:  []Permission{BusinessManualCreditApprove},
		},
		{
			name:    "finance entry",
			roleKey: BusinessRoleFinanceEntry,
			allowed: BusinessManualCreditCreate,
			denied:  []Permission{BusinessManualCreditApprove},
		},
		{
			name:    "finance approver",
			roleKey: BusinessRoleFinanceApprover,
			allowed: BusinessManualCreditApprove,
			denied:  []Permission{BusinessManualCreditCreate},
		},
		{
			name:    "sales supervisor",
			roleKey: BusinessRoleSalesSupervisor,
			allowed: BusinessFollowUpCreate,
			denied: []Permission{
				BusinessCustomerAssignmentUpdate,
				BusinessReportRead,
				BusinessManualCreditCreate,
				BusinessCompanyRead,
				BusinessProjectRead,
				BusinessLedgerRead,
			},
		},
		{
			name:    "operations reader",
			roleKey: BusinessRoleOperationsReader,
			allowed: BusinessOperationsRead,
			denied: []Permission{
				BusinessOperationsUpdate,
				BusinessCompanyRead,
				BusinessProjectRead,
				BusinessLedgerRead,
				BusinessManualCreditRead,
				BusinessSalesRead,
				BusinessFollowUpRead,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := model.User{
				Username: tt.roleKey,
				Password: "password",
				AffCode:  "aff-" + tt.roleKey,
				Role:     common.RoleCommonUser,
				Status:   common.UserStatusEnabled,
			}
			require.NoError(t, db.Create(&user).Error)
			require.NoError(t, SetUserBusinessRoles(db, user.Id, []string{tt.roleKey}))
			assert.True(t, Can(user.Id, common.RoleCommonUser, tt.allowed))
			assert.True(t, HasBusinessRole(user.Id, common.RoleCommonUser, tt.roleKey))
			for _, permission := range tt.denied {
				assert.False(t, Can(user.Id, common.RoleCommonUser, permission))
			}
		})
	}

	assert.True(t, Can(1, common.RoleRootUser, BusinessManualCreditApprove))
	assert.True(t, Can(1, common.RoleRootUser, BusinessOperationsUpdate))
	assert.True(t, HasBusinessRole(1, common.RoleRootUser, BusinessRolePlatformManager))
}

func TestBusinessRoleAssignmentRejectsNonOrdinaryUsersAndUnknownRoles(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	admin := model.User{
		Username: "system-admin",
		Password: "password",
		AffCode:  "system-admin-aff",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&admin).Error)

	require.Error(t, SetUserBusinessRoles(db, admin.Id, []string{BusinessRoleFinanceEntry}))
	require.Error(t, SetUserBusinessRoles(db, admin.Id, []string{"unknown"}))
	assert.Empty(t, mustUserBusinessRoles(t, db, admin.Id))
}

func TestSetUserBusinessRolesInTxRollsBack(t *testing.T) {
	db := newAuthzTestDB(t)
	require.NoError(t, Init(db))

	user := model.User{
		Username: "finance-entry-rollback",
		Password: "password",
		AffCode:  "finance-entry-rollback-aff",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&user).Error)

	tx := db.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, SetUserBusinessRolesInTx(tx, user.Id, []string{BusinessRoleFinanceEntry}))
	require.NoError(t, tx.Rollback().Error)
	require.NoError(t, ReloadPolicy())

	assert.Empty(t, mustUserBusinessRoles(t, db, user.Id))
	assert.False(t, Can(user.Id, common.RoleCommonUser, BusinessManualCreditCreate))
}

func mustUserBusinessRoles(t *testing.T, db *gorm.DB, userID int) []string {
	t.Helper()
	roles, err := UserBusinessRoles(db, userID)
	require.NoError(t, err)
	return roles
}
