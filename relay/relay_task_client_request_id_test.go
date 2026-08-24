package relay

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTaskClientRequestIDContext(t *testing.T, body string) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("test_recorder", recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeDoubaoVideo)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://example.invalid")
	common.SetContextKey(c, constant.ContextKeyChannelId, 77)
	info := &relaycommon.RelayInfo{
		UserId:          1001,
		UsingGroup:      "test-group",
		TokenId:         501,
		TokenKey:        "test-token",
		OriginModelName: "mj_inpaint",
		UserSetting: dto.UserSetting{
			AcceptUnsetRatioModel: true,
			BillingPreference:     "wallet_only",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_123",
		},
	}
	return c, info
}

func parseTaskRequestForClientRequestIDTest(t *testing.T, c *gin.Context, info *relaycommon.RelayInfo) {
	t.Helper()
	taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
	require.Nil(t, taskErr)
}

func requireInvalidClientRequestIDTaskError(t *testing.T, taskErr *dto.TaskError) {
	t.Helper()
	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	require.Equal(t, relaycommon.ClientRequestIDErrorCode, taskErr.Code)
	require.Equal(t, relaycommon.ClientRequestIDErrorMessage, taskErr.Message)
	openAIError, ok := taskErr.Data.(types.OpenAIError)
	require.True(t, ok)
	require.Equal(t, relaycommon.ClientRequestIDErrorType, openAIError.Type)
	require.Equal(t, "metadata.client_request_id", openAIError.Param)
	require.Equal(t, relaycommon.ClientRequestIDErrorCode, openAIError.Code)
}

func TestValidateTaskClientRequestIDMissingKeepsCarrierEmpty(t *testing.T) {
	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance"}`)
	parseTaskRequestForClientRequestIDTest(t, c, info)

	taskErr := validateTaskClientRequestID(c, info)

	require.Nil(t, taskErr)
	require.Empty(t, info.ClientRequestID)
	require.Empty(t, info.ClientRequestHash)
}

func TestValidateTaskClientRequestIDValidStoresCarrierOnly(t *testing.T) {
	setupRelayTaskTestDB(t)
	body := `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":"req_123"}}`
	c, info := newTaskClientRequestIDContext(t, body)
	parseTaskRequestForClientRequestIDTest(t, c, info)

	taskErr := validateTaskClientRequestID(c, info)

	require.Nil(t, taskErr)
	require.Equal(t, "req_123", info.ClientRequestID)
	require.NotEmpty(t, info.ClientRequestHash)
	expectedHash, err := relaycommon.GenerateClientRequestHash([]byte(body))
	require.NoError(t, err)
	require.Equal(t, expectedHash, info.ClientRequestHash)

	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestValidateTaskClientRequestIDInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "non string",
			body: `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":123}}`,
		},
		{
			name: "empty",
			body: `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":""}}`,
		},
		{
			name: "whitespace",
			body: `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":"   "}}`,
		},
		{
			name: "invalid characters",
			body: `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":"req/123"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, info := newTaskClientRequestIDContext(t, tt.body)
			parseTaskRequestForClientRequestIDTest(t, c, info)

			taskErr := validateTaskClientRequestID(c, info)

			requireInvalidClientRequestIDTaskError(t, taskErr)
			require.Empty(t, info.ClientRequestID)
			require.Empty(t, info.ClientRequestHash)
			require.False(t, c.Writer.Written())
		})
	}
}

func TestRelayTaskSubmitInvalidClientRequestIDStopsBeforeUpstreamAndBilling(t *testing.T) {
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"upstream_task_123"}`))
	}))
	defer upstream.Close()

	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":123}}`)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)

	result, taskErr := RelayTaskSubmit(c, info)

	require.Nil(t, result)
	requireInvalidClientRequestIDTaskError(t, taskErr)
	require.EqualValues(t, 0, atomic.LoadInt32(&upstreamCalls))
	require.Nil(t, info.Billing)
	require.False(t, c.Writer.Written())
}

func TestRelayTaskSubmitNoClientRequestIDUsesOldWritePath(t *testing.T) {
	setupRelayTaskTestDB(t)
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"upstream_task_123"}`))
	}))
	defer upstream.Close()

	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance"}`)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)

	result, taskErr := RelayTaskSubmit(c, info)

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.Nil(t, result.Response)
	require.False(t, result.IdempotentReplay)
	require.True(t, c.Writer.Written())
	require.EqualValues(t, 1, atomic.LoadInt32(&upstreamCalls))
	recorderValue, exists := c.Get("test_recorder")
	require.True(t, exists)
	recorder := recorderValue.(*httptest.ResponseRecorder)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &video))
	require.NotContains(t, video.Metadata, "client_request_id")
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, model.DB.Model(&model.Task{}).Where("client_request_id IS NOT NULL").Count(&count).Error)
	require.Zero(t, count)
}

