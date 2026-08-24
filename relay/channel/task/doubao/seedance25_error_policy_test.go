package doubao

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestSeedance25ExactSafeErrorCatalogIsSharedBySubmitAndTerminal(t *testing.T) {
	require.Len(t, seedance25ExactSafeErrors, 23)
	adaptor := &TaskAdaptor{}
	for code, message := range seedance25ExactSafeErrors {
		code, message := code, message
		t.Run(code, func(t *testing.T) {
			body := []byte(`{"error":{"code":"` + code + `","message":"raw-message Request ID: request-marker endpoint-marker model-marker account-marker channel-marker routing-marker"}}`)
			submitErr := adaptor.ClassifyTaskSubmitHTTPError(http.StatusBadRequest, body)
			require.Equal(t, http.StatusBadRequest, submitErr.StatusCode)
			require.Equal(t, code, submitErr.Code)
			require.Equal(t, message, submitErr.Message)
			require.NotNil(t, submitErr.Retryable)
			require.False(t, *submitErr.Retryable)
			submitJSON, err := common.Marshal(submitErr)
			require.NoError(t, err)
			require.Contains(t, string(submitJSON), `"retryable":false`)
			for _, forbidden := range []string{"raw-message", "Request ID", "request-marker", "endpoint-marker", "model-marker", "account-marker", "channel-marker", "routing-marker"} {
				require.NotContains(t, string(submitJSON), forbidden)
			}

			result := &relaycommon.TaskInfo{
				Status:            string(model.TaskStatusFailure),
				UpstreamErrorCode: code,
				Reason:            "raw terminal diagnostic request-marker model-marker",
			}
			persisted, err := adaptor.ApplyTaskResultPolicy(seedance25TerminalTask(false), result, body)
			require.NoError(t, err)
			var safe responseTask
			require.NoError(t, common.Unmarshal(persisted, &safe))
			require.Equal(t, code, safe.Error.Code)
			require.Equal(t, message, safe.Error.Message)
			require.NotNil(t, safe.Error.Retryable)
			require.False(t, *safe.Error.Retryable)
			require.NotContains(t, string(persisted), "raw terminal diagnostic")
			require.NotContains(t, string(persisted), "request-marker")
			require.NotContains(t, string(persisted), "model-marker")
		})
	}
}

func TestSeedance25DynamicParameterCodesNormalizeWithoutDetails(t *testing.T) {
	tests := []struct {
		upstreamCode string
		publicCode   string
		message      string
	}{
		{"MissingParameter", "MissingParameter", seedance25MissingParameterMessage},
		{"MissingParameter.dynamic-secret-field", "MissingParameter", seedance25MissingParameterMessage},
		{"InvalidParameter", "InvalidParameter", seedance25InvalidParameterMessage},
		{"InvalidParameter.dynamic-secret-field", "InvalidParameter", seedance25InvalidParameterMessage},
	}
	for _, tt := range tests {
		t.Run(tt.upstreamCode, func(t *testing.T) {
			body := []byte(`{"error":{"code":"` + tt.upstreamCode + `","message":"Issues: secret details. Request ID: request-marker"}}`)
			taskErr := (&TaskAdaptor{}).ClassifyTaskSubmitHTTPError(http.StatusUnprocessableEntity, body)
			require.Equal(t, http.StatusUnprocessableEntity, taskErr.StatusCode)
			require.Equal(t, tt.publicCode, taskErr.Code)
			require.Equal(t, tt.message, taskErr.Message)
			require.NotNil(t, taskErr.Retryable)
			require.False(t, *taskErr.Retryable)
			publicJSON, err := common.Marshal(taskErr)
			require.NoError(t, err)
			require.NotContains(t, string(publicJSON), "dynamic-secret-field")
			require.NotContains(t, string(publicJSON), "Issues")
			require.NotContains(t, string(publicJSON), "request-marker")

			result := &relaycommon.TaskInfo{Status: string(model.TaskStatusFailure), UpstreamErrorCode: tt.upstreamCode}
			persisted, err := (&TaskAdaptor{}).ApplyTaskResultPolicy(seedance25TerminalTask(false), result, body)
			require.NoError(t, err)
			require.Contains(t, string(persisted), `"code":"`+tt.publicCode+`"`)
			require.NotContains(t, string(persisted), "dynamic-secret-field")
		})
	}
}

