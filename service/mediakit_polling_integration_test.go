package service_test

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/mediakit"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	service "github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

var mediaKitIntegrationSequence atomic.Int64

func seedMediaKitSettlementTask(t *testing.T, precharge int, declaredDuration float64) (*model.Task, int) {
	t.Helper()
	id := 70_000 + int(mediaKitIntegrationSequence.Add(1))
	initialBalance := 200_000
	user := &model.User{
		Id: id, Username: "mediakit-user-" + strconv.Itoa(id), AffCode: "mediakit-aff-" + strconv.Itoa(id),
		Status: common.UserStatusEnabled, Quota: initialBalance - precharge, Group: "default",
	}
	now := time.Now().Unix()
	task := &model.Task{
		TaskID: "task_mediakit_" + strconv.Itoa(id), UserId: id, Group: "default", ChannelId: id,
		Quota: precharge, Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMediaKit)),
		Action: constant.TaskActionGenerate, Status: model.TaskStatusInProgress, Progress: "30%",
		SubmitTime: now, CreatedAt: now, UpdatedAt: now,
		Properties: model.Properties{OriginModelName: "lsf-video-enhancement-tenant", UpstreamModelName: mediakit.CanonicalModel},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream-id-placeholder", BillingSource: service.BillingSourceWallet,
			BillingContext: &model.TaskBillingContext{
				GroupRatio: 1, BillingFamily: mediakit.BillingFamily, BillingRuleVersion: mediakit.BillingRuleVersion,
				OriginModelName: "lsf-video-enhancement-tenant",
				BillingMetadata: map[string]any{
					"declared_duration": declaredDuration,
					"tool_version":      "standard",
					"resolution_tier":   "1080p",
				},
			},
		},
		Data: json.RawMessage(`{"status":"processing"}`),
	}
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(task).Error)
	t.Cleanup(func() {
		_ = model.LOG_DB.Where("user_id = ?", id).Delete(&model.Log{}).Error
		_ = model.DB.Where("id = ?", task.ID).Delete(&model.Task{}).Error
		_ = model.DB.Where("id = ?", id).Delete(&model.User{}).Error
	})
	return task, initialBalance
}

func mediaKitUserQuota(t *testing.T, userID int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").First(&user, userID).Error)
	return user.Quota
}

func mediaKitSuccess(duration, fps float64) *relaycommon.TaskInfo {
	return &relaycommon.TaskInfo{
		Status: string(model.TaskStatusSuccess), Progress: "100%", Duration: duration, FPS: fps,
		Resolution: "1080p", ToolVersion: "standard",
		Url: "https://signed.example.invalid/result.mp4?signature=must-not-persist", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
}

func TestMediaKitSettlementRefundsOrSupplementsFullDifference(t *testing.T) {
	tests := []struct {
		name        string
		precharge   int
		duration    float64
		fps         float64
		actualQuota int
	}{
		{name: "refund", precharge: 13773, duration: 1, fps: 30, actualQuota: 3443},
		{name: "supplement", precharge: 3443, duration: 2, fps: 30, actualQuota: 6886},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task, initialBalance := seedMediaKitSettlementTask(t, test.precharge, 1)
			result := mediaKitSuccess(test.duration, test.fps)
			require.NoError(t, service.ApplyVideoTaskResult(context.Background(), &mediakit.TaskAdaptor{}, task, result, []byte(`{"raw":"provider-response-marker"}`)))
			require.Equal(t, initialBalance-test.actualQuota, mediaKitUserQuota(t, task.UserId))

			var stored model.Task
			require.NoError(t, model.DB.First(&stored, task.ID).Error)
			require.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.Status)
			require.Empty(t, stored.PrivateData.ResultURL)
			require.NotContains(t, string(stored.Data), "signed.example.invalid")
			require.NotContains(t, string(stored.Data), "provider-response-marker")
			require.Equal(t, test.duration, stored.PrivateData.BillingContext.BillingMetadata["actual_duration"])
			require.EqualValues(t, test.actualQuota, stored.PrivateData.BillingContext.BillingMetadata["actual_quota"])
			var billingLog model.Log
			require.NoError(t, model.LOG_DB.Where("user_id = ?", task.UserId).Last(&billingLog).Error)
			require.Zero(t, billingLog.ChannelId)
			require.Empty(t, billingLog.Group)
			require.NotContains(t, billingLog.Other, "signed.example.invalid")
			require.NotContains(t, billingLog.Other, mediakit.CanonicalModel)

			// A later GET may fetch the same completed result but cannot settle it again.
			require.NoError(t, service.ApplyVideoTaskResult(context.Background(), &mediakit.TaskAdaptor{}, &stored, mediaKitSuccess(test.duration, test.fps), nil))
			require.Equal(t, initialBalance-test.actualQuota, mediaKitUserQuota(t, task.UserId))
		})
	}
}

func TestMediaKitEnvelopeCompletionSettlesOnlyOnce(t *testing.T) {
	task, initialBalance := seedMediaKitSettlementTask(t, 13773, 1)
	body := []byte(`{
		"success":true,
		"status":"completed",
		"result":{
			"video_url":"https://signed.example.invalid/result.mp4?signature=must-not-persist",
			"duration":1,
			"fps":30,
			"resolution":"1080p",
			"tool_version":"standard"
		}
	}`)
	adaptor := &mediakit.TaskAdaptor{}
	result, err := adaptor.ParseTaskResultForTask(task, body)
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), result.Status)
	require.NoError(t, service.ApplyVideoTaskResult(context.Background(), adaptor, task, result, body))
	require.Equal(t, initialBalance-3443, mediaKitUserQuota(t, task.UserId))

	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	replayed, err := adaptor.ParseTaskResultForTask(&stored, body)
	require.NoError(t, err)
	require.NoError(t, service.ApplyVideoTaskResult(context.Background(), adaptor, &stored, replayed, body))
	require.Equal(t, initialBalance-3443, mediaKitUserQuota(t, task.UserId))
	require.Empty(t, stored.PrivateData.ResultURL)
}

func TestMediaKitConcurrentCompletionSettlesOnce(t *testing.T) {
	task, initialBalance := seedMediaKitSettlementTask(t, 13773, 1)
	var first, second model.Task
	require.NoError(t, model.DB.First(&first, task.ID).Error)
	require.NoError(t, model.DB.First(&second, task.ID).Error)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, candidate := range []*model.Task{&first, &second} {
		wg.Add(1)
		go func(candidate *model.Task) {
			defer wg.Done()
			<-start
			errs <- service.ApplyVideoTaskResult(context.Background(), &mediakit.TaskAdaptor{}, candidate, mediaKitSuccess(1, 30), nil)
		}(candidate)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, initialBalance-3443, mediaKitUserQuota(t, task.UserId))
}

func TestMediaKitFailureRefundsReservationOnce(t *testing.T) {
	task, initialBalance := seedMediaKitSettlementTask(t, 13773, 1)
	result := &relaycommon.TaskInfo{Status: string(model.TaskStatusFailure), Progress: "100%", Reason: "raw upstream failure"}
	require.NoError(t, service.ApplyVideoTaskResult(context.Background(), &mediakit.TaskAdaptor{}, task, result, []byte(`{"error":"signed-url-marker"}`)))
	require.Equal(t, initialBalance, mediaKitUserQuota(t, task.UserId))
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	require.NotContains(t, string(stored.Data), "signed-url-marker")
}
