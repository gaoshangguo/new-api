package service

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type billingProjectBudgetTestFunding struct {
	settleCalls int
	settleErr   error
}

func (funding *billingProjectBudgetTestFunding) Source() string {
	return BillingSourceWallet
}

func (funding *billingProjectBudgetTestFunding) PreConsume(int) error {
	return nil
}

func (funding *billingProjectBudgetTestFunding) Settle(int) error {
	funding.settleCalls++
	return funding.settleErr
}

func (funding *billingProjectBudgetTestFunding) Refund() error {
	return nil
}

func setupBillingProjectBudgetSession(t *testing.T, budgetQuota int, reservedQuota int) (*BillingSession, *model.BusinessProject, string, *billingProjectBudgetTestFunding) {
	t.Helper()
	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Company{},
		&model.BusinessProject{},
		&model.Token{},
		&model.BusinessConsumption{},
		&model.BusinessProjectBudgetReservation{},
		&model.BalanceLedger{},
	))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
	})

	const userID = 42
	require.NoError(t, db.Create(&model.User{
		Id:       userID,
		Username: "billing-project-user",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Quota:    common.MaxQuota,
		AffCode:  "billing-project-aff",
	}).Error)
	company := &model.Company{Name: "Billing project company", OwnerUserId: userID}
	require.NoError(t, db.Create(company).Error)
	project := &model.BusinessProject{
		CompanyId:   company.Id,
		Name:        "Billing project",
		OwnerUserId: userID,
		BudgetQuota: budgetQuota,
		Status:      model.BusinessProjectStatusEnabled,
	}
	require.NoError(t, db.Create(project).Error)
	token := &model.Token{
		UserId:      userID,
		ProjectId:   project.Id,
		Name:        "billing-project-key",
		Key:         "billing-project-key-0001",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: common.MaxQuota,
	}
	require.NoError(t, db.Create(token).Error)
	const requestID = "billing-project-settle"
	require.NoError(t, db.Create(&model.BusinessProjectBudgetReservation{
		ProjectId:     project.Id,
		RequestId:     requestID,
		UserId:        userID,
		TokenId:       token.Id,
		ReservedQuota: reservedQuota,
		Status:        "reserved",
	}).Error)

	funding := &billingProjectBudgetTestFunding{}
	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{
			UserId:       userID,
			TokenId:      token.Id,
			TokenKey:     token.Key,
			RequestId:    requestID,
			IsPlayground: true,
		},
		funding:          funding,
		preConsumedQuota: reservedQuota,
	}
	return session, project, requestID, funding
}

func TestBillingSessionSettleExpandsProjectBudgetReservationBeforeFunding(t *testing.T) {
	session, project, requestID, funding := setupBillingProjectBudgetSession(t, 100, 60)

	require.NoError(t, session.Settle(90))
	assert.Equal(t, 1, funding.settleCalls)
	assert.True(t, session.settled)

	var reservation model.BusinessProjectBudgetReservation
	require.NoError(t, model.DB.Where("project_id = ? AND request_id = ?", project.Id, requestID).First(&reservation).Error)
	assert.Equal(t, 90, reservation.ReservedQuota)
}

func TestBillingSessionSettleRejectsBudgetOverrunAndRollsBackOnFundingFailure(t *testing.T) {
	t.Run("over budget prevents funding settlement", func(t *testing.T) {
		session, project, requestID, funding := setupBillingProjectBudgetSession(t, 80, 60)

		err := session.Settle(90)
		require.ErrorIs(t, err, model.ErrBusinessProjectBudgetExceeded)
		assert.Equal(t, 0, funding.settleCalls)
		assert.False(t, session.settled)

		var reservation model.BusinessProjectBudgetReservation
		require.NoError(t, model.DB.Where("project_id = ? AND request_id = ?", project.Id, requestID).First(&reservation).Error)
		assert.Equal(t, 60, reservation.ReservedQuota)
	})

	t.Run("funding failure rolls back only settlement expansion", func(t *testing.T) {
		session, project, requestID, funding := setupBillingProjectBudgetSession(t, 100, 60)
		funding.settleErr = errors.New("funding settlement failed")

		err := session.Settle(90)
		require.ErrorIs(t, err, funding.settleErr)
		assert.Equal(t, 1, funding.settleCalls)
		assert.False(t, session.settled)

		var reservation model.BusinessProjectBudgetReservation
		require.NoError(t, model.DB.Where("project_id = ? AND request_id = ?", project.Id, requestID).First(&reservation).Error)
		assert.Equal(t, 60, reservation.ReservedQuota)
	})
}

