package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	BusinessReminderStateActive   = "active"
	BusinessReminderStateResolved = "resolved"

	BusinessReminderTypeLowBalance      = "low_balance"
	BusinessReminderTypeBudgetExceeded  = "budget_exceeded"
	BusinessReminderTypeHighFailureRate = "high_failure_rate"
	BusinessReminderTypeInactiveProject = "inactive_project"

	defaultBusinessReminderInactiveAfterSeconds int64 = 30 * 24 * 60 * 60
	defaultBusinessReminderErrorWindowSeconds   int64 = 24 * 60 * 60
	defaultBusinessReminderErrorRatePercent           = 10
	defaultBusinessReminderMinRequestCount      int64 = 10
)

var businessReminderTypes = []string{
	BusinessReminderTypeLowBalance,
	BusinessReminderTypeBudgetExceeded,
	BusinessReminderTypeHighFailureRate,
	BusinessReminderTypeInactiveProject,
}

// BusinessProjectReminder is a durable in-app notification. A project has at
// most one row for each reminder type, which keeps repeated scheduled scans
// and manual runs from creating a notification storm. State transitions retain
// the row as history: a resolved condition can be reactivated later without
// creating a duplicate record.
type BusinessProjectReminder struct {
	Id              int    `json:"id"`
	CompanyId       int    `json:"company_id" gorm:"index"`
	ProjectId       int    `json:"project_id" gorm:"uniqueIndex:idx_business_project_reminder_kind,priority:1;index"`
	OwnerUserId     int    `json:"owner_user_id" gorm:"index"`
	ReminderType    string `json:"reminder_type" gorm:"size:64;uniqueIndex:idx_business_project_reminder_kind,priority:2;index"`
	State           string `json:"state" gorm:"size:16;index"`
	Title           string `json:"title" gorm:"size:255"`
	Content         string `json:"content" gorm:"type:text"`
	MetricValue     int64  `json:"metric_value"`
	ThresholdValue  int64  `json:"threshold_value"`
	FirstDetectedAt int64  `json:"first_detected_at" gorm:"index"`
	LastDetectedAt  int64  `json:"last_detected_at" gorm:"index"`
	ResolvedAt      int64  `json:"resolved_at" gorm:"index"`
	LastNotifiedAt  int64  `json:"last_notified_at" gorm:"index"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (*BusinessProjectReminder) TableName() string {
	return "business_project_reminders"
}

// BusinessReminderScanOptions makes reminder criteria explicit and testable.
// The scheduler supplies conservative environment-backed defaults; callers
// that run a scan manually still use the same persistence path.
type BusinessReminderScanOptions struct {
	Now                  int64
	InactiveAfterSeconds int64
	ErrorWindowSeconds   int64
	ErrorRatePercent     int
	ErrorMinRequestCount int64
}

// BusinessReminderScanResult is safe to keep in a SystemTask result. It does
// not include owner settings, email addresses, or notification content.
type BusinessReminderScanResult struct {
	ScannedProjectCount        int `json:"scanned_project_count"`
	ActiveReminderCount        int `json:"active_reminder_count"`
	CreatedReminderCount       int `json:"created_reminder_count"`
	ResolvedReminderCount      int `json:"resolved_reminder_count"`
	NotificationCandidateCount int `json:"notification_candidate_count"`
	DeliveredNotificationCount int `json:"delivered_notification_count"`
	FailedNotificationCount    int `json:"failed_notification_count"`
}

// BusinessReminderNotificationCandidate is internal orchestration data for
// service.NotifyUser. It is intentionally not returned by API endpoints or
// written into SystemTask payloads/results.
type BusinessReminderNotificationCandidate struct {
	Reminder *BusinessProjectReminder
	Owner    *User
}

type businessReminderCondition struct {
	reminderType   string
	title          string
	content        string
	metricValue    int64
	thresholdValue int64
}

type businessReminderProjectMetrics struct {
	consumedQuota int64
	lastUsedAt    int64
	consumeCount  int64
	errorCount    int64
}

func normalizeBusinessReminderScanOptions(options BusinessReminderScanOptions) BusinessReminderScanOptions {
	if options.Now <= 0 {
		options.Now = common.GetTimestamp()
	}
	if options.InactiveAfterSeconds <= 0 {
		options.InactiveAfterSeconds = defaultBusinessReminderInactiveAfterSeconds
	}
	if options.ErrorWindowSeconds <= 0 {
		options.ErrorWindowSeconds = defaultBusinessReminderErrorWindowSeconds
	}
	if options.ErrorRatePercent <= 0 || options.ErrorRatePercent > 100 {
		options.ErrorRatePercent = defaultBusinessReminderErrorRatePercent
	}
	if options.ErrorMinRequestCount <= 0 {
		options.ErrorMinRequestCount = defaultBusinessReminderMinRequestCount
	}
	return options
}

// ScanBusinessProjectReminders evaluates every business project and records
// durable active/resolved state. It never sends a notification itself: delivery
// is performed only by the scheduled/manual SystemTask service path, so simply
// reading a reminder endpoint can never trigger email or another notifier.
func ScanBusinessProjectReminders(ctx context.Context, options BusinessReminderScanOptions) (BusinessReminderScanResult, []BusinessReminderNotificationCandidate, error) {
	options = normalizeBusinessReminderScanOptions(options)
	result := BusinessReminderScanResult{}
	candidates := make([]BusinessReminderNotificationCandidate, 0)

	projects := make([]BusinessProject, 0)
	if err := DB.WithContext(ctx).Order("id asc").Find(&projects).Error; err != nil {
		return result, nil, err
	}

	for index := range projects {
		if err := ctx.Err(); err != nil {
			return result, nil, err
		}
		project := projects[index]
		result.ScannedProjectCount++

		if project.Status != BusinessProjectStatusEnabled {
			resolved, err := resolveUndetectedBusinessReminders(ctx, project.Id, nil, options.Now)
			if err != nil {
				return result, nil, err
			}
			result.ResolvedReminderCount += resolved
			continue
		}

		var owner User
		err := DB.WithContext(ctx).
			Select("id", "email", "setting", "quota", "status").
			Where("id = ?", project.OwnerUserId).
			First(&owner).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				resolved, resolveErr := resolveUndetectedBusinessReminders(ctx, project.Id, nil, options.Now)
				if resolveErr != nil {
					return result, nil, resolveErr
				}
				result.ResolvedReminderCount += resolved
				continue
			}
			return result, nil, err
		}
		if owner.Status != common.UserStatusEnabled {
			resolved, err := resolveUndetectedBusinessReminders(ctx, project.Id, nil, options.Now)
			if err != nil {
				return result, nil, err
			}
			result.ResolvedReminderCount += resolved
			continue
		}

		metrics, err := loadBusinessReminderProjectMetrics(ctx, project, options)
		if err != nil {
			return result, nil, err
		}
		conditions := buildBusinessReminderConditions(project, owner, metrics, options)
		detectedTypes := make(map[string]struct{}, len(conditions))
		for _, condition := range conditions {
			detectedTypes[condition.reminderType] = struct{}{}
			reminder, created, reactivated, pendingNotification, err := activateBusinessReminder(ctx, project, owner, condition, options.Now)
			if err != nil {
				return result, nil, err
			}
			result.ActiveReminderCount++
			if created {
				result.CreatedReminderCount++
			}
			if pendingNotification || reactivated {
				candidates = append(candidates, BusinessReminderNotificationCandidate{Reminder: reminder, Owner: &owner})
			}
		}
		resolved, err := resolveUndetectedBusinessReminders(ctx, project.Id, detectedTypes, options.Now)
		if err != nil {
			return result, nil, err
		}
		result.ResolvedReminderCount += resolved
	}
	result.NotificationCandidateCount = len(candidates)
	return result, candidates, nil
}

func loadBusinessReminderProjectMetrics(ctx context.Context, project BusinessProject, options BusinessReminderScanOptions) (businessReminderProjectMetrics, error) {
	metrics := businessReminderProjectMetrics{}
	var consumption struct {
		ConsumedQuota int64
		LastUsedAt    int64
	}
	if err := DB.WithContext(ctx).Model(&BusinessConsumption{}).
		Where("project_id = ?", project.Id).
		Select("COALESCE(SUM(quota), 0) AS consumed_quota, COALESCE(MAX(created_at), 0) AS last_used_at").
		Scan(&consumption).Error; err != nil {
		return metrics, err
	}
	metrics.consumedQuota = consumption.ConsumedQuota
	metrics.lastUsedAt = consumption.LastUsedAt

	tokenIDs := make([]int, 0)
	if err := DB.WithContext(ctx).Unscoped().Model(&Token{}).
		Where("project_id = ?", project.Id).
		Pluck("id", &tokenIDs).Error; err != nil {
		return metrics, err
	}
	if len(tokenIDs) == 0 {
		return metrics, nil
	}

	logBaseQuery := func() *gorm.DB {
		return LOG_DB.WithContext(ctx).
			Model(&Log{}).
			Where("token_id IN ? AND created_at >= ?", tokenIDs, options.Now-options.ErrorWindowSeconds)
	}
	if err := logBaseQuery().Where("type = ?", LogTypeConsume).Count(&metrics.consumeCount).Error; err != nil {
		return metrics, err
	}
	if err := logBaseQuery().Where("type = ?", LogTypeError).Count(&metrics.errorCount).Error; err != nil {
		return metrics, err
	}
	return metrics, nil
}

func buildBusinessReminderConditions(project BusinessProject, owner User, metrics businessReminderProjectMetrics, options BusinessReminderScanOptions) []businessReminderCondition {
	conditions := make([]businessReminderCondition, 0, len(businessReminderTypes))
	if project.LowBalanceQuota > 0 && owner.Quota <= project.LowBalanceQuota {
		conditions = append(conditions, businessReminderCondition{
			reminderType:   BusinessReminderTypeLowBalance,
			title:          "项目余额低于预警阈值",
			content:        fmt.Sprintf("项目 %s 的账户余额为 %d，低于或等于预警阈值 %d。", project.Name, owner.Quota, project.LowBalanceQuota),
			metricValue:    int64(owner.Quota),
			thresholdValue: int64(project.LowBalanceQuota),
		})
	}
	if project.BudgetQuota > 0 && metrics.consumedQuota >= int64(project.BudgetQuota) {
		conditions = append(conditions, businessReminderCondition{
			reminderType:   BusinessReminderTypeBudgetExceeded,
			title:          "项目预算已达到上限",
			content:        fmt.Sprintf("项目 %s 已累计使用 %d 配额，达到预算上限 %d。", project.Name, metrics.consumedQuota, project.BudgetQuota),
			metricValue:    metrics.consumedQuota,
			thresholdValue: int64(project.BudgetQuota),
		})
	}
	attemptCount := metrics.consumeCount + metrics.errorCount
	if attemptCount >= options.ErrorMinRequestCount && metrics.errorCount > 0 && metrics.errorCount*100 >= attemptCount*int64(options.ErrorRatePercent) {
		conditions = append(conditions, businessReminderCondition{
			reminderType:   BusinessReminderTypeHighFailureRate,
			title:          "项目调用失败率异常",
			content:        fmt.Sprintf("项目 %s 在最近监测窗口内共有 %d 次请求，其中 %d 次失败，失败率达到 %d%%。", project.Name, attemptCount, metrics.errorCount, metrics.errorCount*100/attemptCount),
			metricValue:    metrics.errorCount,
			thresholdValue: int64(options.ErrorRatePercent),
		})
	}
	inactiveBefore := options.Now - options.InactiveAfterSeconds
	if project.CreatedAt > 0 && project.CreatedAt <= inactiveBefore && metrics.lastUsedAt <= inactiveBefore {
		conditions = append(conditions, businessReminderCondition{
			reminderType:   BusinessReminderTypeInactiveProject,
			title:          "项目长期未使用",
			content:        fmt.Sprintf("项目 %s 已超过 %d 天没有新的调用记录。", project.Name, options.InactiveAfterSeconds/(24*60*60)),
			metricValue:    metrics.lastUsedAt,
			thresholdValue: inactiveBefore,
		})
	}
	return conditions
}

func activateBusinessReminder(ctx context.Context, project BusinessProject, owner User, condition businessReminderCondition, now int64) (*BusinessProjectReminder, bool, bool, bool, error) {
	var reminder BusinessProjectReminder
	created := false
	reactivated := false
	pendingNotification := false
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).
			Where("project_id = ? AND reminder_type = ?", project.Id, condition.reminderType).
			First(&reminder).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			reminder = BusinessProjectReminder{
				CompanyId:       project.CompanyId,
				ProjectId:       project.Id,
				OwnerUserId:     owner.Id,
				ReminderType:    condition.reminderType,
				State:           BusinessReminderStateActive,
				Title:           condition.title,
				Content:         condition.content,
				MetricValue:     condition.metricValue,
				ThresholdValue:  condition.thresholdValue,
				FirstDetectedAt: now,
				LastDetectedAt:  now,
			}
			if err := tx.Create(&reminder).Error; err != nil {
				return err
			}
			created = true
			pendingNotification = true
			return nil
		}
		if err != nil {
			return err
		}
		reactivated = reminder.State != BusinessReminderStateActive
		pendingNotification = reactivated || reminder.LastNotifiedAt == 0
		updates := map[string]interface{}{
			"company_id":       project.CompanyId,
			"owner_user_id":    owner.Id,
			"state":            BusinessReminderStateActive,
			"title":            condition.title,
			"content":          condition.content,
			"metric_value":     condition.metricValue,
			"threshold_value":  condition.thresholdValue,
			"last_detected_at": now,
			"resolved_at":      0,
		}
		if err := tx.Model(&BusinessProjectReminder{}).Where("id = ?", reminder.Id).Updates(updates).Error; err != nil {
			return err
		}
		reminder.CompanyId = project.CompanyId
		reminder.OwnerUserId = owner.Id
		reminder.State = BusinessReminderStateActive
		reminder.Title = condition.title
		reminder.Content = condition.content
		reminder.MetricValue = condition.metricValue
		reminder.ThresholdValue = condition.thresholdValue
		reminder.LastDetectedAt = now
		reminder.ResolvedAt = 0
		return nil
	})
	if err != nil {
		return nil, false, false, false, err
	}
	return &reminder, created, reactivated, pendingNotification, nil
}

func resolveUndetectedBusinessReminders(ctx context.Context, projectID int, detectedTypes map[string]struct{}, now int64) (int, error) {
	if projectID <= 0 {
		return 0, nil
	}
	query := DB.WithContext(ctx).Model(&BusinessProjectReminder{}).
		Where("project_id = ? AND state = ? AND reminder_type IN ?", projectID, BusinessReminderStateActive, businessReminderTypes)
	if len(detectedTypes) > 0 {
		types := make([]string, 0, len(detectedTypes))
		for reminderType := range detectedTypes {
			types = append(types, reminderType)
		}
		query = query.Where("reminder_type NOT IN ?", types)
	}
	result := query.Updates(map[string]interface{}{
		"state":       BusinessReminderStateResolved,
		"resolved_at": now,
	})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// MarkBusinessProjectReminderNotified records only a successful notifier hand
// off. It is intentionally separate from the scanner transaction so a mail or
// webhook outage never blocks persistence of the in-app notification.
func MarkBusinessProjectReminderNotified(ctx context.Context, reminderID int, notifiedAt int64) error {
	if reminderID <= 0 {
		return errors.New("invalid business reminder id")
	}
	if notifiedAt <= 0 {
		notifiedAt = common.GetTimestamp()
	}
	result := DB.WithContext(ctx).Model(&BusinessProjectReminder{}).
		Where("id = ? AND state = ?", reminderID, BusinessReminderStateActive).
		Update("last_notified_at", notifiedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListBusinessProjectReminders reads only durable state. userID scopes the
// query to a project owner for the self-service in-app notification endpoint;
// privileged callers pass zero and are checked by the router permission.
func ListBusinessProjectReminders(userID, projectID int, activeOnly bool, startIdx, pageSize int) ([]*BusinessProjectReminder, int64, error) {
	query := DB.Model(&BusinessProjectReminder{}).Order("last_detected_at desc, id desc")
	if userID > 0 {
		query = query.Where("owner_user_id = ?", userID)
	}
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if activeOnly {
		query = query.Where("state = ?", BusinessReminderStateActive)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	reminders := make([]*BusinessProjectReminder, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&reminders).Error; err != nil {
		return nil, 0, err
	}
	return reminders, total, nil
}

// ListBusinessProjectRemindersForCompanies scopes active reminders to a set of
// enterprise companies. Sales supervisors read this through a scoped endpoint,
// so only reminders for their assigned customers are ever returned.
func ListBusinessProjectRemindersForCompanies(companyIDs []int, activeOnly bool, startIdx, pageSize int) ([]*BusinessProjectReminder, int64, error) {
	query := DB.Model(&BusinessProjectReminder{}).Where("company_id IN ?", companyIDs).Order("last_detected_at desc, id desc")
	if activeOnly {
		query = query.Where("state = ?", BusinessReminderStateActive)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	reminders := make([]*BusinessProjectReminder, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&reminders).Error; err != nil {
		return nil, 0, err
	}
	return reminders, total, nil
}
