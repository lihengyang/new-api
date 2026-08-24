package service_test

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	taskrelay "github.com/QuantumNous/new-api/relay"
	taskdoubao "github.com/QuantumNous/new-api/relay/channel/task/doubao"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	service "github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	seedance25IntegrationBalance     = 10_000
	seedance25IntegrationRuleVersion = "byteplus_seedance_2_5_intl_2026_08_p0"
	seedance25TenantAliasForPolling  = relaycommon.Seedance25TenantAliasPrefix + "henrytest"
)

var seedance25IntegrationSequence atomic.Int64

type seedance25PollingIntegrationFixture struct {
	task       *model.Task
	channel    *model.Channel
	upstreamID string
	userID     int
	tokenID    int
	channelID  int
}

func nextSeedance25IntegrationID() int {
	return 30_000 + int(seedance25IntegrationSequence.Add(1))
}

func seedance25IntegrationBillingContext(hasVideo bool) *model.TaskBillingContext {
	numerator, denominator := int64(1), int64(1)
	otherRatio := 1.0
	if hasVideo {
		numerator, denominator = 64, 107
		otherRatio = 64.0 / 107.0
	}
	return &model.TaskBillingContext{
		ModelRatio:            5.35,
		GroupRatio:            1,
		OtherRatios:           map[string]float64{"seedance_intl_billing": otherRatio},
		OriginModelName:       seedance25TenantAliasForPolling,
		BillingFamily:         relaycommon.Seedance25BillingFamily,
		BillingRuleVersion:    seedance25IntegrationRuleVersion,
		OtherRatioNumerator:   numerator,
		OtherRatioDenominator: denominator,
	}
}

func seedSeedance25PollingIntegrationFixture(t *testing.T, baseURL string, reservation int) *seedance25PollingIntegrationFixture {
	t.Helper()
	service.InitHttpClient()
	id := nextSeedance25IntegrationID()
	userID, tokenID, channelID := id, id, id
	upstreamID := "placeholder-upstream-task-" + strconv.Itoa(id)
	publicTaskID := "task_public_seedance25_" + strconv.Itoa(id)
	baseURLCopy := baseURL

	user := &model.User{
		Id:       userID,
		Username: "s25-user-" + strconv.Itoa(id),
		Status:   common.UserStatusEnabled,
		Quota:    seedance25IntegrationBalance - reservation,
		Group:    "default",
		AffCode:  "s25-aff-" + strconv.Itoa(id),
	}
	token := &model.Token{
		Id:          tokenID,
		UserId:      userID,
		Key:         "s25-token-placeholder-" + strconv.Itoa(id),
		Name:        "seedance25-token-placeholder",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: seedance25IntegrationBalance - reservation,
		UsedQuota:   reservation,
	}
	channel := &model.Channel{
		Id:        channelID,
		Type:      constant.ChannelTypeDoubaoVideo,
		Key:       "seedance25-channel-key-placeholder",
		Name:      "seedance25-channel-placeholder",
		Status:    common.ChannelStatusEnabled,
		BaseURL:   &baseURLCopy,
		UsedQuota: int64(reservation),
	}
	now := time.Now().Unix()
	task := &model.Task{
		TaskID:     publicTaskID,
		UserId:     userID,
		Group:      "default",
		ChannelId:  channelID,
		Quota:      reservation,
		Platform:   constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Action:     constant.TaskActionGenerate,
		Status:     model.TaskStatusInProgress,
		Progress:   "50%",
		SubmitTime: now,
		CreatedAt:  now,
		UpdatedAt:  now,
		Properties: model.Properties{
			OriginModelName:   seedance25TenantAliasForPolling,
			UpstreamModelName: "placeholder-upstream-model",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: upstreamID,
			BillingSource:  service.BillingSourceWallet,
			TokenId:        tokenID,
			BillingContext: seedance25IntegrationBillingContext(false),
		},
		Data: json.RawMessage(`{"status":"processing"}`),
	}

	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(token).Error)
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(task).Error)
	t.Cleanup(func() {
		require.NoError(t, model.LOG_DB.Where("user_id = ?", userID).Delete(&model.Log{}).Error)
		require.NoError(t, model.DB.Where("id = ?", task.ID).Delete(&model.Task{}).Error)
		require.NoError(t, model.DB.Where("id = ?", channelID).Delete(&model.Channel{}).Error)
		require.NoError(t, model.DB.Where("id = ?", tokenID).Delete(&model.Token{}).Error)
		require.NoError(t, model.DB.Where("id = ?", userID).Delete(&model.User{}).Error)
	})

	return &seedance25PollingIntegrationFixture{
		task:       task,
		channel:    channel,
		upstreamID: upstreamID,
		userID:     userID,
		tokenID:    tokenID,
		channelID:  channelID,
	}
}

