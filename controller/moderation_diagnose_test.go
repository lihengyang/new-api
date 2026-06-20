package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalQueryExecutor := moderationDiagnoseQueryExecutor

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Task{},
		&model.User{},
		&model.Log{},
		&model.Channel{},
		&model.Ability{},
	))
	model.DB = db
	model.LOG_DB = db
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		moderationDiagnoseQueryExecutor = originalQueryExecutor
	})
}

func newModerationDiagnoseTestContext() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/moderation/diagnose", nil)
	return c
}

func createModerationChannel(
	t *testing.T,
	key string,
	group string,
	models string,
	projectName string,
	priority int64,
) model.Channel {
	t.Helper()

	channel := model.Channel{
		Type:        constant.ChannelTypeOpenAI,
		Key:         key,
		Status:      common.ChannelStatusEnabled,
		Name:        "test-channel",
		Group:       group,
		Models:      models,
		Priority:    &priority,
		CreatedTime: time.Now().Unix(),
	}
	channel.SetSetting(dto.ChannelSettings{
		ByteplusProjectName: projectName,
	})
	require.NoError(t, model.DB.Create(&channel).Error)
	if strings.TrimSpace(models) != "" && strings.TrimSpace(group) != "" {
		require.NoError(t, channel.AddAbilities(nil))
	}
	return channel
}

func createModerationDiagnoseTask(
	t *testing.T,
	taskID string,
	channelID int,
	group string,
	upstreamTaskID string,
	data map[string]any,
) model.Task {
	t.Helper()

	dataBytes, err := common.Marshal(data)
	require.NoError(t, err)
	task := model.Task{
		CreatedAt:  time.Now().Unix(),
		TaskID:     taskID,
		ChannelId:  channelID,
		Group:      group,
		Platform:   constant.TaskPlatform("doubao"),
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName:   "tenant-video-alias",
			UpstreamModelName: "seedance-upstream-model",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: upstreamTaskID,
		},
		Data: dataBytes,
	}
	require.NoError(t, model.DB.Create(&task).Error)
	return task
}

func moderationTestCGTID(at time.Time, suffix string) string {
	return "cgt-" + at.UTC().Format(moderationDiagnoseCGTTimestampLayout) + "-" + suffix
}

func createVideoTaskFixture(t *testing.T, group string, projectName string) (model.Channel, model.Task) {
	t.Helper()
	upstreamTaskID := moderationTestCGTID(time.Now(), "abc12")
	videoChannel := createModerationChannel(
		t,
		"video-bearer-placeholder",
		group,
		"tenant-video-alias",
		projectName,
		10,
	)
	task := createModerationDiagnoseTask(
		t,
		"task_fixture_123",
		videoChannel.Id,
		group,
		upstreamTaskID,
		map[string]any{"id": upstreamTaskID},
	)
	return videoChannel, task
}

func TestFindModerationDiagnoseTaskUsesStrictIdentifierRules(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")

	byRecord, err := findModerationDiagnoseTask(strconv.FormatInt(task.ID, 10))
	require.NoError(t, err)
	require.Equal(t, task.ID, byRecord.ID)

	byPublicID, err := findModerationDiagnoseTask(task.TaskID)
	require.NoError(t, err)
	require.Equal(t, task.ID, byPublicID.ID)

	byUpstreamID, err := findModerationDiagnoseTask(task.PrivateData.UpstreamTaskID)
	require.NoError(t, err)
	require.Equal(t, task.ID, byUpstreamID.ID)

	similarID := task.PrivateData.UpstreamTaskID[:len(task.PrivateData.UpstreamTaskID)-5] + "abc13"
	_, err = findModerationDiagnoseTask(similarID)
	require.ErrorIs(t, err, errModerationDiagnoseTaskNotFound)
}

