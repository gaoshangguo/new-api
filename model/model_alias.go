package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"gorm.io/gorm"
)

// 模型别名（P0-10）：对外展示名 → 内部实际模型名（可选渠道限制）。
// 版本切换 = 更新目标后 Version 自增（审计记录前后值）；
// 下线替代 = 标记 deprecated 并配置 Replacement，新请求自动落到替代目标。

const (
	ModelAliasStatusActive     = "active"
	ModelAliasStatusDeprecated = "deprecated"

	modelAliasCacheTTL = 30 * time.Second
)

// ModelAliasEntry 是别名的内存缓存形态（含解析所需全部字段）。
type ModelAliasEntry struct {
	Id          int
	AliasName   string
	ModelName   string
	ChannelIds  string
	Status      string
	Replacement string
	Version     int
	Note        string
}

type ModelAlias struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	AliasName   string `json:"alias_name" gorm:"size:128;uniqueIndex"`
	ModelName   string `json:"model_name" gorm:"size:128;index"`
	ChannelIds  string `json:"channel_ids" gorm:"size:512"` // 可选 JSON 数组：仅允许这些渠道
	Status      string `json:"status" gorm:"size:16;index"`
	Replacement string `json:"replacement" gorm:"size:128"`
	Version     int    `json:"version"`
	Note        string `json:"note" gorm:"size:512"`
	CreatedBy   int    `json:"created_by"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

var (
	modelAliasCache     = types.NewRWMap[string, ModelAliasEntry]()
	modelAliasRefreshed time.Time
	modelAliasMu        = make(chan struct{}, 1) // 串行化缓存刷新
)

// LoadModelAliases 全量加载别名到内存缓存（启动时调用）。
func LoadModelAliases() error {
	modelAliasMu <- struct{}{}
	defer func() { <-modelAliasMu }()
	var aliases []ModelAlias
	if err := DB.Find(&aliases).Error; err != nil {
		return err
	}
	newMap := types.NewRWMap[string, ModelAliasEntry]()
	for _, alias := range aliases {
		newMap.Set(alias.AliasName, alias.toEntry())
	}
	modelAliasCache = newMap
	modelAliasRefreshed = time.Now()
	return nil
}

// RefreshModelAliasCache 写操作后强制刷新本节点缓存。
func RefreshModelAliasCache() error {
	return LoadModelAliases()
}

func (a *ModelAlias) toEntry() ModelAliasEntry {
	return ModelAliasEntry{
		Id:          a.Id,
		AliasName:   a.AliasName,
		ModelName:   a.ModelName,
		ChannelIds:  a.ChannelIds,
		Status:      a.Status,
		Replacement: a.Replacement,
		Version:     a.Version,
		Note:        a.Note,
	}
}

// GetModelAliasEntry 按别名查缓存（TTL 过期时从 DB 刷新）。
func GetModelAliasEntry(aliasName string) (ModelAliasEntry, bool) {
	aliasName = strings.TrimSpace(aliasName)
	if aliasName == "" {
		return ModelAliasEntry{}, false
	}
	// 只以刷新时间判断过期；空表结果同样缓存，避免未配置别名时每个请求都查库。
	if modelAliasRefreshed.IsZero() || time.Since(modelAliasRefreshed) > modelAliasCacheTTL {
		_ = LoadModelAliases()
	}
	entry, ok := modelAliasCache.Get(aliasName)
	return entry, ok
}

// ResolveModelAlias 把客户端传入的模型名解析为内部实际模型名。
// 返回 (resolvedModel, aliasEntry, wasAlias, error)：
//   - 非别名 → (原样返回, 空, false, nil)
//   - active 别名 → 解析目标
//   - deprecated 别名且配置了 Replacement → 跟随替代（替代本身是别名时继续解一层）
//   - deprecated 别名且无 Replacement → 错误（模型不可用）
func ResolveModelAlias(aliasName string) (string, ModelAliasEntry, bool, error) {
	aliasName = strings.TrimSpace(aliasName)
	if aliasName == "" {
		return "", ModelAliasEntry{}, false, nil
	}
	entry, ok := GetModelAliasEntry(aliasName)
	if !ok {
		return aliasName, ModelAliasEntry{}, false, nil
	}
	if entry.Status == ModelAliasStatusDeprecated {
		replacement := strings.TrimSpace(entry.Replacement)
		if replacement == "" {
			return "", entry, true, fmt.Errorf("model alias %s is deprecated and has no replacement", entry.AliasName)
		}
		// 替代目标本身可能是别名：跟随一层（避免循环）。
		if next, ok := GetModelAliasEntry(replacement); ok && next.AliasName != entry.AliasName {
			if next.Status == ModelAliasStatusDeprecated && strings.TrimSpace(next.Replacement) == "" {
				return "", entry, true, fmt.Errorf("model alias %s is deprecated and its replacement is unavailable", entry.AliasName)
			}
			return next.ModelName, entry, true, nil
		}
		return replacement, entry, true, nil
	}
	return entry.ModelName, entry, true, nil
}

// ModelAliasChannelIDs 解析别名可选的渠道限制（JSON 数组），空则不限。
func (e ModelAliasEntry) ModelAliasChannelIDs() ([]int, error) {
	if strings.TrimSpace(e.ChannelIds) == "" {
		return nil, nil
	}
	var ids []int
	if err := common.UnmarshalJsonStr(e.ChannelIds, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

type ModelAliasCreateParams struct {
	AliasName   string
	ModelName   string
	ChannelIds  string
	Status      string
	Replacement string
	Note        string
	Actor       *BusinessActor
}

// CreateModelAlias 创建别名（默认 active）。同名已存在时报错。
func CreateModelAlias(params ModelAliasCreateParams) (*ModelAlias, error) {
	params.AliasName = strings.TrimSpace(params.AliasName)
	params.ModelName = strings.TrimSpace(params.ModelName)
	if params.AliasName == "" {
		return nil, errors.New("alias name is required")
	}
	if params.ModelName == "" {
		return nil, errors.New("target model name is required")
	}
	if len(params.AliasName) > 128 || len(params.ModelName) > 128 {
		return nil, errors.New("alias name or target model name exceeds the 128 byte limit")
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = ModelAliasStatusActive
	}
	if status != ModelAliasStatusActive && status != ModelAliasStatusDeprecated {
		return nil, errors.New("invalid alias status")
	}
	if status == ModelAliasStatusDeprecated && strings.TrimSpace(params.Replacement) == "" {
		return nil, errors.New("a deprecated alias requires a replacement model")
	}

	alias := &ModelAlias{
		AliasName:   params.AliasName,
		ModelName:   params.ModelName,
		ChannelIds:  strings.TrimSpace(params.ChannelIds),
		Status:      status,
		Replacement: strings.TrimSpace(params.Replacement),
		Version:     1,
		Note:        strings.TrimSpace(params.Note),
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&ModelAlias{}).Where("alias_name = ?", alias.AliasName).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return fmt.Errorf("model alias %s already exists", alias.AliasName)
		}
		if params.Actor != nil && params.Actor.valid() {
			alias.CreatedBy = params.Actor.UserId
		}
		if err := tx.Create(alias).Error; err != nil {
			return err
		}
		if params.Actor != nil && params.Actor.valid() {
			return createBusinessAuditEvent(tx, *params.Actor, "model_alias.create", "model_alias", alias.Id, "", nil, alias)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = RefreshModelAliasCache()
	return alias, nil
}

// UpdateModelAlias 更新别名目标/状态。目标变化时 Version 自增，审计前后值。
func UpdateModelAlias(id int, params ModelAliasCreateParams, reason string) (*ModelAlias, error) {
	params.ModelName = strings.TrimSpace(params.ModelName)
	if id <= 0 {
		return nil, errors.New("invalid model alias id")
	}
	if params.ModelName == "" {
		return nil, errors.New("target model name is required")
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = ModelAliasStatusActive
	}
	if status != ModelAliasStatusActive && status != ModelAliasStatusDeprecated {
		return nil, errors.New("invalid alias status")
	}
	if status == ModelAliasStatusDeprecated && strings.TrimSpace(params.Replacement) == "" {
		return nil, errors.New("a deprecated alias requires a replacement model")
	}

	var alias ModelAlias
	if err := DB.First(&alias, id).Error; err != nil {
		return nil, err
	}
	before := alias
	alias.ModelName = params.ModelName
	alias.ChannelIds = strings.TrimSpace(params.ChannelIds)
	alias.Status = status
	alias.Replacement = strings.TrimSpace(params.Replacement)
	alias.Note = strings.TrimSpace(params.Note)
	if alias.ModelName != before.ModelName || alias.ChannelIds != before.ChannelIds {
		alias.Version++
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ModelAlias{}).Where("id = ?", id).Updates(map[string]interface{}{
			"model_name":  alias.ModelName,
			"channel_ids": alias.ChannelIds,
			"status":      alias.Status,
			"replacement": alias.Replacement,
			"version":     alias.Version,
			"note":        alias.Note,
			"updated_at":  common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if params.Actor != nil && params.Actor.valid() {
			return createBusinessAuditEvent(tx, *params.Actor, "model_alias.update", "model_alias", alias.Id, reason, before, alias)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = RefreshModelAliasCache()
	return &alias, nil
}

// DeleteModelAlias 删除别名（审计记录）。别名删除后新请求按原始模型名处理。
func DeleteModelAlias(id int, actor *BusinessActor, reason string) error {
	if id <= 0 {
		return errors.New("invalid model alias id")
	}
	var alias ModelAlias
	if err := DB.First(&alias, id).Error; err != nil {
		return err
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&ModelAlias{}, id).Error; err != nil {
			return err
		}
		if actor != nil && actor.valid() {
			return createBusinessAuditEvent(tx, *actor, "model_alias.delete", "model_alias", alias.Id, reason, alias, nil)
		}
		return nil
	})
	if err != nil {
		return err
	}
	_ = RefreshModelAliasCache()
	return nil
}

func ListModelAliases(includeDeprecated bool) ([]*ModelAlias, error) {
	query := DB.Order("id asc")
	if !includeDeprecated {
		query = query.Where("status = ?", ModelAliasStatusActive)
	}
	var aliases []*ModelAlias
	if err := query.Find(&aliases).Error; err != nil {
		return nil, err
	}
	return aliases, nil
}

func GetModelAliasByID(id int) (*ModelAlias, error) {
	if id <= 0 {
		return nil, errors.New("invalid model alias id")
	}
	var alias ModelAlias
	if err := DB.First(&alias, id).Error; err != nil {
		return nil, err
	}
	return &alias, nil
}
