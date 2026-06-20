package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	moderationDiagnoseSourceVideoTask    = "video_task"
	moderationDiagnoseSourceLibraryAsset = "library_asset"
	moderationDiagnoseSourceManual       = "manual"

	moderationDiagnoseTypeTaskID    = "task_id"
	moderationDiagnoseTypeAssetID   = "asset_id"
	moderationDiagnoseTypeRequestID = "request_id"

	moderationDiagnoseResultFound           = "found"
	moderationDiagnoseResultNotFound        = "not_found"
	moderationDiagnoseResultRequestFailed   = "request_failed"
	moderationDiagnoseResultValidationError = "validation_error"
	moderationDiagnoseResultRateLimited     = "rate_limited"

	seedanceModerationDiagnoseActionName = "GetModerationResult"
	seedanceModerationDiagnoseRegion     = "ap-southeast-1"

	moderationDiagnoseTaskMaxAge          = 14 * 24 * time.Hour
	moderationDiagnoseCGTFutureTolerance  = 2 * time.Hour
	moderationDiagnoseCGTLookupTolerance  = 15 * time.Minute
	moderationDiagnoseCGTTimestampLayout  = "20060102150405"
	moderationDiagnoseCGTExpectedIDFormat = "cgt-YYYYMMDDHHMMSS-xxxxx"
)

var (
	errModerationDiagnoseTaskNotFound = errors.New("task not found")
	moderationDiagnoseCGTIDPattern    = regexp.MustCompile(`^cgt-(\d{14})-([A-Za-z0-9]{5})$`)
	moderationAssetAdminModels        = []string{
		"seedance-virtual-asset-admin",
		"seedance-real-human-asset-admin",
	}
	moderationDiagnoseQueryExecutor = executeModerationDiagnoseQuery
)

type moderationDiagnoseRequest struct {
	SourceType          string `json:"source_type"`
	RecordID            string `json:"record_id"`
	TaskID              string `json:"task_id"`
	AssetID             string `json:"asset_id"`
	RequestID           string `json:"request_id"`
	ID                  string `json:"id"`
	Type                string `json:"type"`
	CredentialChannelID int    `json:"credential_channel_id,omitempty"`
	AssetAdminChannelID int    `json:"asset_admin_channel_id,omitempty"`
}

type moderationDiagnoseQuery struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type moderationDiagnoseUpstreamRequest struct {
	ID   string `json:"Id"`
	Type string `json:"Type"`
}

type moderationDiagnoseAttempt struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	StatusCode     int    `json:"status_code,omitempty"`
	Success        bool   `json:"success"`
	RawRequestBody string `json:"raw_request_body,omitempty"`
	RawResponse    string `json:"raw_response,omitempty"`
	RawError       string `json:"raw_error,omitempty"`
}

type moderationDiagnoseResponse struct {
	ResultStatus     string                      `json:"result_status"`
	SourceType       string                      `json:"source_type"`
	RecordID         string                      `json:"record_id,omitempty"`
	ResolvedQuery    moderationDiagnoseQuery     `json:"resolved_query"`
	AttemptedQueries []moderationDiagnoseAttempt `json:"attempted_queries"`
	RawRequestBody   string                      `json:"raw_request_body,omitempty"`
	RawResponse      string                      `json:"raw_response,omitempty"`
	RawError         string                      `json:"raw_error,omitempty"`
	Resolved         map[string]interface{}      `json:"resolved,omitempty"`

	credentialChannelID int
}

type moderationTenantBoundary struct {
	Group            string
	VideoChannelID   int
	VideoProjectName string
}

type moderationTarget struct {
	SourceType                string
	RecordID                  string
	Queries                   []moderationDiagnoseQuery
	Resolved                  map[string]interface{}
	Tenant                    *moderationTenantBoundary
	ManualCredentialChannelID int
}

