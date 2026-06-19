package controller

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModerationDiagnoseTestDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
	})
}

func createModerationDiagnoseTask(t *testing.T, taskID string, channelID int, data map[string]any) model.Task {
	t.Helper()

	dataBytes, err := common.Marshal(data)
	require.NoError(t, err)

	task := model.Task{
		TaskID:    taskID,
		ChannelId: channelID,
		Platform:  constant.TaskPlatform("doubao"),
		Data:      dataBytes,
	}
	require.NoError(t, model.DB.Create(&task).Error)
	return task
}

func TestResolveModerationDiagnoseVideoTaskIgnoresSuppliedRequestID(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	createModerationDiagnoseTask(t, "task_public", 42, map[string]any{
		"id": "cgt-upstream-generation",
	})

	channelID, recordID, resolved, queries, err := resolveModerationDiagnoseVideoTask(moderationDiagnoseRequest{
		RecordID:  "task_public",
		RequestID: "manually-supplied-request",
	})

	require.NoError(t, err)
	require.Equal(t, 42, channelID)
	require.NotEmpty(t, recordID)
	require.Equal(t, "", resolved["request_id"])
	require.Len(t, queries, 1)
	require.Equal(t, moderationDiagnoseQuery{ID: "cgt-upstream-generation", Type: moderationDiagnoseTypeTaskID}, queries[0])
}

func TestResolveModerationDiagnoseVideoTaskUsesActualRequestIDFallback(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	createModerationDiagnoseTask(t, "task_with_request", 43, map[string]any{
		"id":         "cgt-upstream-generation",
		"request_id": "actual-upstream-request",
	})

	_, _, resolved, queries, err := resolveModerationDiagnoseVideoTask(moderationDiagnoseRequest{
		RecordID: "task_with_request",
	})

	require.NoError(t, err)
	require.Equal(t, "actual-upstream-request", resolved["request_id"])
	require.Len(t, queries, 2)
	require.Equal(t, moderationDiagnoseQuery{ID: "cgt-upstream-generation", Type: moderationDiagnoseTypeTaskID}, queries[0])
	require.Equal(t, moderationDiagnoseQuery{ID: "actual-upstream-request", Type: moderationDiagnoseTypeRequestID}, queries[1])
}

func TestResolveModerationDiagnoseVideoTaskDoesNotUsePublicTaskIDAsUpstreamGenerationID(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	createModerationDiagnoseTask(t, "task_public", 44, map[string]any{
		"id": "task_public",
	})

	_, _, resolved, queries, err := resolveModerationDiagnoseVideoTask(moderationDiagnoseRequest{
		RecordID: "task_public",
	})

	require.NoError(t, err)
	require.Equal(t, "", resolved["upstream_generation_id"])
	require.Empty(t, queries)
}

func TestResolveModerationDiagnoseVideoTaskUsesStoredUpstreamTaskID(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	task := createModerationDiagnoseTask(t, "task_public_with_private_data", 45, map[string]any{
		"id": "task_public_with_private_data",
	})
	require.NoError(t, model.DB.Model(&task).Update("private_data", model.TaskPrivateData{
		UpstreamTaskID: "cgt-database-upstream",
	}).Error)

	channelID, _, resolved, queries, err := resolveModerationDiagnoseVideoTask(moderationDiagnoseRequest{
		RecordID: task.TaskID,
	})

	require.NoError(t, err)
	require.Equal(t, task.ChannelId, channelID)
	require.Equal(t, "cgt-database-upstream", resolved["upstream_generation_id"])
	require.Len(t, queries, 1)
	require.Equal(t, moderationDiagnoseQuery{
		ID:   "cgt-database-upstream",
		Type: moderationDiagnoseTypeTaskID,
	}, queries[0])
}

func TestResolveModerationDiagnoseVideoTaskRejectsMismatchedChannel(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	task := createModerationDiagnoseTask(t, "task_channel_bound", 45, map[string]any{
		"id": "cgt-upstream-generation",
	})

	channelID, recordID, resolved, queries, err := resolveModerationDiagnoseVideoTask(moderationDiagnoseRequest{
		RecordID:  task.TaskID,
		ChannelID: 99,
	})

	require.EqualError(t, err, "channel_id does not match the task record")
	require.Equal(t, task.ChannelId, channelID)
	require.Equal(t, task.ID, mustParseInt64(t, recordID))
	require.Nil(t, resolved)
	require.Nil(t, queries)
}

func TestFindModerationDiagnoseTaskReturnsNumericLookupDatabaseError(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	createModerationDiagnoseTask(t, "123", 45, map[string]any{
		"id": "cgt-upstream-generation",
	})

	expectedErr := errors.New("numeric lookup failed")
	queryCount := 0
	callbackName := "test:moderation-diagnose-fail-first-query"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		queryCount++
		if queryCount == 1 {
			tx.AddError(expectedErr)
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(callbackName)
	})

	task, err := findModerationDiagnoseTask("123")

	require.Nil(t, task)
	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, 1, queryCount)
}

func TestResolveSeedanceModerationConfigUsesInternalRegionWhenChannelRegionEmpty(t *testing.T) {
	channel := &model.Channel{}
	channel.SetSetting(dto.ChannelSettings{
		ByteplusProjectName: "project-for-response-redaction",
		ByteplusRegion:      "",
		Proxy:               "http://proxy.example.com",
	})
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		ByteplusRegion: "",
	})

	config, err := resolveSeedanceModerationConfigFromChannel(channel)

	require.NoError(t, err)
	require.Equal(t, seedanceModerationDiagnoseRegion, config.Region)
	require.Equal(t, "http://proxy.example.com", config.Proxy)
	require.Equal(t, "project-for-response-redaction", config.ProjectName)

	baseURL, err := seedanceAssetAdminBaseURL("", config.Region)
	require.NoError(t, err)
	require.Equal(t, "https://ark.ap-southeast-1.byteplusapi.com", baseURL)
}

func TestBuildModerationDiagnoseRequestUsesOfficialContractAndArkRegion(t *testing.T) {
	const (
		accessKey      = "test-ak-should-not-enter-body"
		secretKey      = "test-sk-should-not-enter-body"
		projectName    = "test-project-should-not-enter-body"
		assetGroupType = "test-group-should-not-enter-body"
	)
	config := &seedanceAssetAdminConfig{
		ProjectName:    projectName,
		AssetGroupType: assetGroupType,
		Region:         seedanceModerationDiagnoseRegion,
		Proxy:          "test-proxy-should-not-enter-body",
	}
	query := moderationDiagnoseQuery{
		ID:   "cgt-database-upstream",
		Type: moderationDiagnoseTypeTaskID,
	}
	requestBody, err := buildModerationDiagnoseRequestBody(query)
	require.NoError(t, err)
	targetURL, err := buildModerationDiagnoseTargetURL("https://ark.example.com")
	require.NoError(t, err)
	httpReq, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(requestBody))
	require.NoError(t, err)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	require.NoError(t, signSeedanceAssetAdminRequest(httpReq, accessKey, secretKey, config.Region))

	require.Equal(t, "GetModerationResult", httpReq.URL.Query().Get("Action"))
	require.Equal(t, "2024-01-01", httpReq.URL.Query().Get("Version"))

	var payload map[string]any
	require.NoError(t, common.Unmarshal(requestBody, &payload))
	require.Equal(t, map[string]any{
		"Id":   "cgt-database-upstream",
		"Type": moderationDiagnoseTypeTaskID,
	}, payload)
	require.Len(t, payload, 2)

	bodyText := string(requestBody)
	require.NotContains(t, bodyText, "ProjectName")
	require.NotContains(t, bodyText, projectName)
	require.NotContains(t, bodyText, accessKey)
	require.NotContains(t, bodyText, secretKey)
	require.NotContains(t, bodyText, assetGroupType)
	require.NotContains(t, bodyText, seedanceModerationDiagnoseRegion)
	require.NotContains(t, bodyText, config.Proxy)

	authorization := httpReq.Header.Get("Authorization")
	require.Contains(t, authorization, "Credential="+accessKey+"/")
	require.Contains(t, authorization, "/"+seedanceModerationDiagnoseRegion+"/ark/request")
	require.NotContains(t, authorization, secretKey)
}

func TestRecordModerationDiagnoseAuditDoesNotPersistRawOrCredentials(t *testing.T) {
	setupModerationDiagnoseTestDB(t)

	admin := model.User{
		Username: "diagnose-admin",
		Password: "unused-password",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&admin).Error)

	c, _ := gin.CreateTestContext(nil)
	c.Set("id", admin.Id)
	c.Set("username", admin.Username)

	response := &moderationDiagnoseResponse{
		SourceType: moderationDiagnoseSourceVideoTask,
		RecordID:   "123",
		ChannelID:  45,
		ResolvedQuery: moderationDiagnoseQuery{
			ID:   "cgt-upstream-generation",
			Type: moderationDiagnoseTypeTaskID,
		},
		RawRequestBody: `{"ProjectName":"project-placeholder","credential":"credential-placeholder","Region":"ap-southeast-1"}`,
		RawResponse:    `{"moderation":"raw-response-placeholder","Region":"ap-southeast-1"}`,
	}
	recordModerationDiagnoseAudit(c, moderationDiagnoseRequest{
		SourceType: moderationDiagnoseSourceVideoTask,
		RecordID:   "task_public",
	}, response, 0, nil)

	var auditLog model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeManage).First(&auditLog).Error)
	require.NotEmpty(t, auditLog.Other)
	require.NotContains(t, auditLog.Other, "project-placeholder")
	require.NotContains(t, auditLog.Other, "credential-placeholder")
	require.NotContains(t, auditLog.Other, "raw-response-placeholder")
	require.NotContains(t, auditLog.Other, seedanceModerationDiagnoseRegion)
	require.True(t, strings.Contains(auditLog.Other, "cgt-upstream-generation"))
}

func mustParseInt64(t *testing.T, value string) int64 {
	t.Helper()

	parsed, err := strconv.ParseInt(value, 10, 64)
	require.NoError(t, err)
	return parsed
}
