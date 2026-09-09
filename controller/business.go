package controller

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func businessAuditActor(c *gin.Context) model.BusinessActor {
	roleSnapshot := fmt.Sprintf("system:%d", c.GetInt("role"))
	if model.DB != nil {
		if roles, err := authz.UserBusinessRoles(model.DB, c.GetInt("id")); err == nil && len(roles) > 0 {
			roleSnapshot += ",business:" + strings.Join(roles, ",")
		}
	}
	return model.BusinessActor{
		UserId:       c.GetInt("id"),
		Username:     c.GetString("username"),
		RoleSnapshot: roleSnapshot,
		IP:           c.ClientIP(),
	}
}

func businessPathID(c *gin.Context, name string) (int, error) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid " + name)
	}
	return id, nil
}

func businessQueryID(c *gin.Context, name string) (int, error) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return 0, nil
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid " + name)
	}
	return id, nil
}

func balanceLedgerFilterFromQuery(c *gin.Context) (model.BalanceLedgerFilter, error) {
	userID, err := businessQueryID(c, "user_id")
	if err != nil {
		return model.BalanceLedgerFilter{}, err
	}
	projectID, err := businessQueryID(c, "project_id")
	if err != nil {
		return model.BalanceLedgerFilter{}, err
	}
	tokenID, err := businessQueryID(c, "token_id")
	if err != nil {
		return model.BalanceLedgerFilter{}, err
	}
	startAt, err := businessQueryUnixTime(c, "start_at")
	if err != nil {
		return model.BalanceLedgerFilter{}, err
	}
	endAt, err := businessQueryUnixTime(c, "end_at")
	if err != nil {
		return model.BalanceLedgerFilter{}, err
	}
	if startAt > 0 && endAt > 0 && startAt > endAt {
		return model.BalanceLedgerFilter{}, errors.New("start_at must not be after end_at")
	}
	return model.BalanceLedgerFilter{
		UserId:    userID,
		ProjectId: projectID,
		TokenId:   tokenID,
		EntryType: strings.TrimSpace(c.Query("entry_type")),
		ModelName: strings.TrimSpace(c.Query("model_name")),
		StartAt:   startAt,
		EndAt:     endAt,
	}, nil
}

func businessConsumptionFilterFromQuery(c *gin.Context) (model.BusinessConsumptionFilter, error) {
	ledgerFilter, err := balanceLedgerFilterFromQuery(c)
	if err != nil {
		return model.BusinessConsumptionFilter{}, err
	}
	return model.BusinessConsumptionFilter{
		UserId:    ledgerFilter.UserId,
		ProjectId: ledgerFilter.ProjectId,
		TokenId:   ledgerFilter.TokenId,
		ModelName: ledgerFilter.ModelName,
		StartAt:   ledgerFilter.StartAt,
		EndAt:     ledgerFilter.EndAt,
	}, nil
}

func businessQueryUnixTime(c *gin.Context, name string) (int64, error) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return 0, nil
	}
	unixTime, err := strconv.ParseInt(value, 10, 64)
	if err != nil || unixTime <= 0 {
		return 0, errors.New("invalid " + name)
	}
	return unixTime, nil
}

