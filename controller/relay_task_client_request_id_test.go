package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
