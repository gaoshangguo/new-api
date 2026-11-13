package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/require"
)

// 令牌模型限制以真实模型名落库、以别名展示，避免别名改指向/删除后限制失效。
func TestTokenModelLimitsUseAliasesOnDisplayAndRealNamesOnSave(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	resetModelAliasCacheForTest(t, db)
	require.NoError(t, db.Create(&model.ModelAlias{
		AliasName: "zz-alias-fast", ModelName: "zz-real-model",
		Status: model.ModelAliasStatusActive, Version: 1,
	}).Error)
	require.NoError(t, model.LoadModelAliases())

	// 保存：别名归一化为真实模型名，未知名称保持原样。
	require.Equal(t, "zz-real-model,zz-other", normalizeTokenModelLimits("zz-alias-fast,zz-other"))
	// 展示：真实模型名替换为排序后的首个别名。
	require.Equal(t, "zz-alias-fast,zz-other", displayTokenModelLimits("zz-real-model,zz-other"))
	require.Equal(t, map[string]bool{"zz-alias-fast": true}, tokenModelLimitDisplayMap("zz-real-model"))

	token := &model.Token{Id: 1, Name: "alias-limit-token", Key: "sk-test", ModelLimits: "zz-real-model", ModelLimitsEnabled: true}
	response := buildMaskedTokenResponse(token)
	require.NotNil(t, response)
	require.Equal(t, "zz-alias-fast", response.ModelLimits)
	// 原 token 结构不被展示转换修改。
	require.Equal(t, "zz-real-model", token.ModelLimits)

	// 关闭别名模式后保持旧行为。
	withModelListAliasMode(t, model_setting.ModelListAliasModeReal)
	require.Equal(t, "zz-alias-fast", normalizeTokenModelLimits("zz-alias-fast"))
	require.Equal(t, "zz-real-model", displayTokenModelLimits("zz-real-model"))
}
