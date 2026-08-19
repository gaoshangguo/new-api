package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupBusinessFixture(t *testing.T, customerQuota int) (BusinessActor, BusinessActor, *User, *Company, *BusinessProject) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(
		&Company{},
		&BusinessProject{},
		&CustomerAssignment{},
		&BalanceLedger{},
		&BusinessConsumption{},
		&BusinessProjectBudgetReservation{},
		&BusinessProjectReminder{},
		&ManualCreditRequest{},
		&BusinessAuditEvent{},
		&BusinessAnnouncement{},
		&BusinessFollowUp{},
	))
	for _, table := range []interface{}{
		&BalanceLedger{},
		&BusinessConsumption{},
		&BusinessProjectBudgetReservation{},
		&BusinessProjectReminder{},
		&ManualCreditRequest{},
		&CustomerAssignment{},
		&BusinessProject{},
		&Company{},
		&BusinessAuditEvent{},
		&BusinessAnnouncement{},
		&BusinessFollowUp{},
		&Token{},
		&Log{},
		&User{},
	} {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Unscoped().Delete(table).Error)
	}

	entryUser := &User{Username: "finance-entry", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "business-entry"}
	approverUser := &User{Username: "finance-approver", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "business-approver"}
	customer := &User{Username: "enterprise-customer", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: customerQuota, AffCode: "business-customer"}
	require.NoError(t, DB.Create(entryUser).Error)
	require.NoError(t, DB.Create(approverUser).Error)
	require.NoError(t, DB.Create(customer).Error)

	entryActor := BusinessActor{UserId: entryUser.Id, Username: entryUser.Username, RoleSnapshot: "business:finance_entry", IP: "127.0.0.1"}
	approverActor := BusinessActor{UserId: approverUser.Id, Username: approverUser.Username, RoleSnapshot: "business:finance_approver", IP: "127.0.0.2"}
	company := &Company{Name: "Example Enterprise", OwnerUserId: customer.Id}
	require.NoError(t, CreateCompany(company, entryActor))
	project := &BusinessProject{CompanyId: company.Id, Name: "Production", OwnerUserId: customer.Id, Status: BusinessProjectStatusEnabled}
	require.NoError(t, CreateBusinessProject(project, entryActor))
	return entryActor, approverActor, customer, company, project
}

func TestManualAdjustmentRequiresIndependentReviewAndCreatesImmutableLedger(t *testing.T) {
	entryActor, approverActor, customer, company, project := setupBusinessFixture(t, 100)
	request := &ManualCreditRequest{
		CompanyId:                company.Id,
		UserId:                   customer.Id,
		ProjectId:                project.Id,
		EntryType:                LedgerEntryManualCredit,
		Amount:                   250,
		ExternalReference:        "bank-20260803-001",
		Reason:                   "Bank transfer received",
		Note:                     "Finance confirmed the transfer",
		InvoiceNumber:            "INV-2026-001",
		ExternalFinanceReference: "bank-statement-001",
		ReconciliationConclusion: "Amount and payer reconciled",
		CreatedBy:                entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(request, entryActor))

	_, err := ApproveManualCreditRequest(request.Id, entryActor)
	require.Error(t, err)
	require.Contains(t, err.Error(), "maker and checker")

	ledger, err := ApproveManualCreditRequest(request.Id, approverActor)
	require.NoError(t, err)
	assert.Equal(t, 250, ledger.Amount)
	assert.Equal(t, 100, ledger.BalanceBefore)
	assert.Equal(t, 350, ledger.BalanceAfter)
	assert.True(t, ledger.BalanceSnapshotAvailable)
	assert.Equal(t, entryActor.UserId, ledger.CreatedBy)
	assert.Equal(t, approverActor.UserId, ledger.ApprovedBy)
	assert.Equal(t, request.InvoiceNumber, ledger.InvoiceNumber)
	assert.Equal(t, request.ExternalFinanceReference, ledger.ExternalFinanceReference)
	assert.Equal(t, request.ReconciliationConclusion, ledger.ReconciliationConclusion)

	var updatedCustomer User
	require.NoError(t, DB.First(&updatedCustomer, customer.Id).Error)
	assert.Equal(t, 350, updatedCustomer.Quota)

	_, err = ApproveManualCreditRequest(request.Id, approverActor)
	require.Error(t, err)

	var ledgerCount int64
	require.NoError(t, DB.Model(&BalanceLedger{}).Count(&ledgerCount).Error)
	assert.Equal(t, int64(1), ledgerCount)
	var audit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource_id = ?", "finance.adjustment.approve", request.Id).First(&audit).Error)
	assert.Equal(t, approverActor.UserId, audit.ActorUserId)
	assert.Equal(t, approverActor.IP, audit.Ip)
	assert.NotEmpty(t, audit.BeforeValue)
	assert.NotEmpty(t, audit.AfterValue)
}

