package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func reservationParams(tokenID int, clientRequestID string) TaskReservationParams {
	return TaskReservationParams{
		TaskID:            GenerateTaskID(),
		TokenId:           tokenID,
		ClientRequestID:   clientRequestID,
		ClientRequestHash: "hash_" + clientRequestID,
		UserId:            1001,
		Group:             "default",
		ChannelId:         7,
		Platform:          constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Action:            constant.TaskActionGenerate,
		OriginModelName:   "seedance-2-0",
		UpstreamModelName: "seedance-2-0-pro",
		SubmitTime:        1234567890,
	}
}

func TestCreateTaskReservationStoresTokenAndClientRequestID(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(101, "req_store"))
	require.NoError(t, err)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.Equal(t, task.TaskID, reloaded.TaskID)
	require.Equal(t, 101, reloaded.TokenId)
	require.NotNil(t, reloaded.ClientRequestID)
	require.Equal(t, "req_store", *reloaded.ClientRequestID)
	require.NotNil(t, reloaded.ClientRequestHash)
	require.Equal(t, "hash_req_store", *reloaded.ClientRequestHash)
	require.Equal(t, 1001, reloaded.UserId)
	require.Equal(t, "default", reloaded.Group)
	require.Equal(t, 7, reloaded.ChannelId)
	require.Equal(t, constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)), reloaded.Platform)
	require.Equal(t, constant.TaskActionGenerate, reloaded.Action)
	require.Equal(t, "seedance-2-0", reloaded.Properties.OriginModelName)
	require.Equal(t, "seedance-2-0-pro", reloaded.Properties.UpstreamModelName)
	require.EqualValues(t, 1234567890, reloaded.SubmitTime)
}

func TestCreateTaskReservationRejectsEmptyClientRequestID(t *testing.T) {
	truncateTables(t)

	_, err := CreateTaskReservation(reservationParams(102, "   "))

	require.Error(t, err)
}

func TestCreateTaskReservationTrimsClientRequestIDBeforeStoring(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(103, "  req_trimmed  "))
	require.NoError(t, err)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.NotNil(t, reloaded.ClientRequestID)
	require.Equal(t, "req_trimmed", *reloaded.ClientRequestID)
}

func TestCreateTaskReservationDuplicateSameTokenDetected(t *testing.T) {
	truncateTables(t)

	_, err := CreateTaskReservation(reservationParams(201, "req_duplicate"))
	require.NoError(t, err)
	_, err = CreateTaskReservation(reservationParams(201, "req_duplicate"))

	require.Error(t, err)
	require.True(t, IsTaskClientRequestDuplicateError(err))
}

func TestGetTaskByTokenClientRequestIDFindsReservation(t *testing.T) {
	truncateTables(t)

	created, err := CreateTaskReservation(reservationParams(301, "req_find"))
	require.NoError(t, err)

	found, exists, err := GetTaskByTokenClientRequestID(301, "req_find")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, created.ID, found.ID)
	require.Equal(t, created.TaskID, found.TaskID)
}

func TestCreateTaskReservationDifferentTokenSameClientRequestIDAllowed(t *testing.T) {
	truncateTables(t)

	_, err := CreateTaskReservation(reservationParams(401, "req_shared"))
	require.NoError(t, err)
	_, err = CreateTaskReservation(reservationParams(402, "req_shared"))
	require.NoError(t, err)
}

func TestCreateTaskReservationStatusReservedAndNotPollable(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(501, "req_reserved"))
	require.NoError(t, err)

	require.Equal(t, TaskStatusReserved, task.Status)
	require.Equal(t, "0%", task.Progress)
	require.False(t, task.Status.IsUpstreamPollable())
}

func TestCreateTaskReservationQuotaZeroAndPrivateTokenStored(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(601, "req_private"))
	require.NoError(t, err)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.Zero(t, reloaded.Quota)
	require.Equal(t, 601, reloaded.PrivateData.TokenId)
}
