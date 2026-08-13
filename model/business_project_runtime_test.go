package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createBusinessProjectRuntimeToken(t *testing.T, project *BusinessProject, customer *User, actor BusinessActor) *Token {
	t.Helper()
	token := &Token{
		UserId:      customer.Id,
		Name:        "project-runtime-key",
		Key:         "project-runtime-key-0001",
		Status:      common.TokenStatusEnabled,
		RemainQuota: common.MaxQuota,
	}
	require.NoError(t, DB.Create(token).Error)
	require.NoError(t, SetBusinessProjectToken(project.Id, token.Id, "bind runtime policy test key", actor))
	return token
}

func TestBusinessProjectRuntimePolicyFailsClosedAndUsesFreshBinding(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, common.MaxQuota)
	token := createBusinessProjectRuntimeToken(t, project, customer, entryActor)

	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Updates(map[string]any{
		"model_limits":   `{"models":["gpt-4o","gpt-4o-gizmo-*"]}`,
		"channel_limits": `{"channels":[7,9]}`,
		"budget_quota":   100,
	}).Error)

	policy, err := LoadBusinessProjectRuntimePolicyForToken(token.Id, customer.Id)
	require.NoError(t, err)
	require.NotNil(t, policy)
	assert.Equal(t, project.Id, policy.ProjectId)
	assert.True(t, policy.HasModelLimits())
	assert.True(t, policy.AllowsModel("gpt-4o"))
	assert.False(t, policy.AllowsModel("gpt-4.1"))
	assert.True(t, policy.HasChannelLimits())
	assert.True(t, policy.AllowsChannel(7))
	assert.False(t, policy.AllowsChannel(8))

	// A malformed non-empty configuration cannot accidentally permit every
	// model. The runtime check deliberately rejects it before relay routing.
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("model_limits", `{"models":"gpt-4o"}`).Error)
	_, err = LoadBusinessProjectRuntimePolicyForToken(token.Id, customer.Id)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBusinessProjectLimitConfiguration))

	// Token.ProjectId may be stale in Redis, so the authoritative DB binding is
	// queried. Disabling the project takes effect immediately.
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Updates(map[string]any{
		"model_limits": `[]`,
		"status":       BusinessProjectStatusDisabled,
	}).Error)
	_, err = LoadBusinessProjectRuntimePolicyForToken(token.Id, customer.Id)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBusinessProjectDisabled))
}

func TestBusinessProjectBudgetReservationPreventsConcurrentOverspendAndSettles(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, common.MaxQuota)
	token := createBusinessProjectRuntimeToken(t, project, customer, entryActor)
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("budget_quota", 100).Error)

	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-request-1", 60))
	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-request-2", 40))
	err := ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-request-3", 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBusinessProjectBudgetExceeded))

	// Recording the final 50 quota and settling the first hold are atomic. The
	// project then has 50 settled + 40 held, leaving exactly 10 available.
	require.NoError(t, RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "budget-request-1",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     50,
		ChannelId: 7,
		ModelName: "gpt-4o",
	}))
	var firstHold BusinessProjectBudgetReservation
	require.NoError(t, DB.Where("project_id = ? AND request_id = ?", project.Id, "budget-request-1").First(&firstHold).Error)
	assert.Equal(t, projectBudgetReservationStatusSettled, firstHold.Status)
	var consumption BusinessConsumption
	require.NoError(t, DB.Where("request_id = ? AND token_id = ?", "budget-request-1", token.Id).First(&consumption).Error)
	var consumptionLedger BalanceLedger
	require.NoError(t, DB.Where("reference_type = ? AND reference_id = ?", "business_consumption", consumption.Id).First(&consumptionLedger).Error)
	assert.Equal(t, LedgerEntryConsumption, consumptionLedger.EntryType)
	assert.Zero(t, consumptionLedger.Amount)
	assert.Equal(t, 50, consumptionLedger.UsageQuota)
	assert.False(t, consumptionLedger.BalanceSnapshotAvailable)
	assert.Equal(t, "budget-request-1", consumptionLedger.RequestId)

	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-request-4", 10))
	err = ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-request-5", 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBusinessProjectBudgetExceeded))

	// A failed request releases its reservation and therefore cannot consume a
	// capped project's budget forever.
	require.NoError(t, ReleaseBusinessProjectBudgetReservation(token.Id, customer.Id, "budget-request-2"))
	require.NoError(t, ReleaseBusinessProjectBudgetReservation(token.Id, customer.Id, "budget-request-4"))
	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-request-6", 50))

	policy, err := LoadBusinessProjectRuntimePolicyForToken(token.Id, customer.Id)
	require.NoError(t, err)
	require.ErrorIs(t, CheckBusinessProjectBudgetAtAuthentication(policy), ErrBusinessProjectBudgetExceeded)

	err = ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-zero-estimate", 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBusinessProjectBudgetEstimate))
}

