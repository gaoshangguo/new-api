package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ChannelKeyCipherPrefix marks AES-GCM encrypted channel keys stored at rest.
// Plaintext legacy values (no prefix) stay readable and are encrypted on the
// next write; this keeps the migration safe on all three databases.
const ChannelKeyCipherPrefix = "enc:v1:"

const (
	channelKeyNonceSize = 12
	channelKeyMinKeyLen = 32 // AES-256
)

var (
	channelKeyMasterKeyOnce sync.Once
	channelKeyMasterKey     []byte
	// channelKeyMasterKeyFile 是主密钥持久化文件（0600），位于数据目录
	// （容器 WORKDIR /data 即数据卷）。测试可通过覆盖此变量指向临时路径。
	channelKeyMasterKeyFile = "channel-key-master.key"
)

// ChannelKeyMasterKey returns the AES-256 key used to encrypt channel keys.
// Resolution: CHANNEL_KEY_MASTER_KEY env (>=32 bytes) -> persisted master key
// file (generated once on first start, survives restarts) -> sha256 derivation
// as a last-resort fallback (warns). The file-based key keeps encrypted keys
// decryptable across restarts even when no env var is configured.
func ChannelKeyMasterKey() []byte {
	channelKeyMasterKeyOnce.Do(func() {
		channelKeyMasterKey = resolveChannelKeyMasterKey(
			os.Getenv("CHANNEL_KEY_MASTER_KEY"),
			loadOrCreateChannelKeyFile(),
		)
		if os.Getenv("CHANNEL_KEY_MASTER_KEY") == "" {
			SysError("CHANNEL_KEY_MASTER_KEY not set; using persisted or derived key. Set a dedicated >=32-byte key in production.")
		}
	})
	return channelKeyMasterKey
}

// loadOrCreateChannelKeyFile reads the persisted master key file, generating a
// fresh 32-byte key (0600) when it does not exist or is too short. Generation
// failures return nil so the caller falls back to the derived key.
func loadOrCreateChannelKeyFile() []byte {
	if data, err := os.ReadFile(channelKeyMasterKeyFile); err == nil && len(data) >= channelKeyMinKeyLen {
		return data
	}
	key := make([]byte, channelKeyMinKeyLen)
	if _, err := rand.Read(key); err != nil {
		SysError(fmt.Sprintf("channel key master file generation failed: %v", err))
		return nil
	}
	if err := os.WriteFile(channelKeyMasterKeyFile, key, 0o600); err != nil {
		SysError(fmt.Sprintf("channel key master file write failed: %v", err))
		return nil
	}
	SysLog(fmt.Sprintf("channel key master key persisted to %s", channelKeyMasterKeyFile))
	return key
}

// ChannelKeyPreviousMasterKey returns an optional legacy key used only for
// decryption during a master-key rotation window. Read from env on every call
// so tests and rotations can change it without restarting.
func ChannelKeyPreviousMasterKey() []byte {
	value := os.Getenv("CHANNEL_KEY_MASTER_KEY_PREVIOUS")
	if len(value) >= channelKeyMinKeyLen {
		return []byte(value)
	}
	if value != "" {
		SysError("CHANNEL_KEY_MASTER_KEY_PREVIOUS too short (<32 bytes); ignored")
	}
	return nil
}

// resolveChannelKeyMasterKey prefers an explicit env key, then the persisted
// file key, then a deterministic derivation from the existing secrets.
func resolveChannelKeyMasterKey(masterKeyEnv string, fileKey []byte) []byte {
	if len(masterKeyEnv) >= channelKeyMinKeyLen {
		return []byte(masterKeyEnv)
	}
	if len(fileKey) >= channelKeyMinKeyLen {
		return fileKey
	}
	sum := sha256.Sum256([]byte(CryptoSecret + "\x00" + SessionSecret))
	return sum[:]
}

func IsEncryptedChannelKey(stored string) bool {
	return strings.HasPrefix(stored, ChannelKeyCipherPrefix)
}

// EncryptChannelKey encrypts a plaintext channel key with AES-256-GCM.
// Empty input and already-encrypted input are returned unchanged.
func EncryptChannelKey(plain string) (string, error) {
	if plain == "" || IsEncryptedChannelKey(plain) {
		return plain, nil
	}
	block, err := aes.NewCipher(ChannelKeyMasterKey())
	if err != nil {
		return "", fmt.Errorf("channel key cipher init failed: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("channel key GCM init failed: %w", err)
	}
	nonce := make([]byte, channelKeyNonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("channel key nonce generation failed: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	return ChannelKeyCipherPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptChannelKey decrypts a stored channel key. Values without the cipher
// prefix are legacy plaintext and returned unchanged. Decryption tries the
// current master key first, then the optional previous key (rotation window).
func DecryptChannelKey(stored string) (string, error) {
	if stored == "" || !IsEncryptedChannelKey(stored) {
		return stored, nil
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, ChannelKeyCipherPrefix))
	if err != nil {
		return "", fmt.Errorf("channel key base64 decode failed: %w", err)
	}
	plain, err := decryptChannelKeyWith(payload, ChannelKeyMasterKey())
	if err != nil && len(ChannelKeyPreviousMasterKey()) > 0 {
		plain, err = decryptChannelKeyWith(payload, ChannelKeyPreviousMasterKey())
	}
	if err != nil {
		return "", fmt.Errorf("channel key decrypt failed: %w", err)
	}
	return string(plain), nil
}

func decryptChannelKeyWith(payload, key []byte) ([]byte, error) {
	if len(payload) < channelKeyNonceSize {
		return nil, errors.New("channel key ciphertext too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ciphertext := payload[:channelKeyNonceSize], payload[channelKeyNonceSize:]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}
