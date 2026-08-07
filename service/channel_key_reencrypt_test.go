package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReEncryptAllChannelKeys(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	plain := &model.Channel{Type: 1, Name: "reenc-plain", Key: "sk-plain-rot", Status: 1, Group: "default"}
	require.NoError(t, plain.Insert())
	enc := &model.Channel{Type: 1, Name: "reenc-enc", Key: "sk-enc-rot", Status: 1, Group: "default"}
	require.NoError(t, enc.Insert())
	// plain 已被钩子加密；直写库改回明文模拟轮换前的残留
	require.NoError(t, model.DB.Table("channels").Where("id = ?", plain.Id).Update("key", "sk-plain-rot").Error)

	result, err := ReEncryptAllChannelKeys(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, 2)
	assert.Equal(t, 0, result.Failed)

	// 全部可解密且值不变
	loadedPlain, err := model.GetChannelById(plain.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "sk-plain-rot", loadedPlain.Key)
	loadedEnc, err := model.GetChannelById(enc.Id, true)
	require.NoError(t, err)
	assert.Equal(t, "sk-enc-rot", loadedEnc.Key)
}
