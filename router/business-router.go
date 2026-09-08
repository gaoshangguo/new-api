package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
)

// registerBusinessRoutes keeps commercial workflows separate from legacy
// AdminAuth routes. Business-role users are ordinary system users and gain only
// these explicitly permission-gated endpoints.
func registerBusinessRoutes(apiRouter *gin.RouterGroup) {
	apiRouter.GET("/business/status", controller.GetBusinessPublicStatus)

	businessRoute := apiRouter.Group("/business")
	businessRoute.Use(middleware.UserAuth())
	{
		selfRoute := businessRoute.Group("/self")
		{
			selfRoute.GET("/ledger", controller.ListMyBalanceLedgers)
			selfRoute.GET("/ledger/export", controller.ExportMyBalanceLedgers)
			selfRoute.GET("/consumption", controller.ListMyBusinessConsumptions)
			selfRoute.GET("/consumption/export", controller.ExportMyBusinessConsumptions)
			// P0-04 用户控制台汇总（余额/冻结/错误率/待办）。
			selfRoute.GET("/dashboard", controller.GetUserDashboardSummary)
		}

		// P0-02 企业自助开户与资料维护（无需业务角色，仅限本人企业）。
		businessRoute.POST("/companies/self", controller.CreateSelfCompany)
		businessRoute.PUT("/companies/self/:id", controller.UpdateSelfCompany)
		businessRoute.GET("/self/company", controller.GetMyCompany)

		businessRoute.GET("/role-assignments/:user_id", middleware.RootAuth(), controller.GetUserBusinessRoles)
		businessRoute.PUT("/role-assignments/:user_id", middleware.RootAuth(), controller.SetUserBusinessRoles)
		businessRoute.GET("/sales/accounts/:id", middleware.RootAuth(), controller.GetSalesAccountProfile)
		businessRoute.PUT("/sales/accounts/:id", middleware.RootAuth(), controller.SaveSalesAccountProfile)
		businessRoute.PUT("/sales/accounts/:id/status", middleware.RootAuth(), controller.SetSalesAccountStatus)

		companyRoute := businessRoute.Group("/companies")
		{
			companyRoute.GET("", middleware.RequirePermission(authz.BusinessCompanyRead), controller.ListCompanies)
			companyRoute.POST("", middleware.RequirePermission(authz.BusinessCompanyCreate), controller.CreateCompany)
			companyRoute.PUT("/:id", middleware.RequirePermission(authz.BusinessCompanyUpdate), controller.UpdateCompany)
			companyRoute.GET("/:id/projects", middleware.RequirePermission(authz.BusinessProjectRead), controller.ListCompanyProjects)
			companyRoute.POST("/:id/assignments", middleware.RequirePermission(authz.BusinessCustomerAssignmentUpdate), controller.AssignCustomer)
			companyRoute.POST("/assignments/batch", middleware.RequirePermission(authz.BusinessCustomerAssignmentUpdate), controller.AssignCustomers)
		}

		projectRoute := businessRoute.Group("/projects")
		{
			projectRoute.POST("", middleware.RequirePermission(authz.BusinessProjectCreate), controller.CreateBusinessProject)
			projectRoute.GET("/:id", middleware.RequirePermission(authz.BusinessProjectRead), controller.GetBusinessProject)
			projectRoute.PUT("/:id", middleware.RequirePermission(authz.BusinessProjectUpdate), controller.UpdateBusinessProject)
			projectRoute.GET("/:id/summary", middleware.RequirePermission(authz.BusinessProjectRead), controller.GetBusinessProjectSummary)
			projectRoute.GET("/:id/tokens", middleware.RequirePermission(authz.BusinessProjectRead), controller.ListBusinessProjectTokens)
			projectRoute.POST("/:id/tokens/:token_id", middleware.RequirePermission(authz.BusinessProjectUpdate), controller.AssignBusinessProjectToken)
		}

		financeRoute := businessRoute.Group("/finance")
		{
			financeRoute.GET("/manual-credits", middleware.RequirePermission(authz.BusinessManualCreditRead), controller.ListManualCreditRequests)
			financeRoute.POST("/manual-credits", middleware.RequirePermission(authz.BusinessManualCreditCreate), controller.CreateManualCreditRequest)
			financeRoute.POST("/manual-credits/:id/approve", middleware.RequirePermission(authz.BusinessManualCreditApprove), controller.ApproveManualCreditRequest)
			financeRoute.POST("/manual-credits/:id/approve-escalation", middleware.RootAuth(), controller.ApproveEscalatedManualCreditRequest)
			financeRoute.POST("/manual-credits/:id/reject", middleware.RequirePermission(authz.BusinessManualCreditApprove), controller.RejectManualCreditRequest)
			financeRoute.GET("/budget-reconciliations", middleware.RequirePermission(authz.BusinessManualCreditRead), controller.ListBusinessProjectBudgetReconciliations)
			financeRoute.POST("/budget-reconciliations/:id/resolve", middleware.RequirePermission(authz.BusinessFinanceReconcile), controller.ReconcileBusinessProjectBudgetReservation)
			financeRoute.GET("/ledger", middleware.RequirePermission(authz.BusinessLedgerRead), controller.ListBalanceLedgers)
			financeRoute.GET("/ledger/export", middleware.RequirePermission(authz.BusinessLedgerRead), controller.ExportBalanceLedgers)
			financeRoute.GET("/consumption/export", middleware.RequirePermission(authz.BusinessLedgerRead), controller.ExportBusinessConsumptions)
			financeRoute.POST("/ledger/:id/reverse", middleware.RequirePermission(authz.BusinessManualCreditCreate), controller.ReverseBalanceLedger)
			financeRoute.POST("/companies/:company_id/opening-balance", middleware.RootAuth(), controller.InitializeBusinessOpeningBalance)
		}

		assignmentRoute := businessRoute.Group("/customer-assignments")
		{
			assignmentRoute.GET("", middleware.RequirePermission(authz.BusinessCustomerAssignmentRead), controller.ListCustomerAssignments)
		}

		salesRoute := businessRoute.Group("/sales")
		salesRoute.Use(middleware.RequirePermission(authz.BusinessSalesRead))
		{
			salesRoute.GET("/customers", controller.ListSalesCustomers)
			salesRoute.GET("/customers/:id/summary", controller.GetSalesCustomerSummary)
			salesRoute.GET("/customers/:id/logs", controller.GetSalesCustomerLogs)
			salesRoute.GET("/customers/:id/ledger/export", controller.ExportSalesCustomerLedger)
			salesRoute.GET("/customers/:id/consumption/export", controller.ExportSalesCustomerConsumption)
			salesRoute.GET("/customers/:id/projects", controller.GetSalesCustomerProjects)
			salesRoute.GET("/customers/:id/follow-ups", middleware.RequirePermission(authz.BusinessFollowUpRead), controller.ListSalesCustomerFollowUps)
			salesRoute.POST("/customers/:id/follow-ups", middleware.RequirePermission(authz.BusinessFollowUpCreate), controller.CreateSalesCustomerFollowUp)
			salesRoute.PUT("/follow-ups/:id", middleware.RequirePermission(authz.BusinessFollowUpUpdate), controller.UpdateSalesCustomerFollowUp)
			salesRoute.GET("/reminders", middleware.RequirePermission(authz.BusinessFollowUpRead), controller.ListSalesCustomerReminders)
		}

		operationsRoute := businessRoute.Group("/operations")
		{
			operationsRoute.GET("/overview", middleware.RequirePermission(authz.BusinessReportRead), controller.GetBusinessOperationsOverview)
			operationsRoute.GET("/users", middleware.RequirePermission(authz.BusinessOperationsRead), controller.ListPlatformReadOnlyUsers)
			operationsRoute.GET("/ledger", middleware.RequirePermission(authz.BusinessOperationsRead), controller.ListPlatformReadOnlyLedger)
			operationsRoute.GET("/consumption", middleware.RequirePermission(authz.BusinessOperationsRead), controller.ListPlatformReadOnlyConsumptions)
			operationsRoute.GET("/adjustments", middleware.RequirePermission(authz.BusinessOperationsRead), controller.ListPlatformReadOnlyAdjustments)
			operationsRoute.GET("/finance", middleware.RequirePermission(authz.BusinessOperationsRead), controller.GetPlatformReadOnlyFinanceOverview)
			operationsRoute.GET("/audit", middleware.RequirePermission(authz.BusinessOperationsRead), controller.ListBusinessAuditEvents)
			operationsRoute.GET("/channel-health", middleware.RequirePermission(authz.BusinessOperationsRead), controller.ListBusinessChannelHealth)
			operationsRoute.GET("/announcements", middleware.RequirePermission(authz.BusinessOperationsRead), controller.ListBusinessAnnouncements)
			operationsRoute.POST("/announcements", middleware.RequirePermission(authz.BusinessOperationsUpdate), controller.SaveBusinessAnnouncement)
			operationsRoute.PUT("/announcements", middleware.RequirePermission(authz.BusinessOperationsUpdate), controller.SaveBusinessAnnouncement)
		}
	}
}