func businessPageSuccess(c *gin.Context, items interface{}, total int64) {
	pageInfo := common.GetPageQuery(c)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateCompany(c *gin.Context) {
	var company model.Company
	if err := c.ShouldBindJSON(&company); err != nil {
		common.ApiError(c, err)
		return
	}
	if company.OwnerUserId == 0 {
		company.OwnerUserId = c.GetInt("id")
	}
	actor := businessAuditActor(c)
	owner, err := model.GetUserById(company.OwnerUserId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	inviteSalesOwner, err := isSalesSupervisorUser(owner.InviterId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if inviteSalesOwner {
		err = model.CreateCompanyWithSalesAssignment(&company, owner.InviterId, "sales invitation registration", actor)
	} else {
		err = model.CreateCompany(&company, actor)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, company.OwnerUserId, "company.create", map[string]interface{}{"company_id": company.Id, "company_name": company.Name})
	common.ApiSuccess(c, company)
}

// CreateSelfCompany 用户自助企业开户（P0-02）：无需业务角色即可创建
// 由本人所有的企业（同一用户最多一个 active 企业），并保存主体资料：
// 企业名称、统一社会信用代码、联系人、邮箱、手机号、开票备注。
func CreateSelfCompany(c *gin.Context) {
	var company model.Company
	if err := c.ShouldBindJSON(&company); err != nil {
		common.ApiError(c, err)
		return
	}
	company.OwnerUserId = c.GetInt("id")
	company.Id = 0
	company.Name = strings.TrimSpace(company.Name)
	if company.Name == "" {
		common.ApiErrorMsg(c, "company name is required")
		return
	}
	if len(company.Name) > 128 || len(company.CreditCode) > 64 || len(company.ContactPhone) > 32 {
		common.ApiErrorMsg(c, "company name, credit code, or contact phone exceeds length limit")
		return
	}
	ownedCount, err := model.CountBusinessCompaniesOwnedBy(company.OwnerUserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ownedCount > 0 {
		common.ApiErrorMsg(c, "you already own an enterprise profile")
		return
	}
	// 自助开户仅允许保存主体资料；限流/并发等运营字段由平台侧配置，客户端不可自设。
	company.RateLimitRPM = 0
	company.RateLimitTPM = 0
	company.MaxConcurrentRequests = 0
	actor := businessAuditActor(c)
	company.OwnerUserId = c.GetInt("id")
	if err := model.CreateCompany(&company, actor); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, company.OwnerUserId, "company.self.create", map[string]interface{}{"company_id": company.Id, "company_name": company.Name})
	common.ApiSuccess(c, company)
}

// UpdateSelfCompany 企业主自助维护本人企业资料（P0-02）：仅允许 owner 修改
// 自己的企业，越权请求被拒绝。
func UpdateSelfCompany(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input model.Company
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetBusinessCompany(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing.OwnerUserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "only the company owner can update their profile")
		return
	}
	input.Id = id
	input.OwnerUserId = existing.OwnerUserId // 不允许转移所有权
	// 自助更新仅允许维护主体资料；限流/并发等运营字段保持平台侧配置不变。
	input.RateLimitRPM = existing.RateLimitRPM
	input.RateLimitTPM = existing.RateLimitTPM
	input.MaxConcurrentRequests = existing.MaxConcurrentRequests
	actor := businessAuditActor(c)
	if err := model.UpdateCompany(&input, actor); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, existing.OwnerUserId, "company.self.update", map[string]interface{}{"company_id": id})
	common.ApiSuccess(c, input)
}

// GetMyCompany 返回当前用户作为 owner 的企业资料（P0-02）；未开户时 data 为 null。
func GetMyCompany(c *gin.Context) {
	userId := c.GetInt("id")
	company, err := model.GetBusinessCompanyByOwner(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": nil})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, company)
}

// GetUserDashboardSummary 用户端控制台汇总（P0-04）：可用/冻结余额、用量、
// 24 小时错误率与待办提醒。
func GetUserDashboardSummary(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	var todayUsed, monthUsed int64
	_ = model.DB.Model(&model.BalanceLedger{}).
		Where("user_id = ? AND entry_type = ? AND created_at >= ?", userId, model.LedgerEntryConsumption, todayStart).
		Select("COALESCE(SUM(usage_quota), 0)").Scan(&todayUsed).Error
	_ = model.DB.Model(&model.BalanceLedger{}).
		Where("user_id = ? AND entry_type = ? AND created_at >= ?", userId, model.LedgerEntryConsumption, monthStart).
		Select("COALESCE(SUM(usage_quota), 0)").Scan(&monthUsed).Error
	errorRate := model.GetUserConsumeErrorRate(userId, 24)
	// 待办：本人（企业项目 owner）的活跃提醒；无企业/项目时为空。
	pendingItems, _, _ := model.ListBusinessProjectReminders(userId, 0, true, 0, 50)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"available_quota": user.Quota,
			"frozen_quota":    user.FrozenQuota,
			"used_quota":      user.UsedQuota,
			"request_count":   user.RequestCount,
			"today_used":      todayUsed,
			"month_used":      monthUsed,
			"error_rate_24h":  errorRate,
			"pending_items":   pendingItems,
		},
	})
}

func ListCompanies(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	companies, total, err := model.ListBusinessCompanies(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, companies, total)
}

func UpdateCompany(c *gin.Context) {
	companyID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var company model.Company
	if err := c.ShouldBindJSON(&company); err != nil {
		common.ApiError(c, err)
		return
	}
	company.Id = companyID
	if err := model.UpdateCompany(&company, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, company.OwnerUserId, "company.update", map[string]interface{}{"company_id": company.Id, "company_name": company.Name})
	common.ApiSuccess(c, company)
}

func ListCompanyProjects(c *gin.Context) {
	companyID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	projects, err := model.ListBusinessProjects(companyID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, projects)
}

func CreateBusinessProject(c *gin.Context) {
	var project model.BusinessProject
	if err := c.ShouldBindJSON(&project); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CreateBusinessProject(&project, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, project.OwnerUserId, "project.create", map[string]interface{}{"project_id": project.Id, "company_id": project.CompanyId})
	common.ApiSuccess(c, project)
}

func GetBusinessProject(c *gin.Context) {
	projectID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	project, err := model.GetBusinessProject(projectID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, project)
}

