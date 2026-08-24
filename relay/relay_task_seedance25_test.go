package relay

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	corecommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

const seedance25TenantAliasForRelayTest = relaycommon.Seedance25TenantAliasPrefix + "henrytest"

func TestRelayTaskSubmitRejectsSeedance25InvalidRequestsBeforeBilling(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing prompt for text only", body: `{"model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p"}}`},
		{name: "blank prompt for text only", body: `{"prompt":"  ","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p"}}`},
		{name: "explicit reference without assets", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p","omni_reference_task_type":"reference"}}`},
		{name: "unsupported resolution", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"4k"}}`},
		{name: "disabled false field", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p","camera_fixed":false}}`},
		{name: "first frame fixed ratio", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p","ratio":"21:9","content":[{"type":"image_url","role":"first_frame","image_url":{"url":"https://example.invalid/reference.png"}}]}}`},
		{name: "edit duration", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"client_request_id":"req-edit-duration","duration":4,"resolution":"480p","ratio":"adaptive","omni_reference_task_type":"edit","content":[{"type":"video_url","role":"reference_video","video_url":{"url":"https://example.invalid/reference.mp4"}}]}}`},
		{name: "extend ratio", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"client_request_id":"req-extend-ratio","duration":5,"resolution":"480p","ratio":"16:9","omni_reference_task_type":"extend","content":[{"type":"video_url","role":"reference_video","video_url":{"url":"https://example.invalid/reference.mp4"}}]}}`},
		{name: "edit missing reference video", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"client_request_id":"req-edit-no-video","duration":-1,"resolution":"480p","ratio":"adaptive","omni_reference_task_type":"edit","content":[{"type":"image_url","role":"reference_image","image_url":{"url":"https://example.invalid/reference.png"}}]}}`},
		{name: "extend missing reference video", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"client_request_id":"req-extend-no-video","duration":5,"resolution":"480p","ratio":"adaptive","omni_reference_task_type":"extend","content":[{"type":"image_url","role":"reference_image","image_url":{"url":"https://example.invalid/reference.png"}}]}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupRelayTaskTestDB(t)
			var upstreamCalls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				upstreamCalls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer upstream.Close()

			c, info := newTaskClientRequestIDContext(t, tt.body)
			info.OriginModelName = seedance25TenantAliasForRelayTest
			c.Set("model_mapping", `{"`+seedance25TenantAliasForRelayTest+`":"mapped-provider-model-placeholder"}`)
			corecommon.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
			beforeQuota, err := model.GetUserQuota(info.UserId, true)
			require.NoError(t, err)

			result, taskErr := RelayTaskSubmitNoWrite(c, info)

			require.Nil(t, result)
			require.NotNil(t, taskErr)
			require.Equal(t, "invalid_request_error", taskErr.Code)
			require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			require.NotNil(t, taskErr.Retryable)
			require.False(t, *taskErr.Retryable)
			require.Nil(t, info.Billing)
			require.False(t, c.Writer.Written())
			require.Zero(t, upstreamCalls.Load())
			var taskCount int64
			require.NoError(t, model.DB.Model(&model.Task{}).Count(&taskCount).Error)
			require.Zero(t, taskCount)
			var logCount int64
			require.NoError(t, model.DB.Model(&model.Log{}).Count(&logCount).Error)
			require.Zero(t, logCount)
			afterQuota, err := model.GetUserQuota(info.UserId, true)
			require.NoError(t, err)
			require.Equal(t, beforeQuota, afterQuota)
		})
	}
}

func TestRelayTaskSubmitRejectsSeedance25MissingMappingBeforeBilling(t *testing.T) {
	body := `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p"}}`
	c, info := newTaskClientRequestIDContext(t, body)
	info.OriginModelName = seedance25TenantAliasForRelayTest

	result, taskErr := RelayTaskSubmit(c, info)

	require.Nil(t, result)
	require.NotNil(t, taskErr)
	require.Equal(t, "UPSTREAM_MAPPING_MISSING", taskErr.Code)
	require.Equal(t, http.StatusServiceUnavailable, taskErr.StatusCode)
	require.Nil(t, info.Billing)
	require.False(t, c.Writer.Written())
}

