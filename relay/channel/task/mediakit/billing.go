package mediakit

import (
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	BillingFamily      = relaycommon.MediaKitBillingFamily
	BillingRuleVersion = "mediakit-video-enhancement-v1-20260811"
)

var rateCard = map[string]map[string][3]string{
	"standard": {
		"1080p": {"0.4132", "0.8264", "1.6528"},
		"4k":    {"1.6528", "3.3056", "6.6112"},
	},
	"professional": {
		"1080p": {"4.1322", "8.2644", "16.5288"},
		"4k":    {"16.5288", "33.0576", "66.1152"},
	},
}

func fpsTier(fps float64) (int, bool) {
	if !finitePositive(fps) || fps > 120 {
		return 0, false
	}
	if fps <= 30 {
		return 0, true
	}
	if fps <= 60 {
		return 1, true
	}
	return 2, true
}

func rateFor(toolVersion, resolution string, fps float64) (decimal.Decimal, bool) {
	tier, ok := fpsTier(fps)
	if !ok {
		return decimal.Zero, false
	}
	resolutions, ok := rateCard[toolVersion]
	if !ok {
		return decimal.Zero, false
	}
	rates, ok := resolutions[resolution]
	if !ok {
		return decimal.Zero, false
	}
	rate, err := decimal.NewFromString(rates[tier])
	return rate, err == nil
}

func quotaFor(rate decimal.Decimal, seconds int64, groupRatio float64) (int, bool) {
	if !rate.IsPositive() || seconds <= 0 || math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) || groupRatio < 0 {
		return 0, false
	}
	quota := rate.
		Div(decimal.NewFromInt(60)).
		Mul(decimal.NewFromInt(seconds)).
		Mul(decimal.NewFromFloat(groupRatio)).
		Mul(decimal.NewFromInt(int64(common.QuotaPerUnit))).
		Floor()
	maxInt := int64(^uint(0) >> 1)
	if quota.IsNegative() || quota.GreaterThan(decimal.NewFromInt(maxInt)) {
		return 0, false
	}
	return int(quota.IntPart()), true
}

func (a *TaskAdaptor) InitializeTaskPrice(c *gin.Context, info *relaycommon.RelayInfo) (types.PriceData, *dto.TaskError) {
	normalized, err := normalizedFromContext(c)
	if err != nil {
		return types.PriceData{}, service.TaskErrorWrapperLocal(err, "billing_configuration_error", http.StatusServiceUnavailable)
	}
	groupRatioInfo := helper.HandleGroupRatio(c, info)
	prechargeFPS := normalized.FPS
	if !normalized.FPSProvided {
		prechargeFPS = 120
	}
	rate, ok := rateFor(normalized.ToolVersion, normalized.ResolutionTier, prechargeFPS)
	if !ok {
		return types.PriceData{}, service.TaskErrorWrapperLocal(errors.New("MediaKit rate card is unavailable"), "billing_configuration_error", http.StatusServiceUnavailable)
	}
	estimatedSeconds := int64(math.Ceil(normalized.Duration))
	quota, ok := quotaFor(rate, estimatedSeconds, groupRatioInfo.GroupRatio)
	if !ok {
		return types.PriceData{}, service.TaskErrorWrapperLocal(errors.New("MediaKit precharge could not be calculated"), "billing_configuration_error", http.StatusServiceUnavailable)
	}
	metadata := map[string]any{
		"declared_duration":             normalized.Duration,
		"estimated_seconds":             estimatedSeconds,
		"tool_version":                  normalized.ToolVersion,
		"resolution_tier":               normalized.ResolutionTier,
		"fps_provided":                  normalized.FPSProvided,
		"precharge_fps_tier":            fpsTierName(prechargeFPS),
		"precharge_rate_usd_per_minute": rate.String(),
	}
	if normalized.FPSProvided {
		metadata["declared_fps"] = normalized.FPS
	}
	return types.PriceData{
		FreeModel:          quota == 0,
		ModelPrice:         -1,
		ModelRatio:         0,
		UsePrice:           false,
		Quota:              quota,
		GroupRatioInfo:     groupRatioInfo,
		BillingFamily:      BillingFamily,
		BillingRuleVersion: BillingRuleVersion,
		BillingMetadata:    metadata,
	}, nil
}

