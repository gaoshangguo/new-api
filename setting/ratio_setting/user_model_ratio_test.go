package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserModelRatioUpdateAndRead(t *testing.T) {
	original := UserModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateUserModelRatioByJSONString(original))
	})

	require.NoError(t, UpdateUserModelRatioByJSONString(`{"42":{"gpt-4o":0.9,"gpt-4o-mini":1}}`))

	ratio, ok := GetUserModelRatio(42, "gpt-4o")
	require.True(t, ok)
	assert.Equal(t, 0.9, ratio)

	ratio, ok = GetUserModelRatio(42, "gpt-4o-mini")
	require.True(t, ok)
	assert.Equal(t, 1.0, ratio)

	// 未配置的用户/模型不命中
	_, ok = GetUserModelRatio(42, "claude-3-5-sonnet")
	assert.False(t, ok)
	_, ok = GetUserModelRatio(43, "gpt-4o")
	assert.False(t, ok)

	// JSON 往返
	parsed := GetUserModelRatioCopy()
	assert.Equal(t, 0.9, parsed[42]["gpt-4o"])
}

func TestUserModelRatioRejectsInvalidValues(t *testing.T) {
	original := UserModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateUserModelRatioByJSONString(original))
	})

	cases := []struct {
		name    string
		jsonStr string
	}{
		{name: "negative ratio", jsonStr: `{"1":{"gpt-4o":-0.1}}`},
		{name: "NaN ratio", jsonStr: `{"1":{"gpt-4o":NaN}}`},
		{name: "positive infinity", jsonStr: `{"1":{"gpt-4o":1e999}}`},
		{name: "malformed json", jsonStr: `{"1":`},
		{name: "not a nested map", jsonStr: `{"1":0.9}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := UpdateUserModelRatioByJSONString(tc.jsonStr)
			require.Error(t, err)
			// 更新失败后配置保持原样，脏值不得进入热路径
			_, ok := GetUserModelRatio(1, "gpt-4o")
			assert.False(t, ok)
		})
	}

	// 校验函数同样拒绝非法值
	require.Error(t, CheckUserModelRatio(`{"1":{"gpt-4o":-2}}`))
	require.NoError(t, CheckUserModelRatio(`{"1":{"gpt-4o":0.5}}`))
	require.NoError(t, CheckUserModelRatio(`{}`))
}

func TestUserModelRatioZeroMeansFree(t *testing.T) {
	original := UserModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateUserModelRatioByJSONString(original))
	})

	require.NoError(t, UpdateUserModelRatioByJSONString(`{"7":{"gpt-4o":0}}`))

	ratio, ok := GetUserModelRatio(7, "gpt-4o")
	require.True(t, ok)
	assert.Zero(t, ratio)
}

func TestUserModelRatioJSONStringViaCommonMarshal(t *testing.T) {
	original := UserModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateUserModelRatioByJSONString(original))
	})

	require.NoError(t, UpdateUserModelRatioByJSONString(`{"9":{"gemini-pro":1.25}}`))
	jsonStr := UserModelRatio2JSONString()
	parsed := make(map[int64]map[string]float64)
	require.NoError(t, common.UnmarshalJsonStr(jsonStr, &parsed))
	assert.Equal(t, 1.25, parsed[9]["gemini-pro"])
}