package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	// ErrBusinessProjectDisabled is returned when a project-owned token is used
	// after its project has been disabled.
	ErrBusinessProjectDisabled = errors.New("business project is disabled")
	// ErrBusinessProjectTokenBinding protects against a token pointing at a
	// project owned by another user or a project that no longer exists.
	ErrBusinessProjectTokenBinding = errors.New("invalid business project token binding")
	// ErrBusinessProjectLimitConfiguration deliberately fails closed. A project
	// with a non-empty but unsupported limits JSON must not silently become
	// unrestricted.
	ErrBusinessProjectLimitConfiguration = errors.New("invalid business project runtime limits")
	ErrBusinessProjectModelForbidden     = errors.New("model is not enabled for this business project")
	ErrBusinessProjectChannelForbidden   = errors.New("channel is not enabled for this business project")
	ErrBusinessProjectBudgetExceeded     = errors.New("business project budget is exhausted")
	ErrBusinessProjectBudgetEstimate     = errors.New("business project budget requires a positive pre-consume estimate")
)

const (
	projectBudgetReservationStatusReserved                   = "reserved"
	projectBudgetReservationStatusSettled                    = "settled"
	projectBudgetReservationStatusReleased                   = "released"
	projectBudgetReservationStatusOverBudgetPendingReconcile = "over_budget_pending_reconciliation"
	projectBudgetReservationStatusReconciled                 = "reconciled"
)

// BusinessProjectRuntimePolicy is an immutable request-time snapshot. It is
// loaded from the primary database on every API-token authentication instead
// of trusting a possibly stale token cache: assigning a token to a project or
// disabling a project therefore takes effect immediately.
//
// Empty ModelLimits / ChannelLimits mean unrestricted. A non-empty config is
// an allow-list and only accepts these portable JSON forms:
//
//   - models: ["model-a"] or {"model-a": true}
//   - channels: [1, 2] or {"1": true, "2": true}
//   - wrappers {"models": [...]} / {"allowed_models": [...]} and
//     {"channels": [...]} / {"allowed_channels": [...]}
//
// Empty arrays/maps are an explicit deny-all allow-list. Unknown structures
// fail closed with ErrBusinessProjectLimitConfiguration.
type BusinessProjectRuntimePolicy struct {
	ProjectId   int
	CompanyId   int
	OwnerUserId int
	BudgetQuota int

	// RateLimitRPM/TPM mirror the project's rate limit configuration at load
	// time so relay scope rate limits can be enforced without a second lookup.
	RateLimitRPM int
	RateLimitTPM int64

	modelLimitsConfigured   bool
	channelLimitsConfigured bool
	allowedModels           map[string]struct{}
	allowedChannels         map[int]struct{}
}

func (policy *BusinessProjectRuntimePolicy) HasModelLimits() bool {
	return policy != nil && policy.modelLimitsConfigured
}

func (policy *BusinessProjectRuntimePolicy) HasChannelLimits() bool {
	return policy != nil && policy.channelLimitsConfigured
}

func (policy *BusinessProjectRuntimePolicy) AllowsModel(modelName string) bool {
	if policy == nil || !policy.modelLimitsConfigured {
		return true
	}
	_, ok := policy.allowedModels[strings.TrimSpace(modelName)]
	return ok
}

func (policy *BusinessProjectRuntimePolicy) AllowsChannel(channelID int) bool {
	if policy == nil || !policy.channelLimitsConfigured {
		return true
	}
	_, ok := policy.allowedChannels[channelID]
	return ok
}

// AllowedChannelIDs returns a copy so channel selection cannot mutate policy
// state shared by the rest of the request.
func (policy *BusinessProjectRuntimePolicy) AllowedChannelIDs() map[int]struct{} {
	if policy == nil || !policy.channelLimitsConfigured {
		return nil
	}
	allowed := make(map[int]struct{}, len(policy.allowedChannels))
	for channelID := range policy.allowedChannels {
		allowed[channelID] = struct{}{}
	}
	return allowed
}

