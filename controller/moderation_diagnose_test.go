package controller

import (
	"bytes"
	"errors"
	"net/http"
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
	})
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

func createVideoTaskFixture(t *testing.T, group string, projectName string) (model.Channel, model.Task) {
	t.Helper()
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
		"cgt-fixture-123",
		map[string]any{"id": "cgt-fixture-123"},
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

	_, err = findModerationDiagnoseTask(task.PrivateData.UpstreamTaskID + "-similar")
	require.ErrorIs(t, err, errModerationDiagnoseTaskNotFound)
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
		CredentialChannelID: 999,
	})

	require.NoError(t, err)
	require.Equal(t, task.Group, target.Tenant.Group)
	require.Equal(t, videoChannel.Id, target.Tenant.VideoChannelID)
	require.Equal(t, "project-a", target.Tenant.VideoProjectName)
	require.Zero(t, target.ManualCredentialChannelID)
	require.Equal(t, "cgt-fixture-123", target.Queries[0].ID)
	require.NotContains(t, target.Resolved, "channel_id")
	require.NotContains(t, target.Resolved, "group")
	require.NotContains(t, target.Resolved, "project_name")
}

func TestResolveModerationVideoTaskTargetUsesStoredUpstreamIDBeforeTaskData(t *testing.T) {
	setupModerationDiagnoseTestDB(t)
	videoChannel := createModerationChannel(t, "video-bearer-placeholder", "tenant-a", "tenant-video-alias", "project-a", 10)
	task := createModerationDiagnoseTask(
		t,
		"task_private_wins",
		videoChannel.Id,
		"tenant-a",
		"cgt-private",
		map[string]any{"id": "cgt-data", "request_id": "request-from-data"},
	)

	target, err := resolveModerationVideoTaskTarget(task.TaskID)

	require.NoError(t, err)
	require.Equal(t, []moderationDiagnoseQuery{
		{ID: "cgt-private", Type: moderationDiagnoseTypeTaskID},
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
		CredentialChannelID: manualChannel.Id,
	})

	require.NoError(t, err)
	require.Equal(t, moderationDiagnoseSourceVideoTask, target.SourceType)
	require.NotNil(t, target.Tenant)
	require.Zero(t, target.ManualCredentialChannelID)
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