func seedance25IntegrationQuotaState(t *testing.T, fixture *seedance25PollingIntegrationFixture) (int, int, int) {
	t.Helper()
	var user model.User
	var token model.Token
	require.NoError(t, model.DB.Select("quota").First(&user, fixture.userID).Error)
	require.NoError(t, model.DB.Select("remain_quota", "used_quota").First(&token, fixture.tokenID).Error)
	return user.Quota, token.RemainQuota, token.UsedQuota
}

func seedance25IntegrationLogs(t *testing.T, userID int) []model.Log {
	t.Helper()
	var logs []model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ?", userID).Order("id asc").Find(&logs).Error)
	return logs
}

func seedance25PublicPollingBody(t *testing.T, task *model.Task) []byte {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+task.TaskID, nil)
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Set("id", task.UserId)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, task.Group)
	require.Nil(t, taskrelay.RelayTaskFetch(c, relayconstant.RelayModeVideoFetchByID))
	return recorder.Body.Bytes()
}

func newSeedance25PollingServer(t *testing.T, responseBody string, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasPrefix(r.URL.Path, "/api/v3/contents/generations/tasks/"))
		w.Header().Set("Content-Type", "application/json")
		upstreamID := strings.TrimPrefix(r.URL.Path, "/api/v3/contents/generations/tasks/")
		_, _ = io.WriteString(w, strings.ReplaceAll(responseBody, "{{UPSTREAM_TASK_ID}}", upstreamID))
	}))
}

