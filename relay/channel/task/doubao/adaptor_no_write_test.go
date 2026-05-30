package doubao

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newDoubaoResponseTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	return c, recorder
}

func newDoubaoSubmitHTTPResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"upstream_task_123"}`)),
	}
}

func newDoubaoSubmitRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-0",
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_123",
		},
	}
}

func TestTaskAdaptorDoResponseNoWriteDoesNotWrite(t *testing.T) {
	c, recorder := newDoubaoResponseTestContext(t)
	adaptor := &TaskAdaptor{}

	taskID, taskData, submitResponse, taskErr := adaptor.DoResponseNoWrite(c, newDoubaoSubmitHTTPResponse(), newDoubaoSubmitRelayInfo())

	require.Nil(t, taskErr)
	require.Equal(t, "upstream_task_123", taskID)
	require.JSONEq(t, `{"id":"upstream_task_123"}`, string(taskData))
	require.NotNil(t, submitResponse)
	require.False(t, c.Writer.Written())
	require.Empty(t, recorder.Body.String())
}

func TestTaskAdaptorDoResponseNoWriteReturnsOpenAIVideoResponse(t *testing.T) {
	c, _ := newDoubaoResponseTestContext(t)
	adaptor := &TaskAdaptor{}

	_, _, submitResponse, taskErr := adaptor.DoResponseNoWrite(c, newDoubaoSubmitHTTPResponse(), newDoubaoSubmitRelayInfo())

	require.Nil(t, taskErr)
	require.NotNil(t, submitResponse)
	require.Equal(t, http.StatusOK, submitResponse.StatusCode)

	video, ok := submitResponse.Body.(*dto.OpenAIVideo)
	require.True(t, ok)
	require.Equal(t, "task_public_123", video.ID)
	require.Equal(t, "task_public_123", video.TaskID)
	require.Equal(t, "video", video.Object)
	require.Equal(t, "seedance-2-0", video.Model)
	require.Equal(t, dto.VideoStatusQueued, video.Status)
	require.NotZero(t, video.CreatedAt)

	marshaled, err := common.Marshal(submitResponse.Body)
	require.NoError(t, err)
	require.NotContains(t, string(marshaled), "upstream_task_123")
}

func TestTaskAdaptorDoResponseWritesResponseAsBefore(t *testing.T) {
	c, recorder := newDoubaoResponseTestContext(t)
	adaptor := &TaskAdaptor{}

	taskID, taskData, taskErr := adaptor.DoResponse(c, newDoubaoSubmitHTTPResponse(), newDoubaoSubmitRelayInfo())

	require.Nil(t, taskErr)
	require.Equal(t, "upstream_task_123", taskID)
	require.JSONEq(t, `{"id":"upstream_task_123"}`, string(taskData))
	require.True(t, c.Writer.Written())
	require.Equal(t, http.StatusOK, recorder.Code)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &video))
	require.Equal(t, "task_public_123", video.ID)
	require.Equal(t, "task_public_123", video.TaskID)
	require.Equal(t, "video", video.Object)
	require.Equal(t, "seedance-2-0", video.Model)
	require.Equal(t, dto.VideoStatusQueued, video.Status)
	require.NotZero(t, video.CreatedAt)
}