func fpsTierName(fps float64) string {
	tier, ok := fpsTier(fps)
	if !ok {
		return ""
	}
	return []string{"lte_30", "gt_30_lte_60", "gt_60_lte_120"}[tier]
}

func (a *TaskAdaptor) ApplyTaskResultPolicy(task *model.Task, result *relaycommon.TaskInfo, _ []byte) ([]byte, error) {
	if task == nil || result == nil {
		return nil, errors.New("MediaKit task result is missing")
	}
	if !isMediaKitTask(task) {
		return common.Marshal(map[string]any{"status": result.Status})
	}
	status := model.TaskStatus(result.Status)
	if status == model.TaskStatusSuccess {
		if !finitePositive(result.Duration) || !finitePositive(result.FPS) || result.FPS > 120 {
			return nil, errors.New("MediaKit completion is missing valid duration or fps")
		}
		actualTool := strings.ToLower(strings.TrimSpace(result.ToolVersion))
		actualResolution, ok := normalizeActualResolution(result.Resolution)
		if !ok || (actualTool != "standard" && actualTool != "professional") {
			return nil, errors.New("MediaKit completion is missing valid resolution or tool_version")
		}
		billing := task.PrivateData.BillingContext
		if !validBillingSnapshot(billing) {
			return nil, errors.New("MediaKit billing snapshot is invalid")
		}
		expectedTool, _ := billing.BillingMetadata["tool_version"].(string)
		expectedResolution, _ := billing.BillingMetadata["resolution_tier"].(string)
		if actualTool != expectedTool || actualResolution != expectedResolution {
			return nil, errors.New("MediaKit completion conflicts with the submitted billing tier")
		}
		declared, ok := metadataFloat(billing.BillingMetadata, "declared_duration")
		if !ok {
			return nil, errors.New("MediaKit declared duration snapshot is invalid")
		}
		billing.BillingMetadata["actual_duration"] = result.Duration
		billing.BillingMetadata["duration_difference"] = result.Duration - declared
		billing.BillingMetadata["actual_seconds"] = int64(math.Ceil(result.Duration))
		billing.BillingMetadata["actual_fps"] = result.FPS
		billing.BillingMetadata["actual_resolution"] = actualResolution
		billing.BillingMetadata["actual_tool_version"] = actualTool
		rate, rateOK := rateFor(actualTool, actualResolution, result.FPS)
		actualQuota, quotaOK := quotaFor(rate, int64(math.Ceil(result.Duration)), billing.GroupRatio)
		if !rateOK || !quotaOK {
			return nil, errors.New("MediaKit actual charge could not be calculated")
		}
		billing.BillingMetadata["actual_rate_usd_per_minute"] = rate.String()
		billing.BillingMetadata["actual_quota"] = actualQuota
		return safeTaskData(result, true)
	}
	if status == model.TaskStatusFailure {
		result.Url = ""
		result.UpstreamErrorCode, result.Reason = sanitizeMediaKitErrorDetails(result.UpstreamErrorCode, result.Reason)
		return safeTaskData(result, false)
	}
	return safeTaskData(result, false)
}

func safeTaskData(result *relaycommon.TaskInfo, includeActual bool) ([]byte, error) {
	payload := map[string]any{"status": result.Status}
	if includeActual {
		payload["duration"] = result.Duration
		payload["fps"] = result.FPS
		payload["resolution"] = result.Resolution
		payload["tool_version"] = result.ToolVersion
		if result.ExpiresAt > 0 {
			payload["expires_at"] = result.ExpiresAt
		}
	}
	if model.TaskStatus(result.Status) == model.TaskStatusFailure {
		code, message := sanitizeMediaKitErrorDetails(result.UpstreamErrorCode, result.Reason)
		payload["error"] = map[string]any{"code": code, "message": message}
	}
	return common.Marshal(payload)
}

func normalizeActualResolution(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1080", "1080p":
		return "1080p", true
	case "2160", "2160p", "4k":
		return "4k", true
	default:
		return "", false
	}
}

func validBillingSnapshot(billing *model.TaskBillingContext) bool {
	return billing != nil && billing.BillingFamily == BillingFamily &&
		billing.BillingRuleVersion == BillingRuleVersion &&
		billing.GroupRatio >= 0 && !math.IsNaN(billing.GroupRatio) && !math.IsInf(billing.GroupRatio, 0) &&
		billing.BillingMetadata != nil
}

