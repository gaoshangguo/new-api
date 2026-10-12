package middleware

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUsageLogExportRateLimitCannotBeBypassedAcrossViewsOrByDisablingSearchLimits(t *testing.T) {
	useRateLimitMiniRedis(t)
	previous := common.SearchRateLimitEnable
	common.SearchRateLimitEnable = false
	t.Cleanup(func() { common.SearchRateLimitEnable = previous })
	r := gin.New()
	identity := func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Query("user"))
		c.Set("id", id)
	}
	respond := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	r.GET("/admin", identity, UsageLogExportRateLimit(), respond)
	r.GET("/self", identity, UsageLogExportRateLimit(), respond)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(r, "/admin?user=1", "192.0.2.1:1234").Code)
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(r, "/self?user=1", "192.0.2.2:1234").Code)
	limited := performRateLimitRequest(r, "/admin?user=1", "192.0.2.3:1234")
	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.NotEmpty(t, limited.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusNoContent, performRateLimitRequest(r, "/self?user=2", "192.0.2.1:1234").Code)
	assert.Equal(t, http.StatusUnauthorized, performRateLimitRequest(r, "/self", "192.0.2.1:1234").Code)
}
