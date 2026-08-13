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

func TestCreateModelAliasDuplicateRejected(t *testing.T) {
	setupModelAliasTest(t)

	_, err := CreateModelAlias(ModelAliasCreateParams{AliasName: "dup-alias", ModelName: "gpt-4o"})
	require.NoError(t, err)
	_, err = CreateModelAlias(ModelAliasCreateParams{AliasName: "dup-alias", ModelName: "gpt-4.1"})
	require.Error(t, err)
}