func TestManualAdjustmentDuplicateVoucherAmountRequiresEscalation(t *testing.T) {
	entryActor, approverActor, customer, company, project := setupBusinessFixture(t, 0)
	first := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		Amount:            100,
		ExternalReference: "voucher-duplicate",
		Reason:            "Initial entry",
		Note:              "Initial finance record",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(first, entryActor))
	_, err := ApproveManualCreditRequest(first.Id, approverActor)
	require.NoError(t, err)
	second := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		Amount:            100,
		ExternalReference: "voucher-duplicate",
		Reason:            "Duplicate entry",
		Note:              "Duplicate finance record",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(second, entryActor))
	assert.Equal(t, ManualAdjustmentStatusEscalated, second.Status)
	assert.Equal(t, first.Id, second.DuplicateOfRequestId)

	_, err = ApproveManualCreditRequest(second.Id, approverActor)
	require.Error(t, err)
	ledger, err := ApproveEscalatedManualCreditRequest(second.Id, "Duplicate confirmed by finance director", approverActor)
	require.NoError(t, err)
	assert.Equal(t, 100, ledger.Amount)

	var persisted ManualCreditRequest
	require.NoError(t, DB.First(&persisted, second.Id).Error)
	assert.Equal(t, ManualAdjustmentStatusApproved, persisted.Status)
	assert.Equal(t, "Duplicate confirmed by finance director", persisted.EscalationReason)
}

// P0-24：并发提交同一客户、同一凭证与金额时，必须恰好一条 pending、
// 一条 escalated。旧实现 check-then-insert 无行锁，并发窗口内两条都会
// 判为无重复，可被分别批准造成同一凭证双重入账。
func TestManualAdjustmentConcurrentDuplicateVoucherEscalatesExactlyOnce(t *testing.T) {
	entryActor, _, customer, company, project := setupBusinessFixture(t, 0)
	start := make(chan struct{})
	results := make(chan *ManualCreditRequest, 2)
	create := func() {
		<-start
		req := &ManualCreditRequest{
			CompanyId:         company.Id,
			UserId:            customer.Id,
			ProjectId:         project.Id,
			Amount:            100,
			ExternalReference: "voucher-concurrent",
			Reason:            "Concurrent entry",
			Note:              "Concurrent finance record",
			CreatedBy:         entryActor.UserId,
		}
		_ = CreateManualCreditRequest(req, entryActor)
		results <- req
	}
	go create()
	go create()
	close(start)
	first := <-results
	second := <-results

	statuses := []string{first.Status, second.Status}
	assert.ElementsMatch(t,
		[]string{ManualAdjustmentStatusPending, ManualAdjustmentStatusEscalated},
		statuses)
	if first.Status == ManualAdjustmentStatusEscalated {
		assert.Equal(t, second.Id, first.DuplicateOfRequestId)
	} else {
		assert.Equal(t, first.Id, second.DuplicateOfRequestId)
	}
}