func TestBusinessProjectBudgetReservationExpansionRollsBackOnlyItsDelta(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, common.MaxQuota)
	token := createBusinessProjectRuntimeToken(t, project, customer, entryActor)
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("budget_quota", 100).Error)

	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-expand-request", 30))
	require.NoError(t, ExpandBusinessProjectBudgetReservation(token.Id, customer.Id, "budget-expand-request", 50))
	err := ExpandBusinessProjectBudgetReservation(token.Id, customer.Id, "budget-expand-request", 21)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBusinessProjectBudgetExceeded))

	require.NoError(t, RollbackBusinessProjectBudgetReservationExpansion(token.Id, customer.Id, "budget-expand-request", 50))
	var reservation BusinessProjectBudgetReservation
	require.NoError(t, DB.Where("project_id = ? AND request_id = ?", project.Id, "budget-expand-request").First(&reservation).Error)
	assert.Equal(t, 30, reservation.ReservedQuota)
	assert.Equal(t, projectBudgetReservationStatusReserved, reservation.Status)
}

func TestBusinessProjectBudgetOverageIsTerminalAndBlocksFutureRequests(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, common.MaxQuota)
	token := createBusinessProjectRuntimeToken(t, project, customer, entryActor)
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("budget_quota", 100).Error)
	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-overage-request", 60))

	// Some relay paths historically log after a post-settlement error. The
	// actual use is still written once, but it must become an explicit
	// reconciliation terminal state rather than a normal settled reservation.
	err := RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "budget-overage-request",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     90,
	})
	require.ErrorIs(t, err, ErrBusinessProjectBudgetExceeded)

	var consumption BusinessConsumption
	require.NoError(t, DB.Where("request_id = ? AND token_id = ?", "budget-overage-request", token.Id).First(&consumption).Error)
	assert.Equal(t, 90, consumption.Quota)
	var reservation BusinessProjectBudgetReservation
	require.NoError(t, DB.Where("project_id = ? AND request_id = ?", project.Id, "budget-overage-request").First(&reservation).Error)
	assert.Equal(t, projectBudgetReservationStatusOverBudgetPendingReconcile, reservation.Status)

	policy, err := LoadBusinessProjectRuntimePolicyForToken(token.Id, customer.Id)
	require.NoError(t, err)
	require.ErrorIs(t, CheckBusinessProjectBudgetAtAuthentication(policy), ErrBusinessProjectBudgetExceeded)
	require.ErrorIs(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "budget-overage-follow-up", 1), ErrBusinessProjectBudgetExceeded)

	// The terminal state is intentionally not permanent: an authorized caller
	// supplies a documented reconciliation reason, which changes only the
	// terminal status and writes an immutable audit event.
	require.NoError(t, ReconcileBusinessProjectBudgetReservation(reservation.Id, "confirmed upstream completion and financial settlement", entryActor))
	require.NoError(t, CheckBusinessProjectBudgetAtAuthentication(policy))
	var audit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource_id = ?", "project_budget.reconciliation.resolve", reservation.Id).First(&audit).Error)
	assert.Equal(t, entryActor.UserId, audit.ActorUserId)
}