func TestSeedance25RealDoubaoPollingSuccessSettlesAndAuditsExactlyOnce(t *testing.T) {
	tests := []struct {
		name             string
		reservation      int
		expectedLogType  int
		expectedLogQuota int
	}{
		{name: "positive delta", reservation: 500, expectedLogType: model.LogTypeConsume, expectedLogQuota: 35},
		{name: "zero delta", reservation: 535, expectedLogType: model.LogTypeConsume, expectedLogQuota: 0},
		{name: "negative delta", reservation: 600, expectedLogType: model.LogTypeRefund, expectedLogQuota: 65},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := newSeedance25PollingServer(t, `{
				"id":"{{UPSTREAM_TASK_ID}}",
				"model":"placeholder-upstream-model",
				"status":"succeeded",
				"content":{"video_url":" https://example.invalid/seedance25-result.mp4 "},
				"usage":{"completion_tokens":100,"total_tokens":999},
				"created_at":1000,"updated_at":2000,
				"resolution":"1080p","duration":4,"ratio":"16:9","seed":123,
				"generate_audio":false,"framespersecond":24,"output_format":"mp4",
				"service_tier":"default","draft":false,"priority":"normal",
				"execution_expires_after":3600
			}`, &calls)
			defer server.Close()
			fixture := seedSeedance25PollingIntegrationFixture(t, server.URL, tt.reservation)
			adaptor := &taskdoubao.TaskAdaptor{}
			taskMap := map[string]*model.Task{fixture.upstreamID: fixture.task}

			require.NoError(t, service.UpdateVideoSingleTaskForTest(context.Background(), adaptor, fixture.channel, fixture.upstreamID, taskMap))
			require.NoError(t, service.UpdateVideoSingleTaskForTest(context.Background(), adaptor, fixture.channel, fixture.upstreamID, taskMap))
			require.EqualValues(t, 1, calls.Load())

			userQuota, tokenRemain, tokenUsed := seedance25IntegrationQuotaState(t, fixture)
			require.Equal(t, seedance25IntegrationBalance-535, userQuota)
			require.Equal(t, seedance25IntegrationBalance-535, tokenRemain)
			require.EqualValues(t, 535, tokenUsed)

			var reloaded model.Task
			require.NoError(t, model.DB.First(&reloaded, fixture.task.ID).Error)
			require.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)
			require.Equal(t, "https://example.invalid/seedance25-result.mp4", reloaded.PrivateData.ResultURL)
			require.Contains(t, string(reloaded.Data), `"total_tokens":999`)
			require.Contains(t, string(reloaded.Data), `"model":"placeholder-upstream-model"`)
			require.Contains(t, string(reloaded.Data), `"resolution":"1080p"`)
			require.Contains(t, string(reloaded.Data), `"generate_audio":false`)
			require.Contains(t, string(reloaded.Data), `"framespersecond":24`)
			var consoleData map[string]any
			require.NoError(t, common.Unmarshal(reloaded.Data, &consoleData))
			require.Equal(t, fixture.upstreamID, consoleData["id"])
			require.Equal(t, "placeholder-upstream-model", consoleData["model"])
			require.Equal(t, "succeeded", consoleData["status"])
			require.Equal(t, "1080p", consoleData["resolution"])
			require.EqualValues(t, 4, consoleData["duration"])
			require.Equal(t, "16:9", consoleData["ratio"])
			require.EqualValues(t, 123, consoleData["seed"])
			require.Equal(t, false, consoleData["generate_audio"])
			require.EqualValues(t, 24, consoleData["framespersecond"])
			require.Equal(t, "mp4", consoleData["output_format"])
			require.Equal(t, "default", consoleData["service_tier"])
			require.Equal(t, false, consoleData["draft"])
			require.Equal(t, "normal", consoleData["priority"])
			require.EqualValues(t, 3600, consoleData["execution_expires_after"])

			logs := seedance25IntegrationLogs(t, fixture.userID)
			require.Len(t, logs, 1)
			require.Equal(t, tt.expectedLogType, logs[0].Type)
			require.Equal(t, tt.expectedLogQuota, logs[0].Quota)
			require.Equal(t, seedance25TenantAliasForPolling, logs[0].ModelName)
			var other map[string]any
			require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &other))
			require.EqualValues(t, 535, other["actual_quota"])
			require.NotContains(t, logs[0].Other, "placeholder-upstream")

			publicBody := seedance25PublicPollingBody(t, &reloaded)
			var video dto.OpenAIVideo
			require.NoError(t, common.Unmarshal(publicBody, &video))
			require.Equal(t, dto.VideoStatusCompleted, video.Status)
			require.Equal(t, seedance25TenantAliasForPolling, video.Model)
			require.Equal(t, "https://example.invalid/seedance25-result.mp4", video.Metadata["url"])
			require.NotNil(t, video.Usage)
			require.Equal(t, 100, video.Usage.CompletionTokens)
			require.Equal(t, 999, video.Usage.TotalTokens)
			require.NotContains(t, string(publicBody), "placeholder-upstream")
			require.NotContains(t, string(publicBody), "placeholder-upstream-model")
		})
	}
}

