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
var ErrTaskExclusiveActive = errors.New("a non-terminal task is already active for this user")

type TaskReservationParams struct {
	TaskID                      string
	TokenId                     int
	ClientRequestID             string
	ClientRequestHash           string
	UserId                      int
	Group                       string
	ChannelId                   int
	Platform                    constant.TaskPlatform
	Action                      string
	OriginModelName             string
	UpstreamModelName           string
	SubmitTime                  int64
	ExclusiveNonTerminalPerUser bool
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
	Data       json.RawMessage
	UpdatedAt  int64
}

func CreateTaskReservation(params TaskReservationParams) (*Task, error) {
	clientRequestID := strings.TrimSpace(params.ClientRequestID)
	if clientRequestID == "" && !params.ExclusiveNonTerminalPerUser {
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

	var clientRequestIDPtr *string
	if clientRequestID != "" {
		clientRequestIDPtr = &clientRequestID
	}
	var clientRequestHash *string
	if params.ClientRequestHash != "" {
		clientRequestHash = &params.ClientRequestHash
	}

	task := &Task{
		TaskID:            taskID,
		TokenId:           params.TokenId,
		ClientRequestID:   clientRequestIDPtr,
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
	if !params.ExclusiveNonTerminalPerUser {
		if err := DB.Create(task).Error; err != nil {
			return nil, err
		}
		return task, nil
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		// Serialize admission on the existing user row. This is portable across
		// the supported databases and avoids a schema migration or process-local
		// mutex that would not protect a multi-instance deployment.
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Select("id").First(&user, params.UserId).Error; err != nil {
			return err
		}
		if clientRequestID != "" {
			var existing Task
			err := tx.Where("token_id = ? and client_request_id = ?", params.TokenId, clientRequestID).First(&existing).Error
			if err == nil {
				return gorm.ErrDuplicatedKey
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		var active int64
		if err := tx.Model(&Task{}).
			Where("user_id = ? AND platform = ?", params.UserId, params.Platform).
			Where("status NOT IN ?", []TaskStatus{TaskStatusFailure, TaskStatusSuccess}).
			Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrTaskExclusiveActive
		}
		return tx.Create(task).Error
	})
	if err != nil {
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

	updates := map[string]any{
		"status":      TaskStatusFailure,
		"progress":    "100%",
		"quota":       0,
		"fail_reason": params.FailReason,
		"updated_at":  updatedAt,
	}
	if len(params.Data) > 0 {
		updates["data"] = params.Data
	}

	result := DB.Model(&Task{}).
		Where("id = ? AND status = ?", params.ID, TaskStatusReserved).
		Updates(updates)
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
