package relay

import (
	"bytes"
	"context"
	"errors"
	"expvar"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	seedAudioAliasPrefix             = "lsf-seed-audio-1.0"
	seedAudioUpstreamModel           = "seed-audio-1.0"
	seedAudioDefaultBaseURL          = "https://voice.ap-southeast-1.bytepluses.com"
	seedAudioCreatePath              = "/api/v3/tts/create"
	seedAudioDefaultFormat           = "mp3"
	seedAudioMaxTextRunes            = 3000
	seedAudioMaxBodyBytes            = 128 * 1024
	seedAudioDiagnosticsPreviewBytes = 128 * 1024
	seedAudioBaseQuotaPerSecond      = 1250.0
	seedAudioMaxGeneratedSeconds     = 120.0
	seedAudioIdempotencyTTL          = 2 * time.Hour
	seedAudioIdempotencyTombstoneTTL = 24 * time.Hour
	seedAudioURLTTL                  = 2 * time.Hour
)

var seedAudioMetric = struct {
	requests                *expvar.Int
	validationFailed        *expvar.Int
	prechargeRejected       *expvar.Int
	upstreamTimeout         *expvar.Int
	upstreamError           *expvar.Int
	missingOriginalDuration *expvar.Int
	missingURL              *expvar.Int
	invalidReferenceURL     *expvar.Int
	idempotencyReplay       *expvar.Int
	idempotencyConflict     *expvar.Int
	idempotencyUnavailable  *expvar.Int
	actualQuota             *expvar.Int
}{
	requests:                expvar.NewInt("seed_audio_requests_total"),
	validationFailed:        expvar.NewInt("seed_audio_validation_failed_total"),
	prechargeRejected:       expvar.NewInt("seed_audio_precharge_rejected_total"),
	upstreamTimeout:         expvar.NewInt("seed_audio_upstream_timeout_total"),
	upstreamError:           expvar.NewInt("seed_audio_upstream_error_total"),
	missingOriginalDuration: expvar.NewInt("seed_audio_missing_original_duration_total"),
	missingURL:              expvar.NewInt("seed_audio_missing_url_total"),
	invalidReferenceURL:     expvar.NewInt("seed_audio_invalid_reference_url_total"),
	idempotencyReplay:       expvar.NewInt("seed_audio_idempotency_replay_total"),
	idempotencyConflict:     expvar.NewInt("seed_audio_idempotency_conflict_total"),
	idempotencyUnavailable:  expvar.NewInt("seed_audio_idempotency_unavailable_total"),
	actualQuota:             expvar.NewInt("seed_audio_actual_quota_total"),
}

type seedAudioNormalizedRequest struct {
	TextPrompt      string
	Format          string
	SampleRate      *int
	SpeechRate      *float64
	LoudnessRate    *float64
	PitchRate       *float64
	References      []seedAudioReference
	ReferenceMode   string
	ClientRequestID string
}

type seedAudioReference struct {
	Type string
	URL  string
	Host string
	HMAC string
}

type seedAudioUpstreamRequest struct {
	Model       string                       `json:"model"`
	TextPrompt  string                       `json:"text_prompt"`
	AudioConfig seedAudioUpstreamAudioConfig `json:"audio_config"`
	References  []seedAudioUpstreamReference `json:"references,omitempty"`
}

type seedAudioUpstreamAudioConfig struct {
	Format       string   `json:"format"`
	SampleRate   *int     `json:"sample_rate,omitempty"`
	SpeechRate   *float64 `json:"speech_rate,omitempty"`
	LoudnessRate *float64 `json:"loudness_rate,omitempty"`
	PitchRate    *float64 `json:"pitch_rate,omitempty"`
}

