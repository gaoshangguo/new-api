package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrBusinessConsumptionImmutable = errors.New("business consumption records are append-only")

// BusinessConsumption is an append-only projection of a successful relay
// consumption for a project-owned token. It intentionally lives in the main
// database instead of LOG_DB: LOG_DB may be ClickHouse or a separately managed
// database, while the project and token ownership records are in DB.
//
// The request_id + token_id unique constraint makes the projection idempotent
// when a relay/logging path is retried. It is a consumption association, not a
// second billing source of truth; wallet and token quota settlement continues
// to be handled by the existing billing session.
type BusinessConsumption struct {
	Id        int    `json:"id"`
	CompanyId int    `json:"company_id" gorm:"index"`
	ProjectId int    `json:"project_id" gorm:"index"`
	UserId    int    `json:"user_id" gorm:"index"`
	TokenId   int    `json:"token_id" gorm:"uniqueIndex:idx_business_consumption_request_token,priority:2"`
	RequestId string `json:"request_id" gorm:"size:64;uniqueIndex:idx_business_consumption_request_token,priority:1"`
	Quota     int    `json:"quota"`
	ChannelId int    `json:"channel_id" gorm:"index"`
	ModelName string `json:"model_name" gorm:"size:128"`
	// PriceVersionId is the price version bound at pre-consume (P0-28); 0 means
	// the charge predates versioning.
	PriceVersionId int   `json:"price_version_id"`
	CreatedAt      int64 `json:"created_at" gorm:"autoCreateTime;index"`
}

func (BusinessConsumption) BeforeUpdate(*gorm.DB) error {
	return ErrBusinessConsumptionImmutable
}

func (BusinessConsumption) BeforeDelete(*gorm.DB) error {
	return ErrBusinessConsumptionImmutable
}

type RecordBusinessConsumptionParams struct {
	RequestId      string
	UserId         int
	TokenId        int
	Quota          int
	ChannelId      int
	ModelName      string
	CreatedAt      int64
	PriceVersionId int
}

type BusinessConsumptionFilter struct {
	CompanyId int
	UserId    int
	ProjectId int
	TokenId   int
	ModelName string
	StartAt   int64
	EndAt     int64
}

// ListCompanyConsumptionModelNames returns the distinct model names recorded
// in the consumption projection for one company, ordered for a stable export
// filter dropdown.
func ListCompanyConsumptionModelNames(companyID int) ([]string, error) {
	names := make([]string, 0)
	if companyID <= 0 {
		return names, nil
	}
	err := DB.Model(&BusinessConsumption{}).
		Where("company_id = ?", companyID).
		Distinct().
		Order("model_name asc").
		Pluck("model_name", &names).Error
	return names, err
}

type CompanyUsageSummary struct {
	Last7DaysQuota  int64 `json:"last_7_days_quota"`
	Last30DaysQuota int64 `json:"last_30_days_quota"`
	Last30DaysCalls int64 `json:"last_30_days_calls"`
	Last30DaysCredit int64 `json:"last_30_days_credit"`
	NewProjects30Days int64 `json:"new_projects_30_days"`
}

type CompanyModelUsage struct {
	ModelName string `json:"model_name"`
	Quota     int64  `json:"quota"`
	Calls     int64  `json:"calls"`
}

type PlatformReadOnlyConsumption struct {
	Id        int    `json:"id"`
	CompanyId int    `json:"company_id"`
	ProjectId int    `json:"project_id"`
	ModelName string `json:"model_name"`
	Quota     int    `json:"quota"`
	ChannelId int    `json:"channel_id"`
	CreatedAt int64  `json:"created_at"`
}