func TestFindModerationDiagnoseTaskRejectsInvalidCGTBeforeDatabaseQuery(t *testing.T) {
	testCases := []struct {
		name      string
		taskID    string
		errorText string
	}{
		{
			name:      "malformed",
			taskID:    "cgt-not-valid",
			errorText: "BP task ID must match cgt-YYYYMMDDHHMMSS-xxxxx",
		},
		{
			name:      "invalid timestamp",
			taskID:    "cgt-20261340000000-abc12",
			errorText: "BP task ID must match cgt-YYYYMMDDHHMMSS-xxxxx",
		},
		{
			name:      "older than fourteen days",
			taskID:    moderationTestCGTID(time.Now().Add(-15*24*time.Hour), "old12"),
			errorText: "BP task ID is outside the 14-day moderation lookup window",
		},
		{
			name:      "too far in future",
			taskID:    moderationTestCGTID(time.Now().Add(3*time.Hour), "fut12"),
			errorText: "BP task ID timestamp is too far in the future",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupModerationDiagnoseTestDB(t)
			queryCount := 0
			callbackName := "test:moderation-diagnose-query-count:" + strings.ReplaceAll(tc.name, " ", "-")
			require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(_ *gorm.DB) {
				queryCount++
			}))
			t.Cleanup(func() {
				_ = model.DB.Callback().Query().Remove(callbackName)
			})

			task, err := findModerationDiagnoseTask(tc.taskID)

			require.Nil(t, task)
			require.EqualError(t, err, tc.errorText)
			require.Zero(t, queryCount)
		})
	}
}

func TestFindModerationDiagnoseTaskValidMissingCGTReturnsNotFound(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	queryCount := 0
	callbackName := "test:moderation-diagnose-valid-missing-query-count"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(_ *gorm.DB) {
		queryCount++
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(callbackName)
	})

	task, err := findModerationDiagnoseTask(moderationTestCGTID(time.Now(), "none1"))

	require.Nil(t, task)
	require.ErrorIs(t, err, errModerationDiagnoseTaskNotFound)
	require.Equal(t, 1, queryCount)
}

func TestFindModerationDiagnoseTaskDoesNotFallbackNumericInputToPublicTaskID(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	videoChannel := createModerationChannel(t, "video-bearer-placeholder", "tenant-a", "tenant-video-alias", "project-a", 10)
	createModerationDiagnoseTask(t, "123", videoChannel.Id, "tenant-a", "cgt-numeric-public-id", nil)

	_, err := findModerationDiagnoseTask("123")

	require.ErrorIs(t, err, errModerationDiagnoseTaskNotFound)
}

func TestFindModerationDiagnoseTaskReturnsNumericLookupDatabaseError(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	expectedErr := errors.New("numeric lookup failed")
	callbackName := "test:moderation-diagnose-fail-query"
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		tx.AddError(expectedErr)
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Query().Remove(callbackName)
	})

	task, err := findModerationDiagnoseTask("123")

	require.Nil(t, task)
	require.ErrorIs(t, err, expectedErr)
}

func TestResolveModerationVideoTaskTargetKeepsStoredOwnership(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	videoChannel, task := createVideoTaskFixture(t, "tenant-a", "project-a")

	target, err := resolveModerationTarget(moderationDiagnoseRequest{
		SourceType:          moderationDiagnoseSourceVideoTask,
		RecordID:            task.TaskID,
		AssetAdminChannelID: 999,
	})

	require.NoError(t, err)
	require.Equal(t, task.Group, target.Tenant.Group)
	require.Equal(t, videoChannel.Id, target.Tenant.VideoChannelID)
	require.Equal(t, "project-a", target.Tenant.VideoProjectName)
	require.Zero(t, target.ManualCredentialChannelID)
	require.Equal(t, task.PrivateData.UpstreamTaskID, target.Queries[0].ID)
	require.NotContains(t, target.Resolved, "channel_id")
	require.NotContains(t, target.Resolved, "group")
	require.NotContains(t, target.Resolved, "project_name")
}

func TestResolveModerationVideoTaskTargetUsesStoredUpstreamIDBeforeTaskData(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	videoChannel := createModerationChannel(t, "video-bearer-placeholder", "tenant-a", "tenant-video-alias", "project-a", 10)
	privateTaskID := moderationTestCGTID(time.Now(), "pri12")
	dataTaskID := moderationTestCGTID(time.Now(), "dat12")
	task := createModerationDiagnoseTask(
		t,
		"task_private_wins",
		videoChannel.Id,
		"tenant-a",
		privateTaskID,
		map[string]any{"id": dataTaskID, "request_id": "request-from-data"},
	)

	target, err := resolveModerationVideoTaskTarget(task.TaskID)

	require.NoError(t, err)
	require.Equal(t, []moderationDiagnoseQuery{
		{ID: privateTaskID, Type: moderationDiagnoseTypeTaskID},
		{ID: "request-from-data", Type: moderationDiagnoseTypeRequestID},
	}, target.Queries)
}