type seedAudioUpstreamReference struct {
	AudioURL string `json:"audio_url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type seedAudioUpstreamResult struct {
	Audio            string
	URL              string
	Duration         float64
	OriginalDuration float64
	XTTLogID         string
}

type seedAudioIdempotencyContext struct {
	recordID    int64
	requestHMAC string
	clientID    string
}

// IsSeedAudioAlias returns true for the LSF customer-facing Seed Audio aliases.
func IsSeedAudioAlias(modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	return strings.HasPrefix(modelName, seedAudioAliasPrefix) && modelName != seedAudioUpstreamModel
}

func SeedAudioHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	seedAudioMetric.requests.Add(1)
	info.InitChannelMeta(c)

	audioReq, ok := info.Request.(*dto.AudioRequest)
	if !ok {
		return seedAudioError(http.StatusBadRequest, "invalid_request_error", "invalid_request", "invalid Seed Audio request", "")
	}

	bodyBytes, newAPIError := seedAudioRequestBody(c)
	if newAPIError != nil {
		return newAPIError
	}
	var bodyMap map[string]interface{}
	if err := common.Unmarshal(bodyBytes, &bodyMap); err != nil {
		return seedAudioValidationError("request body must be valid JSON", "")
	}

	normalized, newAPIError := buildSeedAudioNormalizedRequest(audioReq, bodyMap)
	if newAPIError != nil {
		return newAPIError
	}
	if err := seedAudioCheckSensitiveText(c, normalized.TextPrompt); err != nil {
		return err
	}

	idempotency, cached, newAPIError := seedAudioPrepareIdempotency(c, info, normalized)
	if newAPIError != nil {
		return newAPIError
	}
	if cached != nil {
		seedAudioMetric.idempotencyReplay.Add(1)
		c.JSON(http.StatusOK, seedAudioResponseFromRecord(audioReq.Model, *cached, normalized.ClientRequestID))
		return nil
	}

	groupRatioInfo := seedAudioGroupRatio(c, info)
	info.PriceData.GroupRatioInfo = groupRatioInfo
	info.ForcePreConsume = true
	prechargeQuota := seedAudioPrechargeQuota(groupRatioInfo.GroupRatio)

	var settled bool
	var upstreamDispatched bool
	var upstreamDiagnostics *dto.SeedAudioUpstreamDiagnostics
	defer func() {
		if newAPIError == nil {
			return
		}
		if info.Billing != nil && !settled {
			info.Billing.Refund(c)
		}
		if idempotency == nil {
			return
		}
		if upstreamDispatched {
			_ = idempotency.storeFailure(c, newAPIError, upstreamDiagnostics)
			return
		}
		_ = idempotency.abandon(c)
	}()

	if newAPIError = service.PreConsumeBilling(c, prechargeQuota, info); newAPIError != nil {
		if seedAudioIsInsufficientBalance(newAPIError) {
			seedAudioMetric.prechargeRejected.Add(1)
			return seedAudioInsufficientBalanceError(prechargeQuota)
		}
		return seedAudioError(http.StatusInternalServerError, "internal_error", "internal_error", "failed to reserve Seed Audio quota", "")
	}

	upstreamDispatched = true
	result, diagnostics, dispatchErr := seedAudioDispatchUpstream(c, info, normalized)
	upstreamDiagnostics = diagnostics
	if dispatchErr != nil {
		newAPIError = dispatchErr
		return newAPIError
	}

	actualQuota := seedAudioActualQuota(result.OriginalDuration, groupRatioInfo.GroupRatio)
	if actualQuota > prechargeQuota {
		return seedAudioError(http.StatusInternalServerError, "internal_error", "internal_error", "Seed Audio settlement exceeded reserved quota", "")
	}
	if err := service.SettleBilling(c, info, actualQuota); err != nil {
		return seedAudioError(http.StatusInternalServerError, "internal_error", "internal_error", "failed to settle Seed Audio quota", "")
	}
	settled = true
	seedAudioMetric.actualQuota.Add(int64(actualQuota))
	model.UpdateUserUsedQuotaAndRequestCount(info.UserId, actualQuota)
	seedAudioUpdateChannelUsedQuota(c, info.ChannelId, actualQuota)
	seedAudioRecordConsumeLog(c, info, normalized, result, actualQuota, prechargeQuota)

	now := time.Now()
	response := dto.SeedAudioResponse{
		ID:               "aud_" + common.GetUUID(),
		Object:           "audio.speech",
		Created:          now.Unix(),
		Model:            audioReq.Model,
		URL:              result.URL,
		Duration:         result.Duration,
		OriginalDuration: result.OriginalDuration,
		Usage: dto.SeedAudioUsage{
			Type:             "seconds",
			Duration:         result.Duration,
			OriginalDuration: result.OriginalDuration,
		},
	}
	if result.URL != "" {
		response.URLExpiresAt = now.Add(seedAudioURLTTL).Unix()
	}
	if normalized.ClientRequestID != "" {
		response.Metadata = map[string]interface{}{"client_request_id": normalized.ClientRequestID}
	}
	if idempotency != nil {
		record := dto.SeedAudioIdempotencyRecord{
			RequestHMAC:      idempotency.requestHMAC,
			Status:           "completed",
			ResponseID:       response.ID,
			XTTLogID:         result.XTTLogID,
			TemporaryURL:     result.URL,
			URLExpiresAt:     response.URLExpiresAt,
			Duration:         result.Duration,
			OriginalDuration: result.OriginalDuration,
			ActualQuota:      actualQuota,
			CreatedAt:        response.Created,
		}
		if err := idempotency.storeCompleted(c, record); err != nil {
			return seedAudioError(http.StatusServiceUnavailable, "upstream_error", "seed_audio_idempotency_unavailable", "Seed Audio idempotency store is unavailable", "metadata.client_request_id")
		}
	}

	logger.LogInfo(c, fmt.Sprintf("Seed Audio request settled: duration=%.3f original_duration=%.3f actual_quota=%d x_tt_logid=%s",
		result.Duration, result.OriginalDuration, actualQuota, seedAudioMaskLogID(result.XTTLogID)))
	c.JSON(http.StatusOK, response)
	return nil
}

func seedAudioRequestBody(c *gin.Context) ([]byte, *types.NewAPIError) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			return nil, seedAudioError(http.StatusRequestEntityTooLarge, "invalid_request_error", "payload_too_large", "Seed Audio request body exceeds 128 KB", "")
		}
		return nil, seedAudioValidationError("failed to read request body", "")
	}
	if storage.Size() > seedAudioMaxBodyBytes {
		return nil, seedAudioError(http.StatusRequestEntityTooLarge, "invalid_request_error", "payload_too_large", "Seed Audio request body exceeds 128 KB", "")
	}
	bodyBytes, err := storage.Bytes()
	if err != nil {
		return nil, seedAudioValidationError("failed to read request body", "")
	}
	return bodyBytes, nil
}

func buildSeedAudioNormalizedRequest(audioReq *dto.AudioRequest, bodyMap map[string]interface{}) (*seedAudioNormalizedRequest, *types.NewAPIError) {
	if err := scanSeedAudioForbiddenFields(bodyMap); err != nil {
		return nil, err
	}
	if strings.TrimSpace(audioReq.Voice) != "" {
		return nil, seedAudioValidationError("voice is not supported for Seed Audio P0", "voice")
	}
	textPrompt := seedAudioBuildTextPrompt(audioReq.Instructions, audioReq.Input)
	if textPrompt == "" {
		return nil, seedAudioValidationError("input or instructions is required", "input")
	}
	if utf8.RuneCountInString(textPrompt) > seedAudioMaxTextRunes {
		return nil, seedAudioInputTooLongError()
	}

	format := strings.ToLower(strings.TrimSpace(audioReq.ResponseFormat))
	if format == "" {
		format = seedAudioDefaultFormat
	}
	if format != seedAudioDefaultFormat {
		return nil, seedAudioValidationError("Seed Audio P0 supports response_format mp3 only", "response_format")
	}

	metadata := dto.SeedAudioMetadata{}
	if len(audioReq.Metadata) > 0 && string(audioReq.Metadata) != "null" {
		if err := common.Unmarshal(audioReq.Metadata, &metadata); err != nil {
			return nil, seedAudioValidationError("metadata must be valid JSON", "metadata")
		}
	}

	refs, mode, err := seedAudioNormalizeReferences(metadata.References)
	if err != nil {
		return nil, err
	}
	return &seedAudioNormalizedRequest{
		TextPrompt:      textPrompt,
		Format:          format,
		SampleRate:      metadata.SampleRate,
		SpeechRate:      metadata.SpeechRate,
		LoudnessRate:    metadata.LoudnessRate,
		PitchRate:       metadata.PitchRate,
		References:      refs,
		ReferenceMode:   mode,
		ClientRequestID: strings.TrimSpace(metadata.ClientRequestID),
	}, nil
}

func seedAudioBuildTextPrompt(instructions string, input string) string {
	instructions = strings.TrimSpace(instructions)
	input = strings.TrimSpace(input)
	if instructions != "" && input != "" {
		return instructions + "\n" + input
	}
	if instructions != "" {
		return instructions
	}
	return input
}

func scanSeedAudioForbiddenFields(value interface{}) *types.NewAPIError {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			normalized := seedAudioNormalizeFieldKey(key)
			switch normalized {
			case "projectname", "byteplusprojectname", "seedprojectname":
				return seedAudioValidationError("ProjectName is an admin-only Seed Audio routing field", key)
			case "audiodata", "imagedata", "maxdurationhint", "base64", "speaker", "speakerid", "voiceid", "voiceclone", "voicecloning":
				return seedAudioValidationError(fmt.Sprintf("%s is not supported for Seed Audio P0", key), key)
			}
			if strings.Contains(normalized, "base64") {
				return seedAudioValidationError(fmt.Sprintf("%s is not supported for Seed Audio P0", key), key)
			}
			if err := scanSeedAudioForbiddenFields(child); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, child := range typed {
			if err := scanSeedAudioForbiddenFields(child); err != nil {
				return err
			}
		}
	case string:
		if seedAudioIsForbiddenInlineData(typed) {
			return seedAudioValidationError("inline data, asset URLs, and file URLs are not supported for Seed Audio P0", "")
		}
	}
	return nil
}

func seedAudioNormalizeFieldKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, ".", "")
	return key
}

func seedAudioIsForbiddenInlineData(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(value, "asset://") ||
		strings.HasPrefix(value, "file://") ||
		strings.HasPrefix(value, "data:") ||
		strings.Contains(value, ";base64,")
}

func seedAudioNormalizeReferences(refs []dto.SeedAudioReference) ([]seedAudioReference, string, *types.NewAPIError) {
	if len(refs) == 0 {
		return nil, "text_only", nil
	}
	audioCount := 0
	imageCount := 0
	normalized := make([]seedAudioReference, 0, len(refs))
	for idx, ref := range refs {
		refType := strings.ToLower(strings.TrimSpace(ref.Type))
		refURL := strings.TrimSpace(ref.URL)
		if refURL == "" && strings.TrimSpace(ref.AudioURL) != "" {
			refURL = strings.TrimSpace(ref.AudioURL)
			if refType == "" {
				refType = "audio_url"
			}
		}
		if refURL == "" && strings.TrimSpace(ref.ImageURL) != "" {
			refURL = strings.TrimSpace(ref.ImageURL)
			if refType == "" {
				refType = "image_url"
			}
		}
		switch refType {
		case "audio", "audio_url":
			refType = "audio_url"
			audioCount++
		case "image", "image_url":
			refType = "image_url"
			imageCount++
		default:
			return nil, "", seedAudioValidationError("metadata.references[].type must be audio_url or image_url", fmt.Sprintf("metadata.references[%d].type", idx))
		}
		host, err := seedAudioValidateReferenceURL(refURL)
		if err != nil {
			return nil, "", err
		}
		normalized = append(normalized, seedAudioReference{
			Type: refType,
			URL:  refURL,
			Host: host,
			HMAC: common.GenerateHMAC(refURL),
		})
	}
	if audioCount > 0 && imageCount > 0 {
		return nil, "", seedAudioValidationError("Seed Audio P0 does not support mixing audio_url and image_url references", "metadata.references")
	}
	if audioCount > 3 {
		return nil, "", seedAudioValidationError("Seed Audio P0 supports at most 3 audio_url references", "metadata.references")
	}
	if imageCount > 1 {
		return nil, "", seedAudioValidationError("Seed Audio P0 supports at most 1 image_url reference", "metadata.references")
	}
	if audioCount > 0 {
		return normalized, "audio_url", nil
	}
	return normalized, "image_url", nil
}

func seedAudioValidateReferenceURL(rawURL string) (string, *types.NewAPIError) {
	if seedAudioIsForbiddenInlineData(rawURL) {
		seedAudioMetric.invalidReferenceURL.Add(1)
		return "", seedAudioInvalidReferenceURLError("reference URL must be an HTTPS URL")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		seedAudioMetric.invalidReferenceURL.Add(1)
		return "", seedAudioInvalidReferenceURLError("reference URL must be an HTTPS URL")
	}
	if parsed.User != nil {
		seedAudioMetric.invalidReferenceURL.Add(1)
		return "", seedAudioInvalidReferenceURLError("reference URL must not include credentials")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		seedAudioMetric.invalidReferenceURL.Add(1)
		return "", seedAudioInvalidReferenceURLError("reference URL host is not allowed")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap()
		if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast() {
			seedAudioMetric.invalidReferenceURL.Add(1)
			return "", seedAudioInvalidReferenceURLError("reference URL host is not allowed")
		}
	}
	return host, nil
}

func seedAudioCheckSensitiveText(c *gin.Context, textPrompt string) *types.NewAPIError {
	if !setting.ShouldCheckPromptSensitive() {
		return nil
	}
	contains, words := service.CheckSensitiveText(textPrompt)
	if !contains {
		return nil
	}
	logger.LogWarn(c, fmt.Sprintf("Seed Audio sensitive words detected: %s", strings.Join(words, ", ")))
	return seedAudioError(http.StatusBadRequest, "invalid_request_error", string(types.ErrorCodeSensitiveWordsDetected), "sensitive words detected", "input")
}

func seedAudioPrepareIdempotency(c *gin.Context, info *relaycommon.RelayInfo, normalized *seedAudioNormalizedRequest) (*seedAudioIdempotencyContext, *dto.SeedAudioIdempotencyRecord, *types.NewAPIError) {
	clientID := strings.TrimSpace(normalized.ClientRequestID)
	if clientID == "" {
		return nil, nil, nil
	}
	if !seedAudioValidClientRequestID(clientID) {
		return nil, nil, seedAudioError(http.StatusBadRequest, "invalid_request_error", "invalid_client_request_id", relaycommon.ClientRequestIDErrorMessage, "metadata.client_request_id")
	}
	requestHMAC, err := seedAudioRequestHMAC(normalized)
	if err != nil {
		return nil, nil, seedAudioError(http.StatusInternalServerError, "internal_error", "internal_error", "failed to build Seed Audio idempotency fingerprint", "")
	}

	now := time.Now().Unix()
	pending, err := model.CreateSeedAudioIdempotencyPending(seedAudioPendingParams(info.TokenId, clientID, requestHMAC, now))
	if err == nil {
		return &seedAudioIdempotencyContext{
			recordID:    pending.ID,
			requestHMAC: requestHMAC,
			clientID:    clientID,
		}, nil, nil
	}
	if !model.IsSeedAudioIdempotencyDuplicateError(err) {
		seedAudioMetric.idempotencyUnavailable.Add(1)
		return nil, nil, seedAudioError(http.StatusServiceUnavailable, "upstream_error", "seed_audio_idempotency_unavailable", "Seed Audio idempotency store is unavailable", "metadata.client_request_id")
	}

	existing, exists, findErr := model.GetSeedAudioIdempotencyByTokenClientRequestID(info.TokenId, clientID)
	if findErr != nil || !exists {
		seedAudioMetric.idempotencyUnavailable.Add(1)
		return nil, nil, seedAudioError(http.StatusServiceUnavailable, "upstream_error", "seed_audio_idempotency_unavailable", "Seed Audio idempotency state is unavailable", "metadata.client_request_id")
	}
	return seedAudioHandleExistingIdempotency(existing, requestHMAC, now, info.TokenId, clientID)
}

func seedAudioHandleExistingIdempotency(existing *model.SeedAudioIdempotency, requestHMAC string, now int64, tokenID int, clientID string) (*seedAudioIdempotencyContext, *dto.SeedAudioIdempotencyRecord, *types.NewAPIError) {
	if seedAudioIdempotencyExpired(existing, now) {
		if existing.TombstoneExpiresAt > now {
			return nil, nil, seedAudioError(http.StatusGone, "invalid_request_error", "idempotency_result_expired", "metadata.client_request_id was used previously but the cached Seed Audio result expired", "metadata.client_request_id")
		}
		reclaimed, err := model.ReclaimSeedAudioIdempotencyPending(existing.ID, existing.UpdatedAt, seedAudioPendingParams(tokenID, clientID, requestHMAC, now))
		if err != nil {
			seedAudioMetric.idempotencyConflict.Add(1)
			return nil, nil, seedAudioError(http.StatusConflict, "invalid_request_error", "idempotency_in_progress", "Seed Audio request with this client_request_id is still in progress", "metadata.client_request_id")
		}
		return &seedAudioIdempotencyContext{
			recordID:    reclaimed.ID,
			requestHMAC: requestHMAC,
			clientID:    clientID,
		}, nil, nil
	}

	if existing.RequestHMAC != requestHMAC {
		seedAudioMetric.idempotencyConflict.Add(1)
		return nil, nil, seedAudioError(http.StatusConflict, "invalid_request_error", "idempotency_conflict", "metadata.client_request_id was already used with a different request", "metadata.client_request_id")
	}

	record := existing.ToRecord()
	if record.ErrorStatusCode == 0 {
		record.ErrorStatusCode = http.StatusServiceUnavailable
	}
	if record.ErrorCode == "" {
		record.ErrorCode = "seed_audio_upstream_error"
	}
	switch existing.Status {
	case model.SeedAudioIdempotencyStatusCompleted:
		return &seedAudioIdempotencyContext{
			recordID:    existing.ID,
			requestHMAC: requestHMAC,
			clientID:    clientID,
		}, &record, nil
	case model.SeedAudioIdempotencyStatusPending:
		seedAudioMetric.idempotencyConflict.Add(1)
		return nil, nil, seedAudioError(http.StatusConflict, "invalid_request_error", "idempotency_in_progress", "Seed Audio request with this client_request_id is still in progress", "metadata.client_request_id")
	case model.SeedAudioIdempotencyStatusFailed:
		return nil, nil, seedAudioError(record.ErrorStatusCode, "upstream_error", record.ErrorCode, "Seed Audio request with this client_request_id already failed; use a new client_request_id to retry", "metadata.client_request_id")
	default:
		seedAudioMetric.idempotencyUnavailable.Add(1)
		return nil, nil, seedAudioError(http.StatusServiceUnavailable, "upstream_error", "seed_audio_idempotency_unavailable", "Seed Audio idempotency state is invalid", "metadata.client_request_id")
	}
}

func seedAudioIdempotencyExpired(existing *model.SeedAudioIdempotency, now int64) bool {
	if existing.ExpiresAt <= now {
		return true
	}
	return existing.Status == model.SeedAudioIdempotencyStatusCompleted &&
		existing.URLExpiresAt > 0 &&
		existing.URLExpiresAt <= now
}

func seedAudioPendingParams(tokenID int, clientID string, requestHMAC string, now int64) model.SeedAudioIdempotencyPendingParams {
	return model.SeedAudioIdempotencyPendingParams{
		TokenID:            tokenID,
		ClientRequestID:    clientID,
		RequestHMAC:        requestHMAC,
		Now:                now,
		ExpiresAt:          now + int64(seedAudioIdempotencyTTL.Seconds()),
		TombstoneExpiresAt: now + int64(seedAudioIdempotencyTombstoneTTL.Seconds()),
	}
}

func seedAudioRequestHMAC(normalized *seedAudioNormalizedRequest) (string, error) {
	canonical := map[string]interface{}{
		"model":             seedAudioUpstreamModel,
		"text_prompt":       normalized.TextPrompt,
		"format":            normalized.Format,
		"sample_rate":       normalized.SampleRate,
		"speech_rate":       normalized.SpeechRate,
		"loudness_rate":     normalized.LoudnessRate,
		"pitch_rate":        normalized.PitchRate,
		"reference_mode":    normalized.ReferenceMode,
		"client_request_id": normalized.ClientRequestID,
	}
	refs := make([]map[string]string, 0, len(normalized.References))
	for _, ref := range normalized.References {
		refs = append(refs, map[string]string{"type": ref.Type, "url": ref.URL})
	}
	canonical["references"] = refs
	canonicalBytes, err := common.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return common.GenerateHMAC(string(canonicalBytes)), nil
}

func (idem *seedAudioIdempotencyContext) storeCompleted(_ *gin.Context, record dto.SeedAudioIdempotencyRecord) error {
	if idem == nil {
		return nil
	}
	now := time.Now().Unix()
	return model.CompleteSeedAudioIdempotency(model.SeedAudioIdempotencyCompleteParams{
		ID:                 idem.recordID,
		Record:             record,
		UpdatedAt:          now,
		ExpiresAt:          now + int64(seedAudioIdempotencyTTL.Seconds()),
		TombstoneExpiresAt: now + int64(seedAudioIdempotencyTombstoneTTL.Seconds()),
	})
}

func (idem *seedAudioIdempotencyContext) storeFailure(_ *gin.Context, apiErr *types.NewAPIError, diagnostics *dto.SeedAudioUpstreamDiagnostics) error {
	if idem == nil || apiErr == nil {
		return nil
	}
	now := time.Now().Unix()
	xTTLogID := ""
	if diagnostics != nil {
		xTTLogID = diagnostics.XTTLogID
	}
	return model.FailSeedAudioIdempotency(model.SeedAudioIdempotencyFailParams{
		ID:                 idem.recordID,
		RequestHMAC:        idem.requestHMAC,
		XTTLogID:           xTTLogID,
		ErrorCode:          fmt.Sprintf("%v", apiErr.ToOpenAIError().Code),
		ErrorStatusCode:    apiErr.StatusCode,
		ErrorDiagnostics:   seedAudioDiagnosticsJSON(diagnostics),
		UpdatedAt:          now,
		ExpiresAt:          now + int64(seedAudioIdempotencyTTL.Seconds()),
		TombstoneExpiresAt: now + int64(seedAudioIdempotencyTombstoneTTL.Seconds()),
	})
}

func (idem *seedAudioIdempotencyContext) abandon(_ *gin.Context) error {
	if idem == nil {
		return nil
	}
	return model.DeleteSeedAudioIdempotency(idem.recordID)
}

func seedAudioValidClientRequestID(clientRequestID string) bool {
	if len(clientRequestID) == 0 || len(clientRequestID) > relaycommon.ClientRequestIDMaxLength {
		return false
	}
	for _, r := range clientRequestID {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r == ':' {
			continue
		}
		return false
	}
	return true
}

func seedAudioGroupRatio(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) types.GroupRatioInfo {
	groupRatioInfo := types.GroupRatioInfo{
		GroupRatio:        1.0,
		GroupSpecialRatio: -1,
	}
	if autoGroup, exists := ctx.Get("auto_group"); exists {
		if group, ok := autoGroup.(string); ok {
			relayInfo.UsingGroup = group
		}
	}
	if userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup); ok {
		groupRatioInfo.GroupSpecialRatio = userGroupRatio
		groupRatioInfo.GroupRatio = userGroupRatio
		groupRatioInfo.HasSpecialRatio = true
		return groupRatioInfo
	}
	groupRatioInfo.GroupRatio = ratio_setting.GetGroupRatio(relayInfo.UsingGroup)
	return groupRatioInfo
}

func seedAudioPrechargeQuota(groupRatio float64) int {
	return int(math.Floor(seedAudioMaxGeneratedSeconds * seedAudioBaseQuotaPerSecond * groupRatio))
}

func seedAudioActualQuota(originalDuration float64, groupRatio float64) int {
	if originalDuration <= 0 || groupRatio <= 0 {
		return 0
	}
	return int(math.Floor(originalDuration * seedAudioBaseQuotaPerSecond * groupRatio))
}

func seedAudioDispatchUpstream(c *gin.Context, info *relaycommon.RelayInfo, normalized *seedAudioNormalizedRequest) (*seedAudioUpstreamResult, *dto.SeedAudioUpstreamDiagnostics, *types.NewAPIError) {
	upstreamReq := normalized.toUpstreamRequest()
	requestBody, err := common.Marshal(upstreamReq)
	if err != nil {
		return nil, nil, seedAudioError(http.StatusInternalServerError, "internal_error", "internal_error", "failed to build Seed Audio upstream request", "")
	}

	timeout := seedAudioTimeout()
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, seedAudioEndpoint(info.ChannelBaseUrl), bytes.NewReader(requestBody))
	if err != nil {
		return nil, nil, seedAudioError(http.StatusInternalServerError, "internal_error", "internal_error", "failed to build Seed Audio upstream request", "")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", info.ApiKey)

	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, nil, seedAudioError(http.StatusInternalServerError, "internal_error", "internal_error", "failed to configure Seed Audio upstream client", "")
	}
	if client == nil {
		client = http.DefaultClient
	}
	clientCopy := *client
	clientCopy.Timeout = timeout + 5*time.Second

	startedAt := time.Now()
	resp, err := clientCopy.Do(req)
	latency := time.Since(startedAt)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			seedAudioMetric.upstreamTimeout.Add(1)
			return nil, seedAudioUpstreamDiagnosticsForError(ctx.Err(), latency, "timeout"), seedAudioError(http.StatusGatewayTimeout, "upstream_error", "seed_audio_upstream_timeout", "Seed Audio upstream request timed out", "")
		}
		seedAudioMetric.upstreamError.Add(1)
		return nil, seedAudioUpstreamDiagnosticsForError(err, latency, "network_error"), seedAudioError(http.StatusBadGateway, "upstream_error", "seed_audio_upstream_error", "Seed Audio upstream request failed", "")
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	diagnostics := seedAudioBuildUpstreamDiagnostics(resp, responseBody, latency)
	if readErr != nil {
		seedAudioMetric.upstreamError.Add(1)
		diagnostics.ErrorClass = "read_error"
		return nil, diagnostics, seedAudioError(http.StatusBadGateway, "upstream_error", "seed_audio_upstream_error", "failed to read Seed Audio upstream response", "")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, diagnostics, seedAudioUpstreamStatusErrorWithDiagnostics(resp.StatusCode, responseBody, diagnostics)
	}

	var responseAny interface{}
	if err := common.Unmarshal(responseBody, &responseAny); err != nil {
		seedAudioMetric.upstreamError.Add(1)
		seedAudioMarkInvalidJSONDiagnostics(diagnostics)
		return nil, diagnostics, seedAudioError(http.StatusBadGateway, "upstream_error", "seed_audio_upstream_error", "Seed Audio upstream returned invalid JSON", "")
	}
	seedAudioAttachJSONDiagnostics(diagnostics, responseAny)
	result, schemaOK := seedAudioParseUpstreamSuccess(responseAny, resp.Header.Get("X-Tt-Logid"))
	if !schemaOK {
		seedAudioMetric.upstreamError.Add(1)
		seedAudioMarkSchemaDiagnostics(diagnostics)
		return nil, diagnostics, seedAudioError(http.StatusBadGateway, "upstream_error", "seed_audio_upstream_schema_error", "Seed Audio upstream response schema is invalid", "")
	}
	if result.OriginalDuration > seedAudioMaxGeneratedSeconds {
		seedAudioMetric.upstreamError.Add(1)
		diagnostics.ErrorClass = "duration_exceeds_reserved"
		return nil, diagnostics, seedAudioError(http.StatusBadGateway, "upstream_error", "seed_audio_upstream_error", "Seed Audio upstream duration exceeds reserved maximum", "")
	}
	return result, diagnostics, nil
}

func (normalized *seedAudioNormalizedRequest) toUpstreamRequest() seedAudioUpstreamRequest {
	refs := make([]seedAudioUpstreamReference, 0, len(normalized.References))
	for _, ref := range normalized.References {
		switch ref.Type {
		case "audio_url":
			refs = append(refs, seedAudioUpstreamReference{AudioURL: ref.URL})
		case "image_url":
			refs = append(refs, seedAudioUpstreamReference{ImageURL: ref.URL})
		}
	}
	return seedAudioUpstreamRequest{
		Model:      seedAudioUpstreamModel,
		TextPrompt: normalized.TextPrompt,
		AudioConfig: seedAudioUpstreamAudioConfig{
			Format:       normalized.Format,
			SampleRate:   normalized.SampleRate,
			SpeechRate:   normalized.SpeechRate,
			LoudnessRate: normalized.LoudnessRate,
			PitchRate:    normalized.PitchRate,
		},
		References: refs,
	}
}

func seedAudioParseUpstreamSuccess(responseAny interface{}, xTTLogID string) (*seedAudioUpstreamResult, bool) {
	root, ok := responseAny.(map[string]interface{})
	if !ok {
		return nil, false
	}
	audio := seedAudioTopLevelStringField(root, "audio")
	if audio == "" {
		audio = seedAudioTopLevelStringField(root, "data")
	}
	outputURL := seedAudioTopLevelStringField(root, "url")
	if outputURL == "" && audio == "" {
		seedAudioMetric.missingURL.Add(1)
		return nil, false
	}
	duration, durationOK := seedAudioTopLevelFloatField(root, "duration")
	if !durationOK || duration <= 0 {
		return nil, false
	}
	originalDuration, originalDurationOK := seedAudioTopLevelFloatField(root, "original_duration", "originalDuration")
	if !originalDurationOK || originalDuration <= 0 {
		seedAudioMetric.missingOriginalDuration.Add(1)
		return nil, false
	}
	return &seedAudioUpstreamResult{
		Audio:            audio,
		URL:              outputURL,
		Duration:         duration,
		OriginalDuration: originalDuration,
		XTTLogID:         strings.TrimSpace(xTTLogID),
	}, true
}

func seedAudioTopLevelStringField(root map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := root[key]
		if !ok {
			continue
		}
		if str, ok := value.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

func seedAudioTopLevelFloatField(root map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := root[key]
		if !ok {
			continue
		}
		if val, ok := seedAudioFloatValue(value); ok {
			return val, true
		}
	}
	return 0, false
}

func seedAudioEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = seedAudioDefaultBaseURL
	}
	if strings.HasSuffix(baseURL, seedAudioCreatePath) {
		return baseURL
	}
	return baseURL + seedAudioCreatePath
}

func seedAudioTimeout() time.Duration {
	seconds := common.GetEnvOrDefault("SEED_AUDIO_UPSTREAM_TIMEOUT_SECONDS", 150)
	if seconds < 60 {
		seconds = 60
	}
	if seconds > 180 {
		seconds = 180
	}
	return time.Duration(seconds) * time.Second
}

func seedAudioUpstreamStatusError(statusCode int, responseBody []byte) *types.NewAPIError {
	diagnostics := seedAudioBuildUpstreamDiagnostics(&http.Response{
		StatusCode: statusCode,
		Header:     http.Header{},
	}, responseBody, 0)
	return seedAudioUpstreamStatusErrorWithDiagnostics(statusCode, responseBody, diagnostics)
}

func seedAudioUpstreamStatusErrorWithDiagnostics(statusCode int, responseBody []byte, diagnostics *dto.SeedAudioUpstreamDiagnostics) *types.NewAPIError {
	diagnosticBody := seedAudioDiagnosticPreview(responseBody)
	body := strings.ToLower(string(diagnosticBody))
	if responseAny, ok := seedAudioParseDiagnosticJSON(diagnosticBody); ok {
		seedAudioAttachJSONDiagnostics(diagnostics, responseAny)
	} else if seedAudioLooksLikeJSON(diagnosticBody) {
		seedAudioMarkInvalidJSONDiagnostics(diagnostics)
	}
	if seedAudioLooksLikeReferenceFetchError(body) {
		seedAudioMetric.invalidReferenceURL.Add(1)
		if diagnostics != nil {
			diagnostics.ErrorClass = "invalid_reference_url"
			diagnostics.ReferenceFetchLike = true
		}
		return seedAudioInvalidReferenceURLError("Seed Audio upstream could not access a reference URL")
	}
	seedAudioMetric.upstreamError.Add(1)
	if diagnostics != nil {
		diagnostics.ErrorClass = seedAudioHTTPErrorClass(statusCode, diagnostics)
	}
	if statusCode >= http.StatusInternalServerError {
		return seedAudioError(http.StatusServiceUnavailable, "upstream_error", "seed_audio_upstream_error", "Seed Audio upstream service is unavailable", "")
	}
	return seedAudioError(http.StatusBadGateway, "upstream_error", "seed_audio_upstream_error", "Seed Audio upstream returned an error", "")
}

func seedAudioBuildUpstreamDiagnostics(resp *http.Response, body []byte, latency time.Duration) *dto.SeedAudioUpstreamDiagnostics {
	statusCode := 0
	headers := http.Header{}
	if resp != nil {
		statusCode = resp.StatusCode
		headers = resp.Header
	}
	diagnosticBody := seedAudioDiagnosticPreview(body)
	contentTypeClass := seedAudioContentTypeClass(headers.Get("Content-Type"), diagnosticBody)
	html := seedAudioLooksLikeHTML(headers.Get("Content-Type"), diagnosticBody)
	cloudflareLike := seedAudioLooksLikeCloudflare(diagnosticBody)
	return &dto.SeedAudioUpstreamDiagnostics{
		UpstreamHTTPStatus: statusCode,
		ContentTypeClass:   contentTypeClass,
		XTTLogID:           seedAudioSanitizeDiagnosticValue(headers.Get("X-Tt-Logid"), 128),
		XTTTraceID:         seedAudioSanitizeDiagnosticValue(headers.Get("X-Tt-Trace-Id"), 128),
		RequestID:          seedAudioSanitizeDiagnosticValue(headers.Get("Request-Id"), 128),
		XRequestID:         seedAudioSanitizeDiagnosticValue(headers.Get("X-Request-Id"), 128),
		LatencyMS:          seedAudioLatencyMS(latency),
		BodySizeBucket:     seedAudioBodySizeBucket(len(body)),
		ResponseClass:      seedAudioHTTPResponseClass(statusCode),
		HTML:               html,
		CloudflareLike:     cloudflareLike,
	}
}

func seedAudioDiagnosticPreview(body []byte) []byte {
	if len(body) <= seedAudioDiagnosticsPreviewBytes {
		return body
	}
	return body[:seedAudioDiagnosticsPreviewBytes]
}

func seedAudioUpstreamDiagnosticsForError(_ error, latency time.Duration, errorClass string) *dto.SeedAudioUpstreamDiagnostics {
	return &dto.SeedAudioUpstreamDiagnostics{
		LatencyMS:      seedAudioLatencyMS(latency),
		BodySizeBucket: seedAudioBodySizeBucket(0),
		ResponseClass:  errorClass,
		ErrorClass:     errorClass,
	}
}

func seedAudioDiagnosticsJSON(diagnostics *dto.SeedAudioUpstreamDiagnostics) string {
	if diagnostics == nil {
		return ""
	}
	data, err := common.Marshal(diagnostics)
	if err != nil {
		return ""
	}
	if string(data) == "{}" {
		return ""
	}
	return string(data)
}

func seedAudioParseDiagnosticJSON(body []byte) (interface{}, bool) {
	if !seedAudioLooksLikeJSON(body) {
		return nil, false
	}
	var responseAny interface{}
	if err := common.Unmarshal(body, &responseAny); err != nil {
		return nil, false
	}
	return responseAny, true
}

func seedAudioAttachJSONDiagnostics(diagnostics *dto.SeedAudioUpstreamDiagnostics, responseAny interface{}) {
	if diagnostics == nil {
		return
	}
	if requestID := seedAudioFindResponseMetadataRequestID(responseAny); requestID != "" {
		diagnostics.ResponseMetadataRequestID = requestID
	}
}

func seedAudioMarkInvalidJSONDiagnostics(diagnostics *dto.SeedAudioUpstreamDiagnostics) {
	if diagnostics == nil {
		return
	}
	diagnostics.InvalidJSON = true
	diagnostics.ErrorClass = "invalid_json"
}

func seedAudioMarkSchemaDiagnostics(diagnostics *dto.SeedAudioUpstreamDiagnostics) {
	if diagnostics == nil {
		return
	}
	diagnostics.InvalidJSON = false
	diagnostics.ErrorClass = "upstream_schema_error"
}

func seedAudioFindResponseMetadataRequestID(value interface{}) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if seedAudioNormalizeFieldKey(key) == "responsemetadata" {
				return seedAudioSanitizeDiagnosticValue(seedAudioFindStringField(child, "RequestId", "RequestID", "request_id"), 128)
			}
		}
		for _, child := range typed {
			if found := seedAudioFindResponseMetadataRequestID(child); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := seedAudioFindResponseMetadataRequestID(child); found != "" {
				return found
			}
		}
	}
	return ""
}

func seedAudioHTTPErrorClass(statusCode int, diagnostics *dto.SeedAudioUpstreamDiagnostics) string {
	base := "upstream_http_" + seedAudioHTTPResponseClass(statusCode)
	if diagnostics == nil {
		return base
	}
	switch {
	case diagnostics.CloudflareLike:
		return base + "_cloudflare"
	case diagnostics.HTML:
		return base + "_html"
	case diagnostics.InvalidJSON:
		return base + "_invalid_json"
	case diagnostics.ContentTypeClass != "":
		return base + "_" + diagnostics.ContentTypeClass
	default:
		return base
	}
}

func seedAudioHTTPResponseClass(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "1xx"
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500 && statusCode < 600:
		return "5xx"
	case statusCode == 0:
		return ""
	default:
		return "other"
	}
}

func seedAudioContentTypeClass(contentType string, body []byte) string {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch {
	case strings.Contains(mediaType, "json"):
		return "json"
	case mediaType == "text/html" || mediaType == "application/xhtml+xml":
		return "html"
	case strings.HasPrefix(mediaType, "text/"):
		return "text"
	case mediaType == "application/octet-stream":
		return "binary"
	case mediaType != "":
		return "other"
	case len(strings.TrimSpace(string(body))) == 0:
		return "empty"
	case seedAudioLooksLikeJSON(body):
		return "json"
	case seedAudioLooksLikeHTML(contentType, body):
		return "html"
	default:
		return "unknown"
	}
}

func seedAudioLooksLikeJSON(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func seedAudioLooksLikeHTML(contentType string, body []byte) bool {
	mediaType := strings.ToLower(contentType)
	trimmed := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.Contains(mediaType, "text/html") ||
		strings.HasPrefix(trimmed, "<!doctype html") ||
		strings.HasPrefix(trimmed, "<html") ||
		strings.Contains(trimmed, "<body")
}

func seedAudioLooksLikeCloudflare(body []byte) bool {
	trimmed := strings.ToLower(string(body))
	return strings.Contains(trimmed, "cloudflare") ||
		strings.Contains(trimmed, "cf-ray") ||
		strings.Contains(trimmed, "cf-error") ||
		strings.Contains(trimmed, "attention required")
}

func seedAudioBodySizeBucket(size int) string {
	switch {
	case size <= 0:
		return "empty"
	case size <= 1024:
		return "le_1kb"
	case size <= 4*1024:
		return "le_4kb"
	case size <= 16*1024:
		return "le_16kb"
	case size <= 64*1024:
		return "le_64kb"
	case size <= 128*1024:
		return "le_128kb"
	default:
		return "gt_128kb"
	}
}

func seedAudioLatencyMS(latency time.Duration) int64 {
	if latency <= 0 {
		return 0
	}
	return latency.Milliseconds()
}

func seedAudioSanitizeDiagnosticValue(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxRunes <= 0 {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
		return "redacted_url_like"
	}
	if strings.Contains(lower, "bearer ") || strings.Contains(lower, "x-api-key") || strings.Contains(lower, "sql_dsn") {
		return "redacted_sensitive_like"
	}
	var b strings.Builder
	count := 0
	for _, r := range value {
		if count >= maxRunes {
			break
		}
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '.' || r == '_' || r == '-' || r == ':' || r == '/' || r == '=' || r == '+':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		count++
	}
	return strings.Trim(b.String(), "_")
}

func seedAudioLooksLikeReferenceFetchError(body string) bool {
	hasExplicitReferenceURL := strings.Contains(body, "url") || strings.Contains(body, "reference")
	hasReferenceMedia := strings.Contains(body, "audio") ||
		strings.Contains(body, "image") ||
		strings.Contains(body, "media") ||
		strings.Contains(body, "resource") ||
		strings.Contains(body, "file")
	hasFetchAccessFailure := strings.Contains(body, "fetch") ||
		strings.Contains(body, "download") ||
		strings.Contains(body, "access") ||
		strings.Contains(body, "inaccessible") ||
		strings.Contains(body, "not found") ||
		strings.Contains(body, "403") ||
		strings.Contains(body, "404") ||
		strings.Contains(body, "forbidden") ||
		strings.Contains(body, "permission denied") ||
		strings.Contains(body, "unreachable") ||
		strings.Contains(body, "connect") ||
		strings.Contains(body, "resolve") ||
		strings.Contains(body, "dns") ||
		strings.Contains(body, "tls") ||
		strings.Contains(body, "ssl")
	hasInvalidURLFailure := strings.Contains(body, "invalid") ||
		strings.Contains(body, "unsupported")
	hasReferenceTimeout := strings.Contains(body, "timeout") ||
		strings.Contains(body, "timed out")
	if hasExplicitReferenceURL && (hasFetchAccessFailure || hasInvalidURLFailure || hasReferenceTimeout) {
		return true
	}
	return hasReferenceMedia && hasFetchAccessFailure
}

func seedAudioFindStringField(value interface{}, keys ...string) string {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[seedAudioNormalizeFieldKey(key)] = struct{}{}
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if _, ok := keySet[seedAudioNormalizeFieldKey(key)]; ok {
				if str, ok := child.(string); ok {
					return str
				}
			}
		}
		for _, child := range typed {
			if found := seedAudioFindStringField(child, keys...); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := seedAudioFindStringField(child, keys...); found != "" {
				return found
			}
		}
	}
	return ""
}

func seedAudioFindFloatField(value interface{}, keys ...string) float64 {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[seedAudioNormalizeFieldKey(key)] = struct{}{}
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if _, ok := keySet[seedAudioNormalizeFieldKey(key)]; ok {
				if val, ok := seedAudioFloatValue(child); ok {
					return val
				}
			}
		}
		for _, child := range typed {
			if found := seedAudioFindFloatField(child, keys...); found != 0 {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := seedAudioFindFloatField(child, keys...); found != 0 {
				return found
			}
		}
	}
	return 0
}

func seedAudioFloatValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func seedAudioResponseFromRecord(modelName string, record dto.SeedAudioIdempotencyRecord, clientRequestIDs ...string) dto.SeedAudioResponse {
	response := dto.SeedAudioResponse{
		ID:               record.ResponseID,
		Object:           "audio.speech",
		Created:          record.CreatedAt,
		Model:            modelName,
		URL:              record.TemporaryURL,
		URLExpiresAt:     record.URLExpiresAt,
		Duration:         record.Duration,
		OriginalDuration: record.OriginalDuration,
		Usage: dto.SeedAudioUsage{
			Type:             "seconds",
			Duration:         record.Duration,
			OriginalDuration: record.OriginalDuration,
		},
	}
	if len(clientRequestIDs) > 0 {
		if clientRequestID := strings.TrimSpace(clientRequestIDs[0]); clientRequestID != "" {
			response.Metadata = map[string]interface{}{"client_request_id": clientRequestID}
		}
	}
	return response
}

func seedAudioRecordConsumeLog(c *gin.Context, info *relaycommon.RelayInfo, normalized *seedAudioNormalizedRequest, result *seedAudioUpstreamResult, actualQuota int, prechargeQuota int) {
	if c == nil || info == nil || normalized == nil || result == nil {
		return
	}
	startTime := info.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}
	useTimeSeconds := int(time.Since(startTime).Seconds())
	if useTimeSeconds < 0 {
		useTimeSeconds = 0
	}
	requestPath := ""
	if c.Request != nil && c.Request.URL != nil {
		requestPath = c.Request.URL.Path
	}
	other := map[string]interface{}{
		"request_path":              requestPath,
		"seed_audio":                true,
		"usage_type":                "seconds",
		"reference_mode":            normalized.ReferenceMode,
		"duration":                  result.Duration,
		"original_duration":         result.OriginalDuration,
		"actual_quota":              actualQuota,
		"pre_consumed_quota":        prechargeQuota,
		"base_quota_per_second":     seedAudioBaseQuotaPerSecond,
		"group_ratio":               info.PriceData.GroupRatioInfo.GroupRatio,
		"client_request_id_present": normalized.ClientRequestID != "",
		"output_url_present":        result.URL != "",
		"url_expires_seconds":       int(seedAudioURLTTL.Seconds()),
	}
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	if info.BillingSource != "" {
		other["billing_source"] = info.BillingSource
	}
	content := fmt.Sprintf("Seed Audio usage settled: original_duration=%.3fs, actual_quota=%d", result.OriginalDuration, actualQuota)
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId:      info.ChannelId,
		ModelName:      info.OriginModelName,
		TokenName:      c.GetString("token_name"),
		Quota:          actualQuota,
		Content:        content,
		TokenId:        info.TokenId,
		UseTimeSeconds: useTimeSeconds,
		IsStream:       false,
		Group:          info.UsingGroup,
		Other:          other,
	})
}

func seedAudioUpdateChannelUsedQuota(c *gin.Context, channelID int, quota int) {
	if quota == 0 || channelID == 0 {
		return
	}
	if common.BatchUpdateEnabled {
		model.UpdateChannelUsedQuota(channelID, quota)
		return
	}
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error; err != nil {
		logger.LogError(c, "failed to update Seed Audio channel used quota")
	}
}

func seedAudioIsInsufficientBalance(apiErr *types.NewAPIError) bool {
	if apiErr == nil {
		return false
	}
	return apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota ||
		apiErr.GetErrorCode() == types.ErrorCodePreConsumeTokenQuotaFailed
}

func seedAudioInsufficientBalanceError(requiredQuota int) *types.NewAPIError {
	return types.WithOpenAIError(types.OpenAIError{
		Message: fmt.Sprintf("Seed Audio requires a pre-charge of %d quota for this request. Please deposit more or reduce concurrency.", requiredQuota),
		Type:    "insufficient_balance",
		Code:    "seed_audio_precharge_required",
	}, http.StatusPaymentRequired, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
}

func seedAudioValidationError(message string, param string) *types.NewAPIError {
	seedAudioMetric.validationFailed.Add(1)
	return seedAudioError(http.StatusBadRequest, "invalid_request_error", "invalid_request_error", message, param)
}

func seedAudioInputTooLongError() *types.NewAPIError {
	seedAudioMetric.validationFailed.Add(1)
	return seedAudioError(http.StatusBadRequest, "invalid_request_error", "seed_audio_input_too_long", fmt.Sprintf("Seed Audio text_prompt exceeds %d characters", seedAudioMaxTextRunes), "input")
}

func seedAudioInvalidReferenceURLError(message string) *types.NewAPIError {
	return seedAudioError(http.StatusBadRequest, "invalid_request_error", "invalid_reference_url", message, "metadata.references")
}

func seedAudioError(statusCode int, errorType string, code string, message string, param string) *types.NewAPIError {
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}
	openAIError := types.OpenAIError{
		Message: message,
		Type:    errorType,
		Param:   param,
		Code:    code,
	}
	opts := []types.NewAPIErrorOptions{types.ErrOptionWithSkipRetry()}
	if statusCode < http.StatusInternalServerError {
		opts = append(opts, types.ErrOptionWithNoRecordErrorLog())
	}
	return types.WithOpenAIError(openAIError, statusCode, opts...)
}

func seedAudioMaskLogID(logID string) string {
	logID = strings.TrimSpace(logID)
	if logID == "" {
		return ""
	}
	if len(logID) <= 8 {
		return "****"
	}
	return "****" + logID[len(logID)-8:]
}
