package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createEncryptionTestChannel(t *testing.T, key string) *Channel {
	t.Helper()
	channel := &Channel{
		Type:   1,
		Name:   "enc-test-" + common.GetRandomString(6),
		Key:    key,
		Status: 1,
		Group:  "default",
	}
	require.NoError(t, channel.Insert())
	return channel
}

func rawStoredChannelKey(t *testing.T, id int) string {
	t.Helper()
	var stored []string
	require.NoError(t, DB.Table("channels").Where("id = ?", id).Pluck("key", &stored).Error)
	require.Len(t, stored, 1)
	return stored[0]
}

func TestChannelKeyEncryptedAtRest(t *testing.T) {
	channel := createEncryptionTestChannel(t, "sk-test-secret-0001")
	raw := rawStoredChannelKey(t, channel.Id)
	assert.True(t, common.IsEncryptedChannelKey(raw))
	assert.NotContains(t, raw, "sk-test-secret-0001")
}

func TestChannelKeyDecryptedOnLoad(t *testing.T) {
	channel := createEncryptionTestChannel(t, "sk-test-secret-0002")
	loaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "sk-test-secret-0002", loaded.Key)
}

func TestLegacyPlaintextKeyCompatibleOnLoad(t *testing.T) {
	channel := createEncryptionTestChannel(t, "sk-plain-legacy")
	// 直写库模拟历史明文（绕过钩子）
	require.NoError(t, DB.Table("channels").Where("id = ?", channel.Id).Update("key", "sk-plain-legacy").Error)
	loaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "sk-plain-legacy", loaded.Key)
}

func TestCorruptedCiphertextFailsClosed(t *testing.T) {
	channel := createEncryptionTestChannel(t, "sk-secret-0003")
	require.NoError(t, DB.Table("channels").Where("id = ?", channel.Id).Update("key", common.ChannelKeyCipherPrefix+"AAAA").Error)
	loaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "", loaded.Key) // 解密失败清空，不 panic
	_, _, apiErr := loaded.GetNextEnabledKey()
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeChannelNoAvailableKey, apiErr.GetErrorCode())
}

func TestMultiKeyEncryptDecryptRoundTrip(t *testing.T) {
	channel := createEncryptionTestChannel(t, "key-1\nkey-2\nkey-3")
	require.NoError(t, channel.Update()) // struct 更新路径触发钩子
	loaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Len(t, loaded.GetKeys(), 3)
	assert.Equal(t, "key-2", loaded.GetKeys()[1])
}

func TestSelectLimitedUpdatesDoNotEncryptSharedInMemoryKey(t *testing.T) {
	channel := createEncryptionTestChannel(t, "sk-select-guard")
	// 模拟共享缓存对象：AfterFind 已解密为明文
	loaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, "sk-select-guard", loaded.Key)

	// Select 限定更新（不含 key）不得修改内存对象的 Key
	require.NoError(t, DB.Model(loaded).Select("balance", "balance_updated_time").Updates(Channel{Balance: 1.5, BalanceUpdatedTime: 1}).Error)
	assert.Equal(t, "sk-select-guard", loaded.Key)

	// DB 中 key 仍为密文（未被破坏）
	assert.True(t, common.IsEncryptedChannelKey(rawStoredChannelKey(t, channel.Id)))
}

func TestKeyExpiresAtPersisted(t *testing.T) {
	expiresAt := time.Now().Add(7 * 24 * time.Hour).Unix()
	channel := createEncryptionTestChannel(t, "sk-expires")
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("key_expires_at", expiresAt).Error)
	loaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, expiresAt, loaded.KeyExpiresAt)
}

func TestMigrateLegacyChannelKeys(t *testing.T) {
	// 清场：迁移掉其他测试可能遗留的明文数据，保证断言确定性
	_, _ = MigrateLegacyChannelKeys()

	plain1 := createEncryptionTestChannel(t, "sk-legacy-1")
	plain2 := createEncryptionTestChannel(t, "sk-legacy-2")
	// 直写库模拟历史明文（绕过钩子）
	require.NoError(t, DB.Table("channels").Where("id = ?", plain1.Id).Update("key", "sk-legacy-1").Error)
	require.NoError(t, DB.Table("channels").Where("id = ?", plain2.Id).Update("key", "sk-legacy-2").Error)

	migrated, err := MigrateLegacyChannelKeys()
	require.NoError(t, err)
	assert.Equal(t, int64(2), migrated)

	// 幂等：再次执行无新增
	migrated, err = MigrateLegacyChannelKeys()
	require.NoError(t, err)
	assert.Equal(t, int64(0), migrated)

	// 迁移后读取仍为明文
	loaded1, err := GetChannelById(plain1.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "sk-legacy-1", loaded1.Key)
	assert.True(t, common.IsEncryptedChannelKey(rawStoredChannelKey(t, plain1.Id)))
}