func TestResolveModerationVideoTaskTargetRejectsOldTask(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	require.NoError(t, model.DB.Model(&task).Update("submit_time", time.Now().Add(-15*24*time.Hour).Unix()).Error)

	_, err := resolveModerationVideoTaskTarget(task.TaskID)

	require.EqualError(t, err, "task is outside the 14-day moderation lookup window")
}

func TestResolveModerationCredentialUsesAssetAbilityInsteadOfVideoKey(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	videoChannel, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	assetChannel := createModerationChannel(
		t,
		"access-placeholder|secret-placeholder",
		"tenant-a",
		"seedance-virtual-asset-admin",
		"project-a",
		20,
	)

	target, err := resolveModerationVideoTaskTarget(task.TaskID)
	require.NoError(t, err)
	credential, err := resolveModerationCredential(target)

	require.NoError(t, err)
	require.Equal(t, assetChannel.Id, credential.ChannelID)
	require.NotEqual(t, videoChannel.Id, credential.ChannelID)
}

func TestResolveModerationCredentialRejectsDifferentGroup(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	createModerationChannel(t, "access-placeholder|secret-placeholder", "tenant-b", "seedance-virtual-asset-admin", "project-a", 20)
	target, err := resolveModerationVideoTaskTarget(task.TaskID)
	require.NoError(t, err)

	_, err = resolveModerationCredential(target)

	require.EqualError(t, err, "asset admin credential channel not found for task tenant")
}

func TestResolveModerationCredentialRejectsProjectMismatch(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	createModerationChannel(t, "access-placeholder|secret-placeholder", "tenant-a", "seedance-virtual-asset-admin", "project-b", 20)
	target, err := resolveModerationVideoTaskTarget(task.TaskID)
	require.NoError(t, err)

	_, err = resolveModerationCredential(target)

	require.EqualError(t, err, "asset admin credential channel not found for task tenant")
}

func TestResolveModerationCredentialRejectsVideoChannelEvenWhenKeyContainsSeparator(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	videoChannel, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	require.NoError(t, model.DB.Model(&videoChannel).Update("key", "looks-like|credentials").Error)
	target, err := resolveModerationVideoTaskTarget(task.TaskID)
	require.NoError(t, err)

	_, err = resolveModerationCredential(target)

	require.EqualError(t, err, "asset admin credential channel not found for task tenant")
}

func TestResolveModerationCredentialRejectsInvalidAssetAdminKey(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	createModerationChannel(t, "bearer-placeholder", "tenant-a", "seedance-virtual-asset-admin", "project-a", 20)
	target, err := resolveModerationVideoTaskTarget(task.TaskID)
	require.NoError(t, err)

	_, err = resolveModerationCredential(target)

	require.EqualError(t, err, "asset admin credential channel not found for task tenant")
}

func TestResolveModerationCredentialRejectsAmbiguousCandidates(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "")
	createModerationChannel(t, "access-one|secret-one", "tenant-a", "seedance-virtual-asset-admin", "project-one", 20)
	createModerationChannel(t, "access-two|secret-two", "tenant-a", "seedance-real-human-asset-admin", "project-two", 10)
	target, err := resolveModerationVideoTaskTarget(task.TaskID)
	require.NoError(t, err)

	_, err = resolveModerationCredential(target)

	require.EqualError(t, err, "credential mapping ambiguous")
}

func TestResolveModerationCredentialSelectsEquivalentCandidateDeterministically(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	lowerPriority := createModerationChannel(t, "shared-access|shared-secret", "tenant-a", "seedance-real-human-asset-admin", "project-a", 10)
	higherPriority := createModerationChannel(t, "shared-access|shared-secret", "tenant-a", "seedance-virtual-asset-admin", "project-a", 20)
	target, err := resolveModerationVideoTaskTarget(task.TaskID)
	require.NoError(t, err)

	first, err := resolveModerationCredential(target)
	require.NoError(t, err)
	second, err := resolveModerationCredential(target)
	require.NoError(t, err)

	require.Equal(t, higherPriority.Id, first.ChannelID)
	require.Equal(t, first.ChannelID, second.ChannelID)
	require.NotEqual(t, lowerPriority.Id, first.ChannelID)
}

