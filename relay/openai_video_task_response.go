package relay

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func BuildOpenAIVideoFromTask(task *model.Task) *dto.OpenAIVideo {
	video := dto.NewOpenAIVideo()
	if task == nil {
		return video
	}

	video.ID = task.TaskID
	video.TaskID = task.TaskID
	video.Model = task.Properties.OriginModelName
	video.Status = openAIVideoStatusFromTaskStatus(task.Status)
	video.SetProgressStr(task.Progress)
	video.CreatedAt = task.CreatedAt
	if video.CreatedAt == 0 {
		video.CreatedAt = task.SubmitTime
	}

	if task.ClientRequestID != nil {
		video.SetMetadata("client_request_id", *task.ClientRequestID)
	}

	if isOpenAIVideoTaskTerminal(video.Status) {
		if task.FinishTime != 0 {
			video.CompletedAt = task.FinishTime
		} else if task.UpdatedAt != 0 {
			video.CompletedAt = task.UpdatedAt
		}
		applySafeOpenAIVideoTaskData(video, task)
		if task.Status == model.TaskStatusSuccess && task.PrivateData.ResultURL != "" {
			video.SetMetadata("url", task.PrivateData.ResultURL)
		}
	}

	return video
}

func openAIVideoStatusFromTaskStatus(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusReserved, model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued:
		return dto.VideoStatusQueued
	case model.TaskStatusInProgress:
		return dto.VideoStatusInProgress
	case model.TaskStatusSuccess:
		return dto.VideoStatusCompleted
	case model.TaskStatusFailure:
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}

func isOpenAIVideoTaskTerminal(status string) bool {
	return status == dto.VideoStatusCompleted || status == dto.VideoStatusFailed
}

func applySafeOpenAIVideoTaskData(video *dto.OpenAIVideo, task *model.Task) {
	if len(task.Data) == 0 {
		return
	}
	var existing dto.OpenAIVideo
	if err := common.Unmarshal(task.Data, &existing); err != nil {
		return
	}
	if existing.Usage != nil {
		video.Usage = existing.Usage
	}
	if existing.Error != nil {
		video.Error = existing.Error
	}
	if existing.Seconds != "" {
		video.Seconds = existing.Seconds
	}
	if existing.Size != "" {
		video.Size = existing.Size
	}
	if existing.CompletedAt != 0 {
		video.CompletedAt = existing.CompletedAt
	}
	if existing.Metadata != nil {
		if url, ok := existing.Metadata["url"]; ok && url != "" {
			video.SetMetadata("url", url)
		}
	}
}
