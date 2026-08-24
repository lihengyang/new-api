package doubao

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	seedance25InvalidParameterMessage = "A request parameter is invalid. Check the request parameters and try again."
	seedance25MissingParameterMessage = "A required request parameter is missing. Check the request parameters and try again."

	seedance25RateLimitMessage          = "The video service rate limit was exceeded. Try again later."
	seedance25OverloadedMessage         = "The video service is temporarily overloaded. Try again later."
	seedance25TemporaryUnavailable      = "The video service is temporarily unavailable. Try again later."
	seedance25UnavailableMessage        = "The video service is unavailable for this request."
	seedance25UpstreamServiceMessage    = "The video service encountered an error. Try again later."
	seedance25UpstreamConnectionMessage = "The video service could not be reached. Try again later."
	seedance25UpstreamTimeoutMessage    = "The video service timed out. Try again later."
	seedance25RequestRejectedMessage    = "The request was rejected. Check the request parameters and try again."
)

type seedance25PublicError struct {
	Code       string
	Message    string
	StatusCode int
	Retryable  bool
}

var seedance25ExactSafeErrors = map[string]string{
	"InvalidParameter.TaskTypeConstraint":                   "The request parameters are incompatible with the task type identified by the model. Update the parameters for that task type and try again.",
	"InvalidParameter.TaskTypeMismatch":                     "The task type identified by the model does not match the specified value. Revise the prompt and input assets, then try again.",
	"SensitiveContentDetected":                              "The input may contain sensitive content. Replace it and try again.",
	"InputTextSensitiveContentDetected":                     "The input text may contain sensitive content. Replace it and try again.",
	"InputImageSensitiveContentDetected":                    "The input image may contain sensitive content. Replace it and try again.",
	"InputVideoSensitiveContentDetected":                    "The input video may contain sensitive content. Replace it and try again.",
	"InputAudioSensitiveContentDetected":                    "The input audio may contain sensitive content. Replace it and try again.",
	"OutputTextSensitiveContentDetected":                    "The generated text may contain sensitive content. Replace the input content and try again.",
	"OutputImageSensitiveContentDetected":                   "The generated image may contain sensitive content. Replace the input content and try again.",
	"OutputVideoSensitiveContentDetected":                   "The generated video may contain sensitive content. Replace the input content and try again.",
	"OutputAudioSensitiveContentDetected":                   "The generated audio may contain sensitive content. Replace the input content and try again.",
	"SensitiveContentDetected.PolicyViolation":              "The input may be related to copyright restrictions. Replace it and try again.",
	"InputTextSensitiveContentDetected.PolicyViolation":     "The input text may be related to copyright restrictions. Replace it and try again.",
	"InputImageSensitiveContentDetected.PolicyViolation":    "The input image may be related to copyright restrictions. Replace it and try again.",
	"InputVideoSensitiveContentDetected.PolicyViolation":    "The input video may be related to copyright restrictions. Replace it and try again.",
	"InputAudioSensitiveContentDetected.PolicyViolation":    "The input audio may be related to copyright restrictions. Replace it and try again.",
	"OutputTextSensitiveContentDetected.PolicyViolation":    "The generated text may be related to copyright restrictions. Replace the input content and try again.",
	"OutputImageSensitiveContentDetected.PolicyViolation":   "The generated image may be related to copyright restrictions. Replace the input content and try again.",
	"OutputVideoSensitiveContentDetected.PolicyViolation":   "The generated video may be related to copyright restrictions. Replace the input content and try again.",
	"OutputAudioSensitiveContentDetected.PolicyViolation":   "The generated audio may be related to copyright restrictions. Please replace the input content and try again.",
	"InputImageSensitiveContentDetected.PrivacyInformation": "The input image may contain privacy-sensitive information. Replace it and try again.",
	"InputVideoSensitiveContentDetected.PrivacyInformation": "The input video may contain privacy-sensitive information. Replace it and try again.",
	"OutputImageSensitiveContentDetected.DeepFake":          "The generated image may involve deepfake content. Replace the input content and try again.",
}

func seedance25TaskError(public seedance25PublicError) *dto.TaskError {
	retryable := public.Retryable
	return &dto.TaskError{
		Code:       public.Code,
		Message:    public.Message,
		Data:       nil,
		Retryable:  &retryable,
		StatusCode: public.StatusCode,
		Error:      errors.New(public.Message),
	}
}

func seedance25CustomerRequestStatus(statusCode int) int {
	if statusCode >= 400 && statusCode <= 499 {
		return statusCode
	}
	return http.StatusBadRequest
}

func seedance25ReasonableUpstream5xx(statusCode int) int {
	if statusCode >= 500 && statusCode <= 599 {
		return statusCode
	}
	return http.StatusBadGateway
}