func TestBusinessProjectConsumptionSettlesReservationAfterTokenSoftDelete(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, common.MaxQuota)
	token := createBusinessProjectRuntimeToken(t, project, customer, entryActor)
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("budget_quota", 100).Error)
	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "soft-deleted-token-request", 60))
	require.NoError(t, DB.Delete(&Token{}, token.Id).Error)

	require.NoError(t, RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "soft-deleted-token-request",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     50,
	}))
	var reservation BusinessProjectBudgetReservation
	require.NoError(t, DB.Unscoped().Where("project_id = ? AND request_id = ?", project.Id, "soft-deleted-token-request").First(&reservation).Error)
	assert.Equal(t, projectBudgetReservationStatusSettled, reservation.Status)
	var count int64
	require.NoError(t, DB.Where("request_id = ? AND token_id = ?", "soft-deleted-token-request", token.Id).Model(&BusinessConsumption{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestBusinessProjectZeroQuotaWithoutHoldDoesNotRequireReconciliation(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, common.MaxQuota)
	token := createBusinessProjectRuntimeToken(t, project, customer, entryActor)
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("budget_quota", 100).Error)

	// A timeout/error log can legitimately have zero final usage and no
	// pre-consume hold (for example a direct realtime session). It has no
	// budget effect, so it must not manufacture a terminal reconciliation row.
	require.NoError(t, RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "zero-without-hold",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     0,
	}))
	var consumptionCount int64
	require.NoError(t, DB.Model(&BusinessConsumption{}).
		Where("request_id = ? AND token_id = ?", "zero-without-hold", token.Id).
		Count(&consumptionCount).Error)
	assert.Zero(t, consumptionCount)
	var reservationCount int64
	require.NoError(t, DB.Model(&BusinessProjectBudgetReservation{}).
		Where("project_id = ? AND request_id = ?", project.Id, "zero-without-hold").
		Count(&reservationCount).Error)
	assert.Zero(t, reservationCount)
	policy, err := LoadBusinessProjectRuntimePolicyForToken(token.Id, customer.Id)
	require.NoError(t, err)
	require.NoError(t, CheckBusinessProjectBudgetAtAuthentication(policy))

	// A real pre-consume hold must still be settled by a zero final amount so
	// the hold is released and cannot consume the budget indefinitely.
	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "zero-with-hold", 60))
	require.NoError(t, RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "zero-with-hold",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     0,
	}))
	var reservation BusinessProjectBudgetReservation
	require.NoError(t, DB.Where("project_id = ? AND request_id = ?", project.Id, "zero-with-hold").First(&reservation).Error)
	assert.Equal(t, projectBudgetReservationStatusSettled, reservation.Status)
	used, reserved, reconciliationPending, err := businessProjectBudgetUsage(DB, project.Id)
	require.NoError(t, err)
	assert.Zero(t, used)
	assert.Zero(t, reserved)
	assert.False(t, reconciliationPending)
}

func TestBusinessProjectSignedAdjustmentIsAppendOnlyAndPositiveDeltaRequiresReconciliation(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, common.MaxQuota)
	token := createBusinessProjectRuntimeToken(t, project, customer, entryActor)
	require.NoError(t, DB.Model(&BusinessProject{}).Where("id = ?", project.Id).Update("budget_quota", 100).Error)
	require.NoError(t, ReserveBusinessProjectBudget(token.Id, customer.Id, "task-base-request", 60))
	require.NoError(t, RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "task-base-request",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     60,
	}))

	// A server-derived refund delta is append-only and reduces the project
	// usage projection without mutating the original consumption record.
	require.NoError(t, RecordBusinessConsumptionAdjustment(RecordBusinessConsumptionAdjustmentParams{
		RequestId: "task-adjust-refund",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     -20,
	}))
	var refundConsumption BusinessConsumption
	require.NoError(t, DB.Where("request_id = ? AND token_id = ?", "task-adjust-refund", token.Id).First(&refundConsumption).Error)
	var refundLedger BalanceLedger
	require.NoError(t, DB.Where("reference_type = ? AND reference_id = ?", "business_consumption", refundConsumption.Id).First(&refundLedger).Error)
	assert.Equal(t, LedgerEntryConsumptionRefund, refundLedger.EntryType)
	assert.Zero(t, refundLedger.Amount)
	assert.Equal(t, -20, refundLedger.UsageQuota)
	assert.False(t, refundLedger.BalanceSnapshotAvailable)
	used, reserved, reconciliationPending, err := businessProjectBudgetUsage(DB, project.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(40), used)
	assert.Zero(t, reserved)
	assert.False(t, reconciliationPending)

	// A positive async adjustment was not bounded before the upstream task
	// completed. It is recorded, marked for reconciliation, and blocks all
	// future capped-project requests rather than silently becoming normal use.
	err = RecordBusinessConsumptionAdjustment(RecordBusinessConsumptionAdjustmentParams{
		RequestId: "task-adjust-overage",
		UserId:    customer.Id,
		TokenId:   token.Id,
		Quota:     10,
	})
	require.ErrorIs(t, err, ErrBusinessProjectBudgetExceeded)
	used, reserved, reconciliationPending, err = businessProjectBudgetUsage(DB, project.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(50), used)
	assert.Zero(t, reserved)
	assert.True(t, reconciliationPending)
}
