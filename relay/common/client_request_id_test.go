package common

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func requireClientRequestIDError(t *testing.T, err *types.OpenAIError) {
	t.Helper()
	require.NotNil(t, err)
	require.Equal(t, ClientRequestIDErrorCode, err.Code)
	require.Equal(t, ClientRequestIDErrorType, err.Type)
	require.Equal(t, "metadata.client_request_id", err.Param)
	require.Equal(t, ClientRequestIDErrorMessage, err.Message)
}

func TestExtractClientRequestIDMissingMetadata(t *testing.T) {
	clientRequestID, openAIError, err := ExtractClientRequestIDFromTaskRequestBody([]byte(`{"prompt":"hello","model":"seedance"}`))

	require.NoError(t, err)
	require.Nil(t, openAIError)
	require.Nil(t, clientRequestID)
}

func TestExtractClientRequestIDMetadataWithoutClientRequestID(t *testing.T) {
	clientRequestID, openAIError, err := ExtractClientRequestIDFromTaskRequestBody([]byte(`{"metadata":{"customer":"test"}}`))

	require.NoError(t, err)
	require.Nil(t, openAIError)
	require.Nil(t, clientRequestID)
}

func TestExtractClientRequestIDValid(t *testing.T) {
	clientRequestID, openAIError, err := ExtractClientRequestIDFromTaskRequestBody([]byte(`{"metadata":{"client_request_id":"Req_123.test:ok-9"}}`))

	require.NoError(t, err)
	require.Nil(t, openAIError)
	require.NotNil(t, clientRequestID)
	require.Equal(t, "Req_123.test:ok-9", *clientRequestID)
}

func TestExtractClientRequestIDTrimsWhitespace(t *testing.T) {
	clientRequestID, openAIError := ExtractClientRequestIDFromTaskRequest(TaskSubmitReq{
		Metadata: map[string]interface{}{
			"client_request_id": "  req_123  ",
		},
	})

	require.Nil(t, openAIError)
	require.NotNil(t, clientRequestID)
	require.Equal(t, "req_123", *clientRequestID)
}

func TestExtractClientRequestIDEmptyStringInvalid(t *testing.T) {
	clientRequestID, openAIError := ExtractClientRequestIDFromTaskRequest(TaskSubmitReq{
		Metadata: map[string]interface{}{
			"client_request_id": "   ",
		},
	})

	require.Nil(t, clientRequestID)
	requireClientRequestIDError(t, openAIError)
}

func TestExtractClientRequestIDNonStringInvalid(t *testing.T) {
	clientRequestID, openAIError, err := ExtractClientRequestIDFromTaskRequestBody([]byte(`{"metadata":{"client_request_id":123}}`))

	require.NoError(t, err)
	require.Nil(t, clientRequestID)
	requireClientRequestIDError(t, openAIError)
}

func TestExtractClientRequestIDTooLongInvalid(t *testing.T) {
	clientRequestID, openAIError := ExtractClientRequestIDFromTaskRequest(TaskSubmitReq{
		Metadata: map[string]interface{}{
			"client_request_id": strings.Repeat("a", ClientRequestIDMaxLength+1),
		},
	})

	require.Nil(t, clientRequestID)
	requireClientRequestIDError(t, openAIError)
}

func TestExtractClientRequestIDInvalidCharacters(t *testing.T) {
	clientRequestID, openAIError, err := ExtractClientRequestIDFromTaskRequestBody([]byte(`{"metadata":{"client_request_id":"req/123"}}`))

	require.NoError(t, err)
	require.Nil(t, clientRequestID)
	requireClientRequestIDError(t, openAIError)
}

func TestExtractClientRequestIDStringifiedMetadata(t *testing.T) {
	clientRequestID, openAIError, err := ExtractClientRequestIDFromTaskRequestBody([]byte(`{"metadata":"{\"client_request_id\":\"req_123\"}"}`))

	require.NoError(t, err)
	require.Nil(t, openAIError)
	require.NotNil(t, clientRequestID)
	require.Equal(t, "req_123", *clientRequestID)
}

func TestGenerateClientRequestHashStableForCanonicalInput(t *testing.T) {
	first, err := GenerateClientRequestHash([]byte(`{"model":"seedance","metadata":{"client_request_id":"req_123"},"prompt":"hello"}`))
	require.NoError(t, err)
	second, err := GenerateClientRequestHash([]byte(`{
		"prompt": "hello",
		"metadata": {
			"client_request_id": "req_123"
		},
		"model": "seedance"
	}`))
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Len(t, first, 64)
}
