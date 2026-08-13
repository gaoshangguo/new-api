package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"

	"github.com/gin-gonic/gin"
)

// registerBusinessReminderRoutes intentionally creates a separate /business
// group so the self-service notification feed remains available to an ordinary
// enterprise project owner, while global views and manual scans require their
// dedicated business permissions.
func registerBusinessReminderRoutes(apiRouter *gin.RouterGroup) {
	businessRoute := apiRouter.Group("/business")
	businessRoute.Use(middleware.UserAuth())
	{
		reminderRoute := businessRoute.Group("/reminders")
		{
			reminderRoute.GET("/self", controller.ListMyBusinessProjectReminders)
			reminderRoute.GET("", middleware.RequirePermission(authz.BusinessReminderRead), controller.ListBusinessProjectReminders)
			reminderRoute.POST("/scan", middleware.RequirePermission(authz.BusinessReminderRun), controller.TriggerBusinessReminderScan)
		}
	}
}
