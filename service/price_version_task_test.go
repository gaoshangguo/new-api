package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPriceVersionApplyTaskTypeAndInterval(t *testing.T) {
	handler := priceVersionApplyTaskHandler{}
	assert.Equal(t, model.SystemTaskTypePriceVersionApply, handler.Type())
	assert.True(t, handler.Enabled())
	assert.GreaterOrEqual(t, handler.Interval(), 30*time.Second)
	assert.LessOrEqual(t, handler.Interval(), 24*time.Hour)
}

func TestPriceVersionApplyTaskAppliesDueVersions(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.PriceVersion{}, &model.BusinessAuditEvent{}, &model.Option{}))
	require.NoError(t, model.DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&model.PriceVersion{}).Error)
	require.NoError(t, model.DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&model.BusinessAuditEvent{}).Error)
	require.NoError(t, model.DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&model.Option{}).Error)
	model.InitOptionMap()
	model.RefreshCurrentPriceVersionId()

	pending, err := model.CreatePriceVersion(model.PriceVersionCreateParams{
		Name:        "scheduled",
		EffectiveAt: common.GetTimestamp() + 3600,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.PriceVersion{}).Where("id = ?", pending.Id).
		UpdateColumn("effective_at", common.GetTimestamp()-1).Error)

	task, created, err := EnqueueSystemTask(model.SystemTaskTypePriceVersionApply, nil)
	require.NoError(t, err)
	require.True(t, created)

	claimed, claimedOK, err := model.ClaimSystemTask(task.ID, model.SystemTaskTypePriceVersionApply, "test-runner", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, claimedOK)

	handler := priceVersionApplyTaskHandler{}
	handler.Run(context.Background(), claimed, "test-runner")

	var finished model.SystemTask
	require.NoError(t, model.DB.First(&finished, "task_id = ?", task.TaskID).Error)
	assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
	assert.Contains(t, finished.Result, `"applied_count":1`)

	var reloaded model.PriceVersion
	require.NoError(t, model.DB.First(&reloaded, pending.Id).Error)
	assert.Equal(t, "active", reloaded.Status)
}