func TestPostConsumeQuotaMarksCappedProjectDirectChargeForReconciliation(t *testing.T) {
	_, project, requestID, _ := setupBillingProjectBudgetSession(t, 100, 0)
	var token model.Token
	require.NoError(t, model.DB.Where("project_id = ?", project.Id).First(&token).Error)
	require.NoError(t, model.DB.Where("project_id = ? AND request_id = ?", project.Id, requestID).Delete(&model.BusinessProjectBudgetReservation{}).Error)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          token.UserId,
		TokenId:         token.Id,
		TokenKey:        token.Key,
		RequestId:       requestID,
		OriginModelName: "gpt-4o",
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 7},
	}
	require.NoError(t, PostConsumeQuota(relayInfo, 25, 0, false))

	var consumption model.BusinessConsumption
	require.NoError(t, model.DB.Where("request_id = ? AND token_id = ?", requestID, token.Id).First(&consumption).Error)
	assert.Equal(t, 25, consumption.Quota)
	var reservation model.BusinessProjectBudgetReservation
	require.NoError(t, model.DB.Where("project_id = ? AND request_id = ?", project.Id, requestID).First(&reservation).Error)
	assert.Equal(t, "over_budget_pending_reconciliation", reservation.Status)

	policy, err := model.LoadBusinessProjectRuntimePolicyForToken(token.Id, token.UserId)
	require.NoError(t, err)
	require.ErrorIs(t, model.CheckBusinessProjectBudgetAtAuthentication(policy), model.ErrBusinessProjectBudgetExceeded)
}

func TestRealtimePrechargesDeferProjectProjectionUntilFinalAggregate(t *testing.T) {
	_, project, requestID, _ := setupBillingProjectBudgetSession(t, 0, 0)
	var token model.Token
	require.NoError(t, model.DB.Where("project_id = ?", project.Id).First(&token).Error)
	require.NoError(t, model.DB.Where("project_id = ? AND request_id = ?", project.Id, requestID).
		Delete(&model.BusinessProjectBudgetReservation{}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		UserId:          token.UserId,
		TokenId:         token.Id,
		TokenKey:        token.Key,
		RequestId:       requestID,
		OriginModelName: "gpt-4o",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 7},
	}
	usage := &dto.RealtimeUsage{
		TotalTokens: 1,
		InputTokens: 1,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens: 1,
		},
	}

	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
	firstPrecharge := relayInfo.FinalPreConsumedQuota
	require.Positive(t, firstPrecharge)
	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
	assert.Equal(t, firstPrecharge*2, relayInfo.FinalPreConsumedQuota)

	var count int64
	require.NoError(t, model.DB.Model(&model.BusinessConsumption{}).
		Where("request_id = ? AND token_id = ?", requestID, token.Id).
		Count(&count).Error)
	assert.Zero(t, count, "incremental realtime precharges must not create an early project projection")

	// This mirrors the positive final settlement delta from PostWssConsumeQuota:
	// the existing total precharge plus the final delta is projected exactly once.
	finalDelta := 5
	require.NoError(t, PostConsumeQuota(relayInfo, finalDelta, relayInfo.FinalPreConsumedQuota, false))
	var consumption model.BusinessConsumption
	require.NoError(t, model.DB.Where("request_id = ? AND token_id = ?", requestID, token.Id).First(&consumption).Error)
	assert.Equal(t, relayInfo.FinalPreConsumedQuota+finalDelta, consumption.Quota)
}

func TestPostWssZeroQuotaWithoutReservationDoesNotBlockProject(t *testing.T) {
	_, project, requestID, _ := setupBillingProjectBudgetSession(t, 100, 0)
	var token model.Token
	require.NoError(t, model.DB.Where("project_id = ?", project.Id).First(&token).Error)
	require.NoError(t, model.DB.Where("project_id = ? AND request_id = ?", project.Id, requestID).
		Delete(&model.BusinessProjectBudgetReservation{}).Error)

	previousLogConsumeEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/v1/realtime", nil)
	ctx.Set(common.RequestIdKey, requestID)
	relayInfo := &relaycommon.RelayInfo{
		UserId:          token.UserId,
		TokenId:         token.Id,
		TokenKey:        token.Key,
		RequestId:       requestID,
		OriginModelName: "gpt-4o-realtime-preview",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 7},
	}

	PostWssConsumeQuota(ctx, relayInfo, relayInfo.OriginModelName, &dto.RealtimeUsage{}, "")

	var consumptionCount int64
	require.NoError(t, model.DB.Model(&model.BusinessConsumption{}).
		Where("request_id = ? AND token_id = ?", requestID, token.Id).
		Count(&consumptionCount).Error)
	assert.Zero(t, consumptionCount)
	var reservationCount int64
	require.NoError(t, model.DB.Model(&model.BusinessProjectBudgetReservation{}).
		Where("project_id = ? AND request_id = ?", project.Id, requestID).
		Count(&reservationCount).Error)
	assert.Zero(t, reservationCount)
	policy, err := model.LoadBusinessProjectRuntimePolicyForToken(token.Id, token.UserId)
	require.NoError(t, err)
	require.NoError(t, model.CheckBusinessProjectBudgetAtAuthentication(policy))
}
