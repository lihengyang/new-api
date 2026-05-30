package relay

import (
	"net/http"
	"net/http/httptest"
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
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeDoubaoVideo)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://example.invalid")
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
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
