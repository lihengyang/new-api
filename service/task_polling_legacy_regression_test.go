package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

type legacyTaskPollingAdaptor struct {
	result     relaycommon.TaskInfo
	fetchCount int
}

func (a *legacyTaskPollingAdaptor) Init(*relaycommon.RelayInfo) {}

func (a *legacyTaskPollingAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	a.fetchCount++
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"legacy":"response"}`)),
	}, nil
}

func (a *legacyTaskPollingAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) {
	result := a.result
	return &result, nil
}

func (a *legacyTaskPollingAdaptor) AdjustBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int {
	return 0
}

func TestLegacyPollingBehaviorRemainsUnscopedFromSeedance25Policy(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus model.TaskStatus
		result        relaycommon.TaskInfo
		expected      model.TaskStatus
	}{
		{
			name:          "terminal task still follows legacy fetch path",
			initialStatus: model.TaskStatusSuccess,
			result: relaycommon.TaskInfo{
				Status: model.TaskStatusSuccess,
				Url:    "https://example.invalid/legacy-terminal.mp4",
			},
			expected: model.TaskStatusSuccess,
		},
		{
			name:          "normal success polling",
			initialStatus: model.TaskStatusInProgress,
			result: relaycommon.TaskInfo{
				Status: model.TaskStatusSuccess,
				Url:    "https://example.invalid/legacy-success.mp4",
			},
			expected: model.TaskStatusSuccess,
		},
		{
			name:          "normal failure polling",
			initialStatus: model.TaskStatusInProgress,
			result: relaycommon.TaskInfo{
				Status: model.TaskStatusFailure,
				Reason: "legacy failure placeholder",
			},
			expected: model.TaskStatusFailure,
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncate(t)
			task := makeTask(0, 0, 0, 0, BillingSourceWallet, 0)
			task.TaskID = "task_legacy_regression_" + string(rune('a'+index))
			task.Status = tt.initialStatus
			task.Properties.OriginModelName = "legacy-model-placeholder"
			task.PrivateData.UpstreamTaskID = "legacy-upstream-task-placeholder"
			require.NoError(t, model.DB.Create(task).Error)

			adaptor := &legacyTaskPollingAdaptor{result: tt.result}
			channel := &model.Channel{Type: constant.ChannelTypeDoubaoVideo, Key: "legacy-key-placeholder"}
			taskMap := map[string]*model.Task{task.GetUpstreamTaskID(): task}

			require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, channel, task.GetUpstreamTaskID(), taskMap))
			require.Equal(t, 1, adaptor.fetchCount)
			require.Equal(t, tt.expected, task.Status)

			var reloaded model.Task
			require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
			require.Equal(t, tt.expected, reloaded.Status)
		})
	}
}
