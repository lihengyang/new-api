package model

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

const TaskClientRequestUniqueIndexName = "idx_tasks_token_client_request"

var ErrTaskReservationNotReserved = errors.New("task reservation not found or no longer reserved")

type TaskReservationParams struct {
	TaskID            string
	TokenId           int
	ClientRequestID   string
	ClientRequestHash string
	UserId            int
	Group             string
	ChannelId         int
	Platform          constant.TaskPlatform
	Action            string
	OriginModelName   string
	UpstreamModelName string
	SubmitTime        int64
}

type FinalizeTaskReservationParams struct {
	ID          int64
	Quota       int
	Action      string
	Platform    constant.TaskPlatform
	ChannelId   int
	Properties  Properties
	PrivateData TaskPrivateData
	Data        json.RawMessage
	UpdatedAt   int64
}

type FailTaskReservationParams struct {
	ID         int64
	FailReason string
	UpdatedAt  int64
}

func CreateTaskReservation(params TaskReservationParams) (*Task, error) {
	clientRequestID := strings.TrimSpace(params.ClientRequestID)
	if clientRequestID == "" {
		return nil, errors.New("client_request_id is required")
	}

	taskID := params.TaskID
	if taskID == "" {
		taskID = GenerateTaskID()
	}
	submitTime := params.SubmitTime
	if submitTime == 0 {
		submitTime = time.Now().Unix()
	}

	var clientRequestHash *string
	if params.ClientRequestHash != "" {
		clientRequestHash = &params.ClientRequestHash
	}

	task := &Task{
		TaskID:            taskID,
		TokenId:           params.TokenId,
		ClientRequestID:   &clientRequestID,
		ClientRequestHash: clientRequestHash,
		UserId:            params.UserId,
		Group:             params.Group,
		ChannelId:         params.ChannelId,
		Platform:          params.Platform,
		Action:            params.Action,
		Status:            TaskStatusReserved,
		Progress:          "0%",
		SubmitTime:        submitTime,
		Quota:             0,
		Properties: Properties{
			OriginModelName:   params.OriginModelName,
			UpstreamModelName: params.UpstreamModelName,
		},
		PrivateData: TaskPrivateData{
			TokenId: params.TokenId,
		},
	}
	if err := DB.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func GetTaskByTokenClientRequestID(tokenID int, clientRequestID string) (*Task, bool, error) {
	if clientRequestID == "" {
		return nil, false, nil
	}
	var task *Task
	err := DB.Where("token_id = ? and client_request_id = ?", tokenID, clientRequestID).First(&task).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return task, exist, nil
}

func FinalizeTaskReservation(params FinalizeTaskReservationParams) error {
	updatedAt := params.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().Unix()
	}

	result := DB.Model(&Task{}).
		Where("id = ? AND status = ?", params.ID, TaskStatusReserved).
		Updates(map[string]any{
			"status":       TaskStatusNotStart,
			"progress":     "0%",
			"quota":        params.Quota,
			"action":       params.Action,
			"platform":     params.Platform,
			"channel_id":   params.ChannelId,
			"properties":   params.Properties,
			"private_data": params.PrivateData,
			"data":         params.Data,
			"updated_at":   updatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskReservationNotReserved
	}
	return nil
}

func FailTaskReservation(params FailTaskReservationParams) error {
	updatedAt := params.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().Unix()
	}

	result := DB.Model(&Task{}).
		Where("id = ? AND status = ?", params.ID, TaskStatusReserved).
		Updates(map[string]any{
			"status":      TaskStatusFailure,
			"progress":    "100%",
			"quota":       0,
			"fail_reason": params.FailReason,
			"updated_at":  updatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskReservationNotReserved
	}
	return nil
}

func IsTaskClientRequestDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	errText := strings.ToLower(err.Error())
	indexName := strings.ToLower(TaskClientRequestUniqueIndexName)
	if strings.Contains(errText, indexName) {
		return true
	}
	return strings.Contains(errText, "unique") &&
		strings.Contains(errText, "token_id") &&
		strings.Contains(errText, "client_request_id")
}
