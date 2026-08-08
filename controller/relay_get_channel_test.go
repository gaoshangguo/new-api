package controller

import (
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestGetChannelBareBranchCarriesRateLimitFields locks the regression where
// getChannel's ChannelMeta==nil branch returned a bare context-derived struct
// with RateLimitRPM/TPM always 0, so the channel-level rate limit never applied
// on the main relay path (first iteration, channel selected by Distribute
// before any relay handler ran InitChannelMeta).
func TestGetChannelBareBranchCarriesRateLimitFields(t *testing.T) {
	os.Setenv("CHANNEL_KEY_MASTER_KEY", strings.Repeat("k", 32))

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false // force the database path of CacheGetChannel
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	channel := &model.Channel{
		Type: 1, Name: "getChannel-rl", Key: "sk-getChannel-rl", Status: 1, Group: "default",
		RateLimitRPM: 7, RateLimitTPM: 90000,
	}
	require.NoError(t, db.Create(channel).Error)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("channel_id", channel.Id)
	c.Set("channel_type", channel.Type)
	c.Set("channel_name", channel.Name)
	c.Set("auto_ban", false)

	// ChannelMeta == nil: the main-path branch. InitChannelMeta only runs
	// inside the relay handlers, i.e. after the first channel selection.
	info := &relaycommon.RelayInfo{}
	got, apiErr := getChannel(c, info, nil)
	require.Nil(t, apiErr)
	require.NotNil(t, got)
	assert.Equal(t, channel.RateLimitRPM, got.RateLimitRPM)
	assert.Equal(t, channel.RateLimitTPM, got.RateLimitTPM)
	assert.Equal(t, channel.Id, got.Id)
}
