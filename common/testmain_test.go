package common

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain 把主密钥持久化文件重定向到临时目录，避免测试在源码树生成
// 游离的 channel-key-master.key（common 包无既有 TestMain，可安全新增）。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "common-key-test")
	if err != nil {
		panic("failed to create temp dir for master key: " + err.Error())
	}
	channelKeyMasterKeyFile = filepath.Join(dir, "master.key")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
