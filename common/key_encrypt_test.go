package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plain := "sk-abcdef1234567890"
	encrypted, err := EncryptChannelKey(plain)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(encrypted, ChannelKeyCipherPrefix))
	assert.NotContains(t, encrypted, plain)

	decrypted, err := DecryptChannelKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plain, decrypted)
}

func TestEncryptTwiceYieldsDifferentCiphertext(t *testing.T) {
	plain := "sk-same-key"
	e1, err := EncryptChannelKey(plain)
	require.NoError(t, err)
	e2, err := EncryptChannelKey(plain)
	require.NoError(t, err)
	assert.NotEqual(t, e1, e2) // 每次随机 nonce
}

func TestEncryptIdempotentOnCiphertextAndEmpty(t *testing.T) {
	encrypted, err := EncryptChannelKey("sk-x")
	require.NoError(t, err)
	same, err := EncryptChannelKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, encrypted, same)

	empty, err := EncryptChannelKey("")
	require.NoError(t, err)
	assert.Equal(t, "", empty)
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	decrypted, err := DecryptChannelKey("sk-legacy-plain")
	require.NoError(t, err)
	assert.Equal(t, "sk-legacy-plain", decrypted)
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	encrypted, err := EncryptChannelKey("sk-tamper")
	require.NoError(t, err)
	tampered := encrypted[:len(encrypted)-2] + "xx"
	_, err = DecryptChannelKey(tampered)
	require.Error(t, err)
}

func TestResolveChannelKeyMasterKeyFallbackChain(t *testing.T) {
	// 环境变量优先
	key := resolveChannelKeyMasterKey(strings.Repeat("k", 32), "", "")
	require.Len(t, key, 32)
	// cryptoSecret 派生
	key = resolveChannelKeyMasterKey("", "crypto-secret", "session-secret")
	require.Len(t, key, 32)
	// sessionSecret 兜底
	key = resolveChannelKeyMasterKey("", "", "session-secret")
	require.Len(t, key, 32)
	// 过短的环境变量忽略
	key = resolveChannelKeyMasterKey("short", "crypto-secret", "session-secret")
	require.Len(t, key, 32)
}

func TestDecryptWithPreviousKeyFallback(t *testing.T) {
	t.Setenv("CHANNEL_KEY_MASTER_KEY", strings.Repeat("n", 32))
	t.Setenv("CHANNEL_KEY_MASTER_KEY_PREVIOUS", strings.Repeat("o", 32))
	encrypted, err := EncryptChannelKey("sk-rotation")
	require.NoError(t, err)
	// 换主密钥（不换 PREVIOUS）后仍可解密
	t.Setenv("CHANNEL_KEY_MASTER_KEY", strings.Repeat("m", 32))
	decrypted, err := DecryptChannelKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, "sk-rotation", decrypted)
}
