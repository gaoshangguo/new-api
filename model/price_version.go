package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"gorm.io/gorm"
)

// Price versioning (P0-28): every successful call binds the price version that
// was active at pre-consume time, and every price change produces a new
// immutable snapshot row. Historical bills therefore never change when prices
// are adjusted later: their version row still holds the exact ratio maps that
// produced the charge.
var ErrPriceVersionImmutable = errors.New("price version snapshots are append-only")

const (
	priceVersionStatusPending = "pending"
	priceVersionStatusActive  = "active"

	// priceVersionCacheTTL bounds how long a node serves a cached "current
	// version id" without re-reading the database. Single-node deployments are
	// exact; multi-node deployments may lag by at most this window.
	priceVersionCacheTTL = 30 * time.Second
)

// PriceVersion freezes every price-affecting map at a point in time. Snapshot
// columns hold the exact option payloads (JSON strings) that UpdateOption
// accepts, so applying a version restores prices deterministically.
type PriceVersion struct {
	Id                   int    `json:"id" gorm:"primaryKey"`
	Name                 string `json:"name" gorm:"size:128"`
	EffectiveAt          int64  `json:"effective_at" gorm:"index"`
	Status               string `json:"status" gorm:"size:16;index"`
	ModelRatio           string `json:"model_ratio" gorm:"type:text"`
	ModelPrice           string `json:"model_price" gorm:"type:text"`
	CompletionRatio      string `json:"completion_ratio" gorm:"type:text"`
	CacheRatio           string `json:"cache_ratio" gorm:"type:text"`
	CreateCacheRatio     string `json:"create_cache_ratio" gorm:"type:text"`
	ImageRatio           string `json:"image_ratio" gorm:"type:text"`
	AudioRatio           string `json:"audio_ratio" gorm:"type:text"`
	AudioCompletionRatio string `json:"audio_completion_ratio" gorm:"type:text"`
	GroupRatio           string `json:"group_ratio" gorm:"type:text"`
	GroupGroupRatio      string `json:"group_group_ratio" gorm:"type:text"`
	AppliedAt            int64  `json:"applied_at"`
	CreatedBy            int    `json:"created_by"`
	CreatedAt            int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PriceVersion) BeforeDelete(*gorm.DB) error {
	return ErrPriceVersionImmutable
}

// BeforeUpdate keeps the snapshot itself immutable. Only the apply transition
// (status -> active with applied_at) may touch a row; everything else is a
// rewrite of historical pricing evidence and is rejected.
func (PriceVersion) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement.Selects != nil {
		for _, name := range tx.Statement.Selects {
			if !priceVersionMutableColumn(name) {
				return ErrPriceVersionImmutable
			}
		}
		return nil
	}
	if dest, ok := tx.Statement.Dest.(map[string]interface{}); ok {
		for name := range dest {
			if !priceVersionMutableColumn(name) {
				return ErrPriceVersionImmutable
			}
		}
		return nil
	}
	if dest, ok := tx.Statement.Dest.(map[string]string); ok {
		for name := range dest {
			if !priceVersionMutableColumn(name) {
				return ErrPriceVersionImmutable
			}
		}
		return nil
	}
	return ErrPriceVersionImmutable
}

func priceVersionMutableColumn(name string) bool {
	switch name {
	case "status", "applied_at":
		return true
	}
	return false
}

// PriceVersionSnapshot is the JSON envelope used for audit before/after values.
type PriceVersionSnapshot struct {
	VersionId         int      `json:"version_id"`
	Name              string   `json:"name"`
	Status            string   `json:"status"`
	EffectiveAt       int64    `json:"effective_at"`
	AppliedAt         int64    `json:"applied_at,omitempty"`
	ChangedOptionKeys []string `json:"changed_option_keys,omitempty"`
}

type PriceVersionCreateParams struct {
	Name        string
	EffectiveAt int64
	// Overlay maps option keys (price snapshot keys only) to new JSON payloads
	// that replace the live values. Empty values are ignored.
	Overlay   map[string]string
	CreatedBy int
	Reason    string
	// Actor is required for audited (admin-created) versions; system-triggered
	// creations (startup initial version) pass nil and skip the audit event.
	Actor *BusinessActor
}

type PriceVersionApplyResult struct {
	AppliedCount     int `json:"applied_count"`
	CurrentVersionId int `json:"current_version_id"`
}