func TestResolveModerationLibraryAssetRequiresManualMode(t *testing.T) {
	target, err := resolveModerationTarget(moderationDiagnoseRequest{
		SourceType: moderationDiagnoseSourceLibraryAsset,
		AssetID:    "asset-placeholder",
	})

	require.EqualError(t, err, "asset ownership mapping is not available; use manual mode")
	require.Equal(t, "asset-placeholder", target.RecordID)
}

func TestResolveModerationManualValidatesAssetAdminChannel(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	videoChannel := createModerationChannel(t, "video-bearer-placeholder", "tenant-a", "tenant-video-alias", "project-a", 10)
	assetChannel := createModerationChannel(t, "access-placeholder|secret-placeholder", "tenant-a", "seedance-virtual-asset-admin", "project-a", 20)

	_, err := resolveManualModerationCredential(videoChannel.Id)
	require.EqualError(t, err, "selected credential channel is not a valid asset admin channel")

	credential, err := resolveManualModerationCredential(assetChannel.Id)
	require.NoError(t, err)
	require.Equal(t, assetChannel.Id, credential.ChannelID)
}

func TestResolveModerationManualPrefersStoredTaskOwnership(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	manualChannel := createModerationChannel(t, "manual-access|manual-secret", "tenant-b", "seedance-virtual-asset-admin", "project-b", 20)

	target, err := resolveModerationTarget(moderationDiagnoseRequest{
		SourceType:          moderationDiagnoseSourceManual,
		ID:                  task.PrivateData.UpstreamTaskID,
		Type:                moderationDiagnoseTypeTaskID,
		AssetAdminChannelID: manualChannel.Id,
	})

	require.NoError(t, err)
	require.Equal(t, moderationDiagnoseSourceVideoTask, target.SourceType)
	require.NotNil(t, target.Tenant)
	require.Zero(t, target.ManualCredentialChannelID)
}

func TestResolveModerationManualTaskOwnershipByAllIdentifiersWithoutCredential(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")

	for _, identifier := range []string{
		strconv.FormatInt(task.ID, 10),
		task.TaskID,
		task.PrivateData.UpstreamTaskID,
	} {
		target, err := resolveModerationTarget(moderationDiagnoseRequest{
			SourceType: moderationDiagnoseSourceManual,
			ID:         identifier,
			Type:       moderationDiagnoseTypeTaskID,
		})

		require.NoError(t, err)
		require.Equal(t, moderationDiagnoseSourceVideoTask, target.SourceType)
		require.NotNil(t, target.Tenant)
		require.Zero(t, target.ManualCredentialChannelID)
	}
}

func TestResolveModerationManualExistingTaskIgnoresCredentialOverride(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
	maliciousChannel := createModerationChannel(
		t,
		"malicious-access|malicious-secret",
		"tenant-b",
		"seedance-virtual-asset-admin",
		"project-b",
		100,
	)

	target, err := resolveModerationTarget(moderationDiagnoseRequest{
		SourceType:          moderationDiagnoseSourceManual,
		ID:                  strconv.FormatInt(task.ID, 10),
		Type:                moderationDiagnoseTypeTaskID,
		AssetAdminChannelID: maliciousChannel.Id,
	})

	require.NoError(t, err)
	require.Equal(t, task.Group, target.Tenant.Group)
	require.Zero(t, target.ManualCredentialChannelID)
}

func TestResolveModerationManualUnknownTaskRequiresCredential(t *testing.T) {
	setupModerationDiagnoseTestDB(t)

	_, err := resolveModerationTarget(moderationDiagnoseRequest{
		SourceType: moderationDiagnoseSourceManual,
		ID:         moderationTestCGTID(time.Now(), "none2"),
		Type:       moderationDiagnoseTypeTaskID,
	})

	require.EqualError(t, err, "task ownership was not found; select an Asset Admin credential channel")
}

func TestRunModerationManualUnknownTaskWithValidCredential(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	assetChannel := createModerationChannel(
		t,
		"access-placeholder|secret-placeholder",
		"tenant-a",
		"seedance-virtual-asset-admin",
		"project-a",
		20,
	)
	moderationDiagnoseQueryExecutor = func(
		_ *gin.Context,
		query moderationDiagnoseQuery,
		_ string,
		_ string,
		_ string,
		_ *seedanceAssetAdminConfig,
	) moderationDiagnoseAttempt {
		return moderationDiagnoseAttempt{
			ID:          query.ID,
			Type:        query.Type,
			StatusCode:  http.StatusOK,
			Success:     true,
			RawResponse: `{"Result":"blocked"}`,
		}
	}

	response, err := runModerationDiagnose(newModerationDiagnoseTestContext(), moderationDiagnoseRequest{
		SourceType:          moderationDiagnoseSourceManual,
		ID:                  "external-task-id",
		Type:                moderationDiagnoseTypeTaskID,
		AssetAdminChannelID: assetChannel.Id,
	})

	require.NoError(t, err)
	require.Equal(t, moderationDiagnoseResultFound, response.ResultStatus)
	require.Len(t, response.AttemptedQueries, 1)
}