func TestSeedance25RealDoubaoPollingFailuresRefundOnceAndStaySanitized(t *testing.T) {
	tests := []struct {
		name            string
		responseBody    string
		mutate          func(*model.Task)
		expectedCode    string
		expectedMessage string
	}{
		{name: "usage missing", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"total_tokens":999}}`, expectedCode: "invalid_upstream_usage"},
		{name: "usage null", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":null}}`, expectedCode: "invalid_upstream_usage"},
		{name: "usage string", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":"100"}}`, expectedCode: "invalid_upstream_usage"},
		{name: "usage float", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100.5}}`, expectedCode: "invalid_upstream_usage"},
		{name: "usage zero", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":0}}`, expectedCode: "invalid_upstream_usage"},
		{name: "usage negative", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":-1}}`, expectedCode: "invalid_upstream_usage"},
		{name: "url missing", responseBody: `{"status":"succeeded","usage":{"completion_tokens":100}}`, expectedCode: "invalid_upstream_output"},
		{name: "url empty", responseBody: `{"status":"succeeded","content":{"video_url":""},"usage":{"completion_tokens":100}}`, expectedCode: "invalid_upstream_output"},
		{name: "url blank", responseBody: `{"status":"succeeded","content":{"video_url":"  \t "},"usage":{"completion_tokens":100}}`, expectedCode: "invalid_upstream_output"},
		{name: "task type constraint", responseBody: `{"status":"failed","error":{"code":"InvalidParameter.TaskTypeConstraint","message":"raw-upstream-diagnostic-placeholder"}}`, expectedCode: "InvalidParameter.TaskTypeConstraint", expectedMessage: "The request parameters are incompatible with the task type identified by the model. Update the parameters for that task type and try again."},
		{name: "task type mismatch", responseBody: `{"status":"failed","error":{"code":"InvalidParameter.TaskTypeMismatch","message":"raw-upstream-diagnostic-placeholder"}}`, expectedCode: "InvalidParameter.TaskTypeMismatch", expectedMessage: "The task type identified by the model does not match the specified value. Revise the prompt and input assets, then try again."},
		{name: "audio policy violation", responseBody: `{"status":"failed","error":{"code":"OutputAudioSensitiveContentDetected.PolicyViolation","message":"raw-upstream-diagnostic-placeholder"}}`, expectedCode: "OutputAudioSensitiveContentDetected.PolicyViolation", expectedMessage: "The generated audio may be related to copyright restrictions. Please replace the input content and try again."},
		{name: "ordinary upstream failure", responseBody: `{"status":"failed","error":{"code":"ProviderFailurePlaceholder","message":"raw-upstream-diagnostic-placeholder"}}`, expectedCode: "video_generation_failed"},
		{name: "expired terminal failure", responseBody: `{"status":"expired","error":{"code":"ProviderExpiredPlaceholder","message":"raw-upstream-diagnostic-placeholder Request ID: private-marker"},"execution_expires_after":3600}`, expectedCode: "video_generation_failed"},
		{name: "missing billing snapshot", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext = nil }, expectedCode: "invalid_billing_context"},
		{name: "damaged billing snapshot", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = 5.34 }, expectedCode: "invalid_billing_context"},
		{name: "non finite model ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = math.NaN() }, expectedCode: "invalid_billing_context"},
		{name: "positive infinite model ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = math.Inf(1) }, expectedCode: "invalid_billing_context"},
		{name: "negative infinite model ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = math.Inf(-1) }, expectedCode: "invalid_billing_context"},
		{name: "zero model ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = 0 }, expectedCode: "invalid_billing_context"},
		{name: "negative model ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.ModelRatio = -1 }, expectedCode: "invalid_billing_context"},
		{name: "nan group ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.GroupRatio = math.NaN() }, expectedCode: "invalid_billing_context"},
		{name: "non finite group ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.GroupRatio = math.Inf(1) }, expectedCode: "invalid_billing_context"},
		{name: "negative infinite group ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.GroupRatio = math.Inf(-1) }, expectedCode: "invalid_billing_context"},
		{name: "zero group ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.GroupRatio = 0 }, expectedCode: "invalid_billing_context"},
		{name: "negative group ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.GroupRatio = -1 }, expectedCode: "invalid_billing_context"},
		{name: "nan other ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) {
			task.PrivateData.BillingContext.OtherRatios["seedance_intl_billing"] = math.NaN()
		}, expectedCode: "invalid_billing_context"},
		{name: "positive infinite other ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) {
			task.PrivateData.BillingContext.OtherRatios["seedance_intl_billing"] = math.Inf(1)
		}, expectedCode: "invalid_billing_context"},
		{name: "negative infinite other ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) {
			task.PrivateData.BillingContext.OtherRatios["seedance_intl_billing"] = math.Inf(-1)
		}, expectedCode: "invalid_billing_context"},
		{name: "zero other ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.OtherRatios["seedance_intl_billing"] = 0 }, expectedCode: "invalid_billing_context"},
		{name: "negative other ratio", responseBody: `{"status":"succeeded","content":{"video_url":"https://example.invalid/should-not-publish.mp4"},"usage":{"completion_tokens":100}}`, mutate: func(task *model.Task) { task.PrivateData.BillingContext.OtherRatios["seedance_intl_billing"] = -1 }, expectedCode: "invalid_billing_context"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := newSeedance25PollingServer(t, tt.responseBody, &calls)
			defer server.Close()
			const reservation = 600
			fixture := seedSeedance25PollingIntegrationFixture(t, server.URL, reservation)
			if tt.mutate != nil {
				tt.mutate(fixture.task)
			}
			adaptor := &taskdoubao.TaskAdaptor{}
			taskMap := map[string]*model.Task{fixture.upstreamID: fixture.task}

			require.NotPanics(t, func() {
				require.NoError(t, service.UpdateVideoSingleTaskForTest(context.Background(), adaptor, fixture.channel, fixture.upstreamID, taskMap))
				require.NoError(t, service.UpdateVideoSingleTaskForTest(context.Background(), adaptor, fixture.channel, fixture.upstreamID, taskMap))
			})
			require.EqualValues(t, 1, calls.Load())

			userQuota, tokenRemain, tokenUsed := seedance25IntegrationQuotaState(t, fixture)
			require.Equal(t, seedance25IntegrationBalance, userQuota)
			require.Equal(t, seedance25IntegrationBalance, tokenRemain)
			require.Zero(t, tokenUsed)

			var reloaded model.Task
			require.NoError(t, model.DB.First(&reloaded, fixture.task.ID).Error)
			require.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
			require.Empty(t, reloaded.PrivateData.ResultURL)
			require.Contains(t, string(reloaded.Data), tt.expectedCode)
			for _, forbidden := range []string{
				"placeholder-upstream-model",
				"placeholder-upstream-task",
				"raw-upstream-diagnostic-placeholder",
				"should-not-publish",
			} {
				require.NotContains(t, string(reloaded.Data), forbidden)
				require.NotContains(t, reloaded.FailReason, forbidden)
			}

			logs := seedance25IntegrationLogs(t, fixture.userID)
			require.Len(t, logs, 1)
			require.Equal(t, model.LogTypeRefund, logs[0].Type)
			require.Equal(t, reservation, logs[0].Quota)
			require.Equal(t, seedance25TenantAliasForPolling, logs[0].ModelName)
			require.NotContains(t, logs[0].Other+logs[0].Content, "placeholder-upstream")
			require.NotContains(t, logs[0].Other+logs[0].Content, "raw-upstream-diagnostic-placeholder")

			publicBody := seedance25PublicPollingBody(t, &reloaded)
			replayedPublicBody := seedance25PublicPollingBody(t, &reloaded)
			require.JSONEq(t, string(publicBody), string(replayedPublicBody))
			require.EqualValues(t, 1, calls.Load())
			require.Len(t, seedance25IntegrationLogs(t, fixture.userID), 1)
			var video dto.OpenAIVideo
			require.NoError(t, common.Unmarshal(publicBody, &video))
			require.Equal(t, dto.VideoStatusFailed, video.Status)
			require.Equal(t, seedance25TenantAliasForPolling, video.Model)
			require.Nil(t, video.Usage)
			if video.Metadata != nil {
				require.Empty(t, video.Metadata["url"])
			}
			require.NotContains(t, string(publicBody), "placeholder-upstream")
			require.NotContains(t, string(publicBody), "raw-upstream-diagnostic-placeholder")
			require.NotContains(t, string(publicBody), "private-marker")
			require.NotContains(t, string(publicBody), "execution_expires_after")
			require.NotContains(t, string(publicBody), "should-not-publish")
			require.NotNil(t, video.Error)
			require.Equal(t, tt.expectedCode, video.Error.Code)
			if tt.expectedMessage != "" {
				require.Equal(t, tt.expectedMessage, reloaded.FailReason)
				require.Equal(t, tt.expectedMessage, video.Error.Message)
				require.NotNil(t, video.Error.Retryable)
				require.False(t, *video.Error.Retryable)
			}
		})
	}
}

func TestSeedance25AsyncMediaFailuresReleaseReservationAndStaySanitized(t *testing.T) {
	for _, upstreamCode := range []string{
		"InvalidParameter.ReferenceDownloadFailed",
		"InvalidParameter.ReferenceFormatError",
	} {
		t.Run(upstreamCode, func(t *testing.T) {
			var calls atomic.Int32
			server := newSeedance25PollingServer(t,
				`{"status":"failed","error":{"code":"`+upstreamCode+`","message":"raw signed URL https://example.invalid/private?signature=secret"}}`,
				&calls,
			)
			defer server.Close()
			const reservation = 600
			fixture := seedSeedance25PollingIntegrationFixture(t, server.URL, reservation)
			adaptor := &taskdoubao.TaskAdaptor{}
			taskMap := map[string]*model.Task{fixture.upstreamID: fixture.task}

			require.NoError(t, service.UpdateVideoSingleTaskForTest(context.Background(), adaptor, fixture.channel, fixture.upstreamID, taskMap))
			require.EqualValues(t, 1, calls.Load())
			userQuota, tokenRemain, tokenUsed := seedance25IntegrationQuotaState(t, fixture)
			require.Equal(t, seedance25IntegrationBalance, userQuota)
			require.Equal(t, seedance25IntegrationBalance, tokenRemain)
			require.Zero(t, tokenUsed)

			var reloaded model.Task
			require.NoError(t, model.DB.First(&reloaded, fixture.task.ID).Error)
			require.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
			require.Contains(t, string(reloaded.Data), `"code":"InvalidParameter"`)
			require.Contains(t, string(reloaded.Data), `"message":"A request parameter is invalid. Check the request parameters and try again."`)
			require.Contains(t, string(reloaded.Data), `"retryable":false`)
			require.NotContains(t, string(reloaded.Data), upstreamCode)
			require.NotContains(t, string(reloaded.Data), "signature=secret")
			require.NotContains(t, reloaded.FailReason, "signature=secret")
			logs := seedance25IntegrationLogs(t, fixture.userID)
			require.Len(t, logs, 1)
			require.Equal(t, model.LogTypeRefund, logs[0].Type)
			require.Equal(t, reservation, logs[0].Quota)
			require.NotContains(t, logs[0].Content+logs[0].Other, "signature=secret")
		})
	}
}

func TestSeedance25ExpiredPollingCASLoserDoesNotRepeatFinancialAction(t *testing.T) {
	var calls atomic.Int32
	server := newSeedance25PollingServer(t, `{"status":"expired","error":{"code":"ProviderExpiredPlaceholder","message":"raw-upstream-diagnostic-placeholder Request ID: private-marker"},"execution_expires_after":3600}`, &calls)
	defer server.Close()
	const reservation = 600
	fixture := seedSeedance25PollingIntegrationFixture(t, server.URL, reservation)

	// Simulate a competing poller that already won the terminal CAS and refunded
	// the reservation. The stale in-memory task still carries IN_PROGRESS.
	require.NoError(t, model.DB.Model(&model.Task{}).Where("id = ?", fixture.task.ID).Updates(map[string]any{
		"status":      model.TaskStatusFailure,
		"progress":    "100%",
		"fail_reason": "competing terminal worker placeholder",
	}).Error)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", fixture.userID).Update("quota", seedance25IntegrationBalance).Error)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", fixture.tokenID).Updates(map[string]any{
		"remain_quota": seedance25IntegrationBalance,
		"used_quota":   0,
	}).Error)
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    fixture.userID,
		LogType:   model.LogTypeRefund,
		ChannelId: fixture.channelID,
		ModelName: seedance25TenantAliasForPolling,
		Quota:     reservation,
		TokenId:   fixture.tokenID,
		Group:     "default",
		Other:     map[string]any{"task_id": fixture.task.TaskID, "reason": "competing terminal worker placeholder"},
	})

	adaptor := &taskdoubao.TaskAdaptor{}
	taskMap := map[string]*model.Task{fixture.upstreamID: fixture.task}
	require.NoError(t, service.UpdateVideoSingleTaskForTest(context.Background(), adaptor, fixture.channel, fixture.upstreamID, taskMap))
	require.NoError(t, service.UpdateVideoSingleTaskForTest(context.Background(), adaptor, fixture.channel, fixture.upstreamID, taskMap))
	require.EqualValues(t, 1, calls.Load())

	userQuota, tokenRemain, tokenUsed := seedance25IntegrationQuotaState(t, fixture)
	require.Equal(t, seedance25IntegrationBalance, userQuota)
	require.Equal(t, seedance25IntegrationBalance, tokenRemain)
	require.Zero(t, tokenUsed)
	require.Len(t, seedance25IntegrationLogs(t, fixture.userID), 1)

	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, fixture.task.ID).Error)
	require.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
}
