package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPriceVersionTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&PriceVersion{}, &BusinessAuditEvent{}, &Option{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&PriceVersion{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&BusinessAuditEvent{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&Option{}).Error)
	ratio_setting.InitRatioSettings()
	InitOptionMap()
	priceVersionCacheId = 0
	priceVersionCacheRefresh = time.Time{}
}

func priceVersionTestActor() BusinessActor {
	return BusinessActor{UserId: 42, Username: "root", RoleSnapshot: "system:1", IP: "127.0.0.1"}
}

func priceVersionTestActorPtr() *BusinessActor {
	actor := priceVersionTestActor()
	return &actor
}

func TestEnsureInitialPriceVersionSeedsActiveSnapshot(t *testing.T) {
	setupPriceVersionTest(t)

	require.NoError(t, EnsureInitialPriceVersion())
	var version PriceVersion
	require.NoError(t, DB.First(&version).Error)
	assert.Equal(t, priceVersionStatusActive, version.Status)
	assert.LessOrEqual(t, version.EffectiveAt, common.GetTimestamp())
	assert.NotEmpty(t, version.ModelRatio)

	// Idempotent: a second call must not create another row.
	require.NoError(t, EnsureInitialPriceVersion())
	var count int64
	require.NoError(t, DB.Model(&PriceVersion{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestGetCurrentPriceVersionIdPrefersLatestActive(t *testing.T) {
	setupPriceVersionTest(t)
	require.NoError(t, EnsureInitialPriceVersion())

	first, err := CreatePriceVersion(PriceVersionCreateParams{
		Name: "second", EffectiveAt: common.GetTimestamp(), CreatedBy: 1, Actor: priceVersionTestActorPtr(),
	})
	require.NoError(t, err)
	assert.Equal(t, priceVersionStatusActive, first.Status)

	current := GetCurrentPriceVersionId()
	assert.Equal(t, first.Id, current)

	// A pending version must not be picked until it is applied.
	pending, err := CreatePriceVersion(PriceVersionCreateParams{
		Name: "future", EffectiveAt: common.GetTimestamp() + 3600, CreatedBy: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, priceVersionStatusPending, pending.Status)
	assert.Equal(t, first.Id, GetCurrentPriceVersionId())
}

func TestCreatePriceVersionImmediateAppliesOverlayAndAudits(t *testing.T) {
	setupPriceVersionTest(t)
	require.NoError(t, EnsureInitialPriceVersion())

	overlay := map[string]string{"ModelRatio": `{"gpt-version-test": 2.5}`}
	version, err := CreatePriceVersion(PriceVersionCreateParams{
		Name: "overlay change", EffectiveAt: common.GetTimestamp(),
		CreatedBy: 42, Reason: "test", Overlay: overlay, Actor: priceVersionTestActorPtr(),
	})
	require.NoError(t, err)
	assert.Equal(t, priceVersionStatusActive, version.Status)

	ratio, ok := ratio_setting.GetModelRatioCopy()["gpt-version-test"]
	require.True(t, ok)
	assert.Equal(t, 2.5, ratio)

	var persisted Option
	require.NoError(t, DB.Where("key = ?", "ModelRatio").First(&persisted).Error)
	assert.Contains(t, persisted.Value, "gpt-version-test")

	var audit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource = ? AND resource_id = ?",
		"price_version.create", "price_version", version.Id).First(&audit).Error)
	assert.Equal(t, "root", audit.ActorName)
	assert.Contains(t, audit.AfterValue, `"changed_option_keys":["ModelRatio"]`)

	var persistedVersion PriceVersion
	require.NoError(t, DB.First(&persistedVersion, version.Id).Error)
	assert.Contains(t, persistedVersion.ModelRatio, "gpt-version-test")
}

func TestCreatePriceVersionPendingAppliedWhenDue(t *testing.T) {
	setupPriceVersionTest(t)
	require.NoError(t, EnsureInitialPriceVersion())

	pending, err := CreatePriceVersion(PriceVersionCreateParams{
		Name: "scheduled", EffectiveAt: common.GetTimestamp() + 3600,
		Overlay: map[string]string{"ModelRatio": `{"gpt-scheduled-test": 3.5}`},
	})
	require.NoError(t, err)
	assert.Equal(t, priceVersionStatusPending, pending.Status)
	_, inMap := ratio_setting.GetModelRatioCopy()["gpt-scheduled-test"]
	assert.False(t, inMap, "a pending version must not change live prices")

	// Simulate the effective time arriving (system transition, bypass hooks).
	require.NoError(t, DB.Model(&PriceVersion{}).Where("id = ?", pending.Id).
		UpdateColumn("effective_at", common.GetTimestamp()-1).Error)

	result, err := ApplyPendingPriceVersions()
	require.NoError(t, err)
	assert.Equal(t, 1, result.AppliedCount)
	assert.Equal(t, pending.Id, result.CurrentVersionId)

	var reloaded PriceVersion
	require.NoError(t, DB.First(&reloaded, pending.Id).Error)
	assert.Equal(t, priceVersionStatusActive, reloaded.Status)
	assert.NotZero(t, reloaded.AppliedAt)
	ratio, ok := ratio_setting.GetModelRatioCopy()["gpt-scheduled-test"]
	require.True(t, ok)
	assert.Equal(t, 3.5, ratio)

	// A second pass is a no-op.
	again, err := ApplyPendingPriceVersions()
	require.NoError(t, err)
	assert.Equal(t, 0, again.AppliedCount)
}

func TestApplyPriceVersionNowAuditsManualApply(t *testing.T) {
	setupPriceVersionTest(t)
	require.NoError(t, EnsureInitialPriceVersion())

	pending, err := CreatePriceVersion(PriceVersionCreateParams{
		Name: "manual apply", EffectiveAt: common.GetTimestamp() + 3600, CreatedBy: 42,
	})
	require.NoError(t, err)

	err = ApplyPriceVersionNow(pending.Id, priceVersionTestActorPtr(), "finance request")
	require.NoError(t, err)

	var reloaded PriceVersion
	require.NoError(t, DB.First(&reloaded, pending.Id).Error)
	assert.Equal(t, priceVersionStatusActive, reloaded.Status)

	var audit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource_id = ?", "price_version.apply", pending.Id).First(&audit).Error)
	assert.Equal(t, "finance request", audit.Reason)

	// Applying twice is idempotent and must not double-audit.
	require.NoError(t, ApplyPriceVersionNow(pending.Id, priceVersionTestActorPtr(), "again"))
	var count int64
	require.NoError(t, DB.Model(&BusinessAuditEvent{}).Where("action = ? AND resource_id = ?",
		"price_version.apply", pending.Id).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestForceAppliedFutureVersionBecomesCurrent(t *testing.T) {
	setupPriceVersionTest(t)
	require.NoError(t, EnsureInitialPriceVersion())
	first := GetCurrentPriceVersionId()

	// A future-dated pending version force-applied now makes its snapshot live
	// immediately; the version binding must follow the actual apply, otherwise
	// requests would be charged with the new prices while still bound to the
	// previous version.
	pending, err := CreatePriceVersion(PriceVersionCreateParams{
		Name: "future forced", EffectiveAt: common.GetTimestamp() + 7200, CreatedBy: 42,
	})
	require.NoError(t, err)
	assert.NotEqual(t, first, pending.Id)
	assert.Equal(t, first, GetCurrentPriceVersionId())

	require.NoError(t, ApplyPriceVersionNow(pending.Id, priceVersionTestActorPtr(), "force"))
	assert.Equal(t, pending.Id, GetCurrentPriceVersionId())
}

func TestPriceVersionSnapshotIsImmutable(t *testing.T) {
	setupPriceVersionTest(t)
	require.NoError(t, EnsureInitialPriceVersion())

	var version PriceVersion
	require.NoError(t, DB.First(&version).Error)

	// Snapshot content rewrite must be rejected.
	require.ErrorIs(t, DB.Model(&PriceVersion{}).Where("id = ?", version.Id).
		Update("model_ratio", `{"hacked": 1}`).Error, ErrPriceVersionImmutable)
	// Delete must be rejected.
	require.ErrorIs(t, DB.Delete(&PriceVersion{}, version.Id).Error, ErrPriceVersionImmutable)
}

func TestCreatePriceVersionRejectsInvalidOverlay(t *testing.T) {
	setupPriceVersionTest(t)

	cases := []struct {
		name  string
		key   string
		value string
	}{
		{"unknown key", "QuotaForNewUser", `{"x":1}`},
		{"malformed json", "ModelRatio", `{broken`},
		{"negative ratio", "ModelRatio", `{"gpt-neg": -1}`},
		{"negative group ratio", "GroupGroupRatio", `{"vip":{"default":-0.5}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CreatePriceVersion(PriceVersionCreateParams{
				EffectiveAt: common.GetTimestamp(),
				Overlay:     map[string]string{tc.key: tc.value},
				Actor:       priceVersionTestActorPtr(),
			})
			require.Error(t, err)
		})
	}
}

func TestCreatePriceVersionFromLiveChangeScopes(t *testing.T) {	setupPriceVersionTest(t)

	version, err := CreatePriceVersionFromLiveChange("QuotaForNewUser", priceVersionTestActorPtr(), "nope")
	require.NoError(t, err)
	assert.Nil(t, version, "non-price keys must not create a version")

	_, err = CreatePriceVersionFromLiveChange("ModelRatio", nil, "no actor")
	require.Error(t, err)
}