// BusinessProjectBudgetReservation is a short-lived budget hold created before
// the existing billing pre-consume. It closes the race where multiple requests
// all observe the same remaining project budget before any consumption log is
// written. The hold is released on refund and settled atomically with the
// append-only BusinessConsumption projection.
//
// BudgetQuota == 0 intentionally means no project budget cap, preserving the
// project creation default and existing user/token quota behaviour.
type BusinessProjectBudgetReservation struct {
	Id            int    `json:"id"`
	ProjectId     int    `json:"project_id" gorm:"uniqueIndex:idx_business_project_budget_request,priority:1;index"`
	RequestId     string `json:"request_id" gorm:"size:64;uniqueIndex:idx_business_project_budget_request,priority:2"`
	UserId        int    `json:"user_id" gorm:"index"`
	TokenId       int    `json:"token_id" gorm:"index"`
	ReservedQuota int    `json:"reserved_quota"`
	Status        string `json:"status" gorm:"size:48;index"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (*BusinessProjectBudgetReservation) TableName() string {
	return "business_project_budget_reservations"
}

// LoadBusinessProjectRuntimePolicyForToken resolves the authoritative token
// binding from DB. A nil policy means an ordinary personal token and leaves all
// current gateway behaviour unchanged.
func LoadBusinessProjectRuntimePolicyForToken(tokenID int, userID int) (*BusinessProjectRuntimePolicy, error) {
	return loadBusinessProjectRuntimePolicyForToken(DB, tokenID, userID, false)
}

func loadBusinessProjectRuntimePolicyForToken(db *gorm.DB, tokenID int, userID int, lock bool) (*BusinessProjectRuntimePolicy, error) {
	if tokenID <= 0 || userID <= 0 {
		return nil, nil
	}

	tokenQuery := db.Select("id", "user_id", "project_id").Where("id = ? AND user_id = ?", tokenID, userID)
	if lock {
		tokenQuery = lockForUpdate(tokenQuery)
	}
	var token Token
	if err := tokenQuery.First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: token not found", ErrBusinessProjectTokenBinding)
		}
		return nil, err
	}
	if token.ProjectId <= 0 {
		return nil, nil
	}

	projectQuery := db.Select(
		"id", "company_id", "owner_user_id", "budget_quota", "model_limits", "channel_limits", "status",
		"rate_limit_rpm", "rate_limit_tpm",
	).Where("id = ? AND owner_user_id = ?", token.ProjectId, token.UserId)
	if lock {
		projectQuery = lockForUpdate(projectQuery)
	}
	var project BusinessProject
	if err := projectQuery.First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: project ownership mismatch", ErrBusinessProjectTokenBinding)
		}
		return nil, err
	}
	if project.CompanyId <= 0 {
		return nil, fmt.Errorf("%w: project has no company", ErrBusinessProjectTokenBinding)
	}
	if project.Status != BusinessProjectStatusEnabled {
		return nil, ErrBusinessProjectDisabled
	}

	allowedModels, modelConfigured, err := parseBusinessProjectModelLimits(project.ModelLimits)
	if err != nil {
		return nil, err
	}
	allowedChannels, channelConfigured, err := parseBusinessProjectChannelLimits(project.ChannelLimits)
	if err != nil {
		return nil, err
	}
	return &BusinessProjectRuntimePolicy{
		ProjectId:               project.Id,
		CompanyId:               project.CompanyId,
		OwnerUserId:             project.OwnerUserId,
		BudgetQuota:             project.BudgetQuota,
		RateLimitRPM:            project.RateLimitRPM,
		RateLimitTPM:            project.RateLimitTPM,
		modelLimitsConfigured:   modelConfigured,
		channelLimitsConfigured: channelConfigured,
		allowedModels:           allowedModels,
		allowedChannels:         allowedChannels,
	}, nil
}

func parseBusinessProjectModelLimits(config string) (map[string]struct{}, bool, error) {
	config = strings.TrimSpace(config)
	if config == "" {
		return nil, false, nil
	}
	var value any
	if err := common.UnmarshalJsonStr(config, &value); err != nil {
		return nil, true, fmt.Errorf("%w: model limits JSON: %v", ErrBusinessProjectLimitConfiguration, err)
	}
	return parseBusinessProjectStringAllowList(value, "models", "allowed_models")
}

func parseBusinessProjectStringAllowList(value any, wrapperNames ...string) (map[string]struct{}, bool, error) {
	allowed := make(map[string]struct{})
	switch raw := value.(type) {
	case []any:
		for _, item := range raw {
			name, ok := item.(string)
			name = strings.TrimSpace(name)
			if !ok || name == "" {
				return nil, true, fmt.Errorf("%w: allow-list entries must be non-empty strings", ErrBusinessProjectLimitConfiguration)
			}
			allowed[name] = struct{}{}
		}
		return allowed, true, nil
	case map[string]any:
		for _, wrapperName := range wrapperNames {
			if wrapped, ok := raw[wrapperName]; ok {
				if len(raw) != 1 {
					return nil, true, fmt.Errorf("%w: wrapped allow-list cannot contain other fields", ErrBusinessProjectLimitConfiguration)
				}
				return parseBusinessProjectStringAllowList(wrapped)
			}
		}
		for name, enabledValue := range raw {
			enabled, ok := enabledValue.(bool)
			if !ok {
				return nil, true, fmt.Errorf("%w: allow-list object values must be booleans", ErrBusinessProjectLimitConfiguration)
			}
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, true, fmt.Errorf("%w: allow-list entry cannot be empty", ErrBusinessProjectLimitConfiguration)
			}
			if enabled {
				allowed[name] = struct{}{}
			}
		}
		return allowed, true, nil
	default:
		return nil, true, fmt.Errorf("%w: allow-list must be an array or object", ErrBusinessProjectLimitConfiguration)
	}
}

func parseBusinessProjectChannelLimits(config string) (map[int]struct{}, bool, error) {
	config = strings.TrimSpace(config)
	if config == "" {
		return nil, false, nil
	}
	var value any
	if err := common.UnmarshalJsonStr(config, &value); err != nil {
		return nil, true, fmt.Errorf("%w: channel limits JSON: %v", ErrBusinessProjectLimitConfiguration, err)
	}
	return parseBusinessProjectChannelAllowList(value, "channels", "allowed_channels")
}

func parseBusinessProjectChannelAllowList(value any, wrapperNames ...string) (map[int]struct{}, bool, error) {
	allowed := make(map[int]struct{})
	addChannel := func(value any) error {
		floatValue, ok := value.(float64)
		if !ok || floatValue <= 0 || floatValue != float64(int(floatValue)) || floatValue > float64(common.MaxQuota) {
			return fmt.Errorf("%w: channel IDs must be positive integers", ErrBusinessProjectLimitConfiguration)
		}
		allowed[int(floatValue)] = struct{}{}
		return nil
	}

	switch raw := value.(type) {
	case []any:
		for _, item := range raw {
			if err := addChannel(item); err != nil {
				return nil, true, err
			}
		}
		return allowed, true, nil
	case map[string]any:
		for _, wrapperName := range wrapperNames {
			if wrapped, ok := raw[wrapperName]; ok {
				if len(raw) != 1 {
					return nil, true, fmt.Errorf("%w: wrapped allow-list cannot contain other fields", ErrBusinessProjectLimitConfiguration)
				}
				return parseBusinessProjectChannelAllowList(wrapped)
			}
		}
		for rawID, enabledValue := range raw {
			enabled, ok := enabledValue.(bool)
			if !ok {
				return nil, true, fmt.Errorf("%w: allow-list object values must be booleans", ErrBusinessProjectLimitConfiguration)
			}
			channelID, err := strconv.Atoi(strings.TrimSpace(rawID))
			if err != nil || channelID <= 0 || channelID > common.MaxQuota {
				return nil, true, fmt.Errorf("%w: channel IDs must be positive integers", ErrBusinessProjectLimitConfiguration)
			}
			if enabled {
				allowed[channelID] = struct{}{}
			}
		}
		return allowed, true, nil
	default:
		return nil, true, fmt.Errorf("%w: allow-list must be an array or object", ErrBusinessProjectLimitConfiguration)
	}
}

// CheckBusinessProjectBudgetAtAuthentication blocks a project token once
// settled use plus outstanding pre-consume holds reaches its cap. The decisive
// concurrent check lives in ReserveBusinessProjectBudget before billing.
func CheckBusinessProjectBudgetAtAuthentication(policy *BusinessProjectRuntimePolicy) error {
	if policy == nil || policy.BudgetQuota == 0 {
		return nil
	}
	used, reserved, reconciliationPending, err := businessProjectBudgetUsage(DB, policy.ProjectId)
	if err != nil {
		return err
	}
	if reconciliationPending || used < 0 || reserved < 0 || used+reserved >= int64(policy.BudgetQuota) {
		return ErrBusinessProjectBudgetExceeded
	}
	return nil
}

// ReserveBusinessProjectBudget creates a request-scoped hold before the
// existing wallet/subscription pre-consume executes. It serializes on the
// project row with lockForUpdate, which works across MySQL/PostgreSQL and uses
// SQLite's single-writer transaction semantics. This makes concurrent requests
// see each other's pending holds.
func ReserveBusinessProjectBudget(tokenID int, userID int, requestID string, reserveQuota int) error {
	if tokenID <= 0 || userID <= 0 {
		return nil
	}
	if reserveQuota < 0 || reserveQuota > common.MaxQuota {
		return fmt.Errorf("%w: reserve quota is out of range", ErrBusinessProjectBudgetExceeded)
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 64 {
		return fmt.Errorf("%w: request ID is required", ErrBusinessProjectBudgetExceeded)
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		policy, err := loadBusinessProjectRuntimePolicyForToken(tx, tokenID, userID, true)
		if err != nil || policy == nil || policy.BudgetQuota == 0 {
			return err
		}
		if reserveQuota == 0 {
			// A capped project cannot safely use a paid route with an unknown
			// pre-consume amount: it would otherwise bypass the budget before a
			// later settlement can observe the actual charge.
			return ErrBusinessProjectBudgetEstimate
		}

		var existing BusinessProjectBudgetReservation
		err = lockForUpdate(tx).
			Where("project_id = ? AND request_id = ?", policy.ProjectId, requestID).
			First(&existing).Error
		if err == nil {
			if existing.Status == projectBudgetReservationStatusReserved &&
				existing.TokenId == tokenID && existing.UserId == userID && existing.ReservedQuota == reserveQuota {
				return nil
			}
			return fmt.Errorf("%w: request budget hold already exists", ErrBusinessProjectBudgetExceeded)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		used, reserved, reconciliationPending, err := businessProjectBudgetUsage(tx, policy.ProjectId)
		if err != nil {
			return err
		}
		budget := int64(policy.BudgetQuota)
		if reconciliationPending || used < 0 || reserved < 0 || used >= budget || reserved > budget-used || int64(reserveQuota) > budget-used-reserved {
			return ErrBusinessProjectBudgetExceeded
		}

		return tx.Create(&BusinessProjectBudgetReservation{
			ProjectId:     policy.ProjectId,
			RequestId:     requestID,
			UserId:        userID,
			TokenId:       tokenID,
			ReservedQuota: reserveQuota,
			Status:        projectBudgetReservationStatusReserved,
		}).Error
	})
}

// ExpandBusinessProjectBudgetReservation increases the existing hold for a
// request before BillingSession reserves an additional amount. It is not a
// post-settlement correction: callers must invoke it before changing wallet,
// subscription, or token quotas so a capped project cannot bypass its budget
// through a late pre-consume expansion.
func ExpandBusinessProjectBudgetReservation(tokenID int, userID int, requestID string, additionalQuota int) error {
	if additionalQuota == 0 || tokenID <= 0 || userID <= 0 {
		return nil
	}
	requestID = strings.TrimSpace(requestID)
	if additionalQuota < 0 || additionalQuota > common.MaxQuota || requestID == "" || len(requestID) > 64 {
		return fmt.Errorf("%w: reservation expansion is invalid", ErrBusinessProjectBudgetExceeded)
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		policy, err := loadBusinessProjectRuntimePolicyForToken(tx, tokenID, userID, true)
		if err != nil || policy == nil || policy.BudgetQuota == 0 {
			return err
		}
		var reservation BusinessProjectBudgetReservation
		if err := lockForUpdate(tx).
			Where("project_id = ? AND token_id = ? AND user_id = ? AND request_id = ?", policy.ProjectId, tokenID, userID, requestID).
			First(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: no initial request hold", ErrBusinessProjectBudgetExceeded)
			}
			return err
		}
		if reservation.Status != projectBudgetReservationStatusReserved {
			return fmt.Errorf("%w: request hold is no longer pending", ErrBusinessProjectBudgetExceeded)
		}

		used, reserved, reconciliationPending, err := businessProjectBudgetUsage(tx, policy.ProjectId)
		if err != nil {
			return err
		}
		budget := int64(policy.BudgetQuota)
		if reconciliationPending || used < 0 || reserved < 0 || used >= budget || reserved > budget-used || int64(additionalQuota) > budget-used-reserved {
			return ErrBusinessProjectBudgetExceeded
		}
		return tx.Model(&BusinessProjectBudgetReservation{}).
			Where("id = ? AND status = ?", reservation.Id, projectBudgetReservationStatusReserved).
			Update("reserved_quota", gorm.Expr("reserved_quota + ?", additionalQuota)).Error
	})
}

// EnsureBusinessProjectBudgetReservation raises an existing request hold to
// targetQuota when a post-upstream settlement proves that the original
// pre-consume estimate was too small. It returns the amount added so the
// caller can roll back precisely that addition if the following funding
// settlement fails.
//
// Unlike the pre-request reserve path, this function intentionally permits a
// project that was disabled or reconfigured after the request began. It never
// creates a new hold: the request must already have a reservation that was
// established while the project token was valid. This lets an in-flight
// request settle safely without reopening access for new requests.
func EnsureBusinessProjectBudgetReservation(tokenID int, userID int, requestID string, targetQuota int) (int, error) {
	requestID = strings.TrimSpace(requestID)
	if tokenID <= 0 || userID <= 0 || requestID == "" || len(requestID) > 64 {
		return 0, nil
	}
	if targetQuota < 0 || targetQuota > common.MaxQuota {
		return 0, fmt.Errorf("%w: reservation target is out of range", ErrBusinessProjectBudgetExceeded)
	}

	addedQuota := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Find the existing hold first so a project/token reconfiguration after
		// the request began cannot strand financial settlement. The row is
		// reloaded under lock after taking the project lock, preserving the
		// project -> reservation ordering used everywhere else.
		var snapshot BusinessProjectBudgetReservation
		if err := tx.
			Where("token_id = ? AND user_id = ? AND request_id = ?", tokenID, userID, requestID).
			First(&snapshot).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Personal and unlimited-project sessions do not have a hold.
				return nil
			}
			return err
		}

		var project BusinessProject
		if err := lockForUpdate(tx).Select("id", "budget_quota").First(&project, snapshot.ProjectId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: project no longer exists", ErrBusinessProjectTokenBinding)
			}
			return err
		}
		var reservation BusinessProjectBudgetReservation
		if err := lockForUpdate(tx).
			Where("id = ? AND token_id = ? AND user_id = ? AND request_id = ?", snapshot.Id, tokenID, userID, requestID).
			First(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if reservation.Status != projectBudgetReservationStatusReserved || targetQuota <= reservation.ReservedQuota {
			return nil
		}
		if project.BudgetQuota == 0 {
			return nil
		}

		additionalQuota := targetQuota - reservation.ReservedQuota
		used, reserved, reconciliationPending, err := businessProjectBudgetUsage(tx, project.Id)
		if err != nil {
			return err
		}
		budget := int64(project.BudgetQuota)
		if reconciliationPending || used < 0 || reserved < 0 || used >= budget || reserved > budget-used || int64(additionalQuota) > budget-used-reserved {
			return ErrBusinessProjectBudgetExceeded
		}
		if err := tx.Model(&BusinessProjectBudgetReservation{}).
			Where("id = ? AND status = ?", reservation.Id, projectBudgetReservationStatusReserved).
			Update("reserved_quota", gorm.Expr("reserved_quota + ?", additionalQuota)).Error; err != nil {
			return err
		}
		addedQuota = additionalQuota
		return nil
	})
	if err != nil {
		return 0, err
	}
	return addedQuota, nil
}

// RollbackBusinessProjectBudgetReservationExpansion removes only the most
// recent additional hold when the subsequent existing billing reserve fails.
// It intentionally permits a now-disabled project so a disable operation can
// never strand a failed request's budget reservation.
func RollbackBusinessProjectBudgetReservationExpansion(tokenID int, userID int, requestID string, additionalQuota int) error {
	requestID = strings.TrimSpace(requestID)
	if additionalQuota <= 0 || tokenID <= 0 || userID <= 0 || requestID == "" || len(requestID) > 64 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		// Read the project ID without a row lock first, then take the project
		// lock before the reservation lock. Reserve/expand/settle all use the
		// same project -> reservation order, avoiding a lock-order inversion on
		// MySQL/PostgreSQL.
		var snapshot BusinessProjectBudgetReservation
		if err := tx.
			Where("token_id = ? AND user_id = ? AND request_id = ?", tokenID, userID, requestID).
			First(&snapshot).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var project BusinessProject
		if err := lockForUpdate(tx).Select("id").First(&project, snapshot.ProjectId).Error; err != nil {
			return err
		}
		var reservation BusinessProjectBudgetReservation
		if err := lockForUpdate(tx).Where("id = ?", snapshot.Id).First(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if reservation.Status != projectBudgetReservationStatusReserved || reservation.ReservedQuota < additionalQuota {
			return fmt.Errorf("%w: reservation expansion cannot be rolled back", ErrBusinessProjectBudgetExceeded)
		}
		return tx.Model(&BusinessProjectBudgetReservation{}).
			Where("id = ? AND status = ?", reservation.Id, projectBudgetReservationStatusReserved).
			Update("reserved_quota", gorm.Expr("reserved_quota - ?", additionalQuota)).Error
	})
}

// ReleaseBusinessProjectBudgetReservation removes a pending hold after the
// existing billing session refunds a failed request. It is intentionally
// idempotent because relay error handling may execute more than once.
func ReleaseBusinessProjectBudgetReservation(tokenID int, userID int, requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if tokenID <= 0 || userID <= 0 || requestID == "" || len(requestID) > 64 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		// Keep the same project -> reservation lock order as reservation
		// creation, expansion, and settlement. The initial unlocked lookup is
		// only used to identify the project; the row is reloaded under lock
		// before its status changes.
		var snapshot BusinessProjectBudgetReservation
		if err := tx.
			Where("token_id = ? AND user_id = ? AND request_id = ?", tokenID, userID, requestID).
			First(&snapshot).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var project BusinessProject
		if err := lockForUpdate(tx).Select("id").First(&project, snapshot.ProjectId).Error; err != nil {
			return err
		}
		var reservation BusinessProjectBudgetReservation
		if err := lockForUpdate(tx).Where("id = ?", snapshot.Id).First(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		return tx.Model(&BusinessProjectBudgetReservation{}).
			Where("id = ? AND status = ?", reservation.Id, projectBudgetReservationStatusReserved).
			Update("status", projectBudgetReservationStatusReleased).Error
	})
}

// ReconcileBusinessProjectBudgetReservation clears one exceptional terminal
// state after a platform/finance reviewer has reconciled the recorded actual
// usage with the underlying wallet/subscription and operational evidence. The
// caller must enforce the platform-manager or finance-approver permission
// before invoking this model operation. A non-empty reason and immutable
// audit event make the unblock decision reviewable.
func ReconcileBusinessProjectBudgetReservation(reservationID int, reason string, actor BusinessActor) error {
	reason = strings.TrimSpace(reason)
	if reservationID <= 0 || !actor.valid() || reason == "" || len(reason) > 255 {
		return errors.New("reservation, reviewer, and reconciliation reason are required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var reservation BusinessProjectBudgetReservation
		if err := lockForUpdate(tx).First(&reservation, reservationID).Error; err != nil {
			return err
		}
		if reservation.Status != projectBudgetReservationStatusOverBudgetPendingReconcile {
			return errors.New("project budget reservation is not pending reconciliation")
		}
		before := reservation
		if err := tx.Model(&BusinessProjectBudgetReservation{}).
			Where("id = ? AND status = ?", reservation.Id, projectBudgetReservationStatusOverBudgetPendingReconcile).
			Update("status", projectBudgetReservationStatusReconciled).Error; err != nil {
			return err
		}
		reservation.Status = projectBudgetReservationStatusReconciled
		return createBusinessAuditEvent(
			tx,
			actor,
			"project_budget.reconciliation.resolve",
			"project_budget_reservation",
			reservation.Id,
			reason,
			before,
			reservation,
		)
	})
}

// settleBusinessProjectBudgetReservationInTx turns a pending hold into a
// settled one in the same transaction that inserts BusinessConsumption. The
// settled hold is deliberately excluded from the budget SUM because the
// consumption projection now accounts for the final charge exactly once.
func settleBusinessProjectBudgetReservationInTx(tx *gorm.DB, projectID int, tokenID int, requestID string) error {
	if projectID <= 0 || tokenID <= 0 || strings.TrimSpace(requestID) == "" {
		return nil
	}
	return tx.Model(&BusinessProjectBudgetReservation{}).
		Where("project_id = ? AND token_id = ? AND request_id = ? AND status = ?", projectID, tokenID, requestID, projectBudgetReservationStatusReserved).
		Update("status", projectBudgetReservationStatusSettled).Error
}

func businessProjectBudgetUsage(db *gorm.DB, projectID int) (int64, int64, bool, error) {
	var used int64
	if err := db.Model(&BusinessConsumption{}).
		Where("project_id = ?", projectID).
		Select("COALESCE(SUM(quota), 0)").
		Scan(&used).Error; err != nil {
		return 0, 0, false, err
	}
	var reserved int64
	if err := db.Model(&BusinessProjectBudgetReservation{}).
		Where("project_id = ? AND status = ?", projectID, projectBudgetReservationStatusReserved).
		Select("COALESCE(SUM(reserved_quota), 0)").
		Scan(&reserved).Error; err != nil {
		return 0, 0, false, err
	}
	var reconciliationCount int64
	if err := db.Model(&BusinessProjectBudgetReservation{}).
		Where("project_id = ? AND status = ?", projectID, projectBudgetReservationStatusOverBudgetPendingReconcile).
		Count(&reconciliationCount).Error; err != nil {
		return 0, 0, false, err
	}
	return used, reserved, reconciliationCount > 0, nil
}
