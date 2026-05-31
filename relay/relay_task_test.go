package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRelayTaskTestDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	service.InitHttpClient()
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}

	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Channel{}, &model.User{}, &model.Token{}, &model.UserSubscription{}))
	require.NoError(t, db.Create(&model.User{
		Id:       1001,
		Username: "relay-task-test-user",
		Password: "test-password",
		Status:   common.UserStatusEnabled,
		Quota:    1_000_000,
		Group:    "test-group",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:          501,
		UserId:      1001,
		Key:         "test-token",
		Status:      common.TokenStatusEnabled,
		Name:        "relay-task-test-token",
		ExpiredTime: -1,
		RemainQuota: 1_000_000,
	}).Error)
}

func seedVideoFetchTask(t *testing.T, userID int, group string) {
	t.Helper()

	now := time.Now().Unix()
	task := &model.Task{
		TaskID:          "task_public",
		UserId:          userID,
		Group:           group,
		ChannelId:       77,
		Platform:        constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Status:          model.TaskStatusSuccess,
		Progress:        "100%",
		CreatedAt:       now,
		UpdatedAt:       now,
		ClientRequestID: strPtr("req_fetch"),
		Properties: model.Properties{
			OriginModelName:   "doubao-seedance-2-0-260128",
			UpstreamModelName: "doubao-seedance-2-0-260128",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream_task_123",
		},
		Data: json.RawMessage(`{"status":"succeeded","content":{"video_url":"https://example.com/video.mp4"},"usage":{"completion_tokens":108900,"total_tokens":108900}}`),
	}
	require.NoError(t, model.DB.Create(task).Error)
}

func newVideoFetchContext(userID int, group string, taskID string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+taskID, nil)
	ctx.Params = gin.Params{{Key: "task_id", Value: taskID}}
	ctx.Set("id", userID)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, group)
	return ctx
}

func TestVideoFetchSameUserSameGroupSucceeds(t *testing.T) {
	setupRelayTaskTestDB(t)
	seedVideoFetchTask(t, 1001, "xiangpai")

	respBody, taskErr := videoFetchByIDRespBodyBuilder(newVideoFetchContext(1001, "xiangpai", "task_public"))

	require.Nil(t, taskErr)
	var body map[string]any
	require.NoError(t, common.Unmarshal(respBody, &body))
	require.Equal(t, "task_public", body["id"])
	require.Equal(t, "task_public", body["task_id"])
	require.Equal(t, "video", body["object"])
	require.Equal(t, "doubao-seedance-2-0-260128", body["model"])
	require.Equal(t, "completed", body["status"])
	require.EqualValues(t, 100, body["progress"])
	metadata, ok := body["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://example.com/video.mp4", metadata["url"])
	require.Equal(t, "req_fetch", metadata["client_request_id"])

	usage, ok := body["usage"].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, 108900, usage["completion_tokens"])
	require.EqualValues(t, 108900, usage["total_tokens"])
	bodyString := string(respBody)
	require.NotContains(t, bodyString, "token_id")
	require.NotContains(t, bodyString, "private_data")
	require.NotContains(t, bodyString, "ProjectName")
	require.NotContains(t, bodyString, "upstream_task_123")
}

func TestVideoFetchSameUserDifferentGroupReturnsNotFound(t *testing.T) {
	setupRelayTaskTestDB(t)
	seedVideoFetchTask(t, 1001, "henrytest")

	_, taskErr := videoFetchByIDRespBodyBuilder(newVideoFetchContext(1001, "xiangpai", "task_public"))

	require.NotNil(t, taskErr)
	require.Equal(t, "task_not_exist", taskErr.Code)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
}

func TestVideoFetchDifferentUserReturnsNotFound(t *testing.T) {
	setupRelayTaskTestDB(t)
	seedVideoFetchTask(t, 1001, "xiangpai")

	_, taskErr := videoFetchByIDRespBodyBuilder(newVideoFetchContext(2002, "xiangpai", "task_public"))

	require.NotNil(t, taskErr)
	require.Equal(t, "task_not_exist", taskErr.Code)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
}
