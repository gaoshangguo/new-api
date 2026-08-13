package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveOriginTaskRejectsProjectModelForbiddenRemix(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
	})
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Company{},
		&model.BusinessProject{},
		&model.Task{},
		&model.Channel{},
	))

	user := &model.User{Username: "remix-project-user", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(user).Error)
	company := &model.Company{Name: "Remix Enterprise", OwnerUserId: user.Id}
	require.NoError(t, db.Create(company).Error)
	project := &model.BusinessProject{
		CompanyId:   company.Id,
		OwnerUserId: user.Id,
		Name:        "Restricted video project",
		ModelLimits: `["sora-allowed"]`,
		Status:      model.BusinessProjectStatusEnabled,
	}
	require.NoError(t, db.Create(project).Error)
	token := &model.Token{
		UserId:    user.Id,
		ProjectId: project.Id,
		Key:       "remix-project-token",
		Name:      "remix-project-token",
		Status:    common.TokenStatusEnabled,
	}
	require.NoError(t, db.Create(token).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 71, Name: "video-channel", Key: "upstream-key", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "origin-video-task",
		UserId:    user.Id,
		ChannelId: 71,
		Properties: model.Properties{
			OriginModelName: "sora-forbidden",
		},
	}).Error)

	policy, err := model.LoadBusinessProjectRuntimePolicyForToken(token.Id, user.Id)
	require.NoError(t, err)
	require.NotNil(t, policy)

	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/origin-video-task/remix", nil)
	c.Params = append(c.Params, gin.Param{Key: "video_id", Value: "origin-video-task"})
	common.SetContextKey(c, constant.ContextKeyBusinessProjectRuntime, policy)

	taskErr := ResolveOriginTask(c, &relaycommon.RelayInfo{
		UserId:        user.Id,
		TokenId:       token.Id,
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: 71},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	})
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusForbidden, taskErr.StatusCode)
	assert.Equal(t, "project_model_forbidden", taskErr.Code)
}
