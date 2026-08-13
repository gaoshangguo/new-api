package authz

import (
	"fmt"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// resolveSubjectRoles returns the role keys assigned to a subject. Numeric
// system roles remain the compatibility baseline. Business roles are assigned
// only to ordinary users through persisted Casbin g relations.
var resolveSubjectRoles = func(userID int, systemRole int) []string {
	switch {
	case systemRole >= common.RoleRootUser:
		return []string{BuiltInRoleRoot}
	case systemRole >= common.RoleAdminUser:
		return []string{BuiltInRoleAdmin}
	case systemRole != common.RoleCommonUser || userID <= 0:
		return nil
	}
	return activeBusinessRoles(userID)
}

// SetUserBusinessRoles assigns the complete business-role set for one ordinary
// user and refreshes the local authorization snapshot after the transaction.
func SetUserBusinessRoles(db *gorm.DB, userID int, roleKeys []string) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return SetUserBusinessRolesInTx(tx, userID, roleKeys)
	}); err != nil {
		return err
	}
	return ReloadPolicy()
}

// SetUserBusinessRolesInTx assigns the complete business-role set for one
// ordinary user. The caller must reload the authorization policy after a
// successful outer transaction commits.
func SetUserBusinessRolesInTx(tx *gorm.DB, userID int, roleKeys []string) error {
	if tx == nil {
		return fmt.Errorf("database transaction is not initialized")
	}
	if userID <= 0 {
		return fmt.Errorf("invalid user id")
	}
	roles, err := normalizeBusinessRoleKeys(roleKeys)
	if err != nil {
		return err
	}

	var user model.User
	if err := tx.Select("id, role").First(&user, userID).Error; err != nil {
		return err
	}
	if user.Role != common.RoleCommonUser {
		return fmt.Errorf("business roles can only be assigned to ordinary users")
	}

	roleSubjects := businessRoleSubjects()
	if err := tx.Where("ptype = ? AND v0 = ? AND v1 IN ?", "g", UserSubject(userID), roleSubjects).Delete(&model.CasbinRule{}).Error; err != nil {
		return err
	}
	if len(roles) == 0 {
		return nil
	}

	rules := make([]model.CasbinRule, 0, len(roles))
	for _, roleKey := range roles {
		rules = append(rules, newRule("g", []string{UserSubject(userID), RoleSubject(roleKey)}))
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rules).Error
}

// UserBusinessRoles reads the persisted business-role set for a user. It
// accepts a DB or transaction so callers can read their own transaction state.
func UserBusinessRoles(db *gorm.DB, userID int) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id")
	}

	var rules []model.CasbinRule
	if err := db.Where("ptype = ? AND v0 = ? AND v1 IN ?", "g", UserSubject(userID), businessRoleSubjects()).
		Order("v1 asc").Find(&rules).Error; err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(rules))
	for _, rule := range rules {
		roleKey, ok := roleKeyFromSubject(rule.V1)
		if ok && isBusinessRole(roleKey) {
			roles = append(roles, roleKey)
		}
	}
	return roles, nil
}

func activeBusinessRoles(userID int) []string {
	e := currentEnforcer()
	if e == nil {
		return nil
	}
	roleSubjects, err := e.GetRolesForUser(UserSubject(userID))
	if err != nil {
		return nil
	}
	roles := make([]string, 0, len(roleSubjects))
	for _, roleSubject := range roleSubjects {
		roleKey, ok := roleKeyFromSubject(roleSubject)
		if ok && isBusinessRole(roleKey) {
			roles = append(roles, roleKey)
		}
	}
	sort.Strings(roles)
	return roles
}

// HasBusinessRole is intentionally role-based rather than permission-based.
// It is used for data-scope decisions where an ad-hoc permission override must
// not accidentally expand a sales user's customer scope. Root remains the
// bootstrap operator for all business scopes.
func HasBusinessRole(userID int, systemRole int, roleKey string) bool {
	if systemRole >= common.RoleRootUser {
		return true
	}
	if systemRole != common.RoleCommonUser || !isBusinessRole(roleKey) {
		return false
	}
	for _, assignedRole := range activeBusinessRoles(userID) {
		if assignedRole == roleKey {
			return true
		}
	}
	return false
}

func normalizeBusinessRoleKeys(roleKeys []string) ([]string, error) {
	seen := make(map[string]struct{}, len(roleKeys))
	roles := make([]string, 0, len(roleKeys))
	for _, roleKey := range roleKeys {
		if !isBusinessRole(roleKey) {
			return nil, fmt.Errorf("unknown business role %q", roleKey)
		}
		if _, exists := seen[roleKey]; exists {
			continue
		}
		seen[roleKey] = struct{}{}
		roles = append(roles, roleKey)
	}
	sort.Strings(roles)
	return roles, nil
}

func businessRoleSubjects() []string {
	return []string{
		RoleSubject(BusinessRolePlatformManager),
		RoleSubject(BusinessRoleFinanceEntry),
		RoleSubject(BusinessRoleFinanceApprover),
		RoleSubject(BusinessRoleSalesSupervisor),
		RoleSubject(BusinessRoleOperationsReader),
	}
}

func roleKeyFromSubject(subject string) (string, bool) {
	const rolePrefix = "role:"
	if len(subject) <= len(rolePrefix) || subject[:len(rolePrefix)] != rolePrefix {
		return "", false
	}
	return subject[len(rolePrefix):], true
}

// managedRoleKey is the role whose baseline per-user overrides are expressed
// relative to.
const managedRoleKey = BuiltInRoleAdmin
