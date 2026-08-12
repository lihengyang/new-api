package controller

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/mediakit"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestMediaKitSubmitPrechargesBeforeSingleUpstreamPostAndPersistsSnapshot(t *testing.T) {
	const alias = "lsf-video-enhancement-test-tenant"
	requestBody, err := common.Marshal(map[string]any{
		"model": alias,
		"metadata": map[string]any{
			"video_url":    "https://media.example.invalid/source.mp4?signature=input-secret",
			"tool_version": "professional", "resolution": "4k", "duration": 1.01,
			"client_request_id": "mediakit-submit-test",
		},
	})
	require.NoError(t, err)

	var calls atomic.Int32
	var forwarded []byte
	var reservationVisible atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/tools/enhance-video", r.URL.Path)
		require.Equal(t, "Bearer placeholder-channel-key", r.Header.Get("Authorization"))
		var count int64
		if err := model.DB.Model(&model.Task{}).
			Where("status = ?", model.TaskStatusReserved).Count(&count).Error; err == nil && count == 1 {
			reservationVisible.Store(true)
		}
		forwarded, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"result":{"task_id":"provider-task-placeholder"}}`)
	}))
	defer server.Close()

	mapping := `{"` + alias + `":"` + mediakit.CanonicalModel + `"}`
	fixture := setupSeedance25SubmitFixture(t, requestBody, server.URL, mapping)
	fixture.info.OriginModelName = alias
	common.SetContextKey(fixture.context, constant.ContextKeyChannelType, constant.ChannelTypeMediaKit)

	result, taskErr := relay.RelayTaskSubmitNoWrite(fixture.context, fixture.info)
	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.EqualValues(t, 1, calls.Load())
	require.True(t, reservationVisible.Load())
	require.Equal(t, 1_101_920, result.Quota)
	require.NotNil(t, result.Response)
	require.Nil(t, handleTaskSubmitSuccess(fixture.context, fixture.info, result))

	var payload map[string]any
	require.NoError(t, common.Unmarshal(forwarded, &payload))
	require.Equal(t, "professional", payload["tool_version"])
	require.Equal(t, "4k", payload["resolution"])
	require.Equal(t, "mediakit-submit-test", payload["client_token"])
	require.NotContains(t, payload, "duration")
	require.NotContains(t, payload, "client_request_id")
	require.NotContains(t, payload, "metadata")
	require.NotContains(t, payload, "ProjectName")

	var task model.Task
	require.NoError(t, model.DB.First(&task, result.ReservationTaskID).Error)
	require.Equal(t, model.TaskStatusNotStart, task.Status)
	require.Equal(t, "provider-task-placeholder", task.PrivateData.UpstreamTaskID)
	require.NotNil(t, task.PrivateData.BillingContext)
	require.Equal(t, mediakit.BillingFamily, task.PrivateData.BillingContext.BillingFamily)
	require.Equal(t, mediakit.BillingRuleVersion, task.PrivateData.BillingContext.BillingRuleVersion)
	require.Equal(t, "professional", task.PrivateData.BillingContext.BillingMetadata["tool_version"])
	require.Equal(t, "4k", task.PrivateData.BillingContext.BillingMetadata["resolution_tier"])
	require.EqualValues(t, 2, task.PrivateData.BillingContext.BillingMetadata["estimated_seconds"])
	require.NotContains(t, string(task.Data), "source.mp4")
	require.NotContains(t, string(task.Data), "provider-task-placeholder")

	var consumeLog model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", fixture.userID, model.LogTypeConsume).Last(&consumeLog).Error)
	require.Zero(t, consumeLog.ChannelId)
	require.Empty(t, consumeLog.Group)
	require.NotContains(t, consumeLog.Other, "upstream_model_name")
	require.NotContains(t, consumeLog.Other, mediakit.CanonicalModel)
	require.NotContains(t, consumeLog.Other, "input-secret")
	require.NotContains(t, consumeLog.Other, "placeholder-channel-key")
}

func TestMediaKitChannelErrorLogOmitsChannelGroupAndSensitiveValues(t *testing.T) {
	fixture := setupSeedance25SubmitFixture(t, []byte(`{"model":"placeholder"}`), "https://example.invalid", `{}`)
	oldErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = true
	t.Cleanup(func() { constant.ErrorLogEnabled = oldErrorLogEnabled })
	fixture.context.Set("id", fixture.userID)
	fixture.context.Set("token_id", fixture.tokenID)
	fixture.context.Set("group", "private-tenant-group")
	fixture.context.Set("original_model", "lsf-video-enhancement-tenant")
	fixture.context.Set("channel_id", 999)
	fixture.context.Set("channel_name", "private-project-channel")
	fixture.context.Set("channel_type", constant.ChannelTypeMediaKit)

	processChannelError(
		fixture.context,
		*types.NewChannelError(999, constant.ChannelTypeMediaKit, "private-project-channel", false, "placeholder-key", false),
		types.NewOpenAIError(errors.New("upstream task submission failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway),
	)

	var log model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", fixture.userID, model.LogTypeError).Last(&log).Error)
	require.Zero(t, log.ChannelId)
	require.Empty(t, log.Group)
	require.NotContains(t, log.Other, "private-project-channel")
	require.NotContains(t, log.Other, "private-tenant-group")
	require.NotContains(t, log.Other, "placeholder-key")
	require.NotContains(t, log.Other, `"channel_id"`)
}

func TestMediaKitInvalidClientTokenRejectedBeforeBillingAndUpstream(t *testing.T) {
	tests := []struct {
		name            string
		clientRequestID string
	}{
		{name: "too long", clientRequestID: strings.Repeat("a", 65)},
		{name: "non ascii", clientRequestID: "request-测试"},
		{name: "non printable", clientRequestID: "request\n02"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const alias = "lsf-video-enhancement-test-tenant"
			requestBody, err := common.Marshal(map[string]any{
				"model": alias,
				"metadata": map[string]any{
					"video_url":  "https://media.example.invalid/source.mp4",
					"resolution": "1080p", "duration": 6,
					"client_request_id": test.clientRequestID,
				},
			})
			require.NoError(t, err)

			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			mapping := `{"` + alias + `":"` + mediakit.CanonicalModel + `"}`
			fixture := setupSeedance25SubmitFixture(t, requestBody, server.URL, mapping)
			fixture.info.OriginModelName = alias
			common.SetContextKey(fixture.context, constant.ContextKeyChannelType, constant.ChannelTypeMediaKit)

			result, taskErr := relay.RelayTaskSubmitNoWrite(fixture.context, fixture.info)
			require.Nil(t, result)
			require.NotNil(t, taskErr)
			require.Zero(t, calls.Load())
			require.Nil(t, fixture.info.Billing)
			require.Zero(t, fixture.info.ReservationTaskID)

			var taskCount int64
			require.NoError(t, model.DB.Model(&model.Task{}).Count(&taskCount).Error)
			require.Zero(t, taskCount)
		})
	}
}