func metadataFloat(metadata map[string]any, key string) (float64, bool) {
	value, ok := metadata[key]
	if !ok {
		return 0, false
	}
	return numeric(value)
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, result *relaycommon.TaskInfo) int {
	if task == nil || result == nil || model.TaskStatus(result.Status) != model.TaskStatusSuccess {
		return 0
	}
	billing := task.PrivateData.BillingContext
	if !validBillingSnapshot(billing) {
		return 0
	}
	resolution, ok := normalizeActualResolution(result.Resolution)
	if !ok {
		return 0
	}
	rate, ok := rateFor(strings.ToLower(strings.TrimSpace(result.ToolVersion)), resolution, result.FPS)
	if !ok || !finitePositive(result.Duration) {
		return 0
	}
	quota, ok := quotaFor(rate, int64(math.Ceil(result.Duration)), billing.GroupRatio)
	if !ok {
		return 0
	}
	return quota
}

func isMediaKitTask(task *model.Task) bool {
	if task == nil {
		return false
	}
	if billing := task.PrivateData.BillingContext; billing != nil && billing.BillingFamily == BillingFamily {
		return true
	}
	return task.Properties.UpstreamModelName == CanonicalModel
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	return a.ConvertToOpenAIVideoWithResult(task, nil)
}

func (a *TaskAdaptor) ConvertToOpenAIVideoWithResult(task *model.Task, result *relaycommon.TaskInfo) ([]byte, error) {
	if result == nil && task.Status == model.TaskStatusSuccess {
		var stored map[string]any
		if common.Unmarshal(task.Data, &stored) == nil {
			result = &relaycommon.TaskInfo{
				Status:      string(model.TaskStatusSuccess),
				Resolution:  firstString(stored, "resolution"),
				ToolVersion: firstString(stored, "tool_version"),
				ExpiresAt:   parseExpiresAt(stored["expires_at"]),
			}
			result.Duration, _ = numeric(stored["duration"])
			result.FPS, _ = numeric(stored["fps"])
		}
	}
	video := dto.NewOpenAIVideo()
	video.ID, video.TaskID = task.TaskID, task.TaskID
	video.Model = task.Properties.OriginModelName
	switch task.Status {
	case model.TaskStatusReserved, model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued:
		video.Status = dto.VideoStatusQueued
	case model.TaskStatusInProgress:
		video.Status = dto.VideoStatusInProgress
	case model.TaskStatusSuccess:
		video.Status = dto.VideoStatusCompleted
	case model.TaskStatusFailure:
		video.Status = dto.VideoStatusFailed
	default:
		video.Status = dto.VideoStatusUnknown
	}
	video.SetProgressStr(task.Progress)
	video.CreatedAt = task.CreatedAt
	if video.CreatedAt == 0 {
		video.CreatedAt = task.SubmitTime
	}
	if task.FinishTime > 0 {
		video.CompletedAt = task.FinishTime
	}
	if task.ClientRequestID != nil {
		video.SetMetadata("client_request_id", *task.ClientRequestID)
	}
	if result != nil && task.Status == model.TaskStatusSuccess {
		video.SetMetadata("duration", result.Duration)
		video.SetMetadata("fps", result.FPS)
		video.SetMetadata("resolution", result.Resolution)
		video.SetMetadata("tool_version", result.ToolVersion)
		if result.ExpiresAt > 0 {
			video.ExpiresAt = result.ExpiresAt
		}
		if result.ExpiresAt > time.Now().Unix() && strings.TrimSpace(result.Url) != "" {
			video.SetMetadata("video_url", result.Url)
		}
	}
	if task.Status == model.TaskStatusFailure {
		code, message := storedMediaKitError(task)
		if result != nil {
			code, message = sanitizeMediaKitErrorDetails(result.UpstreamErrorCode, result.Reason)
		}
		video.Error = &dto.OpenAIVideoError{Code: code, Message: message}
	}
	return common.Marshal(video)
}

func storedMediaKitError(task *model.Task) (string, string) {
	if task != nil && len(task.Data) > 0 {
		var payload map[string]any
		if common.Unmarshal(task.Data, &payload) == nil {
			if stored := nestedMap(payload, "error"); stored != nil {
				return sanitizeMediaKitErrorDetails(firstString(stored, "code"), firstString(stored, "message"))
			}
		}
	}
	return sanitizeMediaKitErrorDetails("", "")
}