var (
	priceVersionCacheMu      sync.Mutex
	priceVersionCacheId      int
	priceVersionCacheRefresh time.Time

	priceVersionEnsureMu sync.Mutex
)

// GetCurrentPriceVersionId returns the id of the active version in effect at
// this moment. The result is cached for a short TTL to keep the hot relay path
// DB-free; a request's price data is bound at pre-consume time, so a bounded
// cross-node lag never re-prices a settled charge.
func GetCurrentPriceVersionId() int {
	priceVersionCacheMu.Lock()
	defer priceVersionCacheMu.Unlock()
	if priceVersionCacheId > 0 && time.Since(priceVersionCacheRefresh) < priceVersionCacheTTL {
		return priceVersionCacheId
	}
	return refreshCurrentPriceVersionIdLocked()
}

// RefreshCurrentPriceVersionId force-refreshes the cached current version id
// after a version was created or applied.
func RefreshCurrentPriceVersionId() {
	priceVersionCacheMu.Lock()
	defer priceVersionCacheMu.Unlock()
	refreshCurrentPriceVersionIdLocked()
}

// refreshCurrentPriceVersionIdLocked re-reads the latest active version. When
// no version exists at all (e.g. a legacy deployment before price versioning),
// it seeds the initial version from the live maps first. Caller holds
// priceVersionCacheMu.
func refreshCurrentPriceVersionIdLocked() int {
	if DB == nil {
		// Relay paths may run before the database is ready (tests, early
		// startup). Never crash the hot path: bind version 0 (unversioned).
		priceVersionCacheId = 0
		priceVersionCacheRefresh = time.Now()
		return 0
	}
	var version PriceVersion
	// A version is "current" once it was applied (applied_at), not when its
	// planned effective_at was: an admin force-apply makes the snapshot live
	// immediately even for a future-dated version. The initial version has
	// applied_at = 0 and counts as current from the start.
	err := DB.Model(&PriceVersion{}).
		Where("status = ? AND (applied_at = 0 OR applied_at <= ?)", priceVersionStatusActive, common.GetTimestamp()).
		Order("id desc").Limit(1).First(&version).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysError("failed to query current price version: " + err.Error())
			priceVersionCacheId = 0
			priceVersionCacheRefresh = time.Now()
			return 0
		}
		if ensureErr := EnsureInitialPriceVersion(); ensureErr != nil {
			common.SysError("failed to ensure initial price version: " + ensureErr.Error())
			priceVersionCacheId = 0
			priceVersionCacheRefresh = time.Now()
			return 0
		}
		if queryErr := DB.Model(&PriceVersion{}).
			Where("status = ? AND (applied_at = 0 OR applied_at <= ?)", priceVersionStatusActive, common.GetTimestamp()).
			Order("id desc").Limit(1).First(&version).Error; queryErr != nil {
			common.SysError("failed to re-query current price version: " + queryErr.Error())
			priceVersionCacheId = 0
			priceVersionCacheRefresh = time.Now()
			return 0
		}
	}
	priceVersionCacheId = version.Id
	priceVersionCacheRefresh = time.Now()
	return priceVersionCacheId
}

