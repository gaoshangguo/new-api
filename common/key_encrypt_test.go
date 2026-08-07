package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
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
	key := resolveChannelKeyMasterKey(strings.Repeat("k", 32), nil)
	require.Len(t, key, 32)
	// 持久化文件密钥
	key = resolveChannelKeyMasterKey("", []byte(strings.Repeat("f", 32)))
	require.Len(t, key, 32)
	// 派生兜底
	key = resolveChannelKeyMasterKey("", nil)
	require.Len(t, key, 32)
	// 过短的环境变量忽略（回退文件/派生）
	key = resolveChannelKeyMasterKey("short", []byte(strings.Repeat("f", 32)))
	require.Len(t, key, 32)
}

func TestChannelKeyMasterKeyFilePersisted(t *testing.T) {
	old := channelKeyMasterKeyFile
	channelKeyMasterKeyFile = filepath.Join(t.TempDir(), "master.key")
	defer func() { channelKeyMasterKeyFile = old }()

	key1 := loadOrCreateChannelKeyFile()
	require.Len(t, key1, 32)
	data, err := os.ReadFile(channelKeyMasterKeyFile)
	require.NoError(t, err)
	assert.Equal(t, key1, data)

	// 幂等：再次读取返回同一密钥
	key2 := loadOrCreateChannelKeyFile()
	assert.Equal(t, key1, key2)
}

func TestDecryptWithPreviousKeyFallback(t *testing.T) {
	// 白盒构造「旧密钥加密的密文」：当前主密钥解密必失败 → 走 PREVIOUS 回退
	oldKey := strings.Repeat("o", 32)
	block, err := aes.NewCipher([]byte(oldKey))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, channelKeyNonceSize)
	_, err = rand.Read(nonce)
	require.NoError(t, err)
	sealed := aead.Seal(nonce, nonce, []byte("sk-rotation"), nil)
	encrypted := ChannelKeyCipherPrefix + base64.StdEncoding.EncodeToString(sealed)

	t.Setenv("CHANNEL_KEY_MASTER_KEY", strings.Repeat("n", 32))
	t.Setenv("CHANNEL_KEY_MASTER_KEY_PREVIOUS", oldKey)
	decrypted, err := DecryptChannelKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, "sk-rotation", decrypted)

	// 负例：无 PREVIOUS 时解密必须失败
	t.Setenv("CHANNEL_KEY_MASTER_KEY_PREVIOUS", "")
	_, err = DecryptChannelKey(encrypted)
	require.Error(t, err)
}