func ListPlatformReadOnlyConsumptions(startIdx, pageSize int) ([]*PlatformReadOnlyConsumption, int64, error) {
	query := DB.Model(&BusinessConsumption{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]*PlatformReadOnlyConsumption, 0)
	if err := query.Select("id, company_id, project_id, model_name, quota, channel_id, created_at").Order("id desc").Limit(pageSize).Offset(startIdx).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetCompanyUsageSummary(companyID int) (*CompanyUsageSummary, error) {
	if companyID <= 0 {
		return nil, errors.New("company id is required")
	}
	now := time.Now().Unix()
	sevenDaysAgo := now - int64((7 * 24 * time.Hour).Seconds())
	thirtyDaysAgo := now - int64((30 * 24 * time.Hour).Seconds())
	summary := &CompanyUsageSummary{}
	if err := DB.Model(&BusinessConsumption{}).Where("company_id = ? AND created_at >= ?", companyID, sevenDaysAgo).Select("COALESCE(SUM(quota), 0)").Scan(&summary.Last7DaysQuota).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&BusinessConsumption{}).Where("company_id = ? AND created_at >= ?", companyID, thirtyDaysAgo).Select("COALESCE(SUM(quota), 0) AS last_30_days_quota, COUNT(*) AS last_30_days_calls").Scan(summary).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&BalanceLedger{}).Where("company_id = ? AND created_at >= ? AND entry_type IN ?", companyID, thirtyDaysAgo, []string{LedgerEntryManualCredit, LedgerEntryCompensation}).Select("COALESCE(SUM(amount), 0)").Scan(&summary.Last30DaysCredit).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&BusinessProject{}).Where("company_id = ? AND created_at >= ?", companyID, thirtyDaysAgo).Count(&summary.NewProjects30Days).Error; err != nil {
		return nil, err
	}
	return summary, nil
}