func TestSeedance25SubmitFailureReplayIsSingleShotSafeAndFullyRefundedOnce(t *testing.T) {
	setupRelayTaskTestDB(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 1001).Update("quota", 10_000_000).Error)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", 501).Update("remain_quota", 10_000_000).Error)
	previousRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"`+seedance25TenantAliasForRelayTest+`":5.35}`))

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"Unknown.Customer.Rejection","message":"raw-message Request ID: request-marker endpoint-marker model-marker account-marker channel-marker routing-marker"}}`))
	}))
	defer upstream.Close()

	body := `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"client_request_id":"req-seedance25-safe-replay","duration":4,"resolution":"480p","ratio":"adaptive","omni_reference_task_type":"reference","generate_audio":false,"content":[{"type":"image_url","role":"reference_image","image_url":{"url":"https://example.invalid/reference.png"}}]}}`
	beforeUserQuota, err := model.GetUserQuota(1001, true)
	require.NoError(t, err)
	var beforeToken model.Token
	require.NoError(t, model.DB.First(&beforeToken, 501).Error)

	firstContext, firstInfo := newTaskClientRequestIDContext(t, body)
	firstInfo.OriginModelName = seedance25TenantAliasForRelayTest
	firstInfo.IsPlayground = true
	firstContext.Set("model_mapping", `{"`+seedance25TenantAliasForRelayTest+`":"mapped-provider-model-placeholder"}`)
	corecommon.SetContextKey(firstContext, constant.ContextKeyChannelBaseUrl, upstream.URL)
	result, taskErr := RelayTaskSubmitNoWrite(firstContext, firstInfo)
	require.Nil(t, result)
	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	require.Equal(t, "upstream_request_rejected", taskErr.Code)
	require.Equal(t, "The request was rejected. Check the request parameters and try again.", taskErr.Message)
	require.NotNil(t, taskErr.Retryable)
	require.False(t, *taskErr.Retryable)
	require.EqualValues(t, 1, upstreamCalls.Load())
	require.NotNil(t, firstInfo.Billing)
	require.NotZero(t, firstInfo.ReservationTaskID)

	safeData, err := corecommon.Marshal(map[string]any{
		"error": map[string]any{
			"code":      taskErr.Code,
			"message":   taskErr.Message,
			"retryable": *taskErr.Retryable,
		},
	})
	require.NoError(t, err)
	require.NoError(t, model.FailTaskReservation(model.FailTaskReservationParams{
		ID:         firstInfo.ReservationTaskID,
		FailReason: taskErr.Message,
		Data:       safeData,
	}))
	firstInfo.Billing.Refund(firstContext)
	firstInfo.Billing.Refund(firstContext)
	require.Eventually(t, func() bool {
		userQuota, quotaErr := model.GetUserQuota(1001, true)
		if quotaErr != nil || userQuota != beforeUserQuota {
			return false
		}
		var token model.Token
		return model.DB.First(&token, 501).Error == nil &&
			token.RemainQuota == beforeToken.RemainQuota && token.UsedQuota == beforeToken.UsedQuota
	}, 3*time.Second, 10*time.Millisecond)

	replayContext, replayInfo := newTaskClientRequestIDContext(t, body)
	replayInfo.OriginModelName = seedance25TenantAliasForRelayTest
	replayInfo.IsPlayground = true
	replayContext.Set("model_mapping", `{"`+seedance25TenantAliasForRelayTest+`":"mapped-provider-model-placeholder"}`)
	corecommon.SetContextKey(replayContext, constant.ContextKeyChannelBaseUrl, upstream.URL)
	replay, replayErr := RelayTaskSubmitNoWrite(replayContext, replayInfo)
	require.Nil(t, replayErr)
	require.NotNil(t, replay)
	require.True(t, replay.IdempotentReplay)
	require.Nil(t, replayInfo.Billing)
	require.EqualValues(t, 1, upstreamCalls.Load())
	public := BuildOpenAIVideoFromTask(replay.ReplayTask)
	require.Equal(t, dto.VideoStatusFailed, public.Status)
	require.NotNil(t, public.Error)
	require.Equal(t, taskErr.Code, public.Error.Code)
	require.Equal(t, taskErr.Message, public.Error.Message)
	require.NotNil(t, public.Error.Retryable)
	require.False(t, *public.Error.Retryable)
	publicJSON, err := corecommon.Marshal(public)
	require.NoError(t, err)
	for _, forbidden := range []string{"raw-message", "Request ID", "request-marker", "endpoint-marker", "model-marker", "account-marker", "channel-marker", "routing-marker"} {
		require.NotContains(t, string(publicJSON), forbidden)
	}
	var taskCount int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("client_request_id = ?", "req-seedance25-safe-replay").Count(&taskCount).Error)
	require.EqualValues(t, 1, taskCount)
}

func TestSeedance25VideoFetchDoesNotExposePrivateRoutingOrBillingContext(t *testing.T) {
	setupRelayTaskTestDB(t)
	now := time.Now().Unix()
	task := &model.Task{
		TaskID:    "task_public_seedance25",
		UserId:    1001,
		Group:     "test-group",
		ChannelId: 77,
		Platform:  constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		CreatedAt: now,
		UpdatedAt: now,
		Properties: model.Properties{
			OriginModelName:   seedance25TenantAliasForRelayTest,
			UpstreamModelName: "provider_model_marker",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "provider_task_marker",
			BillingContext: &model.TaskBillingContext{
				OriginModelName:       seedance25TenantAliasForRelayTest,
				BillingFamily:         relaycommon.Seedance25BillingFamily,
				BillingRuleVersion:    "private_rule_marker",
				ModelRatio:            5.35,
				GroupRatio:            1,
				OtherRatioNumerator:   64,
				OtherRatioDenominator: 107,
			},
		},
	}
	var err error
	task.Data, err = corecommon.Marshal(map[string]any{
		"status":  "succeeded",
		"content": map[string]any{"video_url": "https://example.invalid/result.mp4"},
		"usage":   map[string]any{"completion_tokens": 100},
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(task).Error)

	body, taskErr := videoFetchByIDRespBodyBuilder(newVideoFetchContext(1001, "test-group", task.TaskID))
	require.Nil(t, taskErr)
	require.Contains(t, string(body), seedance25TenantAliasForRelayTest)
	require.Contains(t, string(body), `"completion_tokens":100`)
	for _, forbidden := range []string{
		"provider_model_marker",
		"provider_task_marker",
		"private_rule_marker",
		"billing_context",
		"billing_family",
		"channel_id",
		"group_ratio",
		"model_ratio",
		"private_data",
		"ProjectName",
		"upstream_model_name",
	} {
		require.NotContains(t, string(body), forbidden)
	}
}
