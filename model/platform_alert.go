package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// 平台级告警规则（P0-29）：对聚合指标（错误率、渠道停用数等）配置阈值规则，
// 由 platform_alert_eval 定时任务评估并产生告警事件（站内 + 可选通知）。

const (
	PlatformAlertStatusActive   = "active"
	PlatformAlertStatusResolved = "resolved"

	PlatformAlertMetricErrorRate             = "error_rate"
	PlatformAlertMetricDisabledChannels      = "disabled_channels"
	PlatformAlertMetricAutoDisabledChannels  = "auto_disabled_channels"
	PlatformAlertMetricRequestCount1h        = "request_count_1h"
)

// PlatformAlertRule 一条平台级告警规则。
type PlatformAlertRule struct {
	Id           int     `json:"id" gorm:"primaryKey"`
	Name         string  `json:"name" gorm:"size:128"`
	Metric       string  `json:"metric" gorm:"size:32;index"`
	Operator     string  `json:"operator" gorm:"size:8"` // > >= < <=
	Threshold    float64 `json:"threshold"`
	WindowMinutes int    `json:"window_minutes"` // 评估窗口（分钟）
	Enabled      bool    `json:"enabled"`
	NotifyEnabled bool   `json:"notify_enabled"` // 触发时是否外发通知
	Reason       string  `json:"reason" gorm:"size:255"`
	CreatedBy    int     `json:"created_by"`
	CreatedAt    int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

// PlatformAlertEvent 一条告警事件（active→resolved 生命周期，append-only 语义）。
type PlatformAlertEvent struct {
	Id         int     `json:"id" gorm:"primaryKey"`
	RuleId     int     `json:"rule_id" gorm:"index"`
	Metric     string  `json:"metric" gorm:"size:32;index"`
	RuleName   string  `json:"rule_name" gorm:"size:128"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
	Operator   string  `json:"operator" gorm:"size:8"`
	Status     string  `json:"status" gorm:"size:16;index"`
	Message    string  `json:"message" gorm:"size:255"`
	CreatedAt  int64   `json:"created_at" gorm:"autoCreateTime;index"`
	ResolvedAt int64   `json:"resolved_at"`
}

// ValidatePlatformAlertRule 校验规则字段。
func ValidatePlatformAlertRule(rule *PlatformAlertRule) error {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Metric = strings.TrimSpace(rule.Metric)
	rule.Operator = strings.TrimSpace(rule.Operator)
	if rule.Name == "" {
		return errors.New("rule name is required")
	}
	if len(rule.Name) > 128 {
		return errors.New("rule name exceeds the 128 byte limit")
	}
	switch rule.Metric {
	case PlatformAlertMetricErrorRate, PlatformAlertMetricDisabledChannels,
		PlatformAlertMetricAutoDisabledChannels, PlatformAlertMetricRequestCount1h:
	default:
		return errors.New("unknown alert metric: " + rule.Metric)
	}
	switch rule.Operator {
	case ">", ">=", "<", "<=":
	default:
		return errors.New("operator must be one of > >= < <=")
	}
	if rule.Metric == PlatformAlertMetricErrorRate && (rule.Threshold < 0 || rule.Threshold > 1) {
		return errors.New("error rate threshold must be between 0 and 1")
	}
	if rule.WindowMinutes <= 0 {
		rule.WindowMinutes = 10
	}
	if rule.WindowMinutes > 7*24*60 {
		return errors.New("window exceeds 7 days")
	}
	return nil
}

// CreatePlatformAlertRule 创建告警规则。
func CreatePlatformAlertRule(rule *PlatformAlertRule, actor *BusinessActor) error {
	if err := ValidatePlatformAlertRule(rule); err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if actor != nil && actor.valid() {
			rule.CreatedBy = actor.UserId
		}
		if err := tx.Create(rule).Error; err != nil {
			return err
		}
		if actor != nil && actor.valid() {
			return createBusinessAuditEvent(tx, *actor, "platform_alert.rule.create", "platform_alert_rule", rule.Id, "", nil, rule)
		}
		return nil
	})
}

// UpdatePlatformAlertRule 更新告警规则（审计前后值）。
func UpdatePlatformAlertRule(id int, rule *PlatformAlertRule, reason string, actor *BusinessActor) error {
	if id <= 0 {
		return errors.New("invalid alert rule id")
	}
	if err := ValidatePlatformAlertRule(rule); err != nil {
		return err
	}
	var existing PlatformAlertRule
	if err := DB.First(&existing, id).Error; err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PlatformAlertRule{}).Where("id = ?", id).Updates(map[string]interface{}{
			"name":           rule.Name,
			"metric":         rule.Metric,
			"operator":       rule.Operator,
			"threshold":      rule.Threshold,
			"window_minutes": rule.WindowMinutes,
			"enabled":        rule.Enabled,
			"notify_enabled": rule.NotifyEnabled,
			"reason":         rule.Reason,
			"updated_at":     common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if actor != nil && actor.valid() {
			return createBusinessAuditEvent(tx, *actor, "platform_alert.rule.update", "platform_alert_rule", id, reason, existing, rule)
		}
		return nil
	})
}

// DeletePlatformAlertRule 删除告警规则（审计）。
func DeletePlatformAlertRule(id int, actor *BusinessActor, reason string) error {
	if id <= 0 {
		return errors.New("invalid alert rule id")
	}
	var rule PlatformAlertRule
	if err := DB.First(&rule, id).Error; err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&PlatformAlertRule{}, id).Error; err != nil {
			return err
		}
		if actor != nil && actor.valid() {
			return createBusinessAuditEvent(tx, *actor, "platform_alert.rule.delete", "platform_alert_rule", id, reason, rule, nil)
		}
		return nil
	})
}

func ListPlatformAlertRules() ([]*PlatformAlertRule, error) {
	var rules []*PlatformAlertRule
	if err := DB.Order("id asc").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func ListPlatformAlertEvents(activeOnly bool, startIdx, pageSize int) ([]*PlatformAlertEvent, int64, error) {
	query := DB.Model(&PlatformAlertEvent{}).Order("created_at desc, id desc")
	if activeOnly {
		query = query.Where("status = ?", PlatformAlertStatusActive)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	events := make([]*PlatformAlertEvent, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

// PlatformAlertMetricValue 返回一个聚合指标在窗口内的当前值。
// error_rate 为 0..1；渠道类为计数；request_count_1h 为近 1 小时消费日志数。
func PlatformAlertMetricValue(metric string, windowMinutes int) (float64, error) {
	now := common.GetTimestamp()
	switch metric {
	case PlatformAlertMetricErrorRate:
		windowSeconds := int64(windowMinutes) * 60
		since := now - windowSeconds
		var total, failed int64
		if err := LOG_DB.Model(&Log{}).Where("type = ? AND created_at >= ?", LogTypeConsume, since).Count(&total).Error; err != nil {
			return 0, err
		}
		if err := LOG_DB.Model(&Log{}).Where("type = ? AND quota = 0 AND created_at >= ?", LogTypeConsume, since).Count(&failed).Error; err != nil {
			return 0, err
		}
		if total < 5 {
			return 0, nil // 样本过少不告警
		}
		return float64(failed) / float64(total), nil
	case PlatformAlertMetricDisabledChannels:
		var count int64
		if err := DB.Model(&Channel{}).Where("status <> ?", common.ChannelStatusEnabled).Count(&count).Error; err != nil {
			return 0, err
		}
		return float64(count), nil
	case PlatformAlertMetricAutoDisabledChannels:
		var count int64
		if err := DB.Model(&Channel{}).Where("status = ?", common.ChannelStatusAutoDisabled).Count(&count).Error; err != nil {
			return 0, err
		}
		return float64(count), nil
	case PlatformAlertMetricRequestCount1h:
		var count int64
		if err := LOG_DB.Model(&Log{}).Where("type = ? AND created_at >= ?", LogTypeConsume, now-3600).Count(&count).Error; err != nil {
			return 0, err
		}
		return float64(count), nil
	default:
		return 0, errors.New("unknown alert metric: " + metric)
	}
}



// GetActivePlatformAlertEvent 返回某规则当前未恢复的告警事件（无则 nil）。
func GetActivePlatformAlertEvent(ruleId int) (*PlatformAlertEvent, error) {
	var event PlatformAlertEvent
	err := DB.Where("rule_id = ? AND status = ?", ruleId, PlatformAlertStatusActive).
		Order("id desc").Limit(1).First(&event).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

// CreatePlatformAlertEvent 创建告警事件（active）。
func CreatePlatformAlertEvent(rule *PlatformAlertRule, value float64) (*PlatformAlertEvent, error) {
	message := fmt.Sprintf("rule %s: %s %s %.2f (actual %.4f)", rule.Name, rule.Metric, rule.Operator, rule.Threshold, value)
	event := &PlatformAlertEvent{
		RuleId:    rule.Id,
		Metric:    rule.Metric,
		RuleName:  rule.Name,
		Value:     value,
		Threshold: rule.Threshold,
		Operator:  rule.Operator,
		Status:    PlatformAlertStatusActive,
		Message:   message,
		CreatedAt: common.GetTimestamp(),
	}
	if err := DB.Create(event).Error; err != nil {
		return nil, err
	}
	return event, nil
}

// UpdatePlatformAlertEventValue 持续触发时刷新事件数值。
func UpdatePlatformAlertEventValue(eventId int, value float64) error {
	return DB.Model(&PlatformAlertEvent{}).Where("id = ?", eventId).
		Update("value", value).Error
}

// ResolveActivePlatformAlertEvent 指标恢复后把该规则的 active 事件标记 resolved。
// 返回是否真的恢复了事件。
func ResolveActivePlatformAlertEvent(ruleId int) (bool, error) {
	result := DB.Model(&PlatformAlertEvent{}).
		Where("rule_id = ? AND status = ?", ruleId, PlatformAlertStatusActive).
		Updates(map[string]interface{}{"status": PlatformAlertStatusResolved, "resolved_at": common.GetTimestamp()})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}


// CompareAlertThreshold 判断指标值是否触发阈值（operator 仅接受 > >= < <=）。
func CompareAlertThreshold(value, threshold float64, operator string) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	}
	return false
}