func UpdateBusinessProject(c *gin.Context) {
	projectID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		model.BusinessProject
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	project := input.BusinessProject
	project.Id = projectID
	if err := model.UpdateBusinessProject(&project, input.Reason, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, project.OwnerUserId, "project.update", map[string]interface{}{"project_id": project.Id, "company_id": project.CompanyId, "reason": input.Reason})
	common.ApiSuccess(c, project)
}

func GetBusinessProjectSummary(c *gin.Context) {
	projectID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	summary, err := model.GetBusinessProjectSummary(projectID, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func ListBusinessProjectTokens(c *gin.Context) {
	projectID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tokens, err := model.ListBusinessProjectTokens(projectID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, tokens)
}

func AssignBusinessProjectToken(c *gin.Context) {
	projectID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	tokenID, err := businessPathID(c, "token_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SetBusinessProjectToken(projectID, tokenID, input.Reason, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "project.token.assign", map[string]interface{}{"project_id": projectID, "token_id": tokenID, "reason": input.Reason})
	common.ApiSuccess(c, gin.H{"project_id": projectID, "token_id": tokenID})
}

func ListManualCreditRequests(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.ManualCreditRequest{}).Order("id desc")
	for _, key := range []string{"status", "entry_type"} {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			query = query.Where(key+" = ?", value)
		}
	}
	for _, key := range []string{"invoice_number", "external_finance_reference", "reconciliation_conclusion"} {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			query = query.Where(key+" LIKE ?", "%"+value+"%")
		}
	}
	for _, key := range []string{"user_id", "company_id", "project_id"} {
		id, err := businessQueryID(c, key)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if id > 0 {
			query = query.Where(key+" = ?", id)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	requests := make([]*model.ManualCreditRequest, 0)
	if err := query.Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&requests).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, requests, total)
}

func CreateManualCreditRequest(c *gin.Context) {
	var request model.ManualCreditRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.CreatedBy = c.GetInt("id")
	if err := model.CreateManualCreditRequest(&request, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, request.UserId, "finance.adjustment.create", map[string]interface{}{"request_id": request.Id, "entry_type": request.EntryType, "amount": request.Amount})
	common.ApiSuccess(c, request)
}

func ApproveManualCreditRequest(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ledger, err := model.ApproveManualCreditRequest(id, businessAuditActor(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, ledger.UserId, "finance.adjustment.approve", map[string]interface{}{"request_id": id, "ledger_id": ledger.Id, "amount": ledger.Amount})
	common.ApiSuccess(c, ledger)
}

func ApproveEscalatedManualCreditRequest(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	ledger, err := model.ApproveEscalatedManualCreditRequest(id, input.Reason, businessAuditActor(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, ledger.UserId, "finance.adjustment.duplicate_escalation.approve", map[string]interface{}{"request_id": id, "ledger_id": ledger.Id, "reason": input.Reason})
	common.ApiSuccess(c, ledger)
}

func RejectManualCreditRequest(c *gin.Context) {
	id, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	request, err := model.RejectManualCreditRequest(id, input.Reason, businessAuditActor(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, request.UserId, "finance.adjustment.reject", map[string]interface{}{"request_id": request.Id, "reason": request.RejectionReason})
	common.ApiSuccess(c, request)
}

func ListBusinessProjectBudgetReconciliations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.BusinessProjectBudgetReservation{}).Order("updated_at desc, id desc")
	if includeResolved := strings.TrimSpace(c.Query("include_resolved")); includeResolved != "true" && includeResolved != "1" {
		query = query.Where("status = ?", "over_budget_pending_reconciliation")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]*model.BusinessProjectBudgetReservation, 0)
	if err := query.Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&items).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, items, total)
}

func ReconcileBusinessProjectBudgetReservation(c *gin.Context) {
	reservationID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ReconcileBusinessProjectBudgetReservation(reservationID, input.Reason, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "project_budget.reconciliation.resolve", map[string]interface{}{"reservation_id": reservationID, "reason": input.Reason})
	common.ApiSuccess(c, gin.H{"id": reservationID, "status": "reconciled"})
}

func ListBalanceLedgers(c *gin.Context) {
	filter, err := balanceLedgerFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	ledgers, total, err := model.ListBalanceLedgersWithFilter(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, ledgers, total)
}

func ExportBalanceLedgers(c *gin.Context) {
	filter, err := balanceLedgerFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ledgers, err := model.ExportBalanceLedgers(filter, 10000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "finance.ledger.export", map[string]interface{}{
		"filters": filter,
		"rows":    len(ledgers),
	})
	writeBalanceLedgerCSV(c, ledgers)
}

func ExportBusinessConsumptions(c *gin.Context) {
	filter, err := businessConsumptionFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	records, err := model.ExportBusinessConsumptions(filter, 10000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "finance.consumption.export", map[string]interface{}{
		"filters": filter,
		"rows":    len(records),
	})
	writeBusinessConsumptionCSV(c, records)
}