func TestResolveModerationManualAssetAndRequestRequireCredential(t *testing.T) {
	setupModerationDiagnoseTestDB(t)

	for _, queryType := range []string{
		moderationDiagnoseTypeAssetID,
		moderationDiagnoseTypeRequestID,
	} {
		_, err := resolveModerationTarget(moderationDiagnoseRequest{
			SourceType: moderationDiagnoseSourceManual,
			ID:         "upstream-id",
			Type:       queryType,
		})
		require.EqualError(t, err, "Asset Admin credential channel is required for manual diagnose")
	}
}

func TestRunModerationLibraryAssetDoesNotAttemptUpstream(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	executorCalls := 0
	moderationDiagnoseQueryExecutor = func(
		_ *gin.Context,
		_ moderationDiagnoseQuery,
		_ string,
		_ string,
		_ string,
		_ *seedanceAssetAdminConfig,
	) moderationDiagnoseAttempt {
		executorCalls++
		return moderationDiagnoseAttempt{}
	}

	response, err := runModerationDiagnose(newModerationDiagnoseTestContext(), moderationDiagnoseRequest{
		SourceType: moderationDiagnoseSourceLibraryAsset,
		AssetID:    "asset-placeholder",
	})

	require.EqualError(t, err, "asset ownership mapping is not available; use manual mode")
	require.Equal(t, moderationDiagnoseResultValidationError, response.ResultStatus)
	require.Empty(t, response.AttemptedQueries)
	require.Zero(t, executorCalls)
}

func TestRunModerationDiagnoseResultStatuses(t *testing.T) {
	testCases := []struct {
		name           string
		attempt        moderationDiagnoseAttempt
		expectedStatus string
		expectError    bool
	}{
		{
			name: "found",
			attempt: moderationDiagnoseAttempt{
				StatusCode:  http.StatusOK,
				Success:     true,
				RawResponse: `{"BlockReason":"Copyright"}`,
			},
			expectedStatus: moderationDiagnoseResultFound,
		},
		{
			name: "not found",
			attempt: moderationDiagnoseAttempt{
				StatusCode:  http.StatusNotFound,
				Success:     true,
				RawResponse: `{"Code":"NotFound.Id"}`,
			},
			expectedStatus: moderationDiagnoseResultNotFound,
		},
		{
			name: "request failed",
			attempt: moderationDiagnoseAttempt{
				StatusCode: http.StatusBadGateway,
				Success:    false,
				RawError:   "upstream request failed",
			},
			expectedStatus: moderationDiagnoseResultRequestFailed,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupModerationDiagnoseTestDB(t)
			_, task := createVideoTaskFixture(t, "tenant-a", "project-a")
			createModerationChannel(
				t,
				"access-placeholder|secret-placeholder",
				"tenant-a",
				"seedance-virtual-asset-admin",
				"project-a",
				20,
			)
			moderationDiagnoseQueryExecutor = func(
				_ *gin.Context,
				query moderationDiagnoseQuery,
				_ string,
				_ string,
				_ string,
				_ *seedanceAssetAdminConfig,
			) moderationDiagnoseAttempt {
				attempt := tc.attempt
				attempt.ID = query.ID
				attempt.Type = query.Type
				return attempt
			}

			response, err := runModerationDiagnose(newModerationDiagnoseTestContext(), moderationDiagnoseRequest{
				SourceType: moderationDiagnoseSourceVideoTask,
				RecordID:   task.TaskID,
			})

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedStatus, response.ResultStatus)
		})
	}
}

func TestRunModerationDiagnoseValidationStatus(t *testing.T) {
	response, err := runModerationDiagnose(newModerationDiagnoseTestContext(), moderationDiagnoseRequest{
		SourceType: moderationDiagnoseSourceManual,
		ID:         "asset-placeholder",
		Type:       moderationDiagnoseTypeAssetID,
	})

	require.Error(t, err)
	require.Equal(t, moderationDiagnoseResultValidationError, response.ResultStatus)
}

