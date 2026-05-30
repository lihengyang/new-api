package common

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	basecommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const (
	ClientRequestIDMaxLength = 128

	ClientRequestIDErrorCode    = "invalid_client_request_id"
	ClientRequestIDErrorType    = "invalid_request_error"
	ClientRequestIDErrorMessage = "metadata.client_request_id must be a string with 1-128 characters using letters, numbers, '.', '_', '-', or ':'"
)

func ExtractClientRequestIDFromTaskRequestBody(body []byte) (*string, *types.OpenAIError, error) {
	var req TaskSubmitReq
	if err := basecommon.Unmarshal(body, &req); err != nil {
		return nil, nil, err
	}
	clientRequestID, openAIError := ExtractClientRequestIDFromTaskRequest(req)
	return clientRequestID, openAIError, nil
}

func ExtractClientRequestIDFromTaskRequest(req TaskSubmitReq) (*string, *types.OpenAIError) {
	if req.Metadata == nil {
		return nil, nil
	}
	value, ok := req.Metadata["client_request_id"]
	if !ok {
		return nil, nil
	}
	raw, ok := value.(string)
	if !ok {
		openAIError := invalidClientRequestIDOpenAIError()
		return nil, &openAIError
	}
	clientRequestID := strings.TrimSpace(raw)
	if !isValidClientRequestID(clientRequestID) {
		openAIError := invalidClientRequestIDOpenAIError()
		return nil, &openAIError
	}
	return &clientRequestID, nil
}

func GenerateClientRequestHash(body []byte) (string, error) {
	var canonical any
	if err := basecommon.Unmarshal(body, &canonical); err != nil {
		return "", err
	}
	canonicalBody, err := basecommon.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonicalBody)
	return hex.EncodeToString(sum[:]), nil
}

func invalidClientRequestIDOpenAIError() types.OpenAIError {
	return types.OpenAIError{
		Message: ClientRequestIDErrorMessage,
		Type:    ClientRequestIDErrorType,
		Param:   "metadata.client_request_id",
		Code:    ClientRequestIDErrorCode,
	}
}

func isValidClientRequestID(clientRequestID string) bool {
	if len(clientRequestID) == 0 || len(clientRequestID) > ClientRequestIDMaxLength {
		return false
	}
	for _, r := range clientRequestID {
		if isAllowedClientRequestIDRune(r) {
			continue
		}
		return false
	}
	return true
}

func isAllowedClientRequestIDRune(r rune) bool {
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') ||
		r == '.' ||
		r == '_' ||
		r == '-' ||
		r == ':'
}
