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

func TestResolveChannelKeyMasterKeyRejectsWrongLengthEnv(t *testing.T) {
	// 33 字节 env 不满足 AES-256 要求：回退文件密钥，且不 panic
	key := resolveChannelKeyMasterKey(strings.Repeat("e", 33), []byte(strings.Repeat("f", 32)))
	require.Len(t, key, 32)
	assert.Equal(t, strings.Repeat("f", 32), string(key))
	// 40 字节 env（旧 >= 校验会放行、运行时 aes.NewCipher 必失败的场景）同样回退
	key = resolveChannelKeyMasterKey(strings.Repeat("e", 40), []byte(strings.Repeat("f", 32)))
	require.Len(t, key, 32)
	assert.Equal(t, strings.Repeat("f", 32), string(key))
	// 空 env + nil 文件 → 派生 32 字节
	key = resolveChannelKeyMasterKey("", nil)
	require.Len(t, key, 32)
}

func TestChannelKeyMasterKeyFileTrimsTrailingNewline(t *testing.T) {
	old := channelKeyMasterKeyFile
	channelKeyMasterKeyFile = filepath.Join(t.TempDir(), "master.key")
	defer func() { channelKeyMasterKeyFile = old }()

	// 手写备份文件常见形态：32 字节密钥 + 尾换行 → 返回 trim 后密钥
	content := append([]byte(strings.Repeat("b", 32)), '\n')
	require.NoError(t, os.WriteFile(channelKeyMasterKeyFile, content, 0o600))
	key := loadOrCreateChannelKeyFile()
	require.Len(t, key, 32)
	assert.Equal(t, strings.Repeat("b", 32), string(key))
	// 文件内容不被改写
	data, err := os.ReadFile(channelKeyMasterKeyFile)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// 恰好 32 字节（末字节为换行）必须原样使用，TrimSpace 不得破坏二进制密钥
	raw := []byte(strings.Repeat("c", 31))
	raw = append(raw, '\n')
	require.NoError(t, os.WriteFile(channelKeyMasterKeyFile, raw, 0o600))
	key = loadOrCreateChannelKeyFile()
	assert.Equal(t, raw, key)

	// 33 字节但 trim 后仍不为 32（如 33 字节纯内容）→ 重新生成 32 字节密钥
	require.NoError(t, os.WriteFile(channelKeyMasterKeyFile, []byte(strings.Repeat("d", 33)), 0o600))
	key = loadOrCreateChannelKeyFile()
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