// EnsureInitialPriceVersion seeds the first version row from the current live
// maps when the table is empty (legacy deployment upgrade). It is idempotent
// and safe to call from every node; a duplicate row race is harmless because
// both snapshots are identical and the latest id wins.
func EnsureInitialPriceVersion() error {
	priceVersionEnsureMu.Lock()
	defer priceVersionEnsureMu.Unlock()
	if DB == nil {
		return errors.New("database is not initialized")
	}
	var count int64
	if err := DB.Model(&PriceVersion{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	snapshot := ratio_setting.GetPriceMapsJSON()
	version := &PriceVersion{
		Name:                 "Initial price version",
		EffectiveAt:          common.GetTimestamp(),
		Status:               priceVersionStatusActive,
		ModelRatio:           snapshot["ModelRatio"],
		ModelPrice:           snapshot["ModelPrice"],
		CompletionRatio:      snapshot["CompletionRatio"],
		CacheRatio:           snapshot["CacheRatio"],
		CreateCacheRatio:     snapshot["CreateCacheRatio"],
		ImageRatio:           snapshot["ImageRatio"],
		AudioRatio:           snapshot["AudioRatio"],
		AudioCompletionRatio: snapshot["AudioCompletionRatio"],
		GroupRatio:           snapshot["GroupRatio"],
		GroupGroupRatio:      snapshot["GroupGroupRatio"],
	}
	if err := DB.Create(version).Error; err != nil {
		return err
	}
	common.SysLog(fmt.Sprintf("price versioning initialized: created initial price version #%d", version.Id))
	return nil
}

// CreatePriceVersionFromLiveChange records a new active version after a price
// option was updated through the regular admin flow. The snapshot is taken
// from the (already mutated) live maps, so the new version always matches what
// subsequent requests will be charged.
func CreatePriceVersionFromLiveChange(key string, actor *BusinessActor, reason string) (*PriceVersion, error) {
	if !ratio_setting.IsPriceSnapshotKey(key) {
		return nil, nil
	}
	if actor == nil || !actor.valid() {
		return nil, errors.New("invalid business audit actor")
	}
	return CreatePriceVersion(PriceVersionCreateParams{
		EffectiveAt: common.GetTimestamp(),
		CreatedBy:   actor.UserId,
		Reason:      reason,
		Actor:       actor,
	})
}

// CreatePriceVersion snapshots the current live maps (overlaid with any
// provided values) into a new version row. When the effective time has already
// passed, the snapshot is applied immediately so live prices match the new
// version; otherwise it stays pending until ApplyPendingPriceVersions runs.
func CreatePriceVersion(params PriceVersionCreateParams) (*PriceVersion, error) {
	if params.EffectiveAt <= 0 {
		params.EffectiveAt = common.GetTimestamp()
	}
	for key, value := range params.Overlay {
		if value == "" {
			continue
		}
		if !ratio_setting.IsPriceSnapshotKey(key) {
			return nil, fmt.Errorf("option key %s is not part of a price snapshot", key)
		}
		if err := validatePriceSnapshotValue(key, value); err != nil {
			return nil, fmt.Errorf("invalid value for %s: %w", key, err)
		}
	}

	snapshot := ratio_setting.GetPriceMapsJSON()
	for key, value := range params.Overlay {
		if value != "" {
			snapshot[key] = value
		}
	}

	previousID := GetCurrentPriceVersionId()
	now := common.GetTimestamp()
	applyNow := params.EffectiveAt <= now

	version := &PriceVersion{
		Name:                 params.Name,
		EffectiveAt:          params.EffectiveAt,
		Status:               priceVersionStatusPending,
		ModelRatio:           snapshot["ModelRatio"],
		ModelPrice:           snapshot["ModelPrice"],
		CompletionRatio:      snapshot["CompletionRatio"],
		CacheRatio:           snapshot["CacheRatio"],
		CreateCacheRatio:     snapshot["CreateCacheRatio"],
		ImageRatio:           snapshot["ImageRatio"],
		AudioRatio:           snapshot["AudioRatio"],
		AudioCompletionRatio: snapshot["AudioCompletionRatio"],
		GroupRatio:           snapshot["GroupRatio"],
		GroupGroupRatio:      snapshot["GroupGroupRatio"],
		CreatedBy:            params.CreatedBy,
	}
	if applyNow {
		version.Status = priceVersionStatusActive
		version.AppliedAt = now
	}
	if version.Name == "" {
		version.Name = "Price version"
	}

	changedKeys := make([]string, 0, len(params.Overlay))
	for key := range params.Overlay {
		if params.Overlay[key] != "" {
			changedKeys = append(changedKeys, key)
		}
	}
	sort.Strings(changedKeys)

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(version).Error; err != nil {
			return err
		}
		// 仅 overlay 场景需要真正应用快照（快照与 live 状态不同）；
		// 无 overlay 的版本（如价格变更自动归档）快照即当前 live 状态，
		// 无需写入选项，也不受顺序守卫影响（记录不改变任何价格）。
		if applyNow && len(changedKeys) > 0 {
			// 顺序守卫：存在更早的 pending 版本时拒绝立即生效（与
			// applyPriceVersionRow 一致），防止逆序应用导致价格错位。
			var earlier int64
			if err := tx.Model(&PriceVersion{}).
				Where("status = ? AND (effective_at < ? OR (effective_at = ? AND id < ?))",
					priceVersionStatusPending, version.EffectiveAt, version.EffectiveAt, version.Id).
				Count(&earlier).Error; err != nil {
				return err
			}
			if earlier > 0 {
				return fmt.Errorf("price version #%d cannot be applied before earlier pending version(s)", version.Id)
			}
			// 版本行与快照选项同一事务写入：任一失败整体回滚，不会出现
			// "版本 active 但价格未变"的中间态。
			if err := writePriceSnapshotOptions(tx, snapshot); err != nil {
				return err
			}
		}
		if params.Actor != nil && params.Actor.valid() {
			before := PriceVersionSnapshot{VersionId: previousID}
			after := PriceVersionSnapshot{
				VersionId:         version.Id,
				Name:              version.Name,
				Status:            version.Status,
				EffectiveAt:       version.EffectiveAt,
				AppliedAt:         version.AppliedAt,
				ChangedOptionKeys: changedKeys,
			}
			if err := createBusinessAuditEvent(tx, *params.Actor, "price_version.create", "price_version", version.Id, params.Reason, before, after); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if version.Name == "Price version" {
		version.Name = fmt.Sprintf("Price version #%d", version.Id)
	}

	if applyNow && len(changedKeys) > 0 {
		// 提交后刷新内存映射（与 UpdateOptionsBulk 语义一致）。
		for k, v := range snapshot {
			if err := updateOptionMap(k, v); err != nil {
				return version, fmt.Errorf("price version #%d created but in-memory price maps could not be refreshed: %w", version.Id, err)
			}
		}
	}
	RefreshCurrentPriceVersionId()
	return version, nil
}

// ApplyPendingPriceVersions transitions every pending version whose effective
// time has arrived into the live price maps and marks it active. Each row
// transition is optimistic (id + pending), so concurrent runners never double
// apply. Runs under the scheduled system task lease and on manual apply-now.
func ApplyPendingPriceVersions() (*PriceVersionApplyResult, error) {
	now := common.GetTimestamp()
	var pending []PriceVersion
	if err := DB.Where("status = ? AND effective_at <= ?", priceVersionStatusPending, now).
		Order("effective_at asc, id asc").Find(&pending).Error; err != nil {
		return nil, err
	}
	result := &PriceVersionApplyResult{}
	for _, version := range pending {
		applied, err := applyPriceVersionRow(&version, nil, "")
		if err != nil {
			return result, fmt.Errorf("apply price version #%d failed: %w", version.Id, err)
		}
		if applied {
			result.AppliedCount++
		}
	}
	RefreshCurrentPriceVersionId()
	result.CurrentVersionId = GetCurrentPriceVersionId()
	if result.AppliedCount > 0 {
		common.SysLog(fmt.Sprintf("price version apply pass: %d version(s) applied, current version #%d", result.AppliedCount, result.CurrentVersionId))
	}
	return result, nil
}

// ApplyPriceVersionNow forces a pending version active immediately and applies
// its snapshot (admin action, audited).
func ApplyPriceVersionNow(versionID int, actor *BusinessActor, reason string) error {
	if versionID <= 0 {
		return errors.New("invalid price version id")
	}
	var version PriceVersion
	if err := DB.First(&version, versionID).Error; err != nil {
		return err
	}
	if version.Status == priceVersionStatusActive {
		return nil
	}
	applied, err := applyPriceVersionRow(&version, actor, reason)
	if err != nil {
		return err
	}
	if applied {
		RefreshCurrentPriceVersionId()
	}
	return nil
}

// applyPriceVersionRow performs the optimistic pending -> active transition
// and loads the snapshot into the live maps and option rows atomically.
// auditActor may be nil for scheduled (system) applies.
func applyPriceVersionRow(version *PriceVersion, actor *BusinessActor, reason string) (bool, error) {
	now := common.GetTimestamp()
	previousID := GetCurrentPriceVersionId()
	applied := false
	snapshot := priceVersionSnapshotMap(version)
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 顺序守卫：存在更早的 pending 版本时拒绝应用，防止手动 apply-now
		// 与调度任务并发时逆序应用，导致 live 价格与当前版本错位。
		var earlier int64
		if err := tx.Model(&PriceVersion{}).
			Where("status = ? AND (effective_at < ? OR (effective_at = ? AND id < ?))",
				priceVersionStatusPending, version.EffectiveAt, version.EffectiveAt, version.Id).
			Count(&earlier).Error; err != nil {
			return err
		}
		if earlier > 0 {
			return fmt.Errorf("price version #%d cannot be applied before earlier pending version(s)", version.Id)
		}
		result := tx.Model(&PriceVersion{}).
			Where("id = ? AND status = ?", version.Id, priceVersionStatusPending).
			Updates(map[string]interface{}{"status": priceVersionStatusActive, "applied_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil // already applied by another runner
		}
		applied = true
		version.Status = priceVersionStatusActive
		version.AppliedAt = now
		// 快照选项写入与状态翻转在同一事务：任一失败整体回滚，版本保持
		// pending，调度任务下一轮可重试（修复"active 但快照未应用"）。
		if err := writePriceSnapshotOptions(tx, snapshot); err != nil {
			return err
		}
		if actor != nil && actor.valid() {
			before := PriceVersionSnapshot{VersionId: previousID, Name: version.Name, Status: priceVersionStatusPending, EffectiveAt: version.EffectiveAt}
			after := PriceVersionSnapshot{VersionId: version.Id, Name: version.Name, Status: priceVersionStatusActive, EffectiveAt: version.EffectiveAt, AppliedAt: now}
			if err := createBusinessAuditEvent(tx, *actor, "price_version.apply", "price_version", version.Id, reason, before, after); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if !applied {
		return false, nil
	}
	// 提交后刷新内存映射（与 UpdateOptionsBulk 语义一致：DB 为权威，进程重启自愈）。
	for k, v := range snapshot {
		if err := updateOptionMap(k, v); err != nil {
			return true, err
		}
	}
	return true, nil
}

// priceVersionSnapshotMap returns the snapshot option rows carried by a version.
func priceVersionSnapshotMap(version *PriceVersion) map[string]string {
	return map[string]string{
		"ModelRatio":           version.ModelRatio,
		"ModelPrice":           version.ModelPrice,
		"CompletionRatio":      version.CompletionRatio,
		"CacheRatio":           version.CacheRatio,
		"CreateCacheRatio":     version.CreateCacheRatio,
		"ImageRatio":           version.ImageRatio,
		"AudioRatio":           version.AudioRatio,
		"AudioCompletionRatio": version.AudioCompletionRatio,
		"GroupRatio":           version.GroupRatio,
		"GroupGroupRatio":      version.GroupGroupRatio,
	}
}

// writePriceSnapshotOptions persists snapshot option rows within the caller's
// transaction so a price-version apply commits atomically with the status flip.
func writePriceSnapshotOptions(tx *gorm.DB, snapshot map[string]string) error {
	for k, v := range snapshot {
		if err := validateOptionValue(k, v); err != nil {
			return err
		}
		option := Option{Key: k}
		if err := tx.FirstOrCreate(&option, Option{Key: k}).Error; err != nil {
			return err
		}
		option.Value = v
		if err := tx.Save(&option).Error; err != nil {
			return err
		}
	}
	return nil
}

// validatePriceSnapshotValue rejects negative ratios before they can reach the
// billing path (overlay values are user-controlled).
func validatePriceSnapshotValue(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("empty snapshot value")
	}
	if key == "GroupGroupRatio" {
		var parsed map[string]map[string]float64
		if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
			return err
		}
		for userGroup, groups := range parsed {
			for groupName, ratio := range groups {
				if ratio < 0 {
					return fmt.Errorf("group ratio must not be negative: %s/%s", userGroup, groupName)
				}
			}
		}
		return nil
	}
	var parsed map[string]float64
	if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
		return err
	}
	for modelName, ratio := range parsed {
		if ratio < 0 {
			return fmt.Errorf("ratio must not be negative: %s", modelName)
		}
	}
	return nil
}

// ListPriceVersionItem is the metadata-only list projection (snapshots are
// served by GetPriceVersion for a single row).
type ListPriceVersionItem struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	EffectiveAt int64  `json:"effective_at"`
	Status      string `json:"status"`
	AppliedAt   int64  `json:"applied_at"`
	CreatedBy   int    `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`
}

func ListPriceVersions(startIdx, pageSize int) ([]*ListPriceVersionItem, int64, error) {
	var total int64
	if err := DB.Model(&PriceVersion{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*ListPriceVersionItem, 0)
	if err := DB.Model(&PriceVersion{}).
		Select("id", "name", "effective_at", "status", "applied_at", "created_by", "created_at").
		Order("id desc").Limit(pageSize).Offset(startIdx).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetPriceVersion(versionID int) (*PriceVersion, error) {
	if versionID <= 0 {
		return nil, errors.New("invalid price version id")
	}
	var version PriceVersion
	if err := DB.First(&version, versionID).Error; err != nil {
		return nil, err
	}
	return &version, nil
}
