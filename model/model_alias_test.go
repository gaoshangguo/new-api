package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelAliasTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&ModelAlias{}, &BusinessAuditEvent{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&ModelAlias{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true, SkipHooks: true}).Delete(&BusinessAuditEvent{}).Error)
	require.NoError(t, LoadModelAliases())
}

func TestModelAliasResolvesActiveAlias(t *testing.T) {
	setupModelAliasTest(t)

	alias, err := CreateModelAlias(ModelAliasCreateParams{
		AliasName: "gpt-4o-fast",
		ModelName: "gpt-4o",
		Actor:     priceVersionTestActorPtr(),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, alias.Version)

	resolved, entry, isAlias, err := ResolveModelAlias("gpt-4o-fast")
	require.NoError(t, err)
	assert.True(t, isAlias)
	assert.Equal(t, "gpt-4o", resolved)
	assert.Equal(t, "gpt-4o-fast", entry.AliasName)

	// Non-alias names pass through untouched.
	resolved, _, isAlias, err = ResolveModelAlias("gpt-4o")
	require.NoError(t, err)
	assert.False(t, isAlias)
	assert.Equal(t, "gpt-4o", resolved)
}

func TestModelAliasDeprecatedFollowsReplacement(t *testing.T) {
	setupModelAliasTest(t)

	_, err := CreateModelAlias(ModelAliasCreateParams{
		AliasName:   "claude-3.5-sonnet-old",
		ModelName:   "claude-3-5-sonnet-20241022",
		Status:      ModelAliasStatusDeprecated,
		Replacement: "claude-sonnet-4",
	})
	require.NoError(t, err)

	resolved, entry, isAlias, err := ResolveModelAlias("claude-3.5-sonnet-old")
	require.NoError(t, err)
	assert.True(t, isAlias)
	assert.Equal(t, "claude-sonnet-4", resolved)
	assert.Equal(t, ModelAliasStatusDeprecated, entry.Status)

	// Replacement may itself be an alias (one level).
	_, err = CreateModelAlias(ModelAliasCreateParams{AliasName: "claude-sonnet-4", ModelName: "claude-sonnet-4-20250514"})
	require.NoError(t, err)
	resolved, _, isAlias, err = ResolveModelAlias("claude-3.5-sonnet-old")
	require.NoError(t, err)
	assert.True(t, isAlias)
	assert.Equal(t, "claude-sonnet-4-20250514", resolved)

}

func TestModelAliasDeprecatedWithoutReplacementRejected(t *testing.T) {
	setupModelAliasTest(t)

	_, err := CreateModelAlias(ModelAliasCreateParams{
		AliasName: "broken-alias",
		ModelName: "gpt-4o",
		Status:    ModelAliasStatusDeprecated,
	})
	require.Error(t, err, "a deprecated alias requires a replacement")

	// Create active, then force-deprecate without replacement via update.
	alias, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "sunset-alias", ModelName: "gpt-4o"})
	require.NoError(t, err)
	_, err = UpdateModelAlias(alias.Id, ModelAliasCreateParams{
		ModelName: "gpt-4o",
		Status:    ModelAliasStatusDeprecated,
	}, "sunset")
	require.Error(t, err)

	// Update with replacement succeeds; resolution follows it.
	_, err = UpdateModelAlias(alias.Id, ModelAliasCreateParams{
		ModelName:   "gpt-4o",
		Status:      ModelAliasStatusDeprecated,
		Replacement: "gpt-5",
	}, "sunset")
	resolved, _, isAlias, err := ResolveModelAlias("sunset-alias")
	require.NoError(t, err)
	assert.True(t, isAlias)
	assert.Equal(t, "gpt-5", resolved)
}

func TestUpdateModelAliasBumpsVersionAndAudits(t *testing.T) {
	setupModelAliasTest(t)

	alias, err := CreateModelAlias(ModelAliasCreateParams{
		AliasName: "versioned-alias",
		ModelName: "gpt-4o",
		Actor:     priceVersionTestActorPtr(),
	})
	require.NoError(t, err)

	updated, err := UpdateModelAlias(alias.Id, ModelAliasCreateParams{
		ModelName: "gpt-4.1",
		Actor:     priceVersionTestActorPtr(),
	}, "upgrade target")
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)

	resolved, _, isAlias, err := ResolveModelAlias("versioned-alias")
	require.NoError(t, err)
	assert.True(t, isAlias)
	assert.Equal(t, "gpt-4.1", resolved)

	var audit BusinessAuditEvent
	require.NoError(t, DB.Where("action = ? AND resource_id = ?", "model_alias.update", alias.Id).First(&audit).Error)
	assert.Contains(t, audit.BeforeValue, "gpt-4o")
	assert.Contains(t, audit.AfterValue, "gpt-4.1")
}

