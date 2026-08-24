package doubao

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const seedance25ModelRatio = 5.35

func seedance25FinitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func seedance25ApprovedModelRatio(value float64) bool {
	return seedance25FinitePositive(value) && value == seedance25ModelRatio
}

func seedance25ExpectedOtherRatio(numerator, denominator int64) (float64, bool) {
	switch {
	case numerator == 1 && denominator == 1:
		return 1, true
	case numerator == 64 && denominator == 107:
		return 64.0 / 107.0, true
	case numerator == 117 && denominator == 107:
		return 117.0 / 107.0, true
	case numerator == 70 && denominator == 107:
		return 70.0 / 107.0, true
	default:
		return 0, false
	}
}

func seedance25ExpectedBillingRuleVersion(numerator, denominator int64) (string, bool) {
	switch {
	case numerator == 1 && denominator == 1,
		numerator == 64 && denominator == 107:
		return seedance25BillingRuleVersion, true
	case numerator == 117 && denominator == 107,
		numerator == 70 && denominator == 107:
		return seedance25Native1080pRuleVersion, true
	default:
		return "", false
	}
}

func seedance25BillingSnapshotValid(bc *model.TaskBillingContext) bool {
	if bc == nil || bc.BillingFamily != relaycommon.Seedance25BillingFamily ||
		!seedance25ApprovedModelRatio(bc.ModelRatio) || !seedance25FinitePositive(bc.GroupRatio) {
		return false
	}
	expectedOtherRatio, ok := seedance25ExpectedOtherRatio(bc.OtherRatioNumerator, bc.OtherRatioDenominator)
	if !ok || len(bc.OtherRatios) != 1 {
		return false
	}
	expectedRuleVersion, ok := seedance25ExpectedBillingRuleVersion(bc.OtherRatioNumerator, bc.OtherRatioDenominator)
	if !ok || bc.BillingRuleVersion != expectedRuleVersion {
		return false
	}
	otherRatio, ok := bc.OtherRatios["seedance_intl_billing"]
	return ok && seedance25FinitePositive(otherRatio) && otherRatio == expectedOtherRatio
}

type seedance25UsageEnvelope struct {
	Status string `json:"status"`
	Usage  struct {
		CompletionTokens json.RawMessage `json:"completion_tokens"`
	} `json:"usage"`
}

func positiveSeedance25JSONInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 || rawJSONIsNull(raw) {
		return 0, false
	}
	var value int
	if err := common.Unmarshal(raw, &value); err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

// ParseTaskResultForTask keeps Seedance 2.5's strict billing-usage gate while
// delegating valid responses to the shared parser used by Standard, Fast, and
// Mini.
func (a *TaskAdaptor) ParseTaskResultForTask(task *model.Task, responseBody []byte) (*relaycommon.TaskInfo, error) {
	if !isSeedance25Task(task) {
		return a.ParseTaskResult(responseBody)
	}
	var envelope seedance25UsageEnvelope
	if err := common.Unmarshal(responseBody, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal seedance-2.5 task result failed: %w", err)
	}
	if envelope.Status == "succeeded" {
		if _, valid := positiveSeedance25JSONInt(envelope.Usage.CompletionTokens); !valid {
			return newDoubaoTaskInfo("succeeded", "", "", "", "", 0, false, 0), nil
		}
	}
	result, err := a.ParseTaskResult(responseBody)
	if err != nil {
		return nil, err
	}
	if envelope.Status == "" && result.UpstreamErrorCode != "" {
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
	}
	return result, nil
}

func (a *TaskAdaptor) ValidatePriceData(_ *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if !relaycommon.IsSeedance25OriginAlias(info.OriginModelName) {
		return nil
	}
	priceData := info.PriceData
	expectedOtherRatio, ratioSnapshotOK := seedance25ExpectedOtherRatio(
		priceData.OtherRatioNumerator,
		priceData.OtherRatioDenominator,
	)
	expectedRuleVersion, ruleVersionOK := seedance25ExpectedBillingRuleVersion(
		priceData.OtherRatioNumerator,
		priceData.OtherRatioDenominator,
	)
	otherRatio, hasOtherRatio := priceData.OtherRatios["seedance_intl_billing"]
	if priceData.UsePrice || !seedance25ApprovedModelRatio(priceData.ModelRatio) ||
		!seedance25FinitePositive(priceData.GroupRatioInfo.GroupRatio) ||
		priceData.BillingFamily != relaycommon.Seedance25BillingFamily ||
		!ruleVersionOK || priceData.BillingRuleVersion != expectedRuleVersion ||
		!ratioSnapshotOK || len(priceData.OtherRatios) != 1 || !hasOtherRatio ||
		!seedance25FinitePositive(otherRatio) || otherRatio != expectedOtherRatio {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("seedance-2.5 billing configuration is unavailable"),
			"billing_configuration_error",
			http.StatusServiceUnavailable,
		)
	}
	return nil
}

