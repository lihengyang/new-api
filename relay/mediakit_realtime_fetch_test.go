package relay

import (
	"encoding/json"
	"io"
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
	"github.com/QuantumNous/new-api/relay/channel/task/mediakit"
	"github.com/stretchr/testify/require"
)

func TestMediaKitCustomerGetSettlesAndReturnsSignedURLWithoutPersistingIt(t *testing.T) {
	setupRelayTaskTestDB(t)
	var calls atomic.Int32
	expiresAt := time.Now().Add(time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/v1/tasks/provider-task-placeholder", r.URL.Path)
		require.Equal(t, "Bearer placeholder-channel-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"result":{"task_id":"provider-task-placeholder","status":"completed","duration":1,"fps":30,"resolution":"1080p","tool_version":"standard","video_url":"https://signed.example.invalid/result.mp4?signature=secret","expires_at":`+strconv.FormatInt(expiresAt, 10)+`}}`)
	}))
	defer server.Close()

	baseURL := server.URL
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: 77, Type: constant.ChannelTypeMediaKit, Name: "private-project-channel",
		Key: "placeholder-channel-key", BaseURL: &baseURL, Status: common.ChannelStatusEnabled,
	}).Error)
	now := time.Now().Unix()
	task := &model.Task{
		TaskID: "task_public_mediakit", UserId: 1001, Group: "test-group", ChannelId: 77,
		Quota: 3443, Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMediaKit)),
		Action: constant.TaskActionGenerate, Status: model.TaskStatusInProgress, Progress: "30%",
		CreatedAt: now, UpdatedAt: now, SubmitTime: now,
		Properties: model.Properties{OriginModelName: "lsf-video-enhancement-tenant", UpstreamModelName: mediakit.CanonicalModel},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "provider-task-placeholder",
			BillingContext: &model.TaskBillingContext{
				GroupRatio: 1, BillingFamily: mediakit.BillingFamily, BillingRuleVersion: mediakit.BillingRuleVersion,
				OriginModelName: "lsf-video-enhancement-tenant",
				BillingMetadata: map[string]any{"declared_duration": 1.0, "tool_version": "standard", "resolution_tier": "1080p"},
			},
		},
		Data: json.RawMessage(`{"status":"processing"}`),
	}
	require.NoError(t, model.DB.Create(task).Error)

	for range 2 {
		response, taskErr := videoFetchByIDRespBodyBuilder(newVideoFetchContext(1001, "test-group", task.TaskID))
		require.Nil(t, taskErr)
		var video dto.OpenAIVideo
		require.NoError(t, common.Unmarshal(response, &video))
		require.Equal(t, dto.VideoStatusCompleted, video.Status)
		require.Equal(t, "https://signed.example.invalid/result.mp4?signature=secret", video.Metadata["video_url"])
		require.Equal(t, 1.0, video.Metadata["duration"])
		require.Equal(t, 30.0, video.Metadata["fps"])
		require.Equal(t, expiresAt, video.ExpiresAt)
	}
	require.EqualValues(t, 2, calls.Load())

	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.Status)
	require.Empty(t, stored.PrivateData.ResultURL)
	require.NotContains(t, string(stored.Data), "signed.example.invalid")
	require.NotContains(t, string(stored.Data), "provider-task-placeholder")
	require.False(t, strings.Contains(string(stored.Data), "signature"))
}