func ListCompanyTopModels(companyID int, startAt int64, limit int) ([]CompanyModelUsage, error) {
	if companyID <= 0 || limit <= 0 || limit > 20 {
		return nil, errors.New("company id and a limit of one to 20 are required")
	}
	items := make([]CompanyModelUsage, 0)
	query := DB.Model(&BusinessConsumption{}).
		Select("model_name, COALESCE(SUM(quota), 0) AS quota, COUNT(*) AS calls").
		Where("company_id = ?", companyID)
	if startAt > 0 {
		query = query.Where("created_at >= ?", startAt)
	}
	if err := query.Group("model_name").Order("quota DESC").Limit(limit).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func businessConsumptionQuery(filter BusinessConsumptionFilter) *gorm.DB {
	query := DB.Model(&BusinessConsumption{}).Order("id desc")
	if filter.CompanyId > 0 {
		query = query.Where("company_id = ?", filter.CompanyId)
	}
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.ProjectId > 0 {
		query = query.Where("project_id = ?", filter.ProjectId)
	}
	if filter.TokenId > 0 {
		query = query.Where("token_id = ?", filter.TokenId)
	}
	if filter.ModelName != "" {
		query = query.Where("model_name = ?", filter.ModelName)
	}
	if filter.StartAt > 0 {
		query = query.Where("created_at >= ?", filter.StartAt)
	}
	if filter.EndAt > 0 {
		query = query.Where("created_at <= ?", filter.EndAt)
	}
	return query
}

func ExportBusinessConsumptions(filter BusinessConsumptionFilter, maxRows int) ([]*BusinessConsumption, error) {
	records := make([]*BusinessConsumption, 0)
	if err := businessConsumptionQuery(filter).Limit(maxRows).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func ListBusinessConsumptions(filter BusinessConsumptionFilter, startIdx, pageSize int) ([]*BusinessConsumption, int64, error) {
	query := businessConsumptionQuery(filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]*BusinessConsumption, 0)
	if err := query.Limit(pageSize).Offset(startIdx).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// RecordBusinessConsumptionAdjustmentParams describes a signed, append-only
// post-settlement adjustment. It is intentionally separate from normal relay
// consumption: callers use it only for server-derived operational deltas such
// as async task reconciliation, never for client-provided values.
type RecordBusinessConsumptionAdjustmentParams struct {
	RequestId string
	UserId    int
	TokenId   int
	Quota     int
	ChannelId int
	ModelName string
	CreatedAt int64
}

// RecordBusinessConsumption records a project-scoped consumption association.
// It never changes user or token quotas, and a token without a project is
// deliberately ignored so existing personal tokens remain fully compatible.
//
// A budget-capped project may record normal consumption only when its pending
// request reservation covers the final quota. Relay callers historically log
// and continue when post-settlement fails, so an under-reserved successful
// upstream response is instead committed as an explicit exceptional terminal
// state. That preserves the actual usage for reconciliation and makes all
// later project budget checks fail closed; it is never marked as a normal
// settlement.
func RecordBusinessConsumption(params RecordBusinessConsumptionParams) error {
	if params.RequestId == "" || params.UserId <= 0 || params.TokenId <= 0 || params.Quota < 0 {
		return nil
	}
	if params.Quota > common.MaxQuota {
		return fmt.Errorf("%w: consumption quota is out of range", ErrBusinessProjectBudgetExceeded)
	}

	createdAt := params.CreatedAt
	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}
	overBudgetPendingReconcile := false
	projectID := 0
	returnErr := DB.Transaction(func(tx *gorm.DB) error {
		// Lock the same project row used by ReserveBusinessProjectBudget so a
		// concurrent pre-consume cannot observe a gap between consumption
		// insertion and settling its temporary reservation.
		var token Token
		// Settlement is permitted to see a soft-deleted token: it was already
		// authorized and reserved before deletion, and leaving that hold pending
		// would hide actual usage forever. New authentication still uses the
		// normal scoped token lookup and remains blocked.
		if err := lockForUpdate(tx.Unscoped()).Select("id", "user_id", "project_id").
			Where("id = ? AND user_id = ? AND project_id > 0", params.TokenId, params.UserId).
			First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		var project BusinessProject
		if err := lockForUpdate(tx).Select("id", "company_id", "owner_user_id", "budget_quota").
			Where("id = ? AND owner_user_id = ?", token.ProjectId, params.UserId).
			First(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if project.CompanyId <= 0 {
			return nil
		}
		projectID = project.Id

		// The project lock serializes duplicate log delivery for this request.
		// If a prior delivery already inserted the append-only projection, do
		// not mutate its terminal reservation state again.
		var existing BusinessConsumption
		if err := tx.Where("request_id = ? AND token_id = ?", params.RequestId, token.Id).First(&existing).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var reservation BusinessProjectBudgetReservation
		reservationFound := false
		if err := lockForUpdate(tx).
			Where("project_id = ? AND token_id = ? AND user_id = ? AND request_id = ?", project.Id, token.Id, params.UserId, params.RequestId).
			First(&reservation).Error; err == nil {
			reservationFound = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// A terminal log with zero quota is common after an upstream timeout.
		// When no project hold exists, it carries no budget mutation and must not
		// create the capped-project reconciliation marker. If a hold does exist,
		// continue below so the zero actual usage atomically settles/releases it.
		if params.Quota == 0 && !reservationFound {
			return nil
		}

		normalSettlement := project.BudgetQuota == 0
		if project.BudgetQuota > 0 {
			normalSettlement = reservationFound &&
				reservation.Status == projectBudgetReservationStatusReserved &&
				reservation.ReservedQuota >= params.Quota
			overBudgetPendingReconcile = !normalSettlement
		}

		record := &BusinessConsumption{
			CompanyId:      project.CompanyId,
			ProjectId:      project.Id,
			UserId:         params.UserId,
			TokenId:        params.TokenId,
			RequestId:      params.RequestId,
			Quota:          params.Quota,
			ChannelId:      params.ChannelId,
			ModelName:      params.ModelName,
			PriceVersionId: params.PriceVersionId,
			CreatedAt:      createdAt,
		}
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "request_id"}, {Name: "token_id"}},
			DoNothing: true,
		}).Create(record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := appendBusinessConsumptionLedgerInTx(tx, record); err != nil {
			return err
		}

		if normalSettlement {
			return settleBusinessProjectBudgetReservationInTx(tx, project.Id, token.Id, params.RequestId)
		}
		if reservationFound {
			return tx.Model(&BusinessProjectBudgetReservation{}).
				Where("id = ?", reservation.Id).
				Update("status", projectBudgetReservationStatusOverBudgetPendingReconcile).Error
		}
		// A capped project should always have received an initial hold before
		// its billing session existed. If a legacy or bypass path did not, write
		// a terminal marker alongside the consumption instead of silently
		// treating the use as budget-compliant.
		return tx.Create(&BusinessProjectBudgetReservation{
			ProjectId:     project.Id,
			RequestId:     params.RequestId,
			UserId:        params.UserId,
			TokenId:       token.Id,
			ReservedQuota: 0,
			Status:        projectBudgetReservationStatusOverBudgetPendingReconcile,
		}).Error
	})
	if returnErr != nil {
		return returnErr
	}
	if overBudgetPendingReconcile {
		common.SysError(fmt.Sprintf("business project budget reconciliation required (projectId=%d, userId=%d, tokenId=%d, requestId=%s, actualQuota=%d)", projectID, params.UserId, params.TokenId, params.RequestId, params.Quota))
		return fmt.Errorf("%w: actual consumption exceeded or lacked its request reservation", ErrBusinessProjectBudgetExceeded)
	}
	return nil
}