func isSeedance25Task(task *model.Task) bool {
	if task == nil {
		return false
	}
	if bc := task.PrivateData.BillingContext; bc != nil {
		if bc.BillingFamily == relaycommon.Seedance25BillingFamily {
			return true
		}
		if relaycommon.IsSeedance25OriginAlias(bc.OriginModelName) {
			return true
		}
	}
	return relaycommon.IsSeedance25OriginAlias(task.Properties.OriginModelName)
}

func seedance25ActualQuota(task *model.Task, completionTokens int) (int, bool) {
	if !isSeedance25Task(task) || completionTokens <= 0 {
		return 0, false
	}
	bc := task.PrivateData.BillingContext
	if !seedance25BillingSnapshotValid(bc) {
		return 0, false
	}

	quota := decimal.NewFromInt(int64(completionTokens)).
		Mul(decimal.NewFromFloat(bc.ModelRatio)).
		Mul(decimal.NewFromFloat(bc.GroupRatio)).
		Mul(decimal.NewFromInt(bc.OtherRatioNumerator)).
		Div(decimal.NewFromInt(bc.OtherRatioDenominator)).
		Floor()
	maxInt := int64(^uint(0) >> 1)
	if quota.LessThanOrEqual(decimal.Zero) || quota.GreaterThan(decimal.NewFromInt(maxInt)) {
		return 0, false
	}
	quotaInt64 := quota.IntPart()
	return int(quotaInt64), true
}

func seedance25SafeTaskData(status string, videoURL string, lastFrameURL string, completionTokens int, errorCode string, errorMessage string, retryable *bool) ([]byte, error) {
	payload := map[string]any{"status": status}
	if status == "succeeded" {
		content := map[string]any{"video_url": videoURL}
		if strings.TrimSpace(lastFrameURL) != "" {
			content["last_frame_url"] = strings.TrimSpace(lastFrameURL)
		}
		payload["content"] = content
		payload["usage"] = map[string]any{"completion_tokens": completionTokens}
	}
	if status == "failed" {
		errorPayload := map[string]any{
			"code":    errorCode,
			"message": errorMessage,
		}
		if retryable != nil {
			errorPayload["retryable"] = *retryable
		}
		payload["error"] = errorPayload
	}
	return common.Marshal(payload)
}

// ApplyTaskResultPolicy enforces Seedance 2.5 terminal fail-safe behavior and
// returns the sanitized task data that may later feed the public GET response.
// Other model families retain the existing polling behavior unchanged.
func (a *TaskAdaptor) ApplyTaskResultPolicy(task *model.Task, taskResult *relaycommon.TaskInfo, responseBody []byte) ([]byte, error) {
	if !isSeedance25Task(task) {
		return responseBody, nil
	}
	if taskResult == nil {
		return nil, fmt.Errorf("seedance-2.5 task result is missing")
	}

	switch model.TaskStatus(taskResult.Status) {
	case model.TaskStatusSuccess:
		if !taskResult.CompletionTokensValid || taskResult.CompletionTokens <= 0 {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Progress = "100%"
			taskResult.Url = ""
			taskResult.Reason = "upstream success response did not contain valid completion token usage"
			return seedance25SafeTaskData("failed", "", "", 0, "invalid_upstream_usage", taskResult.Reason, nil)
		}
		videoURL := strings.TrimSpace(taskResult.Url)
		if videoURL == "" {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Progress = "100%"
			taskResult.Url = ""
			taskResult.Reason = "upstream success response did not contain a video output"
			return seedance25SafeTaskData("failed", "", "", 0, "invalid_upstream_output", taskResult.Reason, nil)
		}
		if _, ok := seedance25ActualQuota(task, taskResult.CompletionTokens); !ok {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Progress = "100%"
			taskResult.Url = ""
			taskResult.Reason = "saved billing context is invalid for terminal settlement"
			// A corrupt non-finite snapshot cannot be marshalled back to the task
			// row. Drop only the unusable billing context so the safe terminal
			// failure and one-time reservation refund can still be persisted.
			task.PrivateData.BillingContext = nil
			return seedance25SafeTaskData("failed", "", "", 0, "invalid_billing_context", taskResult.Reason, nil)
		}
		taskResult.Url = videoURL
		return responseBody, nil
	case model.TaskStatusFailure:
		if public, ok := seedance25ClassifyKnownCode(taskResult.UpstreamErrorCode, 0); ok {
			taskResult.Reason = public.Message
			retryable := public.Retryable
			return seedance25SafeTaskData("failed", "", "", 0, public.Code, taskResult.Reason, &retryable)
		}
		taskResult.Reason = "video generation failed"
		return seedance25SafeTaskData("failed", "", "", 0, "video_generation_failed", taskResult.Reason, nil)
	case model.TaskStatusSubmitted, model.TaskStatusQueued:
		return responseBody, nil
	case model.TaskStatusInProgress:
		return responseBody, nil
	default:
		return nil, fmt.Errorf("unsupported seedance-2.5 task status")
	}
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if taskResult == nil || model.TaskStatus(taskResult.Status) != model.TaskStatusSuccess || !taskResult.CompletionTokensValid {
		return 0
	}
	quota, ok := seedance25ActualQuota(task, taskResult.CompletionTokens)
	if !ok {
		return 0
	}
	return quota
}