func seedance25ClassifyKnownCode(code string, statusCode int) (seedance25PublicError, bool) {
	code = strings.TrimSpace(code)
	if message, ok := seedance25ExactSafeErrors[code]; ok {
		return seedance25PublicError{
			Code:       code,
			Message:    message,
			StatusCode: seedance25CustomerRequestStatus(statusCode),
			Retryable:  false,
		}, true
	}

	switch {
	case code == "MissingParameter" || strings.HasPrefix(code, "MissingParameter."):
		return seedance25PublicError{"MissingParameter", seedance25MissingParameterMessage, seedance25CustomerRequestStatus(statusCode), false}, true
	case code == "InvalidParameter" || strings.HasPrefix(code, "InvalidParameter."):
		return seedance25PublicError{"InvalidParameter", seedance25InvalidParameterMessage, seedance25CustomerRequestStatus(statusCode), false}, true
	case code == "RateLimitExceeded" || strings.HasPrefix(code, "RateLimitExceeded.") ||
		code == "ModelAccountRpmRateLimitExceeded" ||
		code == "ModelAccountTpmRateLimitExceeded" ||
		code == "APIAccountRpmRateLimitExceeded" ||
		code == "ModelAccountIpmRateLimitExceeded" ||
		code == "AccountRateLimitExceeded" ||
		code == "InflightBatchsizeExceeded":
		return seedance25PublicError{"rate_limit_exceeded", seedance25RateLimitMessage, http.StatusTooManyRequests, true}, true
	case code == "ServerOverloaded" || code == "RequestBurstTooFast":
		return seedance25PublicError{"service_overloaded", seedance25OverloadedMessage, http.StatusTooManyRequests, true}, true
	case code == "InvalidEndpoint.ClosedEndpoint":
		return seedance25PublicError{"service_unavailable", seedance25TemporaryUnavailable, http.StatusServiceUnavailable, true}, true
	case code == "InternalServiceError":
		return seedance25PublicError{"upstream_service_error", seedance25UpstreamServiceMessage, seedance25ReasonableUpstream5xx(statusCode), true}, true
	case code == "QuotaExceeded" || strings.Contains(code, "SetLimitExceeded"):
		return seedance25PublicError{"service_unavailable", seedance25UnavailableMessage, http.StatusServiceUnavailable, false}, true
	}

	lowerCode := strings.ToLower(code)
	for _, marker := range []string{
		"auth", "accessdenied", "accesskey", "apikey", "unauthorized",
		"forbidden", "permission", "credential", "signature", "secretkey",
		"invalidtoken",
		"account", "subscription", "balance", "overdue", "endpoint",
		"modelnotopen", "modelaccess", "notactivated", "servicenotopen",
		"operationdenied", "configuration", "configerror",
	} {
		if strings.Contains(lowerCode, marker) {
			return seedance25PublicError{"service_unavailable", seedance25UnavailableMessage, http.StatusServiceUnavailable, false}, true
		}
	}
	return seedance25PublicError{}, false
}

type seedance25SubmitErrorEnvelope struct {
	Code  string `json:"code"`
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
	ResponseMetadata struct {
		Error struct {
			Code string `json:"Code"`
		} `json:"Error"`
	} `json:"ResponseMetadata"`
}

func seedance25UpstreamErrorCode(responseBody []byte) string {
	var envelope seedance25SubmitErrorEnvelope
	if common.Unmarshal(responseBody, &envelope) != nil {
		return ""
	}
	for _, code := range []string{envelope.Error.Code, envelope.Code, envelope.ResponseMetadata.Error.Code} {
		if code = strings.TrimSpace(code); code != "" {
			return code
		}
	}
	return ""
}

func (a *TaskAdaptor) ClassifyTaskSubmitTransportError(err error) *dto.TaskError {
	public := seedance25PublicError{
		Code:       "upstream_connection_error",
		Message:    seedance25UpstreamConnectionMessage,
		StatusCode: http.StatusBadGateway,
		Retryable:  true,
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		public.Code = "upstream_timeout"
		public.Message = seedance25UpstreamTimeoutMessage
		public.StatusCode = http.StatusGatewayTimeout
	}
	return seedance25TaskError(public)
}

func (a *TaskAdaptor) ClassifyTaskSubmitHTTPError(statusCode int, responseBody []byte) *dto.TaskError {
	if public, ok := seedance25ClassifyKnownCode(seedance25UpstreamErrorCode(responseBody), statusCode); ok {
		return seedance25TaskError(public)
	}

	var public seedance25PublicError
	switch {
	case statusCode == http.StatusBadRequest || statusCode == http.StatusUnprocessableEntity:
		public = seedance25PublicError{"upstream_request_rejected", seedance25RequestRejectedMessage, statusCode, false}
	case statusCode == http.StatusTooManyRequests:
		public = seedance25PublicError{"rate_limit_exceeded", seedance25RateLimitMessage, http.StatusTooManyRequests, true}
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden || statusCode == http.StatusNotFound:
		public = seedance25PublicError{"service_unavailable", seedance25UnavailableMessage, http.StatusServiceUnavailable, false}
	case statusCode >= 500 && statusCode <= 599:
		public = seedance25PublicError{"upstream_service_error", seedance25UpstreamServiceMessage, statusCode, true}
	default:
		public = seedance25PublicError{"upstream_service_error", seedance25UpstreamServiceMessage, http.StatusBadGateway, true}
	}
	return seedance25TaskError(public)
}

func seedance25SubmitProtocolError() *dto.TaskError {
	return seedance25TaskError(seedance25PublicError{
		Code:       "upstream_service_error",
		Message:    seedance25UpstreamServiceMessage,
		StatusCode: http.StatusBadGateway,
		Retryable:  true,
	})
}