// RecordBusinessConsumptionAdjustment writes a signed project consumption
// delta after a completed upstream operation. Positive deltas for capped
// projects were not known at preflight, so they are explicitly marked as
// pending reconciliation and block subsequent project requests. Negative
// deltas remain append-only refunds; if they would make the project usage
// negative, they also force reconciliation rather than creating a budget
// credit/bypass.
func RecordBusinessConsumptionAdjustment(params RecordBusinessConsumptionAdjustmentParams) error {
	params.RequestId = strings.TrimSpace(params.RequestId)
	if params.RequestId == "" || len(params.RequestId) > 64 || params.UserId <= 0 || params.TokenId <= 0 || params.Quota == 0 {
		return nil
	}
	if params.Quota > common.MaxQuota || params.Quota < -common.MaxQuota {
		return fmt.Errorf("%w: signed consumption adjustment is out of range", ErrBusinessProjectBudgetExceeded)
	}

	createdAt := params.CreatedAt
	if createdAt <= 0 {
		createdAt = common.GetTimestamp()
	}
	projectID := 0
	reconciliationRequired := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var token Token
		if err := lockForUpdate(tx.Unscoped()).Select("id", "user_id", "project_id").
			Where("id = ? AND user_id = ? AND project_id > 0", params.TokenId, params.UserId).
			First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		var project BusinessProject
		if err := lockForUpdate(tx).Select("id", "company_id", "owner_user_id", "budget_quota").
			Where("id = ? AND owner_user_id = ?", token.ProjectId, params.UserId).
			First(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if project.CompanyId <= 0 {
			return nil
		}
		projectID = project.Id

		var existing BusinessConsumption
		if err := tx.Where("request_id = ? AND token_id = ?", params.RequestId, token.Id).First(&existing).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if project.BudgetQuota > 0 {
			if params.Quota > 0 {
				reconciliationRequired = true
			} else {
				used, _, _, err := businessProjectBudgetUsage(tx, project.Id)
				if err != nil {
					return err
				}
				reconciliationRequired = used+int64(params.Quota) < 0
			}
		}

		record := &BusinessConsumption{
			CompanyId: project.CompanyId,
			ProjectId: project.Id,
			UserId:    params.UserId,
			TokenId:   token.Id,
			RequestId: params.RequestId,
			Quota:     params.Quota,
			ChannelId: params.ChannelId,
			ModelName: params.ModelName,
			CreatedAt: createdAt,
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		if err := appendBusinessConsumptionLedgerInTx(tx, record); err != nil {
			return err
		}
		if !reconciliationRequired {
			return nil
		}

		var reservation BusinessProjectBudgetReservation
		if err := lockForUpdate(tx).
			Where("project_id = ? AND request_id = ?", project.Id, params.RequestId).
			First(&reservation).Error; err == nil {
			return tx.Model(&BusinessProjectBudgetReservation{}).
				Where("id = ?", reservation.Id).
				Update("status", projectBudgetReservationStatusOverBudgetPendingReconcile).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&BusinessProjectBudgetReservation{
			ProjectId:     project.Id,
			RequestId:     params.RequestId,
			UserId:        params.UserId,
			TokenId:       token.Id,
			ReservedQuota: 0,
			Status:        projectBudgetReservationStatusOverBudgetPendingReconcile,
		}).Error
	})
	if err != nil {
		return err
	}
	if reconciliationRequired {
		common.SysError(fmt.Sprintf("business project signed consumption adjustment requires reconciliation (projectId=%d, userId=%d, tokenId=%d, requestId=%s, quota=%d)", projectID, params.UserId, params.TokenId, params.RequestId, params.Quota))
		return fmt.Errorf("%w: post-settlement adjustment requires reconciliation", ErrBusinessProjectBudgetExceeded)
	}
	return nil
}
