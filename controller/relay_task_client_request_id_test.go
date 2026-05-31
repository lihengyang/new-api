package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRespondTaskErrorWritesOpenAIClientRequestIDError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	openAIError := types.OpenAIError{
		Message: relaycommon.ClientRequestIDErrorMessage,
		Type:    relaycommon.ClientRequestIDErrorType,
		Param:   "metadata.client_request_id",
		Code:    relaycommon.ClientRequestIDErrorCode,
	}

	respondTaskError(c, &dto.TaskError{
		Code:       relaycommon.ClientRequestIDErrorCode,
		Message:    relaycommon.ClientRequestIDErrorMessage,
		StatusCode: http.StatusBadRequest,
		Data:       openAIError,
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body map[string]types.OpenAIError
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, openAIError, body["error"])
}

func TestRespondTaskErrorOpenAIErrorDataWithOtherCodeUsesTaskErrorShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	respondTaskError(c, &dto.TaskError{
		Code:       "other_error",
		Message:    "other message",
		StatusCode: http.StatusBadRequest,
		Data: types.OpenAIError{
			Message: "nested message",
			Type:    "invalid_request_error",
			Param:   "metadata.client_request_id",
			Code:    relaycommon.ClientRequestIDErrorCode,
		},
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.NotContains(t, body, "error")
	require.Equal(t, "other_error", body["code"])
	require.Equal(t, "other message", body["message"])
	require.Contains(t, body, "data")
}

func setupControllerTaskReservationTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.LogConsumeEnabled = false
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}))
}

func newControllerTaskContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	return c, recorder
}

func controllerTaskRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UserId:          1001,
		UsingGroup:      "test-group",
		TokenId:         501,
		OriginModelName: "seedance-test",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         77,
			UpstreamModelName: "seedance-upstream",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_reserved",
		},
		PriceData: types.PriceData{
			Quota: 123,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}
}

func createControllerReservation(t *testing.T) *model.Task {
	t.Helper()
	task, err := model.CreateTaskReservation(model.TaskReservationParams{
		TaskID:          "task_reserved",
		TokenId:         501,
		ClientRequestID: "req_controller",
		UserId:          1001,
		Group:           "test-group",
		ChannelId:       77,
		Platform:        constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Action:          constant.TaskActionGenerate,
		OriginModelName: "seedance-test",
	})
	require.NoError(t, err)
	return task
}

func TestHandleTaskSubmitSuccessFinalizesReservationBeforeWritingResponse(t *testing.T) {
	setupControllerTaskReservationTestDB(t)
	reservation := createControllerReservation(t)
	c, recorder := newControllerTaskContext()
	video := dto.NewOpenAIVideo()
	video.ID = reservation.TaskID
	video.TaskID = reservation.TaskID

	taskErr := handleTaskSubmitSuccess(c, controllerTaskRelayInfo(), &relay.TaskSubmitResult{
		UpstreamTaskID:    "upstream_task_123",
		TaskData:          json.RawMessage(`{"id":"upstream_task_123"}`),
		Platform:          constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Quota:             123,
		ReservationTaskID: reservation.ID,
		ClientRequestID:   "req_controller",
		Response: &channel.TaskSubmitResponse{
			StatusCode: http.StatusOK,
			Body:       video,
		},
	})

	require.Nil(t, taskErr)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, c.Writer.Written())
	var responseBody dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &responseBody))
	require.Equal(t, reservation.TaskID, responseBody.ID)
	require.Equal(t, reservation.TaskID, responseBody.TaskID)
	require.Equal(t, "req_controller", responseBody.Metadata["client_request_id"])
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, reservation.ID).Error)
	require.Equal(t, model.TaskStatusNotStart, reloaded.Status)
	require.Equal(t, "0%", reloaded.Progress)
	require.Equal(t, 123, reloaded.Quota)
	require.Equal(t, "upstream_task_123", reloaded.PrivateData.UpstreamTaskID)
	require.Equal(t, 501, reloaded.PrivateData.TokenId)
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestHandleTaskSubmitSuccessFinalizeFailureDoesNotWriteSuccess(t *testing.T) {
	setupControllerTaskReservationTestDB(t)
	reservation := createControllerReservation(t)
	require.NoError(t, model.FinalizeTaskReservation(model.FinalizeTaskReservationParams{ID: reservation.ID}))
	c, recorder := newControllerTaskContext()

	taskErr := handleTaskSubmitSuccess(c, controllerTaskRelayInfo(), &relay.TaskSubmitResult{
		UpstreamTaskID:    "upstream_task_123",
		TaskData:          json.RawMessage(`{"id":"upstream_task_123"}`),
		Platform:          constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Quota:             123,
		ReservationTaskID: reservation.ID,
		Response: &channel.TaskSubmitResponse{
			StatusCode: http.StatusOK,
			Body:       dto.NewOpenAIVideo(),
		},
	})

	require.NotNil(t, taskErr)
	require.Equal(t, "task_reservation_finalize_failed", taskErr.Code)
	require.False(t, c.Writer.Written())
	require.Empty(t, recorder.Body.String())
}

func TestFailTaskReservationIfNeededMarksReservationFailure(t *testing.T) {
	setupControllerTaskReservationTestDB(t)
	reservation := createControllerReservation(t)
	relayInfo := controllerTaskRelayInfo()
	relayInfo.ReservationTaskID = reservation.ID

	failTaskReservationIfNeeded(&dto.TaskError{Message: "upstream failed"}, relayInfo)

	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, reservation.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusFailure), reloaded.Status)
	require.Equal(t, "100%", reloaded.Progress)
	require.Equal(t, "upstream failed", reloaded.FailReason)
	require.Zero(t, reloaded.Quota)
	require.False(t, reloaded.Status.IsUpstreamPollable())
}
