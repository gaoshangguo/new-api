package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRecordConsumeLogCreatesIdempotentProjectConsumptionAssociation(t *testing.T) {
	entryActor, _, customer, _, project := setupBusinessFixture(t, 1000)
	require.NoError(t, DB.AutoMigrate(&BusinessConsumption{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&BusinessConsumption{}).Error)

	token := &Token{
		UserId: customer.Id,
		Name:   "business-consumption-key",
		Key:    "business-consumption-key-0001",
		Status: common.TokenStatusEnabled,
	}
	require.NoError(t, DB.Create(token).Error)
	require.NoError(t, SetBusinessProjectToken(project.Id, token.Id, "Associate consumption key", entryActor))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(common.RequestIdKey, "business-consumption-request")
	ctx.Set("username", customer.Username)
	params := RecordConsumeLogParams{
		ChannelId: 7,
		ModelName: "gpt-test",
		Quota:     120,
		TokenId:   token.Id,
		TokenName: token.Name,
		Group:     "default",
		Other:     map[string]interface{}{},
	}

	RecordConsumeLog(ctx, customer.Id, params)
	// The same request may be replayed by a logging/retry path. The project
	// projection must stay append-only and never double-count it.
	params.Quota = 999
	RecordConsumeLog(ctx, customer.Id, params)

	var records []BusinessConsumption
	require.NoError(t, DB.Where("request_id = ? AND token_id = ?", "business-consumption-request", token.Id).Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, project.Id, records[0].ProjectId)
	assert.Equal(t, customer.Id, records[0].UserId)
	assert.Equal(t, 120, records[0].Quota)
	assert.Equal(t, 7, records[0].ChannelId)
	assert.Equal(t, "gpt-test", records[0].ModelName)

	require.ErrorIs(t, DB.Model(&BusinessConsumption{}).Where("id = ?", records[0].Id).Update("quota", 121).Error, ErrBusinessConsumptionImmutable)
	require.ErrorIs(t, DB.Delete(&BusinessConsumption{}, records[0].Id).Error, ErrBusinessConsumptionImmutable)
	var persisted BusinessConsumption
	require.NoError(t, DB.First(&persisted, records[0].Id).Error)
	assert.Equal(t, 120, persisted.Quota)
}

func TestBusinessConsumptionIgnoresPersonalOrMismatchedTokens(t *testing.T) {
	_, _, customer, _, _ := setupBusinessFixture(t, 1000)
	require.NoError(t, DB.AutoMigrate(&BusinessConsumption{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&BusinessConsumption{}).Error)

	personalToken := &Token{
		UserId: customer.Id,
		Name:   "personal-key",
		Key:    "personal-key-0001",
		Status: common.TokenStatusEnabled,
	}
	require.NoError(t, DB.Create(personalToken).Error)

	require.NoError(t, RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "personal-token-request",
		UserId:    customer.Id,
		TokenId:   personalToken.Id,
		Quota:     1,
	}))
	require.NoError(t, RecordBusinessConsumption(RecordBusinessConsumptionParams{
		RequestId: "wrong-user-request",
		UserId:    customer.Id + 1,
		TokenId:   personalToken.Id,
		Quota:     1,
	}))

	var count int64
	require.NoError(t, DB.Model(&BusinessConsumption{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestExportBusinessConsumptionsHonorsScopeAndFilters(t *testing.T) {
	_, _, customer, company, project := setupBusinessFixture(t, 0)
	require.NoError(t, DB.AutoMigrate(&BusinessConsumption{}))
	matching := &BusinessConsumption{
		CompanyId: company.Id,
		ProjectId: project.Id,
		UserId:    customer.Id,
		TokenId:   101,
		RequestId: "consumption-export-match",
		Quota:     10,
		ModelName: "gpt-test",
		CreatedAt: 100,
	}
	nonMatching := &BusinessConsumption{
		CompanyId: company.Id,
		ProjectId: project.Id,
		UserId:    customer.Id,
		TokenId:   102,
		RequestId: "consumption-export-other",
		Quota:     10,
		ModelName: "claude-test",
		CreatedAt: 200,
	}
	require.NoError(t, DB.Create(matching).Error)
	require.NoError(t, DB.Create(nonMatching).Error)

	records, err := ExportBusinessConsumptions(BusinessConsumptionFilter{
		CompanyId: company.Id,
		UserId:    customer.Id,
		ProjectId: project.Id,
		TokenId:   matching.TokenId,
		ModelName: matching.ModelName,
		StartAt:   50,
		EndAt:     150,
	}, 100)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, matching.Id, records[0].Id)
}