func TestListBalanceLedgersWithFilter(t *testing.T) {
	_, _, customer, company, project := setupBusinessFixture(t, 0)
	matching := &BalanceLedger{
		CompanyId:     company.Id,
		UserId:        customer.Id,
		ProjectId:     project.Id,
		TokenId:       101,
		RequestId:     "ledger-filter-match",
		EntryType:     LedgerEntryManualCredit,
		ReferenceType: "test-ledger-filter",
		ReferenceId:   1,
		CreatedAt:     100,
	}
	nonMatching := &BalanceLedger{
		CompanyId:     company.Id,
		UserId:        customer.Id,
		ProjectId:     project.Id,
		TokenId:       102,
		RequestId:     "ledger-filter-other",
		EntryType:     LedgerEntryManualCredit,
		ReferenceType: "test-ledger-filter",
		ReferenceId:   2,
		CreatedAt:     200,
	}
	require.NoError(t, DB.Create(matching).Error)
	require.NoError(t, DB.Create(nonMatching).Error)
	require.NoError(t, DB.Create(&BusinessConsumption{
		CompanyId: company.Id,
		ProjectId: project.Id,
		UserId:    customer.Id,
		TokenId:   matching.TokenId,
		RequestId: matching.RequestId,
		Quota:     1,
		ModelName: "gpt-test",
		CreatedAt: matching.CreatedAt,
	}).Error)

	ledgers, total, err := ListBalanceLedgersWithFilter(BalanceLedgerFilter{
		UserId:    customer.Id,
		ProjectId: project.Id,
		TokenId:   matching.TokenId,
		ModelName: "gpt-test",
		StartAt:   50,
		EndAt:     150,
	}, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, ledgers, 1)
	assert.Equal(t, matching.Id, ledgers[0].Id)
}

func TestManualAdjustmentRequiresNote(t *testing.T) {
	entryActor, _, customer, company, project := setupBusinessFixture(t, 0)
	request := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		Amount:            100,
		ExternalReference: "note-required-001",
		Reason:            "Bank transfer received",
		Note:              "   ",
		CreatedBy:         entryActor.UserId,
	}

	err := CreateManualCreditRequest(request, entryActor)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "note")

	var requestCount int64
	require.NoError(t, DB.Model(&ManualCreditRequest{}).Count(&requestCount).Error)
	assert.Zero(t, requestCount)
}

func TestRejectedManualAdjustmentKeepsStateTransitionAudit(t *testing.T) {
	entryActor, approverActor, customer, company, project := setupBusinessFixture(t, 0)
	request := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		Amount:            100,
		ExternalReference: "reject-audit-001",
		Reason:            "Needs a second bank check",
		Note:              "Awaiting bank confirmation",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(request, entryActor))
	_, err := RejectManualCreditRequest(request.Id, "Voucher is incomplete", approverActor)
	require.NoError(t, err)

	var audit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource_id = ?", "finance.adjustment.reject", request.Id).First(&audit).Error)
	assert.Contains(t, audit.BeforeValue, ManualAdjustmentStatusPending)
	assert.Contains(t, audit.AfterValue, ManualAdjustmentStatusRejected)
	assert.Equal(t, "Voucher is incomplete", audit.Reason)
}

func TestDebitAdjustmentCannotMakeQuotaNegative(t *testing.T) {
	entryActor, approverActor, customer, company, project := setupBusinessFixture(t, 50)
	request := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		EntryType:         LedgerEntryManualDebit,
		Amount:            51,
		ExternalReference: "debit-001",
		Reason:            "Correction",
		Note:              "Customer billing correction",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(request, entryActor))
	_, err := ApproveManualCreditRequest(request.Id, approverActor)
	require.Error(t, err)
	require.Contains(t, err.Error(), "negative")

	var persistedRequest ManualCreditRequest
	require.NoError(t, DB.First(&persistedRequest, request.Id).Error)
	assert.Equal(t, ManualAdjustmentStatusPending, persistedRequest.Status)
	var persistedCustomer User
	require.NoError(t, DB.First(&persistedCustomer, customer.Id).Error)
	assert.Equal(t, 50, persistedCustomer.Quota)
	var ledgerCount int64
	require.NoError(t, DB.Model(&BalanceLedger{}).Count(&ledgerCount).Error)
	assert.Zero(t, ledgerCount)
}

