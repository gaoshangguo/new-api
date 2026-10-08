package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetMyNotifications(c *gin.Context) {
	before := 0
	if value := c.Query("before_id"); value != "" {
		var err error
		before, err = strconv.Atoi(value)
		if err != nil || before <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid cursor"})
			return
		}
	}
	inbox, err := model.ListUserNotifications(c.Request.Context(), c.GetInt("id"), before)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": inbox})
}

func ReadMyNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid notification id"})
		return
	}
	err = model.MarkUserNotificationRead(c.Request.Context(), c.GetInt("id"), id, common.GetTimestamp())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "notification not found"})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
