package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	BusinessProjectStatusDisabled = 0
	BusinessProjectStatusEnabled  = 1

	ManualAdjustmentStatusPending   = "pending"
	ManualAdjustmentStatusEscalated = "pending_escalation"
	ManualAdjustmentStatusApproved  = "approved"
	ManualAdjustmentStatusRejected  = "rejected"

	LedgerEntryManualCredit      = "manual_credit"
	LedgerEntryManualDebit       = "manual_debit"
	LedgerEntryCompensation      = "compensation"
	LedgerEntryFreeze            = "freeze"
	LedgerEntryUnfreeze          = "unfreeze"
	LedgerEntryOpening           = "opening_balance"
	LedgerEntryConsumption       = "consumption"
	LedgerEntryConsumptionRefund = "consumption_refund"
)

var (
	ErrDuplicateManualAdjustment = errors.New("duplicate external reference and amount")
	ErrLedgerAlreadyReversed     = errors.New("a reversal request already exists for this ledger entry")
	ErrBusinessAuditImmutable    = errors.New("business audit events are append-only")
	ErrCompanyOwnerAlreadyBound  = errors.New("a customer balance account can belong to only one company")
)

// Company binds an enterprise profile to an existing account. Existing personal
// accounts simply have no company and remain fully compatible.
type Company struct {
	Id                    int    `json:"id"`
	Name                  string `json:"name" gorm:"size:128;uniqueIndex"`
	CreditCode            string `json:"credit_code" gorm:"size:64;index"`
	ContactName           string `json:"contact_name" gorm:"size:64"`
	ContactEmail          string `json:"contact_email" gorm:"size:128"`
	ContactPhone          string `json:"contact_phone" gorm:"size:32"`
	Contacts              string `json:"contacts" gorm:"type:text"`
	InvoiceRemark         string `json:"invoice_remark" gorm:"type:text"`
	OwnerUserId           int    `json:"owner_user_id" gorm:"index"`
	RateLimitRPM          int    `json:"rate_limit_rpm"`               // 企业级每分钟请求数（0=不限）
	RateLimitTPM          int64  `json:"rate_limit_tpm" gorm:"bigint"` // 企业级每分钟 token 预估（0=不限）
	MaxConcurrentRequests int    `json:"max_concurrent_requests"`      // 企业级并发上限（0=不限）
	CreatedAt             int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// BusinessProject defines the commercial and routing boundary for tokens.
// ModelLimits and ChannelLimits are JSON configuration strings so the schema is
// portable across SQLite, MySQL, and PostgreSQL.
type BusinessProject struct {
	Id                    int    `json:"id"`
	CompanyId             int    `json:"company_id" gorm:"index"`
	Name                  string `json:"name" gorm:"size:128"`
	OwnerUserId           int    `json:"owner_user_id" gorm:"index"`
	BudgetQuota           int    `json:"budget_quota"`
	LowBalanceQuota       int    `json:"low_balance_quota"`
	ModelLimits           string `json:"model_limits" gorm:"type:text"`
	ChannelLimits         string `json:"channel_limits" gorm:"type:text"`
	Status                int    `json:"status"`
	RateLimitRPM          int    `json:"rate_limit_rpm"`               // 项目级每分钟请求数（0=不限）
	RateLimitTPM          int64  `json:"rate_limit_tpm" gorm:"bigint"` // 项目级每分钟 token 预估（0=不限）
	MaxConcurrentRequests int    `json:"max_concurrent_requests"`      // 项目级并发上限（0=不限）
	CreatedAt             int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// CustomerAssignment keeps sales ownership history. Only the active row is
// used for authorization; previous rows are retained for auditability.
type CustomerAssignment struct {
	Id             int    `json:"id"`
	CompanyId      int    `json:"company_id" gorm:"index"`
	CustomerUserId int    `json:"customer_user_id" gorm:"index"`
	SalesUserId    int    `json:"sales_user_id" gorm:"index"`
	AssignedBy     int    `json:"assigned_by"`
	Active         bool   `json:"active" gorm:"index"`
	Reason         string `json:"reason" gorm:"size:255"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// SalesAccountProfile stores commercial profile data separately from the base
// user record so legacy users remain compatible and credentials stay outside
// the sales management surface.
type SalesAccountProfile struct {
	Id         int    `json:"id"`
	UserId     int    `json:"user_id" gorm:"uniqueIndex"`
	Department string `json:"department" gorm:"size:128;index"`
	Region     string `json:"region" gorm:"size:128;index"`
	Note       string `json:"note" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// PlatformReadOnlyUser deliberately excludes contact details, credentials,
// invite codes and configuration fields from the operations-reader surface.
type PlatformReadOnlyUser struct {
	Id           int    `json:"id"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Quota        int    `json:"quota"`
	UsedQuota    int    `json:"used_quota"`
	RequestCount int    `json:"request_count"`
	Status       int    `json:"status"`
	CreatedAt    int64  `json:"created_at"`
}

type PlatformReadOnlyLedgerEntry struct {
	Id        int    `json:"id"`
	CompanyId int    `json:"company_id"`
	UserId    int    `json:"user_id"`
	Amount    int    `json:"amount"`
	EntryType string `json:"entry_type"`
	CreatedAt int64  `json:"created_at"`
}

type PlatformReadOnlyAdjustment struct {
	Id        int    `json:"id"`
	CompanyId int    `json:"company_id"`
	UserId    int    `json:"user_id"`
	Amount    int    `json:"amount"`
	EntryType string `json:"entry_type"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

type PlatformReadOnlyFinanceOverview struct {
	AvailableQuota          int64 `json:"available_quota"`
	FrozenQuota             int64 `json:"frozen_quota"`
	PendingAdjustmentCount  int64 `json:"pending_adjustment_count"`
	ApprovedAdjustmentCount int64 `json:"approved_adjustment_count"`
}

// BalanceLedger is append-only by design. Corrections must be created as a new
// reverse adjustment request. The GORM hooks below prevent application code
// from editing or deleting an existing ledger row.
type BalanceLedger struct {
	Id                       int    `json:"id"`
	CompanyId                int    `json:"company_id" gorm:"index"`
	UserId                   int    `json:"user_id" gorm:"index"`
	ProjectId                int    `json:"project_id" gorm:"index"`
	TokenId                  int    `json:"token_id" gorm:"index"`
	RequestId                string `json:"request_id" gorm:"size:64;index"`
	Amount                   int    `json:"amount"`
	UsageQuota               int    `json:"usage_quota"`
	FundingSource            string `json:"funding_source" gorm:"size:32;index"`
	BalanceSnapshotAvailable bool   `json:"balance_snapshot_available"`
	BalanceBefore            int    `json:"balance_before"`
	BalanceAfter             int    `json:"balance_after"`
	FrozenBefore             int    `json:"frozen_before"`
	FrozenAfter              int    `json:"frozen_after"`
	EntryType                string `json:"entry_type" gorm:"size:32;index"`
	ReferenceType            string `json:"reference_type" gorm:"size:32;uniqueIndex:idx_balance_ledger_reference,priority:1"`
	ReferenceId              int    `json:"reference_id" gorm:"uniqueIndex:idx_balance_ledger_reference,priority:2"`
	ExternalReference        string `json:"external_reference" gorm:"size:128;index"`
	Reason                   string `json:"reason" gorm:"size:255"`
	Note                     string `json:"note" gorm:"type:text"`
	InvoiceNumber            string `json:"invoice_number" gorm:"size:128;index"`
	ExternalFinanceReference string `json:"external_finance_reference" gorm:"size:128;index"`
	ReconciliationConclusion string `json:"reconciliation_conclusion" gorm:"type:text"`
	CreatedBy                int    `json:"created_by"`
	ApprovedBy               int    `json:"approved_by"`
	ReversesLedgerId         int    `json:"reverses_ledger_id" gorm:"index"`
	CreatedAt                int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (BalanceLedger) BeforeUpdate(*gorm.DB) error {
	return errors.New("balance ledger entries are append-only")
}

func (BalanceLedger) BeforeDelete(*gorm.DB) error {
	return errors.New("balance ledger entries are append-only")
}

// ManualCreditRequest is retained as the API/model name for compatibility,
// while EntryType lets it represent every manually reviewed balance adjustment.
// Amount is always a positive quota value; EntryType determines the signed
// ledger delta.
type ManualCreditRequest struct {
	Id                       int    `json:"id"`
	CompanyId                int    `json:"company_id" gorm:"index"`
	UserId                   int    `json:"user_id" gorm:"index"`
	ProjectId                int    `json:"project_id" gorm:"index"`
	EntryType                string `json:"entry_type" gorm:"size:32;index"`
	Amount                   int    `json:"amount" gorm:"index:idx_manual_credit_external_amount,priority:2"`
	ExternalReference        string `json:"external_reference" gorm:"size:128;index:idx_manual_credit_external_amount,priority:1"`
	Reason                   string `json:"reason" gorm:"size:255"`
	Note                     string `json:"note" gorm:"type:text"`
	InvoiceNumber            string `json:"invoice_number" gorm:"size:128;index"`
	ExternalFinanceReference string `json:"external_finance_reference" gorm:"size:128;index"`
	ReconciliationConclusion string `json:"reconciliation_conclusion" gorm:"type:text"`
	Status                   string `json:"status" gorm:"size:24;index"`
	CreatedBy                int    `json:"created_by"`
	ApprovedBy               int    `json:"approved_by"`
	RejectedBy               int    `json:"rejected_by"`
	RejectionReason          string `json:"rejection_reason" gorm:"size:255"`
	ReversesLedgerId         int    `json:"reverses_ledger_id" gorm:"index"`
	// ReversalReference is nil for ordinary requests and unique for a reversal.
	// A nullable unique key works consistently in SQLite, MySQL, and PostgreSQL,
	// unlike using 0 as a sentinel in a unique integer column.
	ReversalReference    *string `json:"-" gorm:"size:64;uniqueIndex:idx_manual_credit_reversal_reference"`
	DuplicateOfRequestId int     `json:"duplicate_of_request_id" gorm:"index"`
	EscalationReason     string  `json:"escalation_reason" gorm:"size:255"`
	CreatedAt            int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

// BusinessAuditEvent is intentionally in the primary database so it remains
// transactionally coupled to finance writes even when LOG_DB is ClickHouse.
type BusinessAuditEvent struct {
	Id           int    `json:"id"`
	ActorUserId  int    `json:"actor_user_id" gorm:"index"`
	ActorName    string `json:"actor_name" gorm:"size:64"`
	RoleSnapshot string `json:"role_snapshot" gorm:"size:255"`
	Action       string `json:"action" gorm:"size:64;index"`
	Resource     string `json:"resource" gorm:"size:64;index"`
	ResourceId   int    `json:"resource_id" gorm:"index"`
	Reason       string `json:"reason" gorm:"size:255"`
	BeforeValue  string `json:"before_value" gorm:"type:text"`
	AfterValue   string `json:"after_value" gorm:"type:text"`
	Ip           string `json:"ip" gorm:"size:64"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (BusinessAuditEvent) BeforeUpdate(*gorm.DB) error {
	return ErrBusinessAuditImmutable
}

func (BusinessAuditEvent) BeforeDelete(*gorm.DB) error {
	return ErrBusinessAuditImmutable
}

type BusinessAnnouncement struct {
	Id        int    `json:"id"`
	Title     string `json:"title" gorm:"size:128"`
	Content   string `json:"content" gorm:"type:text"`
	Status    string `json:"status" gorm:"size:24;index"`
	StartsAt  int64  `json:"starts_at" gorm:"index"`
	EndsAt    int64  `json:"ends_at" gorm:"index"`
	CreatedBy int    `json:"created_by"`
	UpdatedBy int    `json:"updated_by"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

// BusinessFollowUp is a sales-owned, append-only-in-history work item for an
// assigned enterprise customer. Closing or rescheduling updates the current
// item state; completed items remain retained for handover and audit.
type BusinessFollowUp struct {
	Id             int    `json:"id"`
	CompanyId      int    `json:"company_id" gorm:"index"`
	CustomerUserId int    `json:"customer_user_id" gorm:"index"`
	SalesUserId    int    `json:"sales_user_id" gorm:"index"`
	Title          string `json:"title" gorm:"size:128"`
	Note           string `json:"note" gorm:"type:text"`
	Status         string `json:"status" gorm:"size:24;index"`
	DueAt          int64  `json:"due_at" gorm:"index"`
	CreatedBy      int    `json:"created_by"`
	UpdatedBy      int    `json:"updated_by"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

type BusinessActor struct {
	UserId       int
	Username     string
	RoleSnapshot string
	IP           string
}

// businessTokenAuditSnapshot deliberately contains only the non-secret token
// metadata required to explain a project assignment. Never serialize Token
// directly into a business audit event: Token.Key, IP restrictions and other
// credentials must not become visible through the operations audit feed.
type businessTokenAuditSnapshot struct {
	Id        int    `json:"id"`
	UserId    int    `json:"user_id"`
	ProjectId int    `json:"project_id"`
	Name      string `json:"name"`
	Status    int    `json:"status"`
}

func tokenAuditSnapshot(token Token) businessTokenAuditSnapshot {
	return businessTokenAuditSnapshot{
		Id:        token.Id,
		UserId:    token.UserId,
		ProjectId: token.ProjectId,
		Name:      token.Name,
		Status:    token.Status,
	}
}

func (actor BusinessActor) valid() bool {
	return actor.UserId > 0
}

func validManualAdjustmentEntryType(entryType string) bool {
	switch entryType {
	case LedgerEntryManualCredit, LedgerEntryManualDebit, LedgerEntryCompensation, LedgerEntryFreeze, LedgerEntryUnfreeze:
		return true
	default:
		return false
	}
}

func manualAdjustmentDelta(entryType string, amount int) (int, error) {
	if amount <= 0 || amount > common.MaxQuota {
		return 0, errors.New("amount must be between 1 and the supported quota limit")
	}
	switch entryType {
	case LedgerEntryManualCredit, LedgerEntryCompensation, LedgerEntryUnfreeze:
		return amount, nil
	case LedgerEntryManualDebit, LedgerEntryFreeze:
		return -amount, nil
	default:
		return 0, errors.New("unsupported adjustment entry type")
	}
}

func manualAdjustmentChanges(entryType string, amount, availableQuota, frozenQuota int) (int, int, error) {
	availableDelta, err := manualAdjustmentDelta(entryType, amount)
	if err != nil {
		return 0, 0, err
	}
	frozenDelta := 0
	switch entryType {
	case LedgerEntryFreeze:
		frozenDelta = amount
	case LedgerEntryUnfreeze:
		frozenDelta = -amount
	}
	newAvailableQuota := int64(availableQuota) + int64(availableDelta)
	newFrozenQuota := int64(frozenQuota) + int64(frozenDelta)
	if newAvailableQuota < 0 {
		return 0, 0, errors.New("adjustment would make customer quota negative")
	}
	if newFrozenQuota < 0 {
		return 0, 0, errors.New("adjustment would unfreeze more quota than is frozen")
	}
	if newAvailableQuota > int64(common.MaxQuota) || newFrozenQuota > int64(common.MaxQuota) {
		return 0, 0, errors.New("quota exceeds supported limit")
	}
	return availableDelta, frozenDelta, nil
}

func reversalEntryType(entryType string) (string, error) {
	switch entryType {
	case LedgerEntryManualCredit, LedgerEntryCompensation:
		return LedgerEntryManualDebit, nil
	case LedgerEntryManualDebit:
		return LedgerEntryManualCredit, nil
	case LedgerEntryFreeze:
		return LedgerEntryUnfreeze, nil
	case LedgerEntryUnfreeze:
		return LedgerEntryFreeze, nil
	default:
		return "", errors.New("unsupported ledger entry type for reversal")
	}
}

// appendBusinessConsumptionLedgerInTx writes the immutable usage association
// for a project consumption. BillingSession is the authoritative financial
// settlement source: it may use either a wallet or a subscription. Therefore
// this row deliberately records no synthetic wallet delta or before/after
// reconstruction. UsageQuota is the signed project usage that can be reconciled
// through the request/token/project identifiers without falsely claiming that a
// subscription charge changed the user's wallet balance.
func appendBusinessConsumptionLedgerInTx(tx *gorm.DB, consumption *BusinessConsumption) error {
	if consumption == nil || consumption.Id <= 0 || consumption.Quota == 0 {
		return nil
	}

	entryType := LedgerEntryConsumption
	reason := "gateway consumption association"
	if consumption.Quota < 0 {
		entryType = LedgerEntryConsumptionRefund
		reason = "gateway consumption refund association"
	}

	return tx.Create(&BalanceLedger{
		CompanyId:                consumption.CompanyId,
		UserId:                   consumption.UserId,
		ProjectId:                consumption.ProjectId,
		TokenId:                  consumption.TokenId,
		RequestId:                consumption.RequestId,
		Amount:                   0,
		UsageQuota:               consumption.Quota,
		FundingSource:            "gateway_settlement",
		BalanceSnapshotAvailable: false,
		EntryType:                entryType,
		ReferenceType:            "business_consumption",
		ReferenceId:              consumption.Id,
		Reason:                   reason,
		Note:                     "usage association; balance settlement remains in the existing billing session",
	}).Error
}

func isBusinessUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "duplicate entry")
}

func validateBusinessProject(project *BusinessProject) error {
	project.Name = strings.TrimSpace(project.Name)
	if project.CompanyId <= 0 || project.Name == "" {
		return errors.New("company and project name are required")
	}
	if project.BudgetQuota < 0 || project.LowBalanceQuota < 0 || project.BudgetQuota > common.MaxQuota || project.LowBalanceQuota > common.MaxQuota {
		return errors.New("project quota settings are out of range")
	}
	if project.Status != BusinessProjectStatusDisabled && project.Status != BusinessProjectStatusEnabled {
		return errors.New("invalid project status")
	}
	for _, config := range []string{project.ModelLimits, project.ChannelLimits} {
		if strings.TrimSpace(config) == "" {
			continue
		}
		var value interface{}
		if err := common.UnmarshalJsonStr(config, &value); err != nil {
			return errors.New("project limits must be valid JSON")
		}
	}
	return nil
}

func CreateCompany(company *Company, actor BusinessActor) error {
	company.Name = strings.TrimSpace(company.Name)
	if company.Name == "" || company.OwnerUserId <= 0 {
		return errors.New("company name and owner are required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return createCompanyInTx(tx, company, actor)
	})
}

// CreateCompanyWithSalesAssignment keeps invitation-based ownership binding in
// the same transaction as the enterprise profile, so a new customer never has
// a company without its promised sales owner.
func CreateCompanyWithSalesAssignment(company *Company, salesUserID int, reason string, actor BusinessActor) error {
	company.Name = strings.TrimSpace(company.Name)
	reason = strings.TrimSpace(reason)
	if company.Name == "" || company.OwnerUserId <= 0 || salesUserID <= 0 || reason == "" {
		return errors.New("company, sales user, and assignment reason are required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := createCompanyInTx(tx, company, actor); err != nil {
			return err
		}
		_, err := assignCustomerInTx(tx, company.Id, salesUserID, reason, actor)
		return err
	})
}

func createCompanyInTx(tx *gorm.DB, company *Company, actor BusinessActor) error {
	var owner User
	// A company currently reuses its owner's wallet. Locking that account and
	// allowing only one company prevents a shared wallet from being exposed to
	// different sales owners as though it were a company-scoped balance.
	if err := lockForUpdate(tx).First(&owner, company.OwnerUserId).Error; err != nil {
		return err
	}
	var existingCount int64
	if err := tx.Model(&Company{}).Where("owner_user_id = ?", company.OwnerUserId).Count(&existingCount).Error; err != nil {
		return err
	}
	if existingCount > 0 {
		return ErrCompanyOwnerAlreadyBound
	}
	if err := tx.Create(company).Error; err != nil {
		return err
	}
	return createBusinessAuditEvent(tx, actor, "company.create", "company", company.Id, "", nil, company)
}

func UpdateCompany(company *Company, actor BusinessActor) error {
	company.Name = strings.TrimSpace(company.Name)
	if company.Id <= 0 || company.Name == "" {
		return errors.New("company id and name are required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var previous Company
		if err := lockForUpdate(tx).First(&previous, company.Id).Error; err != nil {
			return err
		}
		if company.OwnerUserId != 0 && company.OwnerUserId != previous.OwnerUserId {
			return errors.New("company owner cannot be changed by a profile update")
		}
		company.OwnerUserId = previous.OwnerUserId
		if err := tx.Model(&Company{}).Where("id = ?", company.Id).Updates(map[string]interface{}{
			"name": company.Name, "credit_code": company.CreditCode, "contact_name": company.ContactName,
			"contact_email": company.ContactEmail, "contact_phone": company.ContactPhone, "contacts": company.Contacts,
			"invoice_remark": company.InvoiceRemark,
			"rate_limit_rpm": company.RateLimitRPM, "rate_limit_tpm": company.RateLimitTPM,
			"max_concurrent_requests": company.MaxConcurrentRequests,
		}).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "company.update", "company", company.Id, "", previous, company)
	})
}

func CreateBusinessProject(project *BusinessProject, actor BusinessActor) error {
	if project.Status == 0 {
		project.Status = BusinessProjectStatusEnabled
	}
	if err := validateBusinessProject(project); err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var company Company
		if err := tx.First(&company, project.CompanyId).Error; err != nil {
			return err
		}
		if project.OwnerUserId == 0 {
			project.OwnerUserId = company.OwnerUserId
		}
		if project.OwnerUserId != company.OwnerUserId {
			return errors.New("project owner must be the company owner")
		}
		if err := tx.Create(project).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "project.create", "project", project.Id, "", nil, project)
	})
}

func UpdateBusinessProject(project *BusinessProject, reason string, actor BusinessActor) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("project update reason is required")
	}
	if err := validateBusinessProject(project); err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var previous BusinessProject
		if err := lockForUpdate(tx).First(&previous, project.Id).Error; err != nil {
			return err
		}
		if previous.CompanyId != project.CompanyId || previous.OwnerUserId != project.OwnerUserId {
			return errors.New("project company and owner cannot be changed")
		}
		if err := tx.Model(&BusinessProject{}).Where("id = ?", project.Id).Updates(map[string]interface{}{
			"name": project.Name, "budget_quota": project.BudgetQuota, "low_balance_quota": project.LowBalanceQuota,
			"model_limits": project.ModelLimits, "channel_limits": project.ChannelLimits, "status": project.Status,
			"rate_limit_rpm": project.RateLimitRPM, "rate_limit_tpm": project.RateLimitTPM,
			"max_concurrent_requests": project.MaxConcurrentRequests,
		}).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "project.update", "project", project.Id, reason, previous, project)
	})
}

func CreateManualCreditRequest(req *ManualCreditRequest, actor BusinessActor) error {
	req.ExternalReference = strings.TrimSpace(req.ExternalReference)
	req.Reason = strings.TrimSpace(req.Reason)
	req.Note = strings.TrimSpace(req.Note)
	req.InvoiceNumber = strings.TrimSpace(req.InvoiceNumber)
	req.ExternalFinanceReference = strings.TrimSpace(req.ExternalFinanceReference)
	req.ReconciliationConclusion = strings.TrimSpace(req.ReconciliationConclusion)
	req.EntryType = strings.TrimSpace(req.EntryType)
	if req.EntryType == "" {
		req.EntryType = LedgerEntryManualCredit
	}
	if req.UserId <= 0 || req.ExternalReference == "" || req.Reason == "" || req.Note == "" {
		return errors.New("customer, external reference, reason, and note are required")
	}
	if _, err := manualAdjustmentDelta(req.EntryType, req.Amount); err != nil {
		return err
	}
	if !actor.valid() || req.CreatedBy != actor.UserId {
		return errors.New("invalid adjustment creator")
	}
	// Reversal linkage is created only by CreateLedgerReversalRequest after the
	// original ledger entry has been validated; clients cannot forge it.
	req.ReversesLedgerId = 0
	req.ReversalReference = nil
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		// Lock the customer wallet row so concurrent submissions with the same
		// external reference serialize here; otherwise the duplicate check below
		// is a TOCTOU race and the same payment can be entered twice (P0-24).
		if err := lockForUpdate(tx).First(&user, req.UserId).Error; err != nil {
			return err
		}
		if req.ProjectId > 0 {
			var project BusinessProject
			if err := tx.First(&project, req.ProjectId).Error; err != nil {
				return err
			}
			if project.OwnerUserId != req.UserId {
				return errors.New("project does not belong to the customer")
			}
			if req.CompanyId == 0 {
				req.CompanyId = project.CompanyId
			}
			if req.CompanyId != project.CompanyId {
				return errors.New("project and company do not match")
			}
		}
		if req.CompanyId > 0 {
			var company Company
			if err := tx.First(&company, req.CompanyId).Error; err != nil {
				return err
			}
			if company.OwnerUserId != req.UserId {
				return errors.New("company does not belong to the customer")
			}
		}
		var duplicate ManualCreditRequest
		if err := tx.Where("user_id = ? AND external_reference = ? AND amount = ?", req.UserId, req.ExternalReference, req.Amount).First(&duplicate).Error; err == nil {
			req.Status = ManualAdjustmentStatusEscalated
			req.DuplicateOfRequestId = duplicate.Id
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else {
			req.Status = ManualAdjustmentStatusPending
		}
		if err := tx.Create(req).Error; err != nil {
			return err
		}
		action := "finance.adjustment.create"
		if req.Status == ManualAdjustmentStatusEscalated {
			action = "finance.adjustment.duplicate_escalation"
		}
		return createBusinessAuditEvent(tx, actor, action, "manual_credit_request", req.Id, req.Reason, nil, req)
	})
}

func ApproveManualCreditRequest(id int, actor BusinessActor) (*BalanceLedger, error) {
	return approveManualCreditRequest(id, ManualAdjustmentStatusPending, "", actor)
}

func ApproveEscalatedManualCreditRequest(id int, escalationReason string, actor BusinessActor) (*BalanceLedger, error) {
	escalationReason = strings.TrimSpace(escalationReason)
	if escalationReason == "" {
		return nil, errors.New("escalation approval reason is required")
	}
	return approveManualCreditRequest(id, ManualAdjustmentStatusEscalated, escalationReason, actor)
}

func approveManualCreditRequest(id int, expectedStatus string, escalationReason string, actor BusinessActor) (*BalanceLedger, error) {
	if !actor.valid() {
		return nil, errors.New("invalid adjustment approver")
	}
	var ledger BalanceLedger
	err := DB.Transaction(func(tx *gorm.DB) error {
		var req ManualCreditRequest
		if err := lockForUpdate(tx).First(&req, id).Error; err != nil {
			return err
		}
		if req.Status != expectedStatus {
			return errors.New("manual adjustment is not pending")
		}
		if req.CreatedBy == actor.UserId {
			return errors.New("maker and checker must be different users")
		}
		var user User
		if err := lockForUpdate(tx).First(&user, req.UserId).Error; err != nil {
			return err
		}
		delta, frozenDelta, err := manualAdjustmentChanges(req.EntryType, req.Amount, user.Quota, user.FrozenQuota)
		if err != nil {
			return err
		}
		newQuota := int64(user.Quota) + int64(delta)
		newFrozenQuota := int64(user.FrozenQuota) + int64(frozenDelta)
		updates := map[string]interface{}{"status": ManualAdjustmentStatusApproved, "approved_by": actor.UserId, "updated_at": time.Now().Unix()}
		if escalationReason != "" {
			updates["escalation_reason"] = escalationReason
		}
		update := tx.Model(&ManualCreditRequest{}).Where("id = ? AND status = ?", req.Id, expectedStatus).Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("manual adjustment has already been reviewed")
		}
		if err := tx.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
			"quota":        int(newQuota),
			"frozen_quota": int(newFrozenQuota),
		}).Error; err != nil {
			return err
		}
		ledger = BalanceLedger{
			CompanyId:                req.CompanyId,
			UserId:                   req.UserId,
			ProjectId:                req.ProjectId,
			Amount:                   delta,
			BalanceSnapshotAvailable: true,
			BalanceBefore:            user.Quota,
			BalanceAfter:             int(newQuota),
			FrozenBefore:             user.FrozenQuota,
			FrozenAfter:              int(newFrozenQuota),
			EntryType:                req.EntryType,
			ReferenceType:            "manual_credit_request",
			ReferenceId:              req.Id,
			ExternalReference:        req.ExternalReference,
			Reason:                   req.Reason,
			Note:                     req.Note,
			InvoiceNumber:            req.InvoiceNumber,
			ExternalFinanceReference: req.ExternalFinanceReference,
			ReconciliationConclusion: req.ReconciliationConclusion,
			CreatedBy:                req.CreatedBy,
			ApprovedBy:               actor.UserId,
			ReversesLedgerId:         req.ReversesLedgerId,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		auditAction := "finance.adjustment.approve"
		auditReason := req.Reason
		if escalationReason != "" {
			auditAction = "finance.adjustment.duplicate_escalation.approve"
			auditReason = escalationReason
		}
		return createBusinessAuditEvent(tx, actor, auditAction, "manual_credit_request", req.Id, auditReason, req, ledger)
	})
	if err != nil {
		return nil, err
	}
	if err := updateUserQuotaCache(ledger.UserId, ledger.BalanceAfter); err != nil {
		// The committed ledger and quota are authoritative. A cache refresh failure
		// must not make a successful approval appear failed to a caller.
		common.SysError("failed to refresh user quota cache after manual adjustment: " + err.Error())
	}
	return &ledger, nil
}

func RejectManualCreditRequest(id int, rejectionReason string, actor BusinessActor) (*ManualCreditRequest, error) {
	rejectionReason = strings.TrimSpace(rejectionReason)
	if !actor.valid() || rejectionReason == "" {
		return nil, errors.New("reviewer and rejection reason are required")
	}
	var request ManualCreditRequest
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&request, id).Error; err != nil {
			return err
		}
		previous := request
		if request.Status != ManualAdjustmentStatusPending {
			return errors.New("manual adjustment is not pending")
		}
		if request.CreatedBy == actor.UserId {
			return errors.New("maker and checker must be different users")
		}
		update := tx.Model(&ManualCreditRequest{}).Where("id = ? AND status = ?", request.Id, ManualAdjustmentStatusPending).
			Updates(map[string]interface{}{"status": ManualAdjustmentStatusRejected, "rejected_by": actor.UserId, "rejection_reason": rejectionReason, "updated_at": time.Now().Unix()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return errors.New("manual adjustment has already been reviewed")
		}
		request.Status = ManualAdjustmentStatusRejected
		request.RejectedBy = actor.UserId
		request.RejectionReason = rejectionReason
		return createBusinessAuditEvent(tx, actor, "finance.adjustment.reject", "manual_credit_request", request.Id, rejectionReason, previous, request)
	})
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func CreateLedgerReversalRequest(ledgerID int, reason string, actor BusinessActor) (*ManualCreditRequest, error) {
	reason = strings.TrimSpace(reason)
	if !actor.valid() || reason == "" {
		return nil, errors.New("creator and reversal reason are required")
	}
	var request ManualCreditRequest
	err := DB.Transaction(func(tx *gorm.DB) error {
		var ledger BalanceLedger
		if err := lockForUpdate(tx).First(&ledger, ledgerID).Error; err != nil {
			return err
		}
		if ledger.Amount == 0 {
			return errors.New("zero-value ledger entries cannot be reversed")
		}
		reversalReference := fmt.Sprintf("ledger:%d", ledger.Id)
		var existing ManualCreditRequest
		if err := tx.Where("reversal_reference = ?", reversalReference).First(&existing).Error; err == nil {
			return ErrLedgerAlreadyReversed
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		entryType, err := reversalEntryType(ledger.EntryType)
		if err != nil {
			return err
		}
		amount := absBusinessQuota(ledger.Amount)
		if _, err := manualAdjustmentDelta(entryType, amount); err != nil {
			return err
		}
		request = ManualCreditRequest{
			CompanyId:         ledger.CompanyId,
			UserId:            ledger.UserId,
			ProjectId:         ledger.ProjectId,
			EntryType:         entryType,
			Amount:            amount,
			ExternalReference: fmt.Sprintf("reversal:%d:%d", ledger.Id, time.Now().UnixNano()),
			Reason:            reason,
			Status:            ManualAdjustmentStatusPending,
			CreatedBy:         actor.UserId,
			ReversesLedgerId:  ledger.Id,
			ReversalReference: &reversalReference,
		}
		if err := tx.Create(&request).Error; err != nil {
			if isBusinessUniqueViolation(err) {
				return ErrLedgerAlreadyReversed
			}
			return err
		}
		return createBusinessAuditEvent(tx, actor, "ledger.reversal.create", "balance_ledger", ledger.Id, reason, ledger, request)
	})
	if err != nil {
		return nil, err
	}
	return &request, nil
}

// InitializeBusinessOpeningBalance creates a zero-delta, immutable opening
// snapshot for an existing enterprise account. It is intentionally explicit
// (never run from AutoMigrate) so a migration rehearsal can validate every
// opening balance before production cutover. The reference constraint makes
// retries idempotent for one company.
func InitializeBusinessOpeningBalance(companyID int, reason string, actor BusinessActor) (*BalanceLedger, error) {
	reason = strings.TrimSpace(reason)
	if companyID <= 0 || reason == "" || !actor.valid() {
		return nil, errors.New("company, opening balance reason, and actor are required")
	}
	var ledger BalanceLedger
	err := DB.Transaction(func(tx *gorm.DB) error {
		var company Company
		if err := lockForUpdate(tx).First(&company, companyID).Error; err != nil {
			return err
		}
		var existing BalanceLedger
		if err := tx.Where("reference_type = ? AND reference_id = ?", "opening_balance", companyID).First(&existing).Error; err == nil {
			ledger = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var user User
		if err := lockForUpdate(tx).First(&user, company.OwnerUserId).Error; err != nil {
			return err
		}
		ledger = BalanceLedger{
			CompanyId:                company.Id,
			UserId:                   user.Id,
			Amount:                   0,
			BalanceSnapshotAvailable: true,
			BalanceBefore:            user.Quota,
			BalanceAfter:             user.Quota,
			FrozenBefore:             user.FrozenQuota,
			FrozenAfter:              user.FrozenQuota,
			EntryType:                LedgerEntryOpening,
			ReferenceType:            "opening_balance",
			ReferenceId:              company.Id,
			Reason:                   reason,
			CreatedBy:                actor.UserId,
			ApprovedBy:               actor.UserId,
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "finance.opening_balance.create", "company", company.Id, reason, nil, ledger)
	})
	if err != nil {
		return nil, err
	}
	return &ledger, nil
}

func AssignCustomer(companyID, salesUserID int, reason string, actor BusinessActor) (*CustomerAssignment, error) {
	reason = strings.TrimSpace(reason)
	if companyID <= 0 || salesUserID <= 0 || !actor.valid() || reason == "" {
		return nil, errors.New("company, sales user, actor, and reason are required")
	}
	var assignment CustomerAssignment
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		assignment, err = assignCustomerInTx(tx, companyID, salesUserID, reason, actor)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &assignment, nil
}

// AssignCustomers atomically assigns a batch of enterprise customers to one
// sales supervisor. It rejects duplicate company IDs so each history entry has
// one unambiguous transfer reason.
func AssignCustomers(companyIDs []int, salesUserID int, reason string, actor BusinessActor) ([]CustomerAssignment, error) {
	reason = strings.TrimSpace(reason)
	if len(companyIDs) == 0 || len(companyIDs) > 100 || salesUserID <= 0 || !actor.valid() || reason == "" {
		return nil, errors.New("one to 100 companies, sales user, actor, and reason are required")
	}
	seen := make(map[int]struct{}, len(companyIDs))
	for _, companyID := range companyIDs {
		if companyID <= 0 {
			return nil, errors.New("company id must be positive")
		}
		if _, ok := seen[companyID]; ok {
			return nil, errors.New("company ids must not repeat")
		}
		seen[companyID] = struct{}{}
	}
	assignments := make([]CustomerAssignment, 0, len(companyIDs))
	err := DB.Transaction(func(tx *gorm.DB) error {
		for _, companyID := range companyIDs {
			assignment, err := assignCustomerInTx(tx, companyID, salesUserID, reason, actor)
			if err != nil {
				return err
			}
			assignments = append(assignments, assignment)
		}
		return nil
	})
	return assignments, err
}

func assignCustomerInTx(tx *gorm.DB, companyID, salesUserID int, reason string, actor BusinessActor) (CustomerAssignment, error) {
	var company Company
	if err := lockForUpdate(tx).First(&company, companyID).Error; err != nil {
		return CustomerAssignment{}, err
	}
	var salesUser User
	if err := tx.First(&salesUser, salesUserID).Error; err != nil {
		return CustomerAssignment{}, err
	}
	var previous []CustomerAssignment
	if err := lockForUpdate(tx).Where("company_id = ? AND active = ?", companyID, true).Find(&previous).Error; err != nil {
		return CustomerAssignment{}, err
	}
	if err := tx.Model(&CustomerAssignment{}).Where("company_id = ? AND active = ?", companyID, true).Update("active", false).Error; err != nil {
		return CustomerAssignment{}, err
	}
	assignment := CustomerAssignment{CompanyId: companyID, CustomerUserId: company.OwnerUserId, SalesUserId: salesUserID, AssignedBy: actor.UserId, Active: true, Reason: reason}
	if err := tx.Create(&assignment).Error; err != nil {
		return CustomerAssignment{}, err
	}
	if err := createBusinessAuditEvent(tx, actor, "customer.assignment.update", "company", companyID, reason, previous, assignment); err != nil {
		return CustomerAssignment{}, err
	}
	return assignment, nil
}

func CreateBusinessFollowUp(followUp *BusinessFollowUp, actor BusinessActor) error {
	followUp.Title = strings.TrimSpace(followUp.Title)
	followUp.Note = strings.TrimSpace(followUp.Note)
	if followUp.CompanyId <= 0 || followUp.CustomerUserId <= 0 || followUp.SalesUserId <= 0 || followUp.Title == "" || followUp.DueAt < 0 || !actor.valid() {
		return errors.New("company, customer, sales owner, title, and actor are required")
	}
	if followUp.Status == "" {
		followUp.Status = "open"
	}
	if followUp.Status != "open" && followUp.Status != "done" {
		return errors.New("follow-up status must be open or done")
	}
	followUp.CreatedBy = actor.UserId
	followUp.UpdatedBy = actor.UserId
	return DB.Transaction(func(tx *gorm.DB) error {
		var company Company
		if err := tx.First(&company, followUp.CompanyId).Error; err != nil {
			return err
		}
		if company.OwnerUserId != followUp.CustomerUserId {
			return errors.New("follow-up customer does not own the company")
		}
		if err := tx.Create(followUp).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "sales.follow_up.create", "business_follow_up", followUp.Id, followUp.Title, nil, followUp)
	})
}

func UpdateBusinessFollowUp(followUp *BusinessFollowUp, actor BusinessActor) error {
	followUp.Title = strings.TrimSpace(followUp.Title)
	followUp.Note = strings.TrimSpace(followUp.Note)
	if followUp.Id <= 0 || followUp.Title == "" || followUp.DueAt < 0 || !actor.valid() {
		return errors.New("follow-up id, title, and actor are required")
	}
	if followUp.Status != "open" && followUp.Status != "done" {
		return errors.New("follow-up status must be open or done")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var previous BusinessFollowUp
		if err := lockForUpdate(tx).First(&previous, followUp.Id).Error; err != nil {
			return err
		}
		if previous.CompanyId != followUp.CompanyId || previous.CustomerUserId != followUp.CustomerUserId || previous.SalesUserId != followUp.SalesUserId {
			return errors.New("follow-up customer ownership cannot be changed")
		}
		followUp.CreatedBy = previous.CreatedBy
		followUp.UpdatedBy = actor.UserId
		if err := tx.Model(&BusinessFollowUp{}).Where("id = ?", followUp.Id).Updates(map[string]interface{}{
			"title": followUp.Title, "note": followUp.Note, "status": followUp.Status,
			"due_at": followUp.DueAt, "updated_by": actor.UserId,
		}).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "sales.follow_up.update", "business_follow_up", followUp.Id, followUp.Title, previous, followUp)
	})
}

func GetBusinessFollowUp(id int) (*BusinessFollowUp, error) {
	if id <= 0 {
		return nil, errors.New("invalid follow-up id")
	}
	var followUp BusinessFollowUp
	if err := DB.First(&followUp, id).Error; err != nil {
		return nil, err
	}
	return &followUp, nil
}

func ListBusinessFollowUps(companyID int, startIdx, pageSize int) ([]*BusinessFollowUp, int64, error) {
	if companyID <= 0 {
		return nil, 0, errors.New("company is required")
	}
	query := DB.Model(&BusinessFollowUp{}).Where("company_id = ?", companyID).Order("status asc, due_at asc, id desc")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*BusinessFollowUp, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func SetBusinessProjectToken(projectID, tokenID int, reason string, actor BusinessActor) error {
	reason = strings.TrimSpace(reason)
	if projectID <= 0 || tokenID <= 0 || reason == "" || !actor.valid() {
		return errors.New("project, token, assignment reason, and actor are required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var project BusinessProject
		if err := lockForUpdate(tx).First(&project, projectID).Error; err != nil {
			return err
		}
		if project.Status != BusinessProjectStatusEnabled {
			return errors.New("project is disabled")
		}
		var token Token
		if err := lockForUpdate(tx).First(&token, tokenID).Error; err != nil {
			return err
		}
		if token.UserId != project.OwnerUserId {
			return errors.New("token does not belong to the project owner")
		}
		// Project assignment is immutable once made. This retains the link from
		// historical logs to their project without relying on a cross-database
		// join when LOG_DB is configured independently.
		if token.ProjectId != 0 && token.ProjectId != projectID {
			return errors.New("a token cannot be reassigned to another project")
		}
		if err := tx.Model(&Token{}).Where("id = ?", tokenID).Update("project_id", projectID).Error; err != nil {
			return err
		}
		before := tokenAuditSnapshot(token)
		after := before
		after.ProjectId = projectID
		return createBusinessAuditEvent(tx, actor, "project.token.assign", "token", tokenID, reason, before, after)
	})
}

// IsSalesCompany checks the active company-level assignment. Do not infer a
// sales scope from OwnerUserId: legacy data can contain multiple companies for
// one wallet account, each with a different sales owner.
func IsSalesCompany(salesUserID, companyID int) (bool, error) {
	if salesUserID <= 0 || companyID <= 0 {
		return false, nil
	}
	var count int64
	err := DB.Model(&CustomerAssignment{}).Where("sales_user_id = ? AND company_id = ? AND active = ?", salesUserID, companyID, true).Count(&count).Error
	return count > 0, err
}

func GetSalesAccountProfile(userID int) (*SalesAccountProfile, error) {
	profile := &SalesAccountProfile{}
	if err := DB.Where("user_id = ?", userID).First(profile).Error; err != nil {
		return nil, err
	}
	return profile, nil
}

func SaveSalesAccountProfile(profile *SalesAccountProfile, actor BusinessActor) error {
	if profile == nil || profile.UserId <= 0 || !actor.valid() {
		return errors.New("sales account and actor are required")
	}
	profile.Department = strings.TrimSpace(profile.Department)
	profile.Region = strings.TrimSpace(profile.Region)
	profile.Note = strings.TrimSpace(profile.Note)
	if len(profile.Department) > 128 || len(profile.Region) > 128 {
		return errors.New("sales account department and region must be at most 128 characters")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.First(&user, profile.UserId).Error; err != nil {
			return err
		}
		var previous SalesAccountProfile
		err := tx.Where("user_id = ?", profile.UserId).First(&previous).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(profile).Error; err != nil {
				return err
			}
			return createBusinessAuditEvent(tx, actor, "sales.account.profile.create", "user", profile.UserId, "sales account profile created", nil, profile)
		}
		if err != nil {
			return err
		}
		profile.Id = previous.Id
		if err := tx.Model(&SalesAccountProfile{}).Where("id = ?", previous.Id).Updates(map[string]interface{}{
			"department": profile.Department,
			"region":     profile.Region,
			"note":       profile.Note,
		}).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "sales.account.profile.update", "user", profile.UserId, "sales account profile updated", previous, profile)
	})
}

type BalanceLedgerFilter struct {
	CompanyId int
	UserId    int
	ProjectId int
	TokenId   int
	EntryType string
	ModelName string
	StartAt   int64
	EndAt     int64
}

func balanceLedgerQuery(filter BalanceLedgerFilter) *gorm.DB {
	ledgerTable := DB.NamingStrategy.TableName("BalanceLedger")
	query := DB.Model(&BalanceLedger{}).Order(ledgerTable + ".id desc")
	if filter.CompanyId > 0 {
		query = query.Where(ledgerTable+".company_id = ?", filter.CompanyId)
	}
	if filter.UserId > 0 {
		query = query.Where(ledgerTable+".user_id = ?", filter.UserId)
	}
	if filter.ProjectId > 0 {
		query = query.Where(ledgerTable+".project_id = ?", filter.ProjectId)
	}
	if filter.TokenId > 0 {
		query = query.Where(ledgerTable+".token_id = ?", filter.TokenId)
	}
	if filter.EntryType != "" {
		query = query.Where(ledgerTable+".entry_type = ?", filter.EntryType)
	}
	if filter.ModelName != "" {
		consumptionTable := DB.NamingStrategy.TableName("BusinessConsumption")
		query = query.Joins("JOIN "+consumptionTable+" ON "+consumptionTable+".request_id = "+ledgerTable+".request_id AND "+consumptionTable+".token_id = "+ledgerTable+".token_id").Where(consumptionTable+".model_name = ?", filter.ModelName)
	}
	if filter.StartAt > 0 {
		query = query.Where(ledgerTable+".created_at >= ?", filter.StartAt)
	}
	if filter.EndAt > 0 {
		query = query.Where(ledgerTable+".created_at <= ?", filter.EndAt)
	}
	return query
}

func ListBalanceLedgersWithFilter(filter BalanceLedgerFilter, startIdx, pageSize int) ([]*BalanceLedger, int64, error) {
	query := balanceLedgerQuery(filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	ledgers := make([]*BalanceLedger, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&ledgers).Error; err != nil {
		return nil, 0, err
	}
	return ledgers, total, nil
}

func ListBalanceLedgers(userID, projectID int, entryType string, startIdx, pageSize int) ([]*BalanceLedger, int64, error) {
	return ListBalanceLedgersWithFilter(BalanceLedgerFilter{
		UserId:    userID,
		ProjectId: projectID,
		EntryType: entryType,
	}, startIdx, pageSize)
}

func ExportBalanceLedgers(filter BalanceLedgerFilter, maxRows int) ([]*BalanceLedger, error) {
	ledgers := make([]*BalanceLedger, 0)
	if err := balanceLedgerQuery(filter).Limit(maxRows).Find(&ledgers).Error; err != nil {
		return nil, err
	}
	return ledgers, nil
}

func ListBusinessAuditEvents(startIdx, pageSize int) ([]*BusinessAuditEvent, int64, error) {
	query := DB.Model(&BusinessAuditEvent{}).Order("id desc")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	events := make([]*BusinessAuditEvent, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func ListBusinessAnnouncements(activeOnly bool) ([]*BusinessAnnouncement, error) {
	query := DB.Order("id desc")
	if activeOnly {
		now := time.Now().Unix()
		query = query.Where("status = ? AND (starts_at = 0 OR starts_at <= ?) AND (ends_at = 0 OR ends_at >= ?)", "published", now, now)
	}
	announcements := make([]*BusinessAnnouncement, 0)
	if err := query.Find(&announcements).Error; err != nil {
		return nil, err
	}
	return announcements, nil
}

func SaveBusinessAnnouncement(announcement *BusinessAnnouncement, actor BusinessActor) error {
	announcement.Title = strings.TrimSpace(announcement.Title)
	announcement.Content = strings.TrimSpace(announcement.Content)
	if announcement.Title == "" || announcement.Content == "" || (announcement.Status != "draft" && announcement.Status != "published") {
		return errors.New("announcement title, content, and status are required")
	}
	if announcement.EndsAt > 0 && announcement.StartsAt > 0 && announcement.EndsAt < announcement.StartsAt {
		return errors.New("announcement end time must be after its start time")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if announcement.Id == 0 {
			announcement.CreatedBy = actor.UserId
			announcement.UpdatedBy = actor.UserId
			if err := tx.Create(announcement).Error; err != nil {
				return err
			}
			return createBusinessAuditEvent(tx, actor, "announcement.create", "announcement", announcement.Id, "", nil, announcement)
		}
		var previous BusinessAnnouncement
		if err := lockForUpdate(tx).First(&previous, announcement.Id).Error; err != nil {
			return err
		}
		if err := tx.Model(&BusinessAnnouncement{}).Where("id = ?", announcement.Id).Updates(map[string]interface{}{
			"title": announcement.Title, "content": announcement.Content, "status": announcement.Status,
			"starts_at": announcement.StartsAt, "ends_at": announcement.EndsAt, "updated_by": actor.UserId,
		}).Error; err != nil {
			return err
		}
		return createBusinessAuditEvent(tx, actor, "announcement.update", "announcement", announcement.Id, "", previous, announcement)
	})
}

func createBusinessAuditEvent(tx *gorm.DB, actor BusinessActor, action, resource string, resourceID int, reason string, before, after interface{}) error {
	if !actor.valid() {
		return errors.New("invalid business audit actor")
	}
	event := BusinessAuditEvent{
		ActorUserId:  actor.UserId,
		ActorName:    actor.Username,
		RoleSnapshot: actor.RoleSnapshot,
		Action:       action,
		Resource:     resource,
		ResourceId:   resourceID,
		Reason:       reason,
		BeforeValue:  common.GetJsonString(before),
		AfterValue:   common.GetJsonString(after),
		Ip:           actor.IP,
	}
	return tx.Create(&event).Error
}

// RecordBusinessAuditEventInTx lets a caller include non-model changes, such
// as a Casbin role assignment, in the same primary-database transaction.
func RecordBusinessAuditEventInTx(tx *gorm.DB, actor BusinessActor, action, resource string, resourceID int, reason string, before, after interface{}) error {
	return createBusinessAuditEvent(tx, actor, action, resource, resourceID, reason, before, after)
}

func absBusinessQuota(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

type BusinessProjectSummary struct {
	Project       BusinessProject `json:"project"`
	Balance       int             `json:"balance"`
	TokenCount    int64           `json:"token_count"`
	ConsumedQuota int64           `json:"consumed_quota"`
	RequestCount  int64           `json:"request_count"`
	ErrorCount    int64           `json:"error_count"`
	Alerts        []string        `json:"alerts"`
}

type BusinessUsageLog struct {
	Id               int    `json:"id"`
	CreatedAt        int64  `json:"created_at"`
	Type             int    `json:"type"`
	ModelName        string `json:"model_name"`
	Quota            int    `json:"quota"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	UseTime          int    `json:"use_time"`
	IsStream         bool   `json:"is_stream"`
	TokenId          int    `json:"token_id"`
	TokenIdentifier  string `json:"token_identifier"`
	RequestId        string `json:"request_id"`
	Masked           bool   `json:"masked"`
}

type BusinessOperationsOverview struct {
	StartTimestamp         int64 `json:"start_timestamp"`
	EndTimestamp           int64 `json:"end_timestamp"`
	CompanyCount           int64 `json:"company_count"`
	ProjectCount           int64 `json:"project_count"`
	PendingAdjustmentCount int64 `json:"pending_adjustment_count"`
	RequestCount           int64 `json:"request_count"`
	ErrorCount             int64 `json:"error_count"`
	ConsumedQuota          int64 `json:"consumed_quota"`
	// P0-27 经营指标：收入=窗口消费，成本=收入×成本系数（operations.cost_ratio，默认 0.6），毛利=收入-成本。
	CostRatio              float64          `json:"cost_ratio"`
	RevenueQuota           int64            `json:"revenue_quota"`
	CostQuota              int64            `json:"cost_quota"`
	GrossMarginQuota       int64            `json:"gross_margin_quota"`
	TopModels              []CompanyModelUsage `json:"top_models"`
	TopCompanies           []CompanyModelUsage `json:"top_companies"`
	EnabledChannelCount    int64 `json:"enabled_channel_count"`
	DegradedChannelCount   int64 `json:"degraded_channel_count"`
}

type BusinessChannelHealth struct {
	Id           int   `json:"id"`
	Status       int   `json:"status"`
	ResponseTime int   `json:"response_time"`
	TestTime     int64 `json:"test_time"`
}

func GetBusinessProject(projectID int) (*BusinessProject, error) {
	if projectID <= 0 {
		return nil, errors.New("invalid project id")
	}
	var project BusinessProject
	if err := DB.First(&project, projectID).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

// IsCompanyOwner reports whether a balance account is attached to an enterprise
// profile. Enterprise balance changes must use the reviewed finance workflow
// instead of legacy direct-admin quota operations.
func IsCompanyOwner(userID int) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var count int64
	if err := DB.Model(&Company{}).Where("owner_user_id = ?", userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetCompanySalesBalance returns a customer balance only when the account is
// exclusively bound to the requested company. Historical data may predate the
// one-company-per-wallet invariant; returning nil for those shared accounts is
// safer than leaking another company's or personal funds through sales APIs.
func GetCompanySalesBalance(companyID int) (*int, error) {
	if companyID <= 0 {
		return nil, errors.New("invalid company id")
	}
	var company Company
	if err := DB.Select("id", "owner_user_id").First(&company, companyID).Error; err != nil {
		return nil, err
	}
	var companyCount int64
	if err := DB.Model(&Company{}).Where("owner_user_id = ?", company.OwnerUserId).Count(&companyCount).Error; err != nil {
		return nil, err
	}
	if companyCount != 1 {
		return nil, nil
	}
	var user User
	if err := DB.Select("id", "quota").First(&user, company.OwnerUserId).Error; err != nil {
		return nil, err
	}
	balance := user.Quota
	return &balance, nil
}

func ListBusinessProjects(companyID int) ([]*BusinessProject, error) {
	query := DB.Order("id desc")
	if companyID > 0 {
		query = query.Where("company_id = ?", companyID)
	}
	projects := make([]*BusinessProject, 0)
	if err := query.Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func ListPlatformReadOnlyUsers(startIdx, pageSize int) ([]*PlatformReadOnlyUser, int64, error) {
	query := DB.Model(&User{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*PlatformReadOnlyUser, 0)
	if err := query.Select("id, username, display_name, quota, used_quota, request_count, status, created_at").Order("id desc").Limit(pageSize).Offset(startIdx).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func ListPlatformReadOnlyLedger(startIdx, pageSize int) ([]*PlatformReadOnlyLedgerEntry, int64, error) {
	query := DB.Model(&BalanceLedger{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*PlatformReadOnlyLedgerEntry, 0)
	if err := query.Select("id, company_id, user_id, amount, entry_type, created_at").Order("id desc").Limit(pageSize).Offset(startIdx).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func ListPlatformReadOnlyAdjustments(startIdx, pageSize int) ([]*PlatformReadOnlyAdjustment, int64, error) {
	query := DB.Model(&ManualCreditRequest{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*PlatformReadOnlyAdjustment, 0)
	if err := query.Select("id, company_id, user_id, amount, entry_type, status, created_at").Order("id desc").Limit(pageSize).Offset(startIdx).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetPlatformReadOnlyFinanceOverview() (*PlatformReadOnlyFinanceOverview, error) {
	overview := &PlatformReadOnlyFinanceOverview{}
	if err := DB.Model(&User{}).Select("COALESCE(SUM(quota), 0) AS available_quota, COALESCE(SUM(frozen_quota), 0) AS frozen_quota").Scan(overview).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ManualCreditRequest{}).Where("status IN ?", []string{ManualAdjustmentStatusPending, ManualAdjustmentStatusEscalated}).Count(&overview.PendingAdjustmentCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&ManualCreditRequest{}).Where("status = ?", ManualAdjustmentStatusApproved).Count(&overview.ApprovedAdjustmentCount).Error; err != nil {
		return nil, err
	}
	return overview, nil
}

func ListBusinessCompanies(startIdx, pageSize int) ([]*Company, int64, error) {
	query := DB.Model(&Company{}).Order("id desc")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	companies := make([]*Company, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&companies).Error; err != nil {
		return nil, 0, err
	}
	return companies, total, nil
}

func GetBusinessProjectSummary(projectID int, startTimestamp, endTimestamp int64) (*BusinessProjectSummary, error) {
	project, err := GetBusinessProject(projectID)
	if err != nil {
		return nil, err
	}
	var user User
	if err := DB.Select("id", "quota").First(&user, project.OwnerUserId).Error; err != nil {
		return nil, err
	}
	summary := &BusinessProjectSummary{Project: *project, Balance: user.Quota, Alerts: make([]string, 0)}
	if err := DB.Model(&Token{}).Where("project_id = ?", projectID).Count(&summary.TokenCount).Error; err != nil {
		return nil, err
	}
	// Main-database consumption projections are the project-level aggregate
	// source. They remain available when detailed LOG_DB storage is disabled,
	// externally managed, or has a shorter retention period.
	consumptionQuery := DB.Model(&BusinessConsumption{}).Where("project_id = ?", projectID)
	if startTimestamp > 0 {
		consumptionQuery = consumptionQuery.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		consumptionQuery = consumptionQuery.Where("created_at <= ?", endTimestamp)
	}
	if err := consumptionQuery.Count(&summary.RequestCount).Error; err != nil {
		return nil, err
	}
	var consumptionResult struct {
		Quota int64
	}
	if err := consumptionQuery.Select("COALESCE(SUM(quota), 0) AS quota").Scan(&consumptionResult).Error; err != nil {
		return nil, err
	}
	summary.ConsumedQuota = consumptionResult.Quota

	tokenIDs, err := listBusinessProjectTokenIDs(projectID)
	if err != nil {
		return nil, err
	}
	if len(tokenIDs) > 0 {
		errorQuery := LOG_DB.Model(&Log{}).Where("token_id IN ?", tokenIDs)
		if startTimestamp > 0 {
			errorQuery = errorQuery.Where("created_at >= ?", startTimestamp)
		}
		if endTimestamp > 0 {
			errorQuery = errorQuery.Where("created_at <= ?", endTimestamp)
		}
		if err := errorQuery.Where("type = ?", LogTypeError).Count(&summary.ErrorCount).Error; err != nil {
			return nil, err
		}
	}
	if project.LowBalanceQuota > 0 && user.Quota <= project.LowBalanceQuota {
		summary.Alerts = append(summary.Alerts, "low_balance")
	}
	if project.BudgetQuota > 0 && summary.ConsumedQuota >= int64(project.BudgetQuota) {
		summary.Alerts = append(summary.Alerts, "budget_exceeded")
	}
	if summary.RequestCount > 0 && summary.ErrorCount*100 >= summary.RequestCount*10 {
		summary.Alerts = append(summary.Alerts, "high_error_rate")
	}
	return summary, nil
}

func ListBusinessProjectTokens(projectID int) ([]*Token, error) {
	tokens := make([]*Token, 0)
	if err := DB.Where("project_id = ?", projectID).Order("id desc").Find(&tokens).Error; err != nil {
		return nil, err
	}
	for _, token := range tokens {
		token.Key = token.GetMaskedKey()
	}
	return tokens, nil
}

func ListMaskedBusinessLogs(userID, projectID int, startAt, endAt int64, startIdx, pageSize int) ([]*BusinessUsageLog, int64, error) {
	if userID <= 0 {
		return nil, 0, errors.New("customer is required")
	}
	if projectID <= 0 {
		return nil, 0, errors.New("project is required for masked business logs")
	}
	tokenIDs, err := listBusinessProjectTokenIDs(projectID)
	if err != nil {
		return nil, 0, err
	}
	return listMaskedBusinessLogsByTokenIDs(userID, tokenIDs, startAt, endAt, startIdx, pageSize)
}

// ListMaskedBusinessCompanyLogs returns only logs whose token belongs to a
// project within the assigned company. It intentionally does not fall back to
// all logs for the owner user because that would expose personal/unrelated
// project activity to a sales role.
func ListMaskedBusinessCompanyLogs(userID, companyID int, startAt, endAt int64, startIdx, pageSize int) ([]*BusinessUsageLog, int64, error) {
	if userID <= 0 || companyID <= 0 {
		return nil, 0, errors.New("customer and company are required")
	}
	projectIDs := DB.Model(&BusinessProject{}).Select("id").Where("company_id = ? AND owner_user_id = ?", companyID, userID)
	tokenIDs := make([]int, 0)
	if err := DB.Unscoped().Model(&Token{}).Where("project_id IN (?)", projectIDs).Pluck("id", &tokenIDs).Error; err != nil {
		return nil, 0, err
	}
	return listMaskedBusinessLogsByTokenIDs(userID, tokenIDs, startAt, endAt, startIdx, pageSize)
}

func listMaskedBusinessLogsByTokenIDs(userID int, tokenIDs []int, startAt, endAt int64, startIdx, pageSize int) ([]*BusinessUsageLog, int64, error) {
	if len(tokenIDs) == 0 {
		return make([]*BusinessUsageLog, 0), 0, nil
	}
	query := LOG_DB.Model(&Log{}).Where("user_id = ? AND token_id IN ?", userID, tokenIDs)
	if startAt > 0 {
		query = query.Where("created_at >= ?", startAt)
	}
	if endAt > 0 {
		query = query.Where("created_at <= ?", endAt)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	logs := make([]*Log, 0)
	if err := query.Order("created_at desc, id desc").Limit(pageSize).Offset(startIdx).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*BusinessUsageLog, 0, len(logs))
	tokens := make([]Token, 0)
	if err := DB.Where("id IN ?", tokenIDs).Find(&tokens).Error; err != nil {
		return nil, 0, err
	}
	tokenIdentifiers := make(map[int]string, len(tokens))
	for _, token := range tokens {
		identifier := token.GetMaskedKey()
		if token.Name != "" {
			identifier = token.Name + " (" + identifier + ")"
		}
		tokenIdentifiers[token.Id] = identifier
	}
	for _, log := range logs {
		items = append(items, &BusinessUsageLog{
			Id:               log.Id,
			CreatedAt:        log.CreatedAt,
			Type:             log.Type,
			ModelName:        log.ModelName,
			Quota:            log.Quota,
			PromptTokens:     log.PromptTokens,
			CompletionTokens: log.CompletionTokens,
			UseTime:          log.UseTime,
			IsStream:         log.IsStream,
			TokenId:          log.TokenId,
			TokenIdentifier:  tokenIdentifiers[log.TokenId],
			RequestId:        maskBusinessRequestID(log.RequestId),
			Masked:           true,
		})
	}
	return items, total, nil
}

func ListSalesCompanies(salesUserID, startIdx, pageSize int) ([]*Company, int64, error) {
	companyIDs := DB.Model(&CustomerAssignment{}).Select("company_id").Where("sales_user_id = ? AND active = ?", salesUserID, true)
	query := DB.Model(&Company{}).Where("id IN (?)", companyIDs).Order("id desc")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	companies := make([]*Company, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&companies).Error; err != nil {
		return nil, 0, err
	}
	return companies, total, nil
}

func ListCustomerAssignments(companyID int, startIdx, pageSize int) ([]*CustomerAssignment, int64, error) {
	query := DB.Model(&CustomerAssignment{}).Order("id desc")
	if companyID > 0 {
		query = query.Where("company_id = ?", companyID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	assignments := make([]*CustomerAssignment, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&assignments).Error; err != nil {
		return nil, 0, err
	}
	return assignments, total, nil
}

func GetBusinessOperationsOverview(startTimestamp, endTimestamp int64) (*BusinessOperationsOverview, error) {
	if startTimestamp <= 0 {
		startTimestamp = time.Now().Add(-24 * time.Hour).Unix()
	}
	if endTimestamp <= 0 {
		endTimestamp = time.Now().Unix()
	}
	overview := &BusinessOperationsOverview{StartTimestamp: startTimestamp, EndTimestamp: endTimestamp}
	for _, target := range []struct {
		model interface{}
		out   *int64
	}{
		{&Company{}, &overview.CompanyCount},
		{&BusinessProject{}, &overview.ProjectCount},
	} {
		if err := DB.Model(target.model).Count(target.out).Error; err != nil {
			return nil, err
		}
	}
	if err := DB.Model(&ManualCreditRequest{}).Where("status = ?", ManualAdjustmentStatusPending).Count(&overview.PendingAdjustmentCount).Error; err != nil {
		return nil, err
	}
	logQuery := LOG_DB.Model(&Log{}).Where("created_at >= ? AND created_at <= ?", startTimestamp, endTimestamp)
	if err := logQuery.Where("type = ?", LogTypeConsume).Count(&overview.RequestCount).Error; err != nil {
		return nil, err
	}
	if err := logQuery.Where("type = ?", LogTypeError).Count(&overview.ErrorCount).Error; err != nil {
		return nil, err
	}
	var quotaResult struct {
		Quota int64
	}
	if err := logQuery.Where("type = ?", LogTypeConsume).Select("COALESCE(SUM(quota), 0) AS quota").Scan(&quotaResult).Error; err != nil {
		return nil, err
	}
	overview.ConsumedQuota = quotaResult.Quota
	if err := DB.Model(&Channel{}).Where("status = ?", common.ChannelStatusEnabled).Count(&overview.EnabledChannelCount).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&Channel{}).Where("status <> ?", common.ChannelStatusEnabled).Count(&overview.DegradedChannelCount).Error; err != nil {
		return nil, err
	}
	// P0-27 经营指标：收入/成本/毛利与模型、企业排行。
	overview.RevenueQuota = overview.ConsumedQuota
	overview.CostRatio = 0.6
	if raw, ok := common.OptionMap["operations.cost_ratio"]; ok && raw != "" {
		if ratio, err := strconv.ParseFloat(raw, 64); err == nil && ratio >= 0 && ratio <= 1 {
			overview.CostRatio = ratio
		}
	}
	overview.CostQuota = int64(float64(overview.RevenueQuota) * overview.CostRatio)
	overview.GrossMarginQuota = overview.RevenueQuota - overview.CostQuota
	if err := DB.Model(&BusinessConsumption{}).
		Select("model_name, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS calls").
		Where("created_at >= ? AND created_at <= ?", startTimestamp, endTimestamp).
		Group("model_name").Order("quota DESC").Limit(5).
		Scan(&overview.TopModels).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&BusinessConsumption{}).
		Select("company_id AS model_name, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS calls").
		Where("created_at >= ? AND created_at <= ?", startTimestamp, endTimestamp).
		Group("company_id").Order("quota DESC").Limit(5).
		Scan(&overview.TopCompanies).Error; err != nil {
		return nil, err
	}
	return overview, nil
}

func ListBusinessChannelHealth() ([]*BusinessChannelHealth, error) {
	items := make([]*BusinessChannelHealth, 0)
	if err := DB.Model(&Channel{}).Select("id", "status", "response_time", "test_time").Order("id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func maskBusinessRequestID(requestID string) string {
	if requestID == "" {
		return ""
	}
	if len(requestID) <= 8 {
		return strings.Repeat("*", len(requestID))
	}
	return requestID[:4] + strings.Repeat("*", len(requestID)-8) + requestID[len(requestID)-4:]
}

func listBusinessProjectTokenIDs(projectID int) ([]int, error) {
	tokenIDs := make([]int, 0)
	// Usage logs may outlive a soft-deleted token. Include historical token IDs
	// so project reports and sales views remain complete.
	if err := DB.Unscoped().Model(&Token{}).Where("project_id = ?", projectID).Pluck("id", &tokenIDs).Error; err != nil {
		return nil, err
	}
	return tokenIDs, nil
}


// GetBusinessCompany 返回单个企业（含主体资料字段）。
func GetBusinessCompany(companyID int) (*Company, error) {
	if companyID <= 0 {
		return nil, errors.New("invalid company id")
	}
	var company Company
	if err := DB.First(&company, companyID).Error; err != nil {
		return nil, err
	}
	return &company, nil
}

// CountBusinessCompaniesOwnedBy 统计某用户作为 owner 的企业数量（P0-02 自助开户限一）。
func CountBusinessCompaniesOwnedBy(userID int) (int64, error) {
	if userID <= 0 {
		return 0, errors.New("invalid user id")
	}
	var count int64
	if err := DB.Model(&Company{}).Where("owner_user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetBusinessCompanyByOwner 返回某用户作为 owner 的企业（P0-02 自助查询）。
func GetBusinessCompanyByOwner(userID int) (*Company, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	var company Company
	if err := DB.Where("owner_user_id = ?", userID).Order("id ASC").First(&company).Error; err != nil {
		return nil, err
	}
	return &company, nil
}