func ListMyBalanceLedgers(c *gin.Context) {
	filter, err := balanceLedgerFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.UserId = c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	ledgers, total, err := model.ListBalanceLedgersWithFilter(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, ledgers, total)
}

func ListMyBusinessConsumptions(c *gin.Context) {
	filter, err := businessConsumptionFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.UserId = c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	records, total, err := model.ListBusinessConsumptions(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, records, total)
}

func ExportMyBalanceLedgers(c *gin.Context) {
	filter, err := balanceLedgerFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.UserId = c.GetInt("id")
	ledgers, err := model.ExportBalanceLedgers(filter, 10000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "customer.ledger.export", map[string]interface{}{
		"filters": filter,
		"rows":    len(ledgers),
	})
	writeBalanceLedgerCSV(c, ledgers)
}

func ExportMyBusinessConsumptions(c *gin.Context) {
	filter, err := businessConsumptionFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.UserId = c.GetInt("id")
	records, err := model.ExportBusinessConsumptions(filter, 10000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "customer.consumption.export", map[string]interface{}{
		"filters": filter,
		"rows":    len(records),
	})
	writeBusinessConsumptionCSV(c, records)
}

func writeBalanceLedgerCSV(c *gin.Context, ledgers []*model.BalanceLedger) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=balance-ledger.csv")
	c.Status(http.StatusOK)
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	_ = writer.Write([]string{
		"id", "company_id", "user_id", "project_id", "token_id", "request_id", "amount", "usage_quota", "funding_source", "balance_snapshot_available", "balance_before", "balance_after", "frozen_before", "frozen_after", "entry_type", "reference_type", "reference_id", "external_reference", "invoice_number", "external_finance_reference", "reconciliation_conclusion", "reason", "note", "created_by", "approved_by", "reverses_ledger_id", "created_at",
	})
	for _, ledger := range ledgers {
		_ = writer.Write([]string{
			strconv.Itoa(ledger.Id), strconv.Itoa(ledger.CompanyId), strconv.Itoa(ledger.UserId), strconv.Itoa(ledger.ProjectId), strconv.Itoa(ledger.TokenId), csvSafeValue(ledger.RequestId), strconv.Itoa(ledger.Amount), strconv.Itoa(ledger.UsageQuota), csvSafeValue(ledger.FundingSource), strconv.FormatBool(ledger.BalanceSnapshotAvailable), strconv.Itoa(ledger.BalanceBefore), strconv.Itoa(ledger.BalanceAfter), strconv.Itoa(ledger.FrozenBefore), strconv.Itoa(ledger.FrozenAfter), csvSafeValue(ledger.EntryType), csvSafeValue(ledger.ReferenceType), strconv.Itoa(ledger.ReferenceId), csvSafeValue(ledger.ExternalReference), csvSafeValue(ledger.InvoiceNumber), csvSafeValue(ledger.ExternalFinanceReference), csvSafeValue(ledger.ReconciliationConclusion), csvSafeValue(ledger.Reason), csvSafeValue(ledger.Note), strconv.Itoa(ledger.CreatedBy), strconv.Itoa(ledger.ApprovedBy), strconv.Itoa(ledger.ReversesLedgerId), strconv.FormatInt(ledger.CreatedAt, 10),
		})
	}
}

func writeBusinessConsumptionCSV(c *gin.Context, records []*model.BusinessConsumption) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=business-consumption.csv")
	c.Status(http.StatusOK)
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()
	_ = writer.Write([]string{
		"id", "company_id", "user_id", "project_id", "token_id", "request_id", "quota", "channel_id", "model_name", "created_at",
	})
	for _, record := range records {
		_ = writer.Write([]string{
			strconv.Itoa(record.Id), strconv.Itoa(record.CompanyId), strconv.Itoa(record.UserId), strconv.Itoa(record.ProjectId), strconv.Itoa(record.TokenId), csvSafeValue(record.RequestId), strconv.Itoa(record.Quota), strconv.Itoa(record.ChannelId), csvSafeValue(record.ModelName), strconv.FormatInt(record.CreatedAt, 10),
		})
	}
}