func TestFreezeAndUnfreezeTrackFrozenQuota(t *testing.T) {
	entryActor, approverActor, customer, company, project := setupBusinessFixture(t, 100)
	freeze := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		EntryType:         LedgerEntryFreeze,
		Amount:            40,
		ExternalReference: "freeze-001",
		Reason:            "Reserve quota for a billing dispute",
		Note:              "Dispute reserve",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(freeze, entryActor))
	frozenLedger, err := ApproveManualCreditRequest(freeze.Id, approverActor)
	require.NoError(t, err)
	assert.Equal(t, -40, frozenLedger.Amount)
	assert.Equal(t, 100, frozenLedger.BalanceBefore)
	assert.Equal(t, 60, frozenLedger.BalanceAfter)
	assert.Equal(t, 0, frozenLedger.FrozenBefore)
	assert.Equal(t, 40, frozenLedger.FrozenAfter)

	overUnfreeze := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		EntryType:         LedgerEntryUnfreeze,
		Amount:            41,
		ExternalReference: "unfreeze-too-much-001",
		Reason:            "Invalid release",
		Note:              "Validation scenario",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(overUnfreeze, entryActor))
	_, err = ApproveManualCreditRequest(overUnfreeze.Id, approverActor)
	require.ErrorContains(t, err, "more quota than is frozen")

	unfreeze := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		EntryType:         LedgerEntryUnfreeze,
		Amount:            40,
		ExternalReference: "unfreeze-001",
		Reason:            "Dispute resolved",
		Note:              "Dispute closure",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(unfreeze, entryActor))
	unfrozenLedger, err := ApproveManualCreditRequest(unfreeze.Id, approverActor)
	require.NoError(t, err)
	assert.Equal(t, 40, unfrozenLedger.Amount)
	assert.Equal(t, 60, unfrozenLedger.BalanceBefore)
	assert.Equal(t, 100, unfrozenLedger.BalanceAfter)
	assert.Equal(t, 40, unfrozenLedger.FrozenBefore)
	assert.Equal(t, 0, unfrozenLedger.FrozenAfter)

	var refreshedCustomer User
	require.NoError(t, DB.First(&refreshedCustomer, customer.Id).Error)
	assert.Equal(t, 100, refreshedCustomer.Quota)
	assert.Zero(t, refreshedCustomer.FrozenQuota)
}

func TestProjectTokenAssociationCannotBeReassigned(t *testing.T) {
	entryActor, _, customer, company, project := setupBusinessFixture(t, 0)
	token := &Token{UserId: customer.Id, Name: "project-key", Key: "project-key-0001", Status: common.TokenStatusEnabled}
	require.NoError(t, DB.Create(token).Error)
	require.NoError(t, SetBusinessProjectToken(project.Id, token.Id, "Associate production key", entryActor))

	var assignmentAudit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource_id = ?", "project.token.assign", token.Id).First(&assignmentAudit).Error)
	assert.NotContains(t, assignmentAudit.BeforeValue, token.Key)
	assert.NotContains(t, assignmentAudit.AfterValue, token.Key)
	assert.Contains(t, assignmentAudit.BeforeValue, `"project_id":0`)
	assert.Contains(t, assignmentAudit.AfterValue, `"project_id":`)

	secondProject := &BusinessProject{CompanyId: company.Id, Name: "Staging", OwnerUserId: customer.Id, Status: BusinessProjectStatusEnabled}
	require.NoError(t, CreateBusinessProject(secondProject, entryActor))
	err := SetBusinessProjectToken(secondProject.Id, token.Id, "Attempt reassignment", entryActor)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be reassigned")
}

func TestCompanyOwnerCannotChangeThroughProfileUpdate(t *testing.T) {
	entryActor, _, _, company, _ := setupBusinessFixture(t, 0)
	newOwner := &User{Username: "replacement-owner", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "replacement-owner-aff"}
	require.NoError(t, DB.Create(newOwner).Error)

	update := *company
	update.OwnerUserId = newOwner.Id
	update.ContactName = "Updated contact"
	err := UpdateCompany(&update, entryActor)
	require.ErrorContains(t, err, "owner cannot be changed")

	var persisted Company
	require.NoError(t, DB.First(&persisted, company.Id).Error)
	assert.Equal(t, company.OwnerUserId, persisted.OwnerUserId)
}