type moderationCredential struct {
	ChannelID int
	AccessKey string
	SecretKey string
	BaseURL   string
	Config    *seedanceAssetAdminConfig
	Priority  int64
}

type moderationCredentialChannelOption struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

func ModerationDiagnose(c *gin.Context) {
	start := time.Now()
	req := moderationDiagnoseRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		response := &moderationDiagnoseResponse{
			ResultStatus:     moderationDiagnoseResultValidationError,
			AttemptedQueries: make([]moderationDiagnoseAttempt, 0),
		}
		diagnoseErr := errors.New("invalid JSON request body")
		recordModerationDiagnoseAudit(c, req, response, time.Since(start), diagnoseErr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": diagnoseErr.Error(),
			"data":    response,
		})
		return
	}
	normalizeModerationDiagnoseRequest(&req)

	response, err := runModerationDiagnose(c, req)
	recordModerationDiagnoseAudit(c, req, response, time.Since(start), err)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    response,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    response,
	})
}

func GetModerationCredentialChannels(c *gin.Context) {
	candidates, err := listModerationCredentialChannels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	options := make([]moderationCredentialChannelOption, 0, len(candidates))
	for _, candidate := range candidates {
		options = append(options, moderationCredentialChannelOption{
			ID:    candidate.ChannelID,
			Label: fmt.Sprintf("Asset Admin credential #%d", candidate.ChannelID),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

func normalizeModerationDiagnoseRequest(req *moderationDiagnoseRequest) {
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.RecordID = strings.TrimSpace(req.RecordID)
	req.TaskID = strings.TrimSpace(req.TaskID)
	req.AssetID = strings.TrimSpace(req.AssetID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ID = strings.TrimSpace(req.ID)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	if req.AssetAdminChannelID == 0 {
		req.AssetAdminChannelID = req.CredentialChannelID
	}
}

func runModerationDiagnose(c *gin.Context, req moderationDiagnoseRequest) (*moderationDiagnoseResponse, error) {
	target, err := resolveModerationTarget(req)
	response := &moderationDiagnoseResponse{
		ResultStatus:     moderationDiagnoseResultValidationError,
		SourceType:       req.SourceType,
		RecordID:         firstNonEmpty(req.RecordID, req.TaskID, req.AssetID, req.ID),
		AttemptedQueries: make([]moderationDiagnoseAttempt, 0),
	}
	if target != nil {
		response.SourceType = target.SourceType
		response.RecordID = target.RecordID
		response.Resolved = target.Resolved
		if len(target.Queries) > 0 {
			response.ResolvedQuery = target.Queries[0]
		}
	}
	if err != nil {
		return response, err
	}
	if len(target.Queries) == 0 {
		return response, errors.New("no moderation diagnose query could be resolved")
	}

	credential, err := resolveModerationCredential(target)
	if err != nil {
		return response, err
	}
	response.credentialChannelID = credential.ChannelID
	response.ResultStatus = moderationDiagnoseResultRequestFailed

	for _, query := range target.Queries {
		attempt := moderationDiagnoseQueryExecutor(
			c,
			query,
			credential.BaseURL,
			credential.AccessKey,
			credential.SecretKey,
			credential.Config,
		)
		response.AttemptedQueries = append(response.AttemptedQueries, attempt)
		response.RawRequestBody = attempt.RawRequestBody
		response.RawResponse = attempt.RawResponse
		response.RawError = attempt.RawError
		if attempt.Success {
			response.ResolvedQuery = moderationDiagnoseQuery{ID: attempt.ID, Type: attempt.Type}
			response.ResultStatus = moderationDiagnoseResultStatusForAttempt(attempt)
			return response, nil
		}
	}

	return response, errors.New("moderation diagnose upstream query failed")
}

func resolveModerationTarget(req moderationDiagnoseRequest) (*moderationTarget, error) {
	if req.SourceType == "" {
		return nil, errors.New("source_type is required")
	}
	switch req.SourceType {
	case moderationDiagnoseSourceVideoTask:
		return resolveModerationVideoTaskTarget(firstNonEmpty(req.RecordID, req.TaskID, req.ID))
	case moderationDiagnoseSourceLibraryAsset:
		return &moderationTarget{
			SourceType: moderationDiagnoseSourceLibraryAsset,
			RecordID:   firstNonEmpty(req.AssetID, req.RecordID, req.ID),
		}, errors.New("asset ownership mapping is not available; use manual mode")
	case moderationDiagnoseSourceManual:
		return resolveModerationManualTarget(req)
	default:
		return nil, fmt.Errorf("unsupported source_type %q", req.SourceType)
	}
}

func resolveModerationVideoTaskTarget(lookupID string) (*moderationTarget, error) {
	if strings.TrimSpace(lookupID) == "" {
		return nil, errors.New("Task record ID / LSF task_id / BP task_id is required")
	}

	task, err := findModerationDiagnoseTask(lookupID)
	if err != nil {
		return &moderationTarget{
			SourceType: moderationDiagnoseSourceVideoTask,
			RecordID:   lookupID,
		}, err
	}
	return buildModerationVideoTaskTarget(task)
}

func buildModerationVideoTaskTarget(task *model.Task) (*moderationTarget, error) {
	if task == nil {
		return nil, errModerationDiagnoseTaskNotFound
	}
	target := &moderationTarget{
		SourceType: moderationDiagnoseSourceVideoTask,
		RecordID:   strconv.FormatInt(task.ID, 10),
	}

	taskTime := task.SubmitTime
	if taskTime == 0 {
		taskTime = task.CreatedAt
	}
	if taskTime <= 0 || time.Since(time.Unix(taskTime, 0)) > moderationDiagnoseTaskMaxAge {
		return target, errors.New("task is outside the 14-day moderation lookup window")
	}
	if strings.TrimSpace(task.Group) == "" {
		return target, errors.New("task tenant group is missing")
	}
	if task.ChannelId <= 0 {
		return target, errors.New("task video channel is missing")
	}

	videoChannel, err := model.GetChannelById(task.ChannelId, true)
	if err != nil {
		return target, err
	}
	videoConfig, err := resolveSeedanceModerationConfigFromChannel(videoChannel)
	if err != nil {
		return target, err
	}

	upstreamGenerationID := strings.TrimSpace(task.PrivateData.UpstreamTaskID)
	if !strings.HasPrefix(upstreamGenerationID, "cgt-") {
		upstreamGenerationID = extractTaskDataString(task.Data, []string{"id", "Id", "ID"}, "cgt-")
	}
	if !strings.HasPrefix(upstreamGenerationID, "cgt-") {
		upstreamGenerationID = ""
	}
	requestID := extractTaskDataString(
		task.Data,
		[]string{"request_id", "requestId", "requestID", "RequestId", "RequestID"},
		"",
	)

	queries := make([]moderationDiagnoseQuery, 0, 2)
	if upstreamGenerationID != "" {
		queries = append(queries, moderationDiagnoseQuery{
			ID:   upstreamGenerationID,
			Type: moderationDiagnoseTypeTaskID,
		})
	}
	if requestID != "" && !queryExists(queries, requestID, moderationDiagnoseTypeRequestID) {
		queries = append(queries, moderationDiagnoseQuery{
			ID:   requestID,
			Type: moderationDiagnoseTypeRequestID,
		})
	}

	target.Queries = queries
	target.Resolved = map[string]interface{}{
		"task_record_id":         task.ID,
		"platform_task_id":       task.TaskID,
		"upstream_generation_id": upstreamGenerationID,
		"request_id":             requestID,
	}
	target.Tenant = &moderationTenantBoundary{
		Group:            task.Group,
		VideoChannelID:   task.ChannelId,
		VideoProjectName: strings.TrimSpace(videoConfig.ProjectName),
	}
	return target, nil
}

func resolveModerationManualTarget(req moderationDiagnoseRequest) (*moderationTarget, error) {
	if req.ID == "" {
		return nil, errors.New("Id is required for manual diagnose")
	}
	if req.Type == moderationDiagnoseTypeTaskID {
		task, err := findManualModerationDiagnoseTask(req.ID)
		if err == nil {
			return buildModerationVideoTaskTarget(task)
		}
		if !errors.Is(err, errModerationDiagnoseTaskNotFound) {
			return nil, err
		}
	}
	if !isAllowedModerationDiagnoseType(req.Type) {
		return nil, errors.New("Type must be one of task_id, request_id, or asset_id")
	}
	if req.AssetAdminChannelID <= 0 {
		if req.Type == moderationDiagnoseTypeTaskID {
			return nil, errors.New("task ownership was not found; select an Asset Admin credential channel")
		}
		return nil, errors.New("Asset Admin credential channel is required for manual diagnose")
	}

	return &moderationTarget{
		SourceType: moderationDiagnoseSourceManual,
		RecordID:   req.ID,
		Queries: []moderationDiagnoseQuery{{
			ID:   req.ID,
			Type: req.Type,
		}},
		Resolved: map[string]interface{}{
			"id":   req.ID,
			"type": req.Type,
		},
		ManualCredentialChannelID: req.AssetAdminChannelID,
	}, nil
}

func findManualModerationDiagnoseTask(lookupID string) (*model.Task, error) {
	lookupID = strings.TrimSpace(lookupID)
	if _, err := strconv.ParseInt(lookupID, 10, 64); err == nil ||
		strings.HasPrefix(lookupID, "task_") ||
		strings.HasPrefix(lookupID, "cgt-") {
		return findModerationDiagnoseTask(lookupID)
	}
	return nil, errModerationDiagnoseTaskNotFound
}

func findModerationDiagnoseTask(lookupID string) (*model.Task, error) {
	lookupID = strings.TrimSpace(lookupID)
	if numericID, err := strconv.ParseInt(lookupID, 10, 64); err == nil {
		if numericID <= 0 {
			return nil, errModerationDiagnoseTaskNotFound
		}
		var task model.Task
		err = model.DB.Where("id = ?", numericID).First(&task).Error
		if err == nil {
			return &task, nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errModerationDiagnoseTaskNotFound
		}
		return nil, err
	}

	var (
		task   *model.Task
		exists bool
		err    error
	)
	switch {
	case strings.HasPrefix(lookupID, "task_"):
		task, exists, err = model.GetByOnlyTaskId(lookupID)
	case strings.HasPrefix(lookupID, "cgt-"):
		createdAtStart, createdAtEnd, parseErr := moderationDiagnoseCGTLookupRange(lookupID, time.Now().UTC())
		if parseErr != nil {
			return nil, parseErr
		}
		task, exists, err = model.GetByUpstreamTaskID(lookupID, createdAtStart, createdAtEnd)
	default:
		return nil, errors.New("task identifier must be a numeric record ID, task_..., or cgt-...")
	}
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errModerationDiagnoseTaskNotFound
	}
	return task, nil
}

func moderationDiagnoseCGTLookupRange(taskID string, now time.Time) (int64, int64, error) {
	matches := moderationDiagnoseCGTIDPattern.FindStringSubmatch(strings.TrimSpace(taskID))
	if len(matches) != 3 {
		return 0, 0, fmt.Errorf("BP task ID must match %s", moderationDiagnoseCGTExpectedIDFormat)
	}

	taskTime, err := time.ParseInLocation(moderationDiagnoseCGTTimestampLayout, matches[1], time.UTC)
	if err != nil {
		return 0, 0, fmt.Errorf("BP task ID must match %s", moderationDiagnoseCGTExpectedIDFormat)
	}

	now = now.UTC()
	if taskTime.After(now.Add(moderationDiagnoseCGTFutureTolerance)) {
		return 0, 0, errors.New("BP task ID timestamp is too far in the future")
	}
	if taskTime.Before(now.Add(-moderationDiagnoseTaskMaxAge)) {
		return 0, 0, errors.New("BP task ID is outside the 14-day moderation lookup window")
	}

	return taskTime.Add(-moderationDiagnoseCGTLookupTolerance).Unix(),
		taskTime.Add(moderationDiagnoseCGTLookupTolerance).Unix(),
		nil
}

func resolveModerationCredential(target *moderationTarget) (*moderationCredential, error) {
	if target == nil {
		return nil, errors.New("moderation target is missing")
	}
	if target.Tenant != nil {
		return resolveModerationCredentialForTenant(target.Tenant)
	}
	return resolveManualModerationCredential(target.ManualCredentialChannelID)
}

func resolveModerationCredentialForTenant(tenant *moderationTenantBoundary) (*moderationCredential, error) {
	if tenant == nil || strings.TrimSpace(tenant.Group) == "" {
		return nil, errors.New("task tenant group is missing")
	}
	candidates, err := loadModerationCredentialCandidates(tenant.Group)
	if err != nil {
		return nil, err
	}

	matching := make([]*moderationCredential, 0, len(candidates))
	for _, candidate := range candidates {
		if tenant.VideoProjectName != "" &&
			strings.TrimSpace(candidate.Config.ProjectName) != tenant.VideoProjectName {
			continue
		}
		matching = append(matching, candidate)
	}
	if len(matching) == 0 {
		return nil, errors.New("asset admin credential channel not found for task tenant")
	}

	reference := matching[0]
	for _, candidate := range matching[1:] {
		if candidate.AccessKey != reference.AccessKey ||
			candidate.SecretKey != reference.SecretKey ||
			strings.TrimSpace(candidate.Config.ProjectName) != strings.TrimSpace(reference.Config.ProjectName) {
			return nil, errors.New("credential mapping ambiguous")
		}
	}
	return reference, nil
}

func resolveManualModerationCredential(channelID int) (*moderationCredential, error) {
	if channelID <= 0 {
		return nil, errors.New("Asset Admin credential channel is required for manual diagnose")
	}
	hasAbility, err := model.ChannelHasEnabledAbilityForModels(channelID, moderationAssetAdminModels)
	if err != nil {
		return nil, err
	}
	if !hasAbility {
		return nil, errors.New("selected credential channel is not a valid asset admin channel")
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	credential, err := moderationCredentialFromChannel(channel, 0)
	if err != nil {
		return nil, errors.New("selected credential channel is not a valid asset admin channel")
	}
	return credential, nil
}

func listModerationCredentialChannels() ([]*moderationCredential, error) {
	return loadModerationCredentialCandidates("")
}

func loadModerationCredentialCandidates(group string) ([]*moderationCredential, error) {
	abilities, err := model.GetEnabledAbilitiesForModels(group, moderationAssetAdminModels)
	if err != nil {
		return nil, err
	}

	priorityByChannel := make(map[int]int64)
	for _, ability := range abilities {
		priority := int64(0)
		if ability.Priority != nil {
			priority = *ability.Priority
		}
		if existing, ok := priorityByChannel[ability.ChannelId]; !ok || priority > existing {
			priorityByChannel[ability.ChannelId] = priority
		}
	}

	candidates := make([]*moderationCredential, 0, len(priorityByChannel))
	for channelID, priority := range priorityByChannel {
		channel, err := model.GetChannelById(channelID, true)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		credential, err := moderationCredentialFromChannel(channel, priority)
		if err != nil {
			continue
		}
		candidates = append(candidates, credential)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].ChannelID < candidates[j].ChannelID
	})
	return candidates, nil
}

func moderationCredentialFromChannel(channel *model.Channel, priority int64) (*moderationCredential, error) {
	if channel == nil || channel.Status != common.ChannelStatusEnabled {
		return nil, errors.New("asset admin channel is disabled")
	}
	accessKey, secretKey, err := parseStrictSeedanceAssetAdminChannelKey(channel)
	if err != nil {
		return nil, err
	}
	config, err := resolveSeedanceModerationConfigFromChannel(channel)
	if err != nil {
		return nil, err
	}
	baseURL, err := seedanceAssetAdminBaseURL(channel.GetBaseURL(), config.Region)
	if err != nil {
		return nil, err
	}
	return &moderationCredential{
		ChannelID: channel.Id,
		AccessKey: accessKey,
		SecretKey: secretKey,
		BaseURL:   baseURL,
		Config:    config,
		Priority:  priority,
	}, nil
}

func parseStrictSeedanceAssetAdminChannelKey(channel *model.Channel) (string, string, error) {
	if channel == nil {
		return "", "", errors.New("selected channel not found")
	}
	key := strings.TrimSpace(channel.Key)
	if key == "" || strings.ContainsAny(key, "\r\n") || strings.HasPrefix(key, "[") {
		return "", "", errors.New("asset admin channel key must use AK|SK format")
	}
	return parseSeedanceAssetAdminKey(key)
}

func buildModerationDiagnoseRequestBody(query moderationDiagnoseQuery) ([]byte, error) {
	return common.Marshal(moderationDiagnoseUpstreamRequest{
		ID:   query.ID,
		Type: query.Type,
	})
}

func buildModerationDiagnoseTargetURL(baseURL string) (string, error) {
	return buildSeedanceAssetTargetURL(baseURL, seedanceAssetAction{
		Name: seedanceModerationDiagnoseActionName,
		Path: "/",
	})
}

func executeModerationDiagnoseQuery(c *gin.Context, query moderationDiagnoseQuery, baseURL string, accessKey string, secretKey string, config *seedanceAssetAdminConfig) moderationDiagnoseAttempt {
	attempt := moderationDiagnoseAttempt{
		ID:   query.ID,
		Type: query.Type,
	}

	requestBody, err := buildModerationDiagnoseRequestBody(query)
	if err != nil {
		attempt.RawError = "failed to encode request body"
		return attempt
	}
	attempt.RawRequestBody = string(requestBody)

	targetURL, err := buildModerationDiagnoseTargetURL(baseURL)
	if err != nil {
		attempt.RawError = "failed to build upstream url"
		return attempt
	}

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		attempt.RawError = "failed to create upstream request"
		return attempt
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	if err = signSeedanceAssetAdminRequest(httpReq, accessKey, secretKey, config.Region); err != nil {
		attempt.RawError = "failed to sign upstream request"
		return attempt
	}

	client, err := service.GetHttpClientWithProxy(config.Proxy)
	if err != nil {
		attempt.RawError = "failed to create proxy client"
		return attempt
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		attempt.RawError = "upstream request failed"
		return attempt
	}
	defer service.CloseResponseBodyGracefully(resp)

	attempt.StatusCode = resp.StatusCode
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		attempt.RawError = "failed to read upstream response"
		return attempt
	}
	responseBody = redactSeedanceAssetResponseBody(responseBody, config.ProjectName)
	attempt.RawResponse = string(responseBody)
	attempt.Success = isModerationDiagnoseSuccessStatus(resp.StatusCode)
	return attempt
}

func isModerationDiagnoseSuccessStatus(statusCode int) bool {
	return (statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices) ||
		statusCode == http.StatusNotFound
}

func moderationDiagnoseResultStatusForAttempt(attempt moderationDiagnoseAttempt) string {
	if attempt.StatusCode == http.StatusNotFound {
		return moderationDiagnoseResultNotFound
	}
	if attempt.Success {
		return moderationDiagnoseResultFound
	}
	return moderationDiagnoseResultRequestFailed
}

func resolveSeedanceModerationConfigFromChannel(channel *model.Channel) (*seedanceAssetAdminConfig, error) {
	if channel == nil {
		return nil, errors.New("selected channel not found")
	}
	config := &seedanceAssetAdminConfig{}

	channelSetting := channel.GetSetting()
	config.ProjectName = strings.TrimSpace(channelSetting.ByteplusProjectName)
	config.Region = seedanceModerationDiagnoseRegion
	config.Proxy = strings.TrimSpace(channelSetting.Proxy)

	channelOtherSettings := channel.GetOtherSettings()
	if config.ProjectName == "" {
		config.ProjectName = strings.TrimSpace(channelOtherSettings.ByteplusProjectName)
	}
	return config, nil
}

func recordModerationDiagnoseAudit(c *gin.Context, req moderationDiagnoseRequest, response *moderationDiagnoseResponse, queryTime time.Duration, diagnoseErr error) {
	adminInfo := map[string]interface{}{
		"operator":              c.GetString("username"),
		"operator_id":           c.GetInt("id"),
		"source_type":           req.SourceType,
		"record_id":             firstNonEmpty(req.RecordID, req.TaskID, req.AssetID, req.ID),
		"credential_channel_id": 0,
		"result_status":         moderationDiagnoseResultValidationError,
		"query_time_ms":         queryTime.Milliseconds(),
		"success":               diagnoseErr == nil,
	}
	if response != nil {
		adminInfo["record_id"] = firstNonEmpty(response.RecordID, fmt.Sprintf("%v", adminInfo["record_id"]))
		adminInfo["credential_channel_id"] = response.credentialChannelID
		adminInfo["result_status"] = response.ResultStatus
		adminInfo["resolved_id"] = response.ResolvedQuery.ID
		adminInfo["resolved_type"] = response.ResolvedQuery.Type
	}
	if diagnoseErr != nil {
		adminInfo["failure"] = diagnoseErr.Error()
	}

	content := fmt.Sprintf(
		"moderation diagnose source_type=%s record_id=%s credential_channel_id=%v success=%t",
		req.SourceType,
		adminInfo["record_id"],
		adminInfo["credential_channel_id"],
		diagnoseErr == nil,
	)
	model.RecordLogWithAdminInfo(c.GetInt("id"), model.LogTypeManage, content, adminInfo)
}

func extractTaskDataString(data []byte, keys []string, preferredPrefix string) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	var payload any
	if err := common.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return findStringByKeys(payload, keys, preferredPrefix)
}

func findStringByKeys(value any, keys []string, preferredPrefix string) string {
	switch typed := value.(type) {
	case map[string]any:
		fallback := ""
		for _, key := range keys {
			if raw, ok := typed[key]; ok {
				if text := stringFromAny(raw); text != "" {
					if preferredPrefix == "" || strings.HasPrefix(text, preferredPrefix) {
						return text
					}
					if fallback == "" {
						fallback = text
					}
				}
			}
		}
		for _, child := range typed {
			if text := findStringByKeys(child, keys, preferredPrefix); text != "" {
				if preferredPrefix == "" || strings.HasPrefix(text, preferredPrefix) {
					return text
				}
				if fallback == "" {
					fallback = text
				}
			}
		}
		return fallback
	case []any:
		for _, child := range typed {
			if text := findStringByKeys(child, keys, preferredPrefix); text != "" {
				return text
			}
		}
	}
	return ""
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func isAllowedModerationDiagnoseType(queryType string) bool {
	switch queryType {
	case moderationDiagnoseTypeTaskID, moderationDiagnoseTypeAssetID, moderationDiagnoseTypeRequestID:
		return true
	default:
		return false
	}
}

func queryExists(queries []moderationDiagnoseQuery, id string, queryType string) bool {
	for _, query := range queries {
		if query.ID == id && query.Type == queryType {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
