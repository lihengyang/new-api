package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

const seedance25TenantAliasForServiceTest = relaycommon.Seedance25TenantAliasPrefix + "henrytest"

func markTaskAsSeedance25(task *model.Task, hasVideo bool) {
	numerator, denominator := int64(1), int64(1)
	if hasVideo {
		numerator, denominator = 64, 107
	}
	task.Properties.OriginModelName = seedance25TenantAliasForServiceTest
	task.Properties.UpstreamModelName = "provider_model_marker"
	if task.PrivateData.BillingContext == nil {
		task.PrivateData.BillingContext = &model.TaskBillingContext{}
	}
	task.PrivateData.BillingContext.OriginModelName = seedance25TenantAliasForServiceTest
	task.PrivateData.BillingContext.BillingFamily = relaycommon.Seedance25BillingFamily
	task.PrivateData.BillingContext.BillingRuleVersion = "seedance25_test_rule"
	task.PrivateData.BillingContext.ModelRatio = 5.35
	task.PrivateData.BillingContext.GroupRatio = 1
	task.PrivateData.BillingContext.OtherRatioNumerator = numerator
	task.PrivateData.BillingContext.OtherRatioDenominator = denominator
}

func taskBillingLogOther(t *testing.T, log *model.Log) map[string]any {
	t.Helper()
	other := make(map[string]any)
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	return other
}

func TestSeedance25RecalculateAuditsPositiveNegativeAndZeroDelta(t *testing.T) {
	tests := []struct {
		name              string
		userID            int
		channelID         int
		preConsumed       int
		actualQuota       int
		expectedUserQuota int
		expectedLogType   int
		expectedLogQuota  int
	}{
		{name: "positive delta", userID: 40, channelID: 40, preConsumed: 500, actualQuota: 535, expectedUserQuota: 9965, expectedLogType: model.LogTypeConsume, expectedLogQuota: 35},
		{name: "negative delta", userID: 41, channelID: 41, preConsumed: 600, actualQuota: 535, expectedUserQuota: 10065, expectedLogType: model.LogTypeRefund, expectedLogQuota: 65},
		{name: "zero delta", userID: 42, channelID: 42, preConsumed: 535, actualQuota: 535, expectedUserQuota: 10000, expectedLogType: model.LogTypeConsume, expectedLogQuota: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncate(t)
			seedUser(t, tt.userID, 10000)
			seedChannel(t, tt.channelID)
			task := makeTask(tt.userID, tt.channelID, tt.preConsumed, 0, BillingSourceWallet, 0)
			markTaskAsSeedance25(task, false)

			originalLogConsumeEnabled := common.LogConsumeEnabled
			common.LogConsumeEnabled = false
			t.Cleanup(func() { common.LogConsumeEnabled = originalLogConsumeEnabled })

			RecalculateTaskQuota(context.Background(), task, tt.actualQuota, "seedance-2.5 completion-token settlement")

			require.Equal(t, tt.expectedUserQuota, getUserQuota(t, tt.userID))
			require.Equal(t, tt.actualQuota, task.Quota)
			require.Equal(t, int64(1), countLogs(t))
			log := getLastLog(t)
			require.NotNil(t, log)
			require.Equal(t, tt.expectedLogType, log.Type)
			require.Equal(t, tt.expectedLogQuota, log.Quota)
			require.Equal(t, seedance25TenantAliasForServiceTest, log.ModelName)
			other := taskBillingLogOther(t, log)
			require.EqualValues(t, tt.preConsumed, other["pre_consumed_quota"])
			require.EqualValues(t, tt.actualQuota, other["actual_quota"])
			require.NotContains(t, other, "upstream_model_name")
			require.NotContains(t, log.Other, "provider_model_marker")
		})
	}
}

func TestSeedance25SettlementNeverFallsBackToTotalTokens(t *testing.T) {
	truncate(t)
	const userID = 43
	seedUser(t, userID, 10000)
	task := makeTask(userID, 0, 100, 0, BillingSourceWallet, 0)
	markTaskAsSeedance25(task, false)

	settleTaskBillingOnComplete(context.Background(), &mockAdaptor{adjustReturn: 0}, task, &relaycommon.TaskInfo{
		Status:      model.TaskStatusSuccess,
		TotalTokens: 100,
	})

	require.Equal(t, 10000, getUserQuota(t, userID))
	require.Equal(t, 100, task.Quota)
	require.Equal(t, int64(0), countLogs(t))
}

type seedance25PollingAdaptor struct {
	result       relaycommon.TaskInfo
	adjustReturn int
	fetchCount   int
	fetchErr     error
}

func (a *seedance25PollingAdaptor) Init(_ *relaycommon.RelayInfo) {}