func TestSeedance25SubmitErrorMatrix(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		expectedCode string
		expectedMsg  string
		expectedHTTP int
		retryable    bool
	}{
		{"safe typed 400", 400, `{"error":{"code":"InvalidParameter.TaskTypeMismatch","message":"raw"}}`, "InvalidParameter.TaskTypeMismatch", seedance25ExactSafeErrors["InvalidParameter.TaskTypeMismatch"], 400, false},
		{"safe typed preserves relevant 403", 403, `{"error":{"code":"InputImageSensitiveContentDetected","message":"raw"}}`, "InputImageSensitiveContentDetected", seedance25ExactSafeErrors["InputImageSensitiveContentDetected"], 403, false},
		{"unknown 400", 400, `{"error":{"code":"Unknown.Customer.Rejection","message":"raw"}}`, "upstream_request_rejected", seedance25RequestRejectedMessage, 400, false},
		{"unlisted internal 400 remains rejected", 400, `{"error":{"code":"InternalError","message":"raw"}}`, "upstream_request_rejected", seedance25RequestRejectedMessage, 400, false},
		{"unknown 422", 422, `{"error":{"code":"Unknown.Customer.Rejection","message":"raw"}}`, "upstream_request_rejected", seedance25RequestRejectedMessage, 422, false},
		{"generic 429", 429, `{"error":{"code":"Unknown.Rate","message":"raw"}}`, "rate_limit_exceeded", seedance25RateLimitMessage, 429, true},
		{"rate family", 400, `{"error":{"code":"RateLimitExceeded.EndpointRPMExceeded","message":"raw"}}`, "rate_limit_exceeded", seedance25RateLimitMessage, 429, true},
		{"overload", 429, `{"error":{"code":"ServerOverloaded","message":"raw"}}`, "service_overloaded", seedance25OverloadedMessage, 429, true},
		{"burst", 429, `{"error":{"code":"RequestBurstTooFast","message":"raw"}}`, "service_overloaded", seedance25OverloadedMessage, 429, true},
		{"closed endpoint", 400, `{"error":{"code":"InvalidEndpoint.ClosedEndpoint","message":"raw"}}`, "service_unavailable", seedance25TemporaryUnavailable, 503, true},
		{"upstream 500", 500, `{"error":{"code":"Unknown.Internal","message":"raw"}}`, "upstream_service_error", seedance25UpstreamServiceMessage, 500, true},
		{"internal service code", 400, `{"error":{"code":"InternalServiceError","message":"raw"}}`, "upstream_service_error", seedance25UpstreamServiceMessage, 502, true},
		{"authentication", 401, `{"error":{"code":"AuthenticationError","message":"credential-marker"}}`, "service_unavailable", seedance25UnavailableMessage, 503, false},
		{"model enablement", 400, `{"error":{"code":"ModelNotOpen","message":"model-marker"}}`, "service_unavailable", seedance25UnavailableMessage, 503, false},
		{"quota remains nonretryable", 429, `{"error":{"code":"QuotaExceeded","message":"account-marker"}}`, "service_unavailable", seedance25UnavailableMessage, 503, false},
		{"set limit remains nonretryable", 429, `{"error":{"code":"AccountSetLimitExceeded","message":"account-marker"}}`, "service_unavailable", seedance25UnavailableMessage, 503, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskErr := (&TaskAdaptor{}).ClassifyTaskSubmitHTTPError(tt.status, []byte(tt.body))
			require.Equal(t, tt.expectedHTTP, taskErr.StatusCode)
			require.Equal(t, tt.expectedCode, taskErr.Code)
			require.Equal(t, tt.expectedMsg, taskErr.Message)
			require.NotNil(t, taskErr.Retryable)
			require.Equal(t, tt.retryable, *taskErr.Retryable)
			publicJSON, err := common.Marshal(taskErr)
			require.NoError(t, err)
			for _, forbidden := range []string{"raw", "credential-marker", "model-marker", "account-marker", "Request ID", "endpoint", "channel", "routing"} {
				require.NotContains(t, string(publicJSON), forbidden)
			}
		})
	}
}

func TestSeedance25SubmitTransportMatrix(t *testing.T) {
	connectionErr := (&TaskAdaptor{}).ClassifyTaskSubmitTransportError(errors.New("dial tcp endpoint-marker: connection reset by peer"))
	require.Equal(t, http.StatusBadGateway, connectionErr.StatusCode)
	require.Equal(t, "upstream_connection_error", connectionErr.Code)
	require.Equal(t, seedance25UpstreamConnectionMessage, connectionErr.Message)
	require.NotNil(t, connectionErr.Retryable)
	require.True(t, *connectionErr.Retryable)

	timeoutErr := (&TaskAdaptor{}).ClassifyTaskSubmitTransportError(context.DeadlineExceeded)
	require.Equal(t, http.StatusGatewayTimeout, timeoutErr.StatusCode)
	require.Equal(t, "upstream_timeout", timeoutErr.Code)
	require.Equal(t, seedance25UpstreamTimeoutMessage, timeoutErr.Message)
	require.NotNil(t, timeoutErr.Retryable)
	require.True(t, *timeoutErr.Retryable)

	for _, taskErr := range []*dto.TaskError{connectionErr, timeoutErr} {
		publicJSON, err := common.Marshal(taskErr)
		require.NoError(t, err)
		require.NotContains(t, string(publicJSON), "endpoint-marker")
		require.NotContains(t, string(publicJSON), "connection reset")
	}
}