func csvSafeValue(value string) string {
	firstContent := strings.TrimLeft(value, " \t\r\n")
	if firstContent == "" {
		return value
	}
	switch firstContent[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func ReverseBalanceLedger(c *gin.Context) {
	ledgerID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	request, err := model.CreateLedgerReversalRequest(ledgerID, input.Reason, businessAuditActor(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, request.UserId, "ledger.reversal.create", map[string]interface{}{"ledger_id": ledgerID, "request_id": request.Id})
	common.ApiSuccess(c, request)
}

func InitializeBusinessOpeningBalance(c *gin.Context) {
	companyID, err := businessPathID(c, "company_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	ledger, err := model.InitializeBusinessOpeningBalance(companyID, input.Reason, businessAuditActor(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, ledger.UserId, "finance.opening_balance.create", map[string]interface{}{"company_id": companyID, "ledger_id": ledger.Id, "reason": input.Reason})
	common.ApiSuccess(c, ledger)
}

func AssignCustomer(c *gin.Context) {
	companyID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		SalesUserId int    `json:"sales_user_id"`
		Reason      string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireSalesSupervisor(input.SalesUserId); err != nil {
		common.ApiError(c, err)
		return
	}
	assignment, err := model.AssignCustomer(companyID, input.SalesUserId, input.Reason, businessAuditActor(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, assignment.CustomerUserId, "customer.assignment.update", map[string]interface{}{"company_id": companyID, "sales_user_id": input.SalesUserId, "reason": input.Reason})
	common.ApiSuccess(c, assignment)
}

func AssignCustomers(c *gin.Context) {
	var input struct {
		CompanyIds  []int  `json:"company_ids"`
		SalesUserId int    `json:"sales_user_id"`
		Reason      string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireSalesSupervisor(input.SalesUserId); err != nil {
		common.ApiError(c, err)
		return
	}
	assignments, err := model.AssignCustomers(input.CompanyIds, input.SalesUserId, input.Reason, businessAuditActor(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, input.SalesUserId, "customer.assignment.batch_update", map[string]interface{}{"company_ids": input.CompanyIds, "reason": input.Reason})
	common.ApiSuccess(c, assignments)
}

func requireSalesSupervisor(userID int) error {
	isSalesSupervisor, err := isSalesSupervisorUser(userID)
	if err != nil {
		return err
	}
	if !isSalesSupervisor {
		return errors.New("customer owner must have the sales supervisor role")
	}
	return nil
}

func isSalesSupervisorUser(userID int) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	roles, err := authz.UserBusinessRoles(model.DB, userID)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if role == authz.BusinessRoleSalesSupervisor {
			return true, nil
		}
	}
	return false, nil
}

func ListCustomerAssignments(c *gin.Context) {
	companyID, err := businessQueryID(c, "company_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	assignments, total, err := model.ListCustomerAssignments(companyID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, assignments, total)
}

func ListSalesCustomers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	if canAccessAllSalesCustomers(c) {
		companies, total, err := model.ListBusinessCompanies(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
		if err != nil {
			common.ApiError(c, err)
			return
		}
		businessPageSuccess(c, salesCustomerViews(companies), total)
		return
	}
	companies, total, err := model.ListSalesCompanies(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, salesCustomerViews(companies), total)
}

func GetSalesAccountProfile(c *gin.Context) {
	userID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	profile, err := model.GetSalesAccountProfile(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func SaveSalesAccountProfile(c *gin.Context) {
	userID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var profile model.SalesAccountProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		common.ApiError(c, err)
		return
	}
	roles, err := authz.UserBusinessRoles(model.DB, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	isSalesSupervisor := false
	for _, role := range roles {
		if role == authz.BusinessRoleSalesSupervisor {
			isSalesSupervisor = true
			break
		}
	}
	if !isSalesSupervisor {
		common.ApiError(c, errors.New("sales account must have the sales supervisor role"))
		return
	}
	profile.UserId = userID
	if err := model.SaveSalesAccountProfile(&profile, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "sales.account.profile.update", map[string]interface{}{"user_id": userID})
	common.ApiSuccess(c, profile)
}

func SetSalesAccountStatus(c *gin.Context) {
	userID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Status int `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if input.Status != common.UserStatusEnabled && input.Status != common.UserStatusDisabled {
		common.ApiError(c, errors.New("invalid sales account status"))
		return
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	user.Status = input.Status
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "sales.account.status.update", map[string]interface{}{"user_id": userID, "status": input.Status})
	common.ApiSuccess(c, gin.H{"user_id": userID, "status": input.Status})
}

func ListSalesAccounts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	accounts, total, err := model.ListSalesAccounts(
		authz.RoleSubject(authz.BusinessRoleSalesSupervisor),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, accounts, total)
}

// CreateSalesAccount provisions an ordinary user, grants it the sales
// supervisor business role, and stores the commercial profile in one
// transaction so an account never exists half-configured.
func CreateSalesAccount(c *gin.Context) {
	var input struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Department  string `json:"department"`
		Region      string `json:"region"`
		Note        string `json:"note"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" || input.Password == "" {
		common.ApiError(c, errors.New("username and password are required"))
		return
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}
	user := model.User{
		Username:    input.Username,
		Password:    input.Password,
		DisplayName: input.DisplayName,
		Email:       input.Email,
		Role:        common.RoleCommonUser,
	}
	if err := common.Validate.Struct(&user); err != nil {
		common.ApiError(c, err)
		return
	}
	actor := businessAuditActor(c)
	createdID := 0
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := user.InsertWithTx(tx, 0); err != nil {
			return err
		}
		createdID = user.Id
		if err := authz.SetUserBusinessRolesInTx(tx, createdID, []string{authz.BusinessRoleSalesSupervisor}); err != nil {
			return err
		}
		profile := model.SalesAccountProfile{
			UserId:     createdID,
			Department: input.Department,
			Region:     input.Region,
			Note:       input.Note,
		}
		return model.SaveSalesAccountProfileInTx(tx, &profile, actor)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	user.FinishInsert(0)
	if err := authz.ReloadPolicy(); err != nil {
		common.SysError("failed to reload authorization policy after sales account creation: " + err.Error())
	}
	recordManageAuditFor(c, createdID, "sales.account.create", map[string]interface{}{"username": user.Username})
	common.ApiSuccess(c, gin.H{"user_id": createdID})
}

// ListSalesAccountCustomers returns the active enterprise customers owned by
// one sales account. Root can inspect any account; the data itself is the same
// narrow company projection the sales workspace uses.
func ListSalesAccountCustomers(c *gin.Context) {
	salesUserID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireSalesSupervisor(salesUserID); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	companies, total, err := model.ListSalesCompanies(salesUserID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, companies, total)
}

func GetSalesCustomerSummary(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	projects, err := model.ListBusinessProjects(company.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	balance, err := model.GetCompanySalesBalance(company.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usage, err := model.GetCompanyUsageSummary(company.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	topModels, err := model.ListCompanyTopModels(company.Id, time.Now().Add(-30*24*time.Hour).Unix(), 5)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	lowBalanceProjectCount := 0
	if balance != nil {
		for _, project := range projects {
			if project.LowBalanceQuota > 0 && *balance <= project.LowBalanceQuota {
				lowBalanceProjectCount++
			}
		}
	}
	common.ApiSuccess(c, gin.H{
		"customer":          salesCustomerView{Id: company.Id, Name: company.Name},
		"balance":           balance,
		"balance_available": balance != nil,
		"projects":          salesProjectViews(projects),
		"usage":             usage,
		"top_models":        topModels,
		"alerts": gin.H{
			"low_balance_project_count": lowBalanceProjectCount,
		},
	})
}

func GetSalesCustomerLogs(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	projectID, err := businessQueryID(c, "project_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if projectID > 0 && !companyOwnsProject(company, projectID) {
		common.ApiError(c, errors.New("project does not belong to this customer"))
		return
	}
	startAt, err := businessQueryUnixTime(c, "start_at")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	endAt, err := businessQueryUnixTime(c, "end_at")
	if err != nil || (startAt > 0 && endAt > 0 && startAt > endAt) {
		if err == nil {
			err = errors.New("start_at must not be after end_at")
		}
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	var logs []*model.BusinessUsageLog
	var total int64
	if projectID > 0 {
		logs, total, err = model.ListMaskedBusinessLogs(company.OwnerUserId, projectID, startAt, endAt, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	} else {
		// Sales ownership is company-scoped, never user-account-scoped. A
		// customer account may also own personal or other-company tokens, so
		// an omitted project filter must still be limited to this company's
		// project tokens.
		logs, total, err = model.ListMaskedBusinessCompanyLogs(company.OwnerUserId, company.Id, startAt, endAt, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]salesUsageLog, 0, len(logs))
	for _, log := range logs {
		items = append(items, salesUsageLog{
			Id:               log.Id,
			CreatedAt:        log.CreatedAt,
			Type:             log.Type,
			ModelName:        log.ModelName,
			Quota:            log.Quota,
			PromptTokens:     log.PromptTokens,
			CompletionTokens: log.CompletionTokens,
			UseTime:          log.UseTime,
			IsStream:         log.IsStream,
			TokenIdentifier:  log.TokenIdentifier,
			RequestId:        log.RequestId,
		})
	}
	businessPageSuccess(c, items, total)
}

func ExportSalesCustomerLedger(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter, err := balanceLedgerFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.CompanyId = company.Id
	ledgers, err := model.ExportBalanceLedgers(filter, 10000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "sales.customer.ledger.export", map[string]interface{}{
		"company_id": company.Id,
		"filters":    filter,
		"rows":       len(ledgers),
	})
	writeBalanceLedgerCSV(c, ledgers)
}

func ExportSalesCustomerConsumption(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter, err := businessConsumptionFilterFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.CompanyId = company.Id
	records, err := model.ExportBusinessConsumptions(filter, 10000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "sales.customer.consumption.export", map[string]interface{}{
		"company_id": company.Id,
		"filters":    filter,
		"rows":       len(records),
	})
	writeBusinessConsumptionCSV(c, records)
}

// GetSalesCustomerExportModelOptions returns the distinct model names a sales
// user may filter on when exporting the customer ledger or consumption. It is
// company-scoped like every other sales customer endpoint.
func GetSalesCustomerExportModelOptions(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	models, err := model.ListCompanyConsumptionModelNames(company.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"company_id": company.Id, "models": models})
}

func ListSalesCustomerFollowUps(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListBusinessFollowUps(company.Id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, items, total)
}

func CreateSalesCustomerFollowUp(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var followUp model.BusinessFollowUp
	if err := c.ShouldBindJSON(&followUp); err != nil {
		common.ApiError(c, err)
		return
	}
	followUp.CompanyId = company.Id
	followUp.CustomerUserId = company.OwnerUserId
	if canAccessAllSalesCustomers(c) {
		if followUp.SalesUserId <= 0 {
			common.ApiError(c, errors.New("sales owner is required for a platform-created follow-up"))
			return
		}
	} else {
		followUp.SalesUserId = c.GetInt("id")
	}
	assigned, err := model.IsSalesCompany(followUp.SalesUserId, company.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !assigned {
		common.ApiError(c, errors.New("follow-up owner is not assigned to this customer"))
		return
	}
	if err := model.CreateBusinessFollowUp(&followUp, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, company.OwnerUserId, "sales.follow_up.create", map[string]interface{}{"follow_up_id": followUp.Id, "company_id": company.Id})
	common.ApiSuccess(c, followUp)
}

func UpdateSalesCustomerFollowUp(c *gin.Context) {
	followUpID, err := businessPathID(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetBusinessFollowUp(followUpID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if _, err := scopedSalesCompanyByID(c, existing.CompanyId); err != nil {
		common.ApiError(c, err)
		return
	}
	if !canAccessAllSalesCustomers(c) && existing.SalesUserId != c.GetInt("id") {
		common.ApiError(c, errors.New("follow-up is owned by another sales user"))
		return
	}
	var input struct {
		Title  string `json:"title"`
		Note   string `json:"note"`
		Status string `json:"status"`
		DueAt  int64  `json:"due_at"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	existing.Title = input.Title
	existing.Note = input.Note
	existing.Status = input.Status
	existing.DueAt = input.DueAt
	if err := model.UpdateBusinessFollowUp(existing, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, existing.CustomerUserId, "sales.follow_up.update", map[string]interface{}{"follow_up_id": existing.Id, "company_id": existing.CompanyId})
	common.ApiSuccess(c, existing)
}

func GetSalesCustomerProjects(c *gin.Context) {
	company, err := scopedSalesCompany(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	projects, err := model.ListBusinessProjects(company.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, salesProjectViews(projects))
}

func GetBusinessOperationsOverview(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	overview, err := model.GetBusinessOperationsOverview(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}

func ListPlatformReadOnlyUsers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListPlatformReadOnlyUsers(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "operations.users.read", map[string]interface{}{"rows": len(items)})
	businessPageSuccess(c, items, total)
}

func ListPlatformReadOnlyLedger(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListPlatformReadOnlyLedger(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "operations.ledger.read", map[string]interface{}{"rows": len(items)})
	businessPageSuccess(c, items, total)
}

func ListPlatformReadOnlyConsumptions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListPlatformReadOnlyConsumptions(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "operations.consumption.read", map[string]interface{}{"rows": len(items)})
	businessPageSuccess(c, items, total)
}

func ListPlatformReadOnlyAdjustments(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListPlatformReadOnlyAdjustments(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "operations.adjustments.read", map[string]interface{}{"rows": len(items)})
	businessPageSuccess(c, items, total)
}

func GetPlatformReadOnlyFinanceOverview(c *gin.Context) {
	overview, err := model.GetPlatformReadOnlyFinanceOverview()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "operations.finance.read", nil)
	common.ApiSuccess(c, overview)
}

func ListBusinessAuditEvents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	events, total, err := model.ListBusinessAuditEvents(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessPageSuccess(c, events, total)
}

func ListBusinessChannelHealth(c *gin.Context) {
	items, err := model.ListBusinessChannelHealth()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func ListBusinessAnnouncements(c *gin.Context) {
	announcements, err := model.ListBusinessAnnouncements(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, announcements)
}

func SaveBusinessAnnouncement(c *gin.Context) {
	var announcement model.BusinessAnnouncement
	if err := c.ShouldBindJSON(&announcement); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveBusinessAnnouncement(&announcement, businessAuditActor(c)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "announcement.save", map[string]interface{}{"announcement_id": announcement.Id, "status": announcement.Status})
	common.ApiSuccess(c, announcement)
}

func GetBusinessPublicStatus(c *gin.Context) {
	announcements, err := model.ListBusinessAnnouncements(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channels, err := model.ListBusinessChannelHealth()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	health := map[int]int{}
	for _, channel := range channels {
		health[channel.Status]++
	}
	generalSetting := operation_setting.GetGeneralSetting()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"channel_status_counts": health,
		"announcements":         announcements,
		// P0-01 商务联系方式（后台配置，展示在首页/页脚）。
		"contact": gin.H{
			"email":  generalSetting.BusinessContactEmail,
			"phone":  generalSetting.BusinessContactPhone,
			"wechat": generalSetting.BusinessContactWechat,
		},
	}})
}

func GetUserBusinessRoles(c *gin.Context) {
	userID, err := businessPathID(c, "user_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	roles, err := authz.UserBusinessRoles(model.DB, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"user_id": userID, "roles": roles})
}

func SetUserBusinessRoles(c *gin.Context) {
	userID, err := businessPathID(c, "user_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var input struct {
		Roles  []string `json:"roles"`
		Reason string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		common.ApiError(c, errors.New("role assignment reason is required"))
		return
	}
	actor := businessAuditActor(c)
	var previousRoles []string
	var assignedRoles []string
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		previousRoles, err = authz.UserBusinessRoles(tx, userID)
		if err != nil {
			return err
		}
		if err := authz.SetUserBusinessRolesInTx(tx, userID, input.Roles); err != nil {
			return err
		}
		assignedRoles, err = authz.UserBusinessRoles(tx, userID)
		if err != nil {
			return err
		}
		return model.RecordBusinessAuditEventInTx(tx, actor, "business.roles.update", "user", userID, input.Reason, previousRoles, assignedRoles)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := authz.ReloadPolicy(); err != nil {
		common.SysError("failed to reload authorization policy after business role assignment: " + err.Error())
	}
	recordManageAuditFor(c, userID, "business.roles.update", map[string]interface{}{"before_roles": previousRoles, "roles": assignedRoles, "reason": input.Reason})
	common.ApiSuccess(c, gin.H{"user_id": userID, "roles": assignedRoles})
}

func scopedSalesCompany(c *gin.Context) (*model.Company, error) {
	companyID, err := businessPathID(c, "id")
	if err != nil {
		return nil, err
	}
	return scopedSalesCompanyByID(c, companyID)
}

func scopedSalesCompanyByID(c *gin.Context, companyID int) (*model.Company, error) {
	if companyID <= 0 {
		return nil, errors.New("invalid company id")
	}
	var company model.Company
	if err := model.DB.First(&company, companyID).Error; err != nil {
		return nil, err
	}
	if canAccessAllSalesCustomers(c) {
		return &company, nil
	}
	allowed, err := model.IsSalesCompany(c.GetInt("id"), company.Id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errors.New("customer is outside your assigned scope")
	}
	return &company, nil
}

func canAccessAllSalesCustomers(c *gin.Context) bool {
	return authz.HasBusinessRole(c.GetInt("id"), c.GetInt("role"), authz.BusinessRolePlatformManager)
}

func companyOwnsProject(company *model.Company, projectID int) bool {
	project, err := model.GetBusinessProject(projectID)
	return err == nil && project.CompanyId == company.Id
}

// The sales workspace deliberately uses narrow projections. Financial
// vouchers, billing notes, contact details, and routing configuration stay in
// platform/finance APIs even when a sales user is assigned to the customer.
type salesCustomerView struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type salesProjectView struct {
	Id              int    `json:"id"`
	Name            string `json:"name"`
	BudgetQuota     int    `json:"budget_quota"`
	LowBalanceQuota int    `json:"low_balance_quota"`
	Status          int    `json:"status"`
}

type salesUsageLog struct {
	Id               int    `json:"id"`
	CreatedAt        int64  `json:"created_at"`
	Type             int    `json:"type"`
	ModelName        string `json:"model_name"`
	Quota            int    `json:"quota"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	UseTime          int    `json:"use_time"`
	IsStream         bool   `json:"is_stream"`
	TokenIdentifier  string `json:"token_identifier"`
	RequestId        string `json:"request_id"`
}

func salesCustomerViews(companies []*model.Company) []salesCustomerView {
	items := make([]salesCustomerView, 0, len(companies))
	for _, company := range companies {
		items = append(items, salesCustomerView{Id: company.Id, Name: company.Name})
	}
	return items
}

func salesProjectViews(projects []*model.BusinessProject) []salesProjectView {
	items := make([]salesProjectView, 0, len(projects))
	for _, project := range projects {
		items = append(items, salesProjectView{
			Id:              project.Id,
			Name:            project.Name,
			BudgetQuota:     project.BudgetQuota,
			LowBalanceQuota: project.LowBalanceQuota,
			Status:          project.Status,
		})
	}
	return items
}
