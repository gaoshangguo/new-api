package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenLimitFieldsPersist(t *testing.T) {
	truncateTables(t)
	user := &User{Username: "limits-user", Password: "p", Role: 1, Status: 1}
	require.NoError(t, DB.Create(user).Error)
	limits := `{"channels":[7,9]}`
	token := &Token{
		UserId: user.Id, Name: "limits-key", Key: "limits-key-0001", Status: 1,
		DailyQuota: 1000, MonthlyQuota: 20000, ChannelLimits: &limits,
		MaxConcurrentRequests: 3, RateLimitRPM: 60, RateLimitTPM: 100000,
	}
	require.NoError(t, DB.Create(token).Error)

	loaded, err := GetTokenByKey(token.Key, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), loaded.DailyQuota)
	assert.Equal(t, int64(20000), loaded.MonthlyQuota)
	require.NotNil(t, loaded.ChannelLimits)
	assert.Equal(t, limits, *loaded.ChannelLimits)
	assert.Equal(t, 3, loaded.MaxConcurrentRequests)
	assert.Equal(t, 60, loaded.RateLimitRPM)
	assert.Equal(t, int64(100000), loaded.RateLimitTPM)

	// 默认零值 = 不限
	plain := &Token{UserId: user.Id, Name: "plain-key", Key: "plain-key-0001", Status: 1}
	require.NoError(t, DB.Create(plain).Error)
	loadedPlain, err := GetTokenByKey(plain.Key, false)
	require.NoError(t, err)
	assert.Equal(t, int64(0), loadedPlain.DailyQuota)
	assert.Nil(t, loadedPlain.ChannelLimits)
	assert.Equal(t, 0, loadedPlain.MaxConcurrentRequests)
}

func TestChannelAndBusinessLimitFieldsPersist(t *testing.T) {
	truncateTables(t)
	// TestMain 仅迁移基础表；Company/BusinessProject 由本测试自迁移（与 business_test.go fixture 模式一致），
	// 且 truncateTables 不清理这两张表，测试结束后由本测试自行清空，避免残留行影响同包后续测试。
	require.NoError(t, DB.AutoMigrate(&Company{}, &BusinessProject{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM companies")
		DB.Exec("DELETE FROM business_projects")
	})

	channel := &Channel{Type: 1, Name: "limits-ch", Key: "sk-limits-ch", Status: 1, Group: "default", RateLimitRPM: 120, RateLimitTPM: 200000}
	require.NoError(t, channel.Insert())
	loaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, 120, loaded.RateLimitRPM)
	assert.Equal(t, int64(200000), loaded.RateLimitTPM)

	company := &Company{Name: "limits-company", OwnerUserId: 1, RateLimitRPM: 30, RateLimitTPM: 50000, MaxConcurrentRequests: 5}
	require.NoError(t, DB.Create(company).Error)
	var loadedCompany Company
	require.NoError(t, DB.Where("id = ?", company.Id).First(&loadedCompany).Error)
	assert.Equal(t, 30, loadedCompany.RateLimitRPM)
	assert.Equal(t, int64(50000), loadedCompany.RateLimitTPM)
	assert.Equal(t, 5, loadedCompany.MaxConcurrentRequests)

	project := &BusinessProject{CompanyId: company.Id, Name: "limits-project", OwnerUserId: 1, RateLimitRPM: 40, RateLimitTPM: 60000, MaxConcurrentRequests: 4}
	require.NoError(t, DB.Create(project).Error)
	var loadedProject BusinessProject
	require.NoError(t, DB.Where("id = ?", project.Id).First(&loadedProject).Error)
	assert.Equal(t, 40, loadedProject.RateLimitRPM)
	assert.Equal(t, int64(60000), loadedProject.RateLimitTPM)
	assert.Equal(t, 4, loadedProject.MaxConcurrentRequests)
}

func TestTokenGetChannelLimitIDs(t *testing.T) {
	valid := `{"channels":[7,9]}`
	token := &Token{ChannelLimits: &valid}
	ids, err := token.GetChannelLimitIDs()
	require.NoError(t, err)
	assert.Equal(t, map[int]struct{}{7: {}, 9: {}}, ids)

	// 空配置 = 不限
	empty := ``
	token2 := &Token{ChannelLimits: &empty}
	ids2, err := token2.GetChannelLimitIDs()
	require.NoError(t, err)
	assert.Nil(t, ids2)

	// 非法 JSON fail-closed
	bad := `{"channels":"gpt-4o"}`
	token3 := &Token{ChannelLimits: &bad}
	_, err = token3.GetChannelLimitIDs()
	require.Error(t, err)
}