func TestDeleteModelAliasRestoresPassThrough(t *testing.T) {
	setupModelAliasTest(t)

	alias, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "delete-me", ModelName: "gpt-4o"})
	require.NoError(t, err)
	require.NoError(t, DeleteModelAlias(alias.Id, priceVersionTestActorPtr(), "cleanup"))

	resolved, _, isAlias, err := ResolveModelAlias("delete-me")
	require.NoError(t, err)
	assert.False(t, isAlias)
	assert.Equal(t, "delete-me", resolved)
}

func TestModelAliasChannelIDs(t *testing.T) {
	setupModelAliasTest(t)

	alias, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "region-pinned", ModelName: "gpt-4o", ChannelIds: "[3, 7]"})
	require.NoError(t, err)
	entry, ok := GetModelAliasEntry("region-pinned")
	require.True(t, ok)
	ids, err := entry.ModelAliasChannelIDs()
	require.NoError(t, err)
	assert.Equal(t, []int{3, 7}, ids)
	assert.Equal(t, alias.Id, entry.Id)

	// Empty channel restriction means no restriction.
	ids, err = entry.ModelAliasChannelIDs()
	require.NoError(t, err)
	assert.Equal(t, []int{3, 7}, ids)
}

func TestGetModelAliasesByTargetGroupsActiveAliases(t *testing.T) {
	setupModelAliasTest(t)

	_, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "fast", ModelName: "gpt-4o"})
	require.NoError(t, err)
	_, err = CreateModelAlias(ModelAliasCreateParams{AliasName: "turbo", ModelName: "gpt-4o"})
	require.NoError(t, err)
	// Deprecated aliases must not leak into the reverse lookup.
	_, err = CreateModelAlias(ModelAliasCreateParams{
		AliasName:   "old",
		ModelName:   "gpt-4o",
		Status:      ModelAliasStatusDeprecated,
		Replacement: "gpt-5",
	})
	require.NoError(t, err)

	byTarget := GetModelAliasesByTarget()
	aliases, ok := byTarget["gpt-4o"]
	require.True(t, ok)
	// 别名按名称排序，保证对外展示稳定。
	assert.Equal(t, []string{"fast", "turbo"}, aliases)
	// Unrelated target has no aliases.
	require.NotContains(t, byTarget, "gpt-5")

	entries := GetActiveAliasEntriesByTarget()["gpt-4o"]
	require.Len(t, entries, 2)
	assert.Equal(t, "fast", entries[0].AliasName)
	assert.Equal(t, "turbo", entries[1].AliasName)
}

func TestCreateModelAliasDuplicateRejected(t *testing.T) {
	setupModelAliasTest(t)

	_, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "dup-alias", ModelName: "gpt-4o"})
	require.NoError(t, err)
	// Same alias name cannot point to a second target.
	_, err = CreateModelAlias(ModelAliasCreateParams{AliasName: "dup-alias", ModelName: "gpt-4.1"})
	require.Error(t, err)
}

func TestGetUsableAliasEntriesByTargetFiltersByChannelRestriction(t *testing.T) {
	setupModelAliasTest(t)

	const group = "alias-usable-group"
	const target = "alias-usable-model"
	t.Cleanup(func() {
		DB.Where("model = ? and channel_id in ?", target, []int{901, 902}).Delete(&Ability{})
	})

	_, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "zz-free-alias", ModelName: target})
	require.NoError(t, err)
	_, err = CreateModelAlias(ModelAliasCreateParams{AliasName: "zz-pinned-alias", ModelName: target, ChannelIds: "[902]"})
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Ability{Group: group, Model: target, ChannelId: 901, Enabled: true}).Error)

	// 受限渠道 902 未启用：只保留不限渠道的别名。
	usable := GetUsableAliasEntriesByTarget([]string{group})[target]
	require.Len(t, usable, 1)
	assert.Equal(t, "zz-free-alias", usable[0].AliasName)

	require.NoError(t, DB.Create(&Ability{Group: group, Model: target, ChannelId: 902, Enabled: true}).Error)
	usable = GetUsableAliasEntriesByTarget([]string{group})[target]
	require.Len(t, usable, 2)
	assert.Equal(t, "zz-free-alias", usable[0].AliasName)
	assert.Equal(t, "zz-pinned-alias", usable[1].AliasName)

	// 分组不匹配时，受限别名不可用（不限渠道的别名仍然可用）。
	usable = GetUsableAliasEntriesByTarget([]string{"alias-other-group"})[target]
	require.Len(t, usable, 1)
	assert.Equal(t, "zz-free-alias", usable[0].AliasName)
}

func TestModelLimitNamesNormalizeAndDisplay(t *testing.T) {
	setupModelAliasTest(t)

	_, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "zz-fast", ModelName: "zz-target"})
	require.NoError(t, err)

	// 保存：别名替换为真实名，去重并保留未知项。
	assert.Equal(t, []string{"zz-target", "zz-other"}, NormalizeModelLimitNames([]string{"zz-fast", "zz-target", "zz-other"}))
	// 展示：真实名替换为排序后的首个别名。
	assert.Equal(t, []string{"zz-fast", "zz-other"}, ModelLimitDisplayNames([]string{"zz-target", "zz-other"}))
}