func TestRunModerationDiagnoseMalformedCGTReturnsValidationStatus(t *testing.T) {
	setupModerationDiagnoseTestDB(t)

	response, err := runModerationDiagnose(newModerationDiagnoseTestContext(), moderationDiagnoseRequest{
		SourceType: moderationDiagnoseSourceVideoTask,
		RecordID:   "cgt-malformed",
	})

	require.EqualError(t, err, "BP task ID must match cgt-YYYYMMDDHHMMSS-xxxxx")
	require.Equal(t, moderationDiagnoseResultValidationError, response.ResultStatus)
	require.Empty(t, response.AttemptedQueries)
}

func TestListModerationCredentialChannelsFiltersUnsafeChannels(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	valid := createModerationChannel(t, "access-placeholder|secret-placeholder", "tenant-a", "seedance-virtual-asset-admin", "project-a", 20)
	createModerationChannel(t, "video-bearer-placeholder", "tenant-a", "tenant-video-alias", "project-a", 20)
	createModerationChannel(t, "invalid-key", "tenant-a", "seedance-real-human-asset-admin", "project-a", 20)

	candidates, err := listModerationCredentialChannels()

	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, valid.Id, candidates[0].ChannelID)
}

func TestGetModerationCredentialChannelsReturnsSafeLabelsOnly(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	valid := createModerationChannel(
		t,
		"access-placeholder|secret-placeholder",
		"tenant-a",
		"seedance-virtual-asset-admin",
		"project-placeholder",
		20,
	)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	GetModerationCredentialChannels(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, fmt.Sprintf("Asset Admin credential #%d", valid.Id))
	require.NotContains(t, body, "access-placeholder")
	require.NotContains(t, body, "secret-placeholder")
	require.NotContains(t, body, "project-placeholder")
	require.NotContains(t, body, "tenant-a")
	require.NotContains(t, body, "seedance-virtual-asset-admin")
}

func TestBuildModerationDiagnoseRequestUsesOfficialContractAndArkRegion(t *testing.T) {
	const (
		accessKey      = "test-access-placeholder"
		secretKey      = "test-secret-placeholder"
		projectName    = "test-project-placeholder"
		assetGroupType = "test-group-placeholder"
	)
	config := &seedanceAssetAdminConfig{
		ProjectName:    projectName,
		AssetGroupType: assetGroupType,
		Region:         seedanceModerationDiagnoseRegion,
		Proxy:          "test-proxy-placeholder",
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

func TestModerationDiagnoseTreatsNotFoundAsCompletedDiagnostic(t *testing.T) {
	require.True(t, isModerationDiagnoseSuccessStatus(http.StatusOK))
	require.True(t, isModerationDiagnoseSuccessStatus(http.StatusNotFound))
	require.False(t, isModerationDiagnoseSuccessStatus(http.StatusBadGateway))
}

func TestModerationDiagnoseResponseDoesNotExposeCredentialOrTenantFields(t *testing.T) {
	response := moderationDiagnoseResponse{
		SourceType: moderationDiagnoseSourceVideoTask,
		RecordID:   "123",
		Resolved: map[string]interface{}{
			"task_record_id":         int64(123),
			"platform_task_id":       "task-placeholder",
			"upstream_generation_id": "cgt-placeholder",
		},
		credentialChannelID: 42,
	}

	encoded, err := common.Marshal(response)

	require.NoError(t, err)
	text := string(encoded)
	require.NotContains(t, text, "credential_channel")
	require.NotContains(t, text, "channel_id")
	require.NotContains(t, text, "ProjectName")
	require.NotContains(t, text, `"group"`)
	require.NotContains(t, text, "access-placeholder")
	require.NotContains(t, text, "secret-placeholder")
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
		ResolvedQuery: moderationDiagnoseQuery{
			ID:   "cgt-upstream-generation",
			Type: moderationDiagnoseTypeTaskID,
		},
		RawRequestBody:      `{"ProjectName":"project-placeholder","credential":"credential-placeholder"}`,
		RawResponse:         `{"moderation":"raw-response-placeholder"}`,
		credentialChannelID: 45,
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
	require.Contains(t, auditLog.Other, "cgt-upstream-generation")
	require.Contains(t, auditLog.Other, `"credential_channel_id":45`)
}
