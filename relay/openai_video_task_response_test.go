package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func strPtr(s string) *string {
	return &s
}

func TestBuildOpenAIVideoFromTaskReservedMapsToQueued(t *testing.T) {
	task := &model.Task{
		TaskID:    "task_reserved",
		Status:    model.TaskStatusReserved,
		Progress:  "0%",
		CreatedAt: 123,
		Properties: model.Properties{
			OriginModelName: "seedance-2-0",
		},
	}

	video := BuildOpenAIVideoFromTask(task)

	require.Equal(t, "task_reserved", video.ID)
	require.Equal(t, "task_reserved", video.TaskID)
	require.Equal(t, "video", video.Object)
	require.Equal(t, "seedance-2-0", video.Model)
	require.Equal(t, dto.VideoStatusQueued, video.Status)
	require.Equal(t, 0, video.Progress)
	require.EqualValues(t, 123, video.CreatedAt)
}

func TestBuildOpenAIVideoFromTaskReturnsClientRequestIDMetadata(t *testing.T) {
	task := &model.Task{
		TaskID:          "task_client_request",
		Status:          model.TaskStatusQueued,
		Progress:        "10%",
		ClientRequestID: strPtr("req_123"),
	}

	video := BuildOpenAIVideoFromTask(task)

	require.NotNil(t, video.Metadata)
	require.Equal(t, "req_123", video.Metadata["client_request_id"])
}

func TestBuildOpenAIVideoFromTaskDoesNotExposeInternalFields(t *testing.T) {
	task := &model.Task{
		ID:              42,
		TaskID:          "task_safe",
		TokenId:         99,
		UserId:          1001,
		Group:           "vip",
		ChannelId:       7,
		Quota:           12345,
		Status:          model.TaskStatusSuccess,
		Progress:        "100%",
		ClientRequestID: strPtr("req_safe"),
		PrivateData: model.TaskPrivateData{
			Key:            "secret-provider-key",
			UpstreamTaskID: "upstream_task_secret",
			ResultURL:      "https://example.com/result.mp4",
			BillingContext: &model.TaskBillingContext{
				ModelRatio: 1.5,
				GroupRatio: 2.0,
				OtherRatios: map[string]float64{
					"otherMultiplier": 3.0,
				},
			},
		},
	}

	body, err := common.Marshal(BuildOpenAIVideoFromTask(task))
	require.NoError(t, err)
	bodyString := string(body)

	require.NotContains(t, bodyString, "token_id")
	require.NotContains(t, bodyString, "user_id")
	require.NotContains(t, bodyString, "group")
	require.NotContains(t, bodyString, "channel_id")
	require.NotContains(t, bodyString, "private_data")
	require.NotContains(t, bodyString, "upstream_task_secret")
	require.NotContains(t, bodyString, "secret-provider-key")
	require.NotContains(t, bodyString, "quota")
	require.NotContains(t, bodyString, "modelRatio")
	require.NotContains(t, bodyString, "groupRatio")
	require.NotContains(t, bodyString, "otherMultiplier")
	require.NotContains(t, bodyString, "pre-deduct")
	require.NotContains(t, bodyString, "refund")
	require.NotContains(t, bodyString, "top-up")
}

func TestBuildOpenAIVideoFromTaskStatusMapping(t *testing.T) {
	tests := []struct {
		name     string
		status   model.TaskStatus
		expected string
	}{
		{name: "queued", status: model.TaskStatusQueued, expected: dto.VideoStatusQueued},
		{name: "submitted", status: model.TaskStatusSubmitted, expected: dto.VideoStatusQueued},
		{name: "not start", status: model.TaskStatusNotStart, expected: dto.VideoStatusQueued},
		{name: "in progress", status: model.TaskStatusInProgress, expected: dto.VideoStatusInProgress},
		{name: "completed", status: model.TaskStatusSuccess, expected: dto.VideoStatusCompleted},
		{name: "failed", status: model.TaskStatusFailure, expected: dto.VideoStatusFailed},
		{name: "unknown", status: model.TaskStatusUnknown, expected: dto.VideoStatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			video := BuildOpenAIVideoFromTask(&model.Task{
				TaskID:   "task_status",
				Status:   tt.status,
				Progress: "50%",
			})
			require.Equal(t, tt.expected, video.Status)
		})
	}
}

func TestBuildOpenAIVideoFromTaskPreservesSafeCompletedFields(t *testing.T) {
	data, err := common.Marshal(dto.OpenAIVideo{
		Metadata: map[string]any{
			"url": "https://example.com/from-data.mp4",
		},
		Usage: &dto.OpenAIVideoUsage{
			CompletionTokens: 12,
			TotalTokens:      34,
		},
		Seconds:     "5",
		Size:        "1280x720",
		CompletedAt: 456,
	})
	require.NoError(t, err)
	task := &model.Task{
		TaskID:    "task_completed",
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		UpdatedAt: 123,
		Data:      data,
	}

	video := BuildOpenAIVideoFromTask(task)

	require.Equal(t, dto.VideoStatusCompleted, video.Status)
	require.Equal(t, "https://example.com/from-data.mp4", video.Metadata["url"])
	require.NotNil(t, video.Usage)
	require.Equal(t, 12, video.Usage.CompletionTokens)
	require.Equal(t, 34, video.Usage.TotalTokens)
	require.Equal(t, "5", video.Seconds)
	require.Equal(t, "1280x720", video.Size)
	require.EqualValues(t, 456, video.CompletedAt)
}

func TestBuildOpenAIVideoFromTaskDoesNotCopyUnsafeMetadataFromData(t *testing.T) {
	data, err := common.Marshal(dto.OpenAIVideo{
		Metadata: map[string]any{
			"url":            "https://example.com/safe.mp4",
			"ProjectName":    "SECRET_PROJECT",
			"project_name":   "SECRET_PROJECT",
			"upstream_model": "dreamina-internal",
			"endpoint_id":    "ep-secret",
			"channel_id":     123,
		},
	})
	require.NoError(t, err)
	task := &model.Task{
		TaskID:   "task_completed",
		Status:   model.TaskStatusSuccess,
		Progress: "100%",
		Data:     data,
	}

	video := BuildOpenAIVideoFromTask(task)

	require.NotNil(t, video.Metadata)
	require.Equal(t, "https://example.com/safe.mp4", video.Metadata["url"])
	require.NotContains(t, video.Metadata, "ProjectName")
	require.NotContains(t, video.Metadata, "project_name")
	require.NotContains(t, video.Metadata, "upstream_model")
	require.NotContains(t, video.Metadata, "endpoint_id")
	require.NotContains(t, video.Metadata, "channel_id")

	body, err := common.Marshal(video)
	require.NoError(t, err)
	bodyString := string(body)
	require.NotContains(t, bodyString, "SECRET_PROJECT")
	require.NotContains(t, bodyString, "dreamina-internal")
	require.NotContains(t, bodyString, "ep-secret")
}
