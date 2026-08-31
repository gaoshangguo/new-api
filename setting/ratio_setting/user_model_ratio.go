package ratio_setting

import (
	"errors"
	"math"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

// UserModelRatioSetting 用户级"模型 → 倍率"覆盖配置（语义 A：替换，不叠加）。
// 结构：userId -> modelName -> ratio。命中后该倍率完全取代用户在该模型上的
// 组倍率（含用户组→使用组特殊倍率），优先级最高。倍率为 0 表示该用户将该
// 模型视为免费（与组倍率 0 的免费语义一致）。
type UserModelRatioSetting struct {
	UserModelRatio *types.RWMap[int, map[string]float64] `json:"user_model_ratio"`
}

var userModelRatioSetting = UserModelRatioSetting{
	UserModelRatio: types.NewRWMap[int, map[string]float64](),
}

func init() {
	config.GlobalConfig.Register("user_model_ratio_setting", &userModelRatioSetting)
}

// GetUserModelRatio 返回指定用户对指定模型的覆盖倍率。模型名使用用户请求时的
// 原始模型名（未被模型映射改写）。未配置或值非法时返回 false。
func GetUserModelRatio(userId int, modelName string) (float64, bool) {
	models, ok := userModelRatioSetting.UserModelRatio.Get(userId)
	if !ok {
		return -1, false
	}
	ratio, ok := models[modelName]
	if !ok || !isValidUserModelRatio(ratio) {
		return -1, false
	}
	return ratio, true
}

// GetUserModelRatioCopy 返回当前配置的深拷贝，供管理端展示。
func GetUserModelRatioCopy() map[int]map[string]float64 {
	raw := userModelRatioSetting.UserModelRatio.ReadAll()
	copied := make(map[int]map[string]float64, len(raw))
	for userId, models := range raw {
		modelCopy := make(map[string]float64, len(models))
		for name, ratio := range models {
			modelCopy[name] = ratio
		}
		copied[userId] = modelCopy
	}
	return copied
}

// UserModelRatio2JSONString 序列化当前配置为 JSON 字符串。
func UserModelRatio2JSONString() string {
	return userModelRatioSetting.UserModelRatio.MarshalJSONString()
}

// CheckUserModelRatio 校验配置合法性：倍率必须是非负的有限数。
// 返回的 error 会直接展示给管理员。
func CheckUserModelRatio(jsonStr string) error {
	parsed := make(map[int]map[string]float64)
	if err := common.UnmarshalJsonStr(jsonStr, &parsed); err != nil {
		return err
	}
	for userId, models := range parsed {
		for modelName, ratio := range models {
			if !isValidUserModelRatio(ratio) {
				return errors.New("user model ratio must be a finite number >= 0: user " +
					strconv.Itoa(userId) + " model " + modelName)
			}
		}
	}
	return nil
}

// UpdateUserModelRatioByJSONString 全量更新用户模型覆盖配置（热生效，无需重启）。
func UpdateUserModelRatioByJSONString(jsonStr string) error {
	if err := CheckUserModelRatio(jsonStr); err != nil {
		return err
	}
	return types.LoadFromJsonString(userModelRatioSetting.UserModelRatio, jsonStr)
}

// isValidUserModelRatio 拒绝负数、NaN 与 ±Inf，防止脏值流入计费链路。
func isValidUserModelRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0) && ratio >= 0
}