package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModerationDiagnoseTestDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
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