func TestMaskedBusinessCompanyLogsExcludePersonalAndOtherCompanyTokens(t *testing.T) {
	entryActor, _, customer, company, project := setupBusinessFixture(t, 100)
	companyToken := &Token{UserId: customer.Id, Name: "company-a-key", Key: "company-a-key-0001", Status: common.TokenStatusEnabled}
	require.NoError(t, DB.Create(companyToken).Error)
	require.NoError(t, SetBusinessProjectToken(project.Id, companyToken.Id, "Bind company A key", entryActor))

	// This is legacy inconsistent data that predates the one-company-per-wallet
	// invariant. Keep it in the fixture to verify sales log filtering fails
	// closed instead of exposing a shared owner's other-company activity.
	otherCompany := &Company{Name: "Other Enterprise", OwnerUserId: customer.Id}
	require.NoError(t, DB.Create(otherCompany).Error)
	otherProject := &BusinessProject{CompanyId: otherCompany.Id, Name: "Other Project", OwnerUserId: customer.Id, Status: BusinessProjectStatusEnabled}
	require.NoError(t, CreateBusinessProject(otherProject, entryActor))
	otherCompanyToken := &Token{UserId: customer.Id, Name: "company-b-key", Key: "company-b-key-0001", Status: common.TokenStatusEnabled}
	require.NoError(t, DB.Create(otherCompanyToken).Error)
	require.NoError(t, SetBusinessProjectToken(otherProject.Id, otherCompanyToken.Id, "Bind company B key", entryActor))

	personalToken := &Token{UserId: customer.Id, Name: "personal-key", Key: "personal-key-0001", Status: common.TokenStatusEnabled}
	require.NoError(t, DB.Create(personalToken).Error)
	for _, tokenID := range []int{companyToken.Id, otherCompanyToken.Id, personalToken.Id} {
		require.NoError(t, createLog(&Log{
			UserId: customer.Id, TokenId: tokenID, Type: LogTypeConsume, ModelName: "gpt-test", Quota: 1,
		}))
	}

	logs, total, err := ListMaskedBusinessCompanyLogs(customer.Id, company.Id, 0, 0, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	assert.Equal(t, companyToken.Id, logs[0].TokenId)
}

func TestCompanyWalletCannotBeBoundToMultipleCompanies(t *testing.T) {
	entryActor, _, customer, _, _ := setupBusinessFixture(t, 0)
	secondCompany := &Company{Name: "Second Enterprise", OwnerUserId: customer.Id}
	err := CreateCompany(secondCompany, entryActor)
	require.ErrorIs(t, err, ErrCompanyOwnerAlreadyBound)
}

func TestCreateCompanyWithSalesAssignmentCreatesBothRecords(t *testing.T) {
	entryActor, _, _, _, _ := setupBusinessFixture(t, 0)
	newCustomer := &User{Username: "invited-customer", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "invited-customer-aff"}
	sales := &User{Username: "inviting-sales", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "inviting-sales-aff"}
	require.NoError(t, DB.Create(newCustomer).Error)
	require.NoError(t, DB.Create(sales).Error)
	company := &Company{Name: "Invited Enterprise", OwnerUserId: newCustomer.Id}
	require.NoError(t, CreateCompanyWithSalesAssignment(company, sales.Id, "sales invitation registration", entryActor))

	var assignment CustomerAssignment
	require.NoError(t, DB.Where("company_id = ? AND active = ?", company.Id, true).First(&assignment).Error)
	assert.Equal(t, newCustomer.Id, assignment.CustomerUserId)
	assert.Equal(t, sales.Id, assignment.SalesUserId)
	assert.Equal(t, "sales invitation registration", assignment.Reason)
}

func TestIsCompanyOwner(t *testing.T) {
	_, _, customer, _, _ := setupBusinessFixture(t, 0)
	isOwner, err := IsCompanyOwner(customer.Id)
	require.NoError(t, err)
	assert.True(t, isOwner)

	personalUser := &User{Username: "personal-user", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "personal-user-aff"}
	require.NoError(t, DB.Create(personalUser).Error)
	isOwner, err = IsCompanyOwner(personalUser.Id)
	require.NoError(t, err)
	assert.False(t, isOwner)
}

func TestCompanySalesBalanceHidesHistoricalSharedWallet(t *testing.T) {
	_, _, customer, company, _ := setupBusinessFixture(t, 321)
	balance, err := GetCompanySalesBalance(company.Id)
	require.NoError(t, err)
	require.NotNil(t, balance)
	assert.Equal(t, customer.Quota, *balance)

	// Historical duplicate ownership may exist before the invariant is enabled.
	// Sales must receive no account-wide balance in that ambiguous state.
	sharedCompany := &Company{Name: "Legacy Shared Wallet", OwnerUserId: customer.Id}
	require.NoError(t, DB.Create(sharedCompany).Error)
	balance, err = GetCompanySalesBalance(company.Id)
	require.NoError(t, err)
	assert.Nil(t, balance)
}

func TestProjectUpdateRequiresAnAuditReason(t *testing.T) {
	entryActor, _, _, _, project := setupBusinessFixture(t, 0)
	update := *project
	update.BudgetQuota = 100
	require.ErrorContains(t, UpdateBusinessProject(&update, "", entryActor), "reason")
	require.NoError(t, UpdateBusinessProject(&update, "raise approved project budget", entryActor))

	var audit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource_id = ?", "project.update", project.Id).First(&audit).Error)
	assert.Equal(t, "raise approved project budget", audit.Reason)
}

func TestSalesCompanyScopeDoesNotFollowSharedWalletOwner(t *testing.T) {
	entryActor, _, customer, company, _ := setupBusinessFixture(t, 0)
	secondCompany := &Company{Name: "Legacy Company B", OwnerUserId: customer.Id}
	require.NoError(t, DB.Create(secondCompany).Error)

	firstSales := &User{Username: "sales-a", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "sales-a-aff"}
	secondSales := &User{Username: "sales-b", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "sales-b-aff"}
	require.NoError(t, DB.Create(firstSales).Error)
	require.NoError(t, DB.Create(secondSales).Error)
	_, err := AssignCustomer(company.Id, firstSales.Id, "company A handoff", entryActor)
	require.NoError(t, err)
	_, err = AssignCustomer(secondCompany.Id, secondSales.Id, "company B handoff", entryActor)
	require.NoError(t, err)

	allowed, err := IsSalesCompany(firstSales.Id, company.Id)
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = IsSalesCompany(firstSales.Id, secondCompany.Id)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestAssignCustomersCreatesHistoryAtomically(t *testing.T) {
	entryActor, _, customer, company, _ := setupBusinessFixture(t, 0)
	secondCompany := &Company{Name: "Batch Enterprise", OwnerUserId: customer.Id}
	require.NoError(t, DB.Create(secondCompany).Error)
	sales := &User{Username: "batch-sales", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "batch-sales-aff"}
	require.NoError(t, DB.Create(sales).Error)

	assignments, err := AssignCustomers([]int{company.Id, secondCompany.Id}, sales.Id, "regional handoff", entryActor)
	require.NoError(t, err)
	require.Len(t, assignments, 2)
	for _, assignment := range assignments {
		assert.True(t, assignment.Active)
		assert.Equal(t, sales.Id, assignment.SalesUserId)
		assert.Equal(t, "regional handoff", assignment.Reason)
	}

	_, err = AssignCustomers([]int{company.Id, 999999}, sales.Id, "invalid batch", entryActor)
	require.Error(t, err)
	var activeCount int64
	require.NoError(t, DB.Model(&CustomerAssignment{}).Where("company_id = ? AND active = ?", company.Id, true).Count(&activeCount).Error)
	assert.Equal(t, int64(1), activeCount)

	_, err = AssignCustomers([]int{company.Id, company.Id}, sales.Id, "duplicate batch", entryActor)
	require.ErrorContains(t, err, "must not repeat")
}

func TestOpeningBalanceSnapshotIsIdempotent(t *testing.T) {
	entryActor, _, customer, company, _ := setupBusinessFixture(t, 321)
	first, err := InitializeBusinessOpeningBalance(company.Id, "Historical balance cutover", entryActor)
	require.NoError(t, err)
	assert.Equal(t, 0, first.Amount)
	assert.Equal(t, customer.Quota, first.BalanceBefore)
	assert.Equal(t, customer.Quota, first.BalanceAfter)
	assert.True(t, first.BalanceSnapshotAvailable)
	assert.Equal(t, LedgerEntryOpening, first.EntryType)

	second, err := InitializeBusinessOpeningBalance(company.Id, "Retry migration", entryActor)
	require.NoError(t, err)
	assert.Equal(t, first.Id, second.Id)
	var count int64
	require.NoError(t, DB.Model(&BalanceLedger{}).Where("reference_type = ? AND reference_id = ?", "opening_balance", company.Id).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestBalanceLedgerIsAppendOnlyAndReversedOnlyOnce(t *testing.T) {
	entryActor, approverActor, customer, company, project := setupBusinessFixture(t, 100)
	request := &ManualCreditRequest{
		CompanyId:         company.Id,
		UserId:            customer.Id,
		ProjectId:         project.Id,
		EntryType:         LedgerEntryManualCredit,
		Amount:            50,
		ExternalReference: "bank-20260803-reversal",
		Reason:            "Bank transfer received",
		Note:              "Original finance record",
		CreatedBy:         entryActor.UserId,
	}
	require.NoError(t, CreateManualCreditRequest(request, entryActor))
	ledger, err := ApproveManualCreditRequest(request.Id, approverActor)
	require.NoError(t, err)

	err = DB.Model(&BalanceLedger{}).Where("id = ?", ledger.Id).Update("reason", "tampered").Error
	require.Error(t, err)
	err = DB.Delete(&BalanceLedger{}, ledger.Id).Error
	require.Error(t, err)

	var stored BalanceLedger
	require.NoError(t, DB.First(&stored, ledger.Id).Error)
	assert.Equal(t, "Bank transfer received", stored.Reason)
	assert.Equal(t, int64(1), countBusinessRows(t, &BalanceLedger{}))

	reversal, err := CreateLedgerReversalRequest(ledger.Id, "Correct the entry", entryActor)
	require.NoError(t, err)
	assert.Equal(t, ledger.Id, reversal.ReversesLedgerId)
	_, err = CreateLedgerReversalRequest(ledger.Id, "Try duplicate reversal", entryActor)
	require.ErrorIs(t, err, ErrLedgerAlreadyReversed)

	reversalLedger, err := ApproveManualCreditRequest(reversal.Id, approverActor)
	require.NoError(t, err)
	assert.Equal(t, -50, reversalLedger.Amount)
	assert.Equal(t, ledger.Id, reversalLedger.ReversesLedgerId)
	assert.Equal(t, 100, reversalLedger.BalanceAfter)
}

func countBusinessRows(t *testing.T, value interface{}) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(value).Count(&count).Error)
	return count
}

func TestCompanyAndProjectRateLimitsPersistAfterUpdate(t *testing.T) {
	entryActor, _, _, company, project := setupBusinessFixture(t, 0)

	companyUpdate := *company
	companyUpdate.RateLimitRPM = 75
	companyUpdate.RateLimitTPM = 90000
	companyUpdate.MaxConcurrentRequests = 8
	require.NoError(t, UpdateCompany(&companyUpdate, entryActor))

	var persistedCompany Company
	require.NoError(t, DB.First(&persistedCompany, company.Id).Error)
	assert.Equal(t, 75, persistedCompany.RateLimitRPM)
	assert.Equal(t, int64(90000), persistedCompany.RateLimitTPM)
	assert.Equal(t, 8, persistedCompany.MaxConcurrentRequests)

	projectUpdate := *project
	projectUpdate.RateLimitRPM = 50
	projectUpdate.RateLimitTPM = 70000
	projectUpdate.MaxConcurrentRequests = 6
	require.NoError(t, UpdateBusinessProject(&projectUpdate, "Raise rate limits", entryActor))

	var persistedProject BusinessProject
	require.NoError(t, DB.First(&persistedProject, project.Id).Error)
	assert.Equal(t, 50, persistedProject.RateLimitRPM)
	assert.Equal(t, int64(70000), persistedProject.RateLimitTPM)
	assert.Equal(t, 6, persistedProject.MaxConcurrentRequests)
}