func (a *seedance25PollingAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	a.fetchCount++
	if a.fetchErr != nil {
		return nil, a.fetchErr
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"status":"test"}`)),
	}, nil
}

func TestSeedance25PollingErrorDoesNotExposeProviderDiagnostic(t *testing.T) {
	truncate(t)
	_, channel, taskMap := seedance25PollingFixture(t, 47, 47, 47, 400)
	adaptor := &seedance25PollingAdaptor{fetchErr: errors.New("provider_task_marker internal endpoint marker")}

	err := updateVideoSingleTask(context.Background(), adaptor, channel, "provider_task_marker", taskMap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "task_public")
	require.NotContains(t, err.Error(), "provider_task_marker")
	require.NotContains(t, err.Error(), "internal endpoint marker")
}

func (a *seedance25PollingAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) {
	result := a.result
	return &result, nil
}

func (a *seedance25PollingAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return a.adjustReturn
}

func (a *seedance25PollingAdaptor) ApplyTaskResultPolicy(_ *model.Task, result *relaycommon.TaskInfo, _ []byte) ([]byte, error) {
	if result.Status == model.TaskStatusSuccess && (!result.CompletionTokensValid || result.CompletionTokens <= 0) {
		result.Status = model.TaskStatusFailure
		result.Url = ""
		result.Reason = "upstream success response did not contain valid completion token usage"
		return common.Marshal(map[string]any{
			"status": "failed",
			"error": map[string]any{
				"code":    "invalid_upstream_usage",
				"message": result.Reason,
			},
		})
	}
	if result.Status == model.TaskStatusFailure {
		result.Reason = "request parameters are not supported for seedance-2.5"
		return common.Marshal(map[string]any{
			"status": "failed",
			"error": map[string]any{
				"code":    "invalid_request_error",
				"message": result.Reason,
			},
		})
	}
	return common.Marshal(map[string]any{
		"status":  "succeeded",
		"content": map[string]any{"video_url": result.Url},
		"usage":   map[string]any{"completion_tokens": result.CompletionTokens},
	})
}

func seedance25PollingFixture(t *testing.T, userID, tokenID, channelID, quota int) (*model.Task, *model.Channel, map[string]*model.Task) {
	t.Helper()
	seedUser(t, userID, 10000)
	seedToken(t, tokenID, userID, "test-token-key", 5000)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, quota, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_public_seedance25"
	markTaskAsSeedance25(task, false)
	task.PrivateData.UpstreamTaskID = "provider_task_marker"
	require.NoError(t, model.DB.Create(task).Error)
	channel := &model.Channel{Id: channelID, Type: constant.ChannelTypeDoubaoVideo, Key: "test-channel-key"}
	return task, channel, map[string]*model.Task{"provider_task_marker": task}
}

func TestSeedance25MissingUsageRefundsOnceAndNeverPublishesSuccess(t *testing.T) {
	truncate(t)
	task, channel, taskMap := seedance25PollingFixture(t, 44, 44, 44, 400)
	adaptor := &seedance25PollingAdaptor{
		result: relaycommon.TaskInfo{
			Status:      model.TaskStatusSuccess,
			Url:         "https://example.invalid/should-not-publish.mp4",
			TotalTokens: 999,
		},
	}

	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "provider_task_marker", taskMap))
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "provider_task_marker", taskMap))

	require.Equal(t, 1, adaptor.fetchCount)
	require.EqualValues(t, model.TaskStatusFailure, task.Status)
	require.Empty(t, task.PrivateData.ResultURL)
	require.Contains(t, string(task.Data), "invalid_upstream_usage")
	require.NotContains(t, string(task.Data), "should-not-publish")
	require.Equal(t, 10400, getUserQuota(t, 44))
	require.Equal(t, int64(1), countLogs(t))
}

func TestSeedance25TaskTypeConstraintRefundsExactlyOnceAcrossReplay(t *testing.T) {
	truncate(t)
	task, channel, taskMap := seedance25PollingFixture(t, 45, 45, 45, 400)
	adaptor := &seedance25PollingAdaptor{
		result: relaycommon.TaskInfo{
			Status:            model.TaskStatusFailure,
			UpstreamErrorCode: "InvalidParameter.TaskTypeConstraint",
			Reason:            "raw provider diagnostic marker",
		},
	}

	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "provider_task_marker", taskMap))
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "provider_task_marker", taskMap))

	require.Equal(t, 1, adaptor.fetchCount)
	require.EqualValues(t, model.TaskStatusFailure, task.Status)
	require.Equal(t, "request parameters are not supported for seedance-2.5", task.FailReason)
	require.Contains(t, string(task.Data), "invalid_request_error")
	require.NotContains(t, string(task.Data), "raw provider diagnostic marker")
	require.Equal(t, 10400, getUserQuota(t, 45))
	require.Equal(t, int64(1), countLogs(t))
}

func TestSeedance25SuccessSettlesExactlyOnceAcrossReplay(t *testing.T) {
	truncate(t)
	task, channel, taskMap := seedance25PollingFixture(t, 46, 46, 46, 600)
	adaptor := &seedance25PollingAdaptor{
		result: relaycommon.TaskInfo{
			Status:                model.TaskStatusSuccess,
			Url:                   "https://example.invalid/result.mp4",
			CompletionTokens:      100,
			CompletionTokensValid: true,
			TotalTokens:           999,
		},
		adjustReturn: 535,
	}

	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "provider_task_marker", taskMap))
	require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, "provider_task_marker", taskMap))

	require.Equal(t, 1, adaptor.fetchCount)
	require.EqualValues(t, model.TaskStatusSuccess, task.Status)
	require.Equal(t, 535, task.Quota)
	require.Equal(t, 10065, getUserQuota(t, 46))
	require.Equal(t, int64(1), countLogs(t))
	log := getLastLog(t)
	require.NotNil(t, log)
	other := taskBillingLogOther(t, log)
	require.EqualValues(t, 535, other["actual_quota"])
}
