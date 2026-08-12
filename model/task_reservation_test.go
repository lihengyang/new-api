package model

import (
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func mediaKitReservationParams(userID int, clientRequestID string) TaskReservationParams {
	return TaskReservationParams{
		TaskID: GenerateTaskID(), TokenId: userID, ClientRequestID: clientRequestID,
		ClientRequestHash: "hash_" + clientRequestID, UserId: userID, Group: "default", ChannelId: userID,
		Platform: constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMediaKit)),
		Action:   constant.TaskActionGenerate, OriginModelName: "lsf-video-enhancement-tenant",
		UpstreamModelName: "mediakit-video-enhancement", ExclusiveNonTerminalPerUser: true,
	}
}

func seedMediaKitReservationUser(t *testing.T, userID int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: userID, Username: "mediakit-reservation-" + strconv.Itoa(userID),
		AffCode: "mediakit-reservation-aff-" + strconv.Itoa(userID), Status: common.UserStatusEnabled,
	}).Error)
}

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

func TestMediaKitExclusiveReservationAllowsEmptyClientRequestID(t *testing.T) {
	truncateTables(t)
	seedMediaKitReservationUser(t, 8101)
	task, err := CreateTaskReservation(mediaKitReservationParams(8101, ""))
	require.NoError(t, err)
	require.Nil(t, task.ClientRequestID)
}

func TestMediaKitExclusiveReservationBlocksSameUserButNotDifferentUsers(t *testing.T) {
	truncateTables(t)
	seedMediaKitReservationUser(t, 8102)
	seedMediaKitReservationUser(t, 8103)
	_, err := CreateTaskReservation(mediaKitReservationParams(8102, "first"))
	require.NoError(t, err)
	_, err = CreateTaskReservation(mediaKitReservationParams(8102, "second"))
	require.ErrorIs(t, err, ErrTaskExclusiveActive)
	_, err = CreateTaskReservation(mediaKitReservationParams(8103, "other-user"))
	require.NoError(t, err)
}

func TestMediaKitExclusiveReservationPreservesIdempotentDuplicate(t *testing.T) {
	truncateTables(t)
	seedMediaKitReservationUser(t, 8104)
	_, err := CreateTaskReservation(mediaKitReservationParams(8104, "same-request"))
	require.NoError(t, err)
	_, err = CreateTaskReservation(mediaKitReservationParams(8104, "same-request"))
	require.True(t, IsTaskClientRequestDuplicateError(err))
}

func TestMediaKitExclusiveReservationConcurrentAdmissionAllowsOnlyOne(t *testing.T) {
	truncateTables(t)
	seedMediaKitReservationUser(t, 8105)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, requestID := range []string{"concurrent-a", "concurrent-b"} {
		wg.Add(1)
		go func(requestID string) {
			defer wg.Done()
			<-start
			_, err := CreateTaskReservation(mediaKitReservationParams(8105, requestID))
			errs <- err
		}(requestID)
	}
	close(start)
	wg.Wait()
	close(errs)
	successes, blocked := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrTaskExclusiveActive):
			blocked++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, blocked)
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

func TestFinalizeTaskReservationUpdatesReservedToNotStart(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(701, "req_finalize"))
	require.NoError(t, err)

	err = FinalizeTaskReservation(FinalizeTaskReservationParams{
		ID:        task.ID,
		Quota:     123,
		Action:    constant.TaskActionGenerate,
		Platform:  constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		ChannelId: 8,
		Properties: Properties{
			OriginModelName:   "seedance-2-0",
			UpstreamModelName: "seedance-2-0-pro",
		},
		PrivateData: TaskPrivateData{
			UpstreamTaskID: "upstream_task_123",
			TokenId:        701,
		},
		UpdatedAt: 222,
	})
	require.NoError(t, err)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.Equal(t, TaskStatusNotStart, reloaded.Status)
	require.Equal(t, "0%", reloaded.Progress)
	require.EqualValues(t, 222, reloaded.UpdatedAt)
}

