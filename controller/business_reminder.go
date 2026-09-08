package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func ListMyBusinessProjectReminders(c *gin.Context) {
	projectID, err := businessReminderProjectID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	reminders, total, err := model.ListBusinessProjectReminders(c.GetInt("id"), projectID, !businessReminderIncludeResolved(c), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessReminderPageSuccess(c, reminders, total)
}

func ListBusinessProjectReminders(c *gin.Context) {
	projectID, err := businessReminderProjectID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	reminders, total, err := model.ListBusinessProjectReminders(0, projectID, !businessReminderIncludeResolved(c), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessReminderPageSuccess(c, reminders, total)
}

func TriggerBusinessReminderScan(c *gin.Context) {
	task, created, err := service.EnqueueBusinessReminderScan()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"created": created,
		"task":    task.ToResponse(),
	})
}

// ListSalesCustomerReminders returns active reminders for a sales user's
// assigned customers. Platform managers see all enterprise reminders; sales
// supervisors are scoped to their current assignments.
func ListSalesCustomerReminders(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	activeOnly := !businessReminderIncludeResolved(c)
	if canAccessAllSalesCustomers(c) {
		reminders, total, err := model.ListBusinessProjectReminders(0, 0, activeOnly, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
		if err != nil {
			common.ApiError(c, err)
			return
		}
		businessReminderPageSuccess(c, reminders, total)
		return
	}
	companies, _, err := model.ListSalesCompanies(c.GetInt("id"), 0, 1000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	companyIDs := make([]int, 0, len(companies))
	for _, company := range companies {
		companyIDs = append(companyIDs, company.Id)
	}
	if len(companyIDs) == 0 {
		businessReminderPageSuccess(c, []*model.BusinessProjectReminder{}, 0)
		return
	}
	reminders, total, err := model.ListBusinessProjectRemindersForCompanies(companyIDs, activeOnly, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	businessReminderPageSuccess(c, reminders, total)
}

func businessReminderProjectID(c *gin.Context) (int, error) {
	value := strings.TrimSpace(c.Query("project_id"))
	if value == "" {
		return 0, nil
	}
	projectID, err := strconv.Atoi(value)
	if err != nil || projectID <= 0 {
		return 0, strconv.ErrSyntax
	}
	return projectID, nil
}

func businessReminderIncludeResolved(c *gin.Context) bool {
	value := strings.TrimSpace(c.Query("include_resolved"))
	return strings.EqualFold(value, "true") || value == "1"
}

func businessReminderPageSuccess(c *gin.Context, items interface{}, total int64) {
	pageInfo := common.GetPageQuery(c)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}
