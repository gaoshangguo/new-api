package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSelfCompanyTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Company{}, &model.BusinessAuditEvent{}, &model.Log{}, &model.User{}))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	require.NoError(t, db.Create(&model.User{Id: 42, Username: "alice"}).Error)
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedis
	})
	return db
}

// P0-02：自助开户只允许保存主体资料，客户端传入的限流/并发字段必须被丢弃，
// 防止自设企业级限流绕过平台运营配置。
func TestCreateSelfCompanyIgnoresClientProvidedRateLimits(t *testing.T) {
	db := setupSelfCompanyTest(t)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	body := `{"name":"Self Co","credit_code":"91110108MA01ABC","contact_name":"Alice","contact_email":"a@example.com","rate_limit_rpm":999,"rate_limit_tpm":888,"max_concurrent_requests":7}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/business/self/company", strings.NewReader(body))
	ctx.Set("id", 42)
	ctx.Set("username", "alice")
	ctx.Set("role", 1)
	CreateSelfCompany(ctx)
	require.Equal(t, http.StatusOK, rec.Code)

	var company model.Company
	require.NoError(t, db.First(&company).Error)
	assert.Equal(t, 42, company.OwnerUserId)
	assert.Equal(t, "Self Co", company.Name)
	assert.Zero(t, company.RateLimitRPM)
	assert.Zero(t, company.RateLimitTPM)
	assert.Zero(t, company.MaxConcurrentRequests)
}

// P0-02：企业主自助更新不得修改平台侧配置的限流/并发字段。
func TestUpdateSelfCompanyPreservesOperationalRateLimits(t *testing.T) {
	db := setupSelfCompanyTest(t)
	require.NoError(t, db.Create(&model.Company{
		Name:                 "Existing Co",
		OwnerUserId:          42,
		RateLimitRPM:         10,
		RateLimitTPM:         100,
		MaxConcurrentRequests: 2,
	}).Error)
	var saved model.Company
	require.NoError(t, db.First(&saved).Error)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = []gin.Param{{Key: "id", Value: strconv.Itoa(saved.Id)}}
	body := `{"name":"Existing Co","contact_name":"Bob","rate_limit_rpm":999,"rate_limit_tpm":888,"max_concurrent_requests":7}`
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/business/self/company/"+strconv.Itoa(saved.Id), strings.NewReader(body))
	ctx.Set("id", 42)
	ctx.Set("username", "alice")
	ctx.Set("role", 1)
	UpdateSelfCompany(ctx)
	require.Equal(t, http.StatusOK, rec.Code)

	var updated model.Company
	require.NoError(t, db.First(&updated).Error)
	assert.Equal(t, "Bob", updated.ContactName)
	assert.Equal(t, 10, updated.RateLimitRPM)
	assert.EqualValues(t, 100, updated.RateLimitTPM)
	assert.Equal(t, 2, updated.MaxConcurrentRequests)
}

// P0-02：非企业主不能自助更新他人企业。
func TestUpdateSelfCompanyRejectsNonOwner(t *testing.T) {
	db := setupSelfCompanyTest(t)
	require.NoError(t, db.Create(&model.User{Id: 43, Username: "mallory"}).Error)
	require.NoError(t, db.Create(&model.Company{Name: "Owned Co", OwnerUserId: 42}).Error)
	var saved model.Company
	require.NoError(t, db.First(&saved).Error)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = []gin.Param{{Key: "id", Value: strconv.Itoa(saved.Id)}}
	body := `{"name":"Hijacked"}`
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/business/self/company/"+strconv.Itoa(saved.Id), strings.NewReader(body))
	ctx.Set("id", 43)
	ctx.Set("username", "mallory")
	ctx.Set("role", 1)
	UpdateSelfCompany(ctx)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "only the company owner can update their profile")

	var updated model.Company
	require.NoError(t, db.First(&updated).Error)
	assert.Equal(t, "Owned Co", updated.Name)
}