func TestFinalizeTaskReservationStoresQuotaDataPrivateDataAndProperties(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(702, "req_finalize_fields"))
	require.NoError(t, err)
	data, err := common.Marshal(map[string]any{"task": "submitted"})
	require.NoError(t, err)

	err = FinalizeTaskReservation(FinalizeTaskReservationParams{
		ID:        task.ID,
		Quota:     456,
		Action:    constant.TaskActionGenerate,
		Platform:  constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		ChannelId: 9,
		Properties: Properties{
			OriginModelName:   "seedance-2-0",
			UpstreamModelName: "seedance-2-0-pro",
		},
		PrivateData: TaskPrivateData{
			UpstreamTaskID: "upstream_task_456",
			TokenId:        702,
			BillingContext: &TaskBillingContext{
				OriginModelName: "seedance-2-0",
				PerCallBilling:  true,
			},
		},
		Data: data,
	})
	require.NoError(t, err)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.Equal(t, 456, reloaded.Quota)
	require.Equal(t, constant.TaskActionGenerate, reloaded.Action)
	require.Equal(t, constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)), reloaded.Platform)
	require.Equal(t, 9, reloaded.ChannelId)
	require.Equal(t, "seedance-2-0", reloaded.Properties.OriginModelName)
	require.Equal(t, "seedance-2-0-pro", reloaded.Properties.UpstreamModelName)
	require.Equal(t, "upstream_task_456", reloaded.PrivateData.UpstreamTaskID)
	require.Equal(t, 702, reloaded.PrivateData.TokenId)
	require.NotNil(t, reloaded.PrivateData.BillingContext)
	require.JSONEq(t, `{"task":"submitted"}`, string(reloaded.Data))
}

func TestFinalizeTaskReservationPollableWhenNotStart(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(703, "req_finalize_pollable"))
	require.NoError(t, err)
	err = FinalizeTaskReservation(FinalizeTaskReservationParams{ID: task.ID})
	require.NoError(t, err)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.True(t, reloaded.Status.IsUpstreamPollable())
}

func TestFinalizeTaskReservationRefusesNonReservedTask(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(704, "req_finalize_refuse"))
	require.NoError(t, err)
	require.NoError(t, FinalizeTaskReservation(FinalizeTaskReservationParams{ID: task.ID}))

	err = FinalizeTaskReservation(FinalizeTaskReservationParams{ID: task.ID})
	require.ErrorIs(t, err, ErrTaskReservationNotReserved)
}

func TestFailTaskReservationUpdatesReservedToFailure(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(801, "req_fail"))
	require.NoError(t, err)
	err = FailTaskReservation(FailTaskReservationParams{
		ID:         task.ID,
		FailReason: "upstream failed",
		UpdatedAt:  333,
	})
	require.NoError(t, err)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.Equal(t, TaskStatus(TaskStatusFailure), reloaded.Status)
	require.Equal(t, "100%", reloaded.Progress)
	require.Zero(t, reloaded.Quota)
	require.Equal(t, "upstream failed", reloaded.FailReason)
	require.EqualValues(t, 333, reloaded.UpdatedAt)
}

func TestFailTaskReservationNotUpstreamPollable(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(802, "req_fail_pollable"))
	require.NoError(t, err)
	require.NoError(t, FailTaskReservation(FailTaskReservationParams{ID: task.ID}))

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	require.False(t, reloaded.Status.IsUpstreamPollable())
}

func TestFailTaskReservationRefusesNonReservedTask(t *testing.T) {
	truncateTables(t)

	task, err := CreateTaskReservation(reservationParams(803, "req_fail_refuse"))
	require.NoError(t, err)
	require.NoError(t, FailTaskReservation(FailTaskReservationParams{ID: task.ID}))

	err = FailTaskReservation(FailTaskReservationParams{ID: task.ID})
	require.ErrorIs(t, err, ErrTaskReservationNotReserved)
}