func TestSeedance25ResponseMetadataErrorCodeIsClassifiedWithoutRawLeakage(t *testing.T) {
	body := []byte(`{"ResponseMetadata":{"RequestId":"request-marker","Error":{"Code":"MissingParameter.SecretField","Message":"raw-message"}}}`)
	taskErr := (&TaskAdaptor{}).ClassifyTaskSubmitHTTPError(http.StatusBadRequest, body)
	require.Equal(t, "MissingParameter", taskErr.Code)
	require.Equal(t, seedance25MissingParameterMessage, taskErr.Message)
	publicJSON, err := common.Marshal(taskErr)
	require.NoError(t, err)
	require.NotContains(t, string(publicJSON), "SecretField")
	require.NotContains(t, string(publicJSON), "request-marker")
	require.NotContains(t, string(publicJSON), "raw-message")
}

func TestSeedance25TemporaryAndServiceCatalogTerminalNormalization(t *testing.T) {
	tests := []struct {
		upstreamCode string
		publicCode   string
		message      string
		retryable    bool
	}{
		{"RateLimitExceeded.EndpointRPMExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"RateLimitExceeded.EndpointTPMExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"ModelAccountRpmRateLimitExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"ModelAccountTpmRateLimitExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"APIAccountRpmRateLimitExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"ModelAccountIpmRateLimitExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"AccountRateLimitExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"InflightBatchsizeExceeded", "rate_limit_exceeded", seedance25RateLimitMessage, true},
		{"ServerOverloaded", "service_overloaded", seedance25OverloadedMessage, true},
		{"RequestBurstTooFast", "service_overloaded", seedance25OverloadedMessage, true},
		{"InvalidEndpoint.ClosedEndpoint", "service_unavailable", seedance25TemporaryUnavailable, true},
		{"InternalServiceError", "upstream_service_error", seedance25UpstreamServiceMessage, true},
		{"QuotaExceeded", "service_unavailable", seedance25UnavailableMessage, false},
		{"AccountSetLimitExceeded", "service_unavailable", seedance25UnavailableMessage, false},
	}
	for _, tt := range tests {
		t.Run(tt.upstreamCode, func(t *testing.T) {
			result := &relaycommon.TaskInfo{
				Status:            string(model.TaskStatusFailure),
				UpstreamErrorCode: tt.upstreamCode,
				Reason:            "raw-message Request ID: request-marker endpoint-marker model-marker account-marker",
			}
			persisted, err := (&TaskAdaptor{}).ApplyTaskResultPolicy(seedance25TerminalTask(false), result, []byte(result.Reason))
			require.NoError(t, err)
			var safe responseTask
			require.NoError(t, common.Unmarshal(persisted, &safe))
			require.Equal(t, tt.publicCode, safe.Error.Code)
			require.Equal(t, tt.message, safe.Error.Message)
			require.NotNil(t, safe.Error.Retryable)
			require.Equal(t, tt.retryable, *safe.Error.Retryable)
			for _, forbidden := range []string{"raw-message", "Request ID", "request-marker", "endpoint-marker", "model-marker", "account-marker"} {
				require.NotContains(t, string(persisted), forbidden)
			}
		})
	}
}

func TestSeedance25UnknownTerminalErrorRemainsGeneric(t *testing.T) {
	result := &relaycommon.TaskInfo{
		Status:            string(model.TaskStatusFailure),
		UpstreamErrorCode: "Unknown.ProviderSpecificCode",
		Reason:            "raw-message Request ID: request-marker endpoint-marker model-marker account-marker",
	}
	persisted, err := (&TaskAdaptor{}).ApplyTaskResultPolicy(seedance25TerminalTask(false), result, []byte(result.Reason))
	require.NoError(t, err)
	var safe responseTask
	require.NoError(t, common.Unmarshal(persisted, &safe))
	require.Equal(t, "video_generation_failed", safe.Error.Code)
	require.Equal(t, "video generation failed", safe.Error.Message)
	require.Nil(t, safe.Error.Retryable)
	require.NotContains(t, string(persisted), "Unknown.ProviderSpecificCode")
	require.NotContains(t, string(persisted), "raw-message")
}

func TestSeedance25MalformedSuccessfulSubmitResponseUsesSafeProtocolError(t *testing.T) {
	info := &relaycommon.RelayInfo{OriginModelName: seedance25TenantAliasForDoubaoTest}
	for _, body := range []string{"not-json raw-message request-marker", `{"unexpected":"endpoint-marker"}`} {
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
		_, _, _, taskErr := (&TaskAdaptor{}).DoResponseNoWrite(nil, resp, info)
		require.NotNil(t, taskErr)
		require.Equal(t, http.StatusBadGateway, taskErr.StatusCode)
		require.Equal(t, "upstream_service_error", taskErr.Code)
		require.Equal(t, seedance25UpstreamServiceMessage, taskErr.Message)
		require.NotNil(t, taskErr.Retryable)
		require.True(t, *taskErr.Retryable)
		publicJSON, err := common.Marshal(taskErr)
		require.NoError(t, err)
		require.NotContains(t, string(publicJSON), "raw-message")
		require.NotContains(t, string(publicJSON), "request-marker")
		require.NotContains(t, string(publicJSON), "endpoint-marker")
	}
}