func TestRelayTaskSubmitNoWriteClientRequestIDCreatesReservationBeforeUpstream(t *testing.T) {
	setupRelayTaskTestDB(t)
	var upstreamCalls int32
	var reservationVisibleBeforeUpstream atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		var count int64
		if err := model.DB.Model(&model.Task{}).
			Where("token_id = ? AND client_request_id = ? AND status = ?", 501, "req_123", model.TaskStatusReserved).
			Count(&count).Error; err == nil && count == 1 {
			reservationVisibleBeforeUpstream.Store(true)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"upstream_task_123"}`))
	}))
	defer upstream.Close()

	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":"req_123"}}`)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)

	result, taskErr := RelayTaskSubmitNoWrite(c, info)

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.NotNil(t, result.Response)
	require.False(t, result.IdempotentReplay)
	require.False(t, c.Writer.Written())
	require.EqualValues(t, 1, atomic.LoadInt32(&upstreamCalls))
	require.True(t, reservationVisibleBeforeUpstream.Load())
	require.NotZero(t, result.ReservationTaskID)
	require.Equal(t, result.ReservationTaskID, info.ReservationTaskID)

	var task model.Task
	require.NoError(t, model.DB.First(&task, result.ReservationTaskID).Error)
	require.Equal(t, model.TaskStatusReserved, task.Status)
	require.Equal(t, 501, task.TokenId)
	require.NotNil(t, task.ClientRequestID)
	require.Equal(t, "req_123", *task.ClientRequestID)
	require.NotNil(t, task.ClientRequestHash)
	require.NotEmpty(t, *task.ClientRequestHash)
	require.Zero(t, task.Quota)
	require.Equal(t, info.OriginModelName, task.Properties.OriginModelName)
	require.Equal(t, "mj_inpaint", task.Properties.OriginModelName)
	require.Equal(t, "task_public_123", result.Response.Body.(*dto.OpenAIVideo).ID)
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestRelayTaskSubmitNoWriteDuplicateReplaySkipsUpstreamAndBilling(t *testing.T) {
	setupRelayTaskTestDB(t)
	created, err := model.CreateTaskReservation(model.TaskReservationParams{
		TaskID:          "task_existing",
		TokenId:         501,
		ClientRequestID: "req_duplicate",
		UserId:          1001,
		Group:           "test-group",
		ChannelId:       77,
		Platform:        constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Action:          constant.TaskActionGenerate,
		OriginModelName: "mj_inpaint",
	})
	require.NoError(t, err)
	var beforeToken model.Token
	require.NoError(t, model.DB.First(&beforeToken, 501).Error)
	beforeUserQuota, err := model.GetUserQuota(1001, true)
	require.NoError(t, err)
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":"req_duplicate"}}`)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)

	result, taskErr := RelayTaskSubmitNoWrite(c, info)

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.True(t, result.IdempotentReplay)
	require.Equal(t, created.ID, result.ReplayTask.ID)
	video := BuildOpenAIVideoFromTask(result.ReplayTask)
	require.Equal(t, created.TaskID, video.ID)
	require.Equal(t, created.TaskID, video.TaskID)
	require.EqualValues(t, 0, atomic.LoadInt32(&upstreamCalls))
	require.Nil(t, info.Billing)
	require.False(t, c.Writer.Written())
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("token_id = ? AND client_request_id = ?", 501, "req_duplicate").Count(&count).Error)
	require.EqualValues(t, 1, count)
	var afterToken model.Token
	require.NoError(t, model.DB.First(&afterToken, 501).Error)
	require.Equal(t, beforeToken.RemainQuota, afterToken.RemainQuota)
	require.Equal(t, beforeToken.UsedQuota, afterToken.UsedQuota)
	afterUserQuota, err := model.GetUserQuota(1001, true)
	require.NoError(t, err)
	require.Equal(t, beforeUserQuota, afterUserQuota)
}

func TestRelayTaskSubmitNoWriteDifferentTokenSameClientRequestIDAllowed(t *testing.T) {
	setupRelayTaskTestDB(t)
	_, err := model.CreateTaskReservation(model.TaskReservationParams{
		TaskID:          "task_existing",
		TokenId:         500,
		ClientRequestID: "req_shared",
		UserId:          1001,
		Group:           "test-group",
		ChannelId:       77,
		Platform:        constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Action:          constant.TaskActionGenerate,
		OriginModelName: "mj_inpaint",
	})
	require.NoError(t, err)
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"upstream_task_123"}`))
	}))
	defer upstream.Close()

	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":"req_shared"}}`)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)

	result, taskErr := RelayTaskSubmitNoWrite(c, info)

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.False(t, result.IdempotentReplay)
	require.EqualValues(t, 1, atomic.LoadInt32(&upstreamCalls))
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("client_request_id = ?", "req_shared").Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestRelayTaskSubmitNoWriteInternalRetryReusesReservation(t *testing.T) {
	setupRelayTaskTestDB(t)
	created, err := model.CreateTaskReservation(model.TaskReservationParams{
		TaskID:          "task_retry",
		TokenId:         501,
		ClientRequestID: "req_retry",
		UserId:          1001,
		Group:           "test-group",
		ChannelId:       77,
		Platform:        constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Action:          constant.TaskActionGenerate,
		OriginModelName: "mj_inpaint",
	})
	require.NoError(t, err)
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"upstream_task_456"}`))
	}))
	defer upstream.Close()

	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance","metadata":{"client_request_id":"req_retry"}}`)
	info.ReservationTaskID = created.ID
	info.PublicTaskID = created.TaskID
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)

	result, taskErr := RelayTaskSubmitNoWrite(c, info)

	require.Nil(t, taskErr)
	require.NotNil(t, result)
	require.False(t, result.IdempotentReplay)
	require.Equal(t, created.ID, result.ReservationTaskID)
	require.EqualValues(t, 1, atomic.LoadInt32(&upstreamCalls))
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Where("token_id = ? AND client_request_id = ?", 501, "req_retry").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestLegacyTaskAdaptorSubmitWrapperRemainsUnchanged(t *testing.T) {
	setupRelayTaskTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"legacy":"raw legacy body"}`))
	}))
	defer upstream.Close()

	c, info := newTaskClientRequestIDContext(t, `{"prompt":"hello","model":"seedance"}`)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	result, taskErr := RelayTaskSubmit(c, info)

	require.Nil(t, result)
	require.NotNil(t, taskErr)
	require.Equal(t, "fail_to_fetch_task", taskErr.Code)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	require.Nil(t, taskErr.Retryable)
	require.Contains(t, taskErr.Message, "raw legacy body")
}
