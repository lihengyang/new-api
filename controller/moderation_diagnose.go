package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	moderationDiagnoseSourceVideoTask    = "video_task"
	moderationDiagnoseSourceLibraryAsset = "library_asset"
	moderationDiagnoseSourceManual       = "manual"

	moderationDiagnoseTypeTaskID    = "task_id"
	moderationDiagnoseTypeAssetID   = "asset_id"
	moderationDiagnoseTypeRequestID = "request_id"

	seedanceModerationDiagnoseActionName = "GetAIGCModerationResult"
)

type moderationDiagnoseRequest struct {
	SourceType string `json:"source_type"`
	RecordID   string `json:"record_id"`
	TaskID     string `json:"task_id"`
	AssetID    string `json:"asset_id"`
	RequestID  string `json:"request_id"`
	ID         string `json:"id"`
	Type       string `json:"type"`
	ChannelID  int    `json:"channel_id"`
}

type moderationDiagnoseQuery struct {
	ID   string `json:"id"`
	Type string `json:"type"`
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
	SourceType       string                      `json:"source_type"`
	RecordID         string                      `json:"record_id,omitempty"`
	ChannelID        int                         `json:"channel_id,omitempty"`
	ResolvedQuery    moderationDiagnoseQuery     `json:"resolved_query"`
	AttemptedQueries []moderationDiagnoseAttempt `json:"attempted_queries"`
	RawRequestBody   string                      `json:"raw_request_body,omitempty"`
	RawResponse      string                      `json:"raw_response,omitempty"`
	RawError         string                      `json:"raw_error,omitempty"`
	Resolved         map[string]interface{}      `json:"resolved,omitempty"`
}

func ModerationDiagnose(c *gin.Context) {
	start := time.Now()
	req := moderationDiagnoseRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, errors.New("invalid JSON request body"))
		return
	}

	req.SourceType = strings.TrimSpace(req.SourceType)
	req.RecordID = strings.TrimSpace(req.RecordID)
	req.TaskID = strings.TrimSpace(req.TaskID)
	req.AssetID = strings.TrimSpace(req.AssetID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ID = strings.TrimSpace(req.ID)
	req.Type = strings.TrimSpace(req.Type)

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

func runModerationDiagnose(c *gin.Context, req moderationDiagnoseRequest) (*moderationDiagnoseResponse, error) {
	if req.SourceType == "" {
		return nil, errors.New("source_type is required")
	}

	channelID, recordID, resolved, queries, err := resolveModerationDiagnoseQueries(req)
	if err != nil {
		return &moderationDiagnoseResponse{
			SourceType: req.SourceType,
			RecordID:   firstNonEmpty(recordID, req.RecordID, req.TaskID, req.AssetID),
			ChannelID:  channelID,
		}, err
	}
	if len(queries) == 0 {
		return &moderationDiagnoseResponse{
			SourceType: req.SourceType,
			RecordID:   recordID,
			ChannelID:  channelID,
			Resolved:   resolved,
		}, errors.New("no moderation diagnose query could be resolved")
	}
	if channelID == 0 {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, errors.New("channel_id is required")
	}

	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, err
	}

	config, err := resolveSeedanceModerationConfigFromChannel(channel)
	if err != nil {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, err
	}
	if strings.TrimSpace(config.ProjectName) == "" {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, errors.New("byteplus_project_name is required on the selected channel")
	}
	if strings.TrimSpace(config.Region) == "" {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, errors.New("byteplus_region is required on the selected channel")
	}

	apiKey, err := firstSeedanceChannelKey(channel)
	if err != nil {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, err
	}
	accessKey, secretKey, err := parseSeedanceAssetAdminKey(apiKey)
	if err != nil {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, err
	}

	baseURL, err := seedanceAssetAdminBaseURL(channel.GetBaseURL(), config.Region)
	if err != nil {
		return &moderationDiagnoseResponse{
			SourceType:    req.SourceType,
			RecordID:      recordID,
			ResolvedQuery: queries[0],
			ChannelID:     channelID,
			Resolved:      resolved,
		}, err
	}

	result := &moderationDiagnoseResponse{
		SourceType:    req.SourceType,
		RecordID:      recordID,
		ChannelID:     channelID,
		ResolvedQuery: queries[0],
		Resolved:      resolved,
	}
	for _, query := range queries {
		attempt := executeModerationDiagnoseQuery(c, query, baseURL, accessKey, secretKey, config)
		result.AttemptedQueries = append(result.AttemptedQueries, attempt)
		result.RawRequestBody = attempt.RawRequestBody
		result.RawResponse = attempt.RawResponse
		result.RawError = attempt.RawError
		if attempt.Success {
			result.ResolvedQuery = moderationDiagnoseQuery{ID: attempt.ID, Type: attempt.Type}
			return result, nil
		}
	}

	return result, errors.New("moderation diagnose upstream query failed")
}

func resolveModerationDiagnoseQueries(req moderationDiagnoseRequest) (int, string, map[string]interface{}, []moderationDiagnoseQuery, error) {
	switch req.SourceType {
	case moderationDiagnoseSourceVideoTask:
		return resolveModerationDiagnoseVideoTask(req)
	case moderationDiagnoseSourceLibraryAsset:
		return resolveModerationDiagnoseLibraryAsset(req)
	case moderationDiagnoseSourceManual:
		return resolveModerationDiagnoseManual(req)
	default:
		return 0, "", nil, nil, fmt.Errorf("unsupported source_type %q", req.SourceType)
	}
}

func resolveModerationDiagnoseVideoTask(req moderationDiagnoseRequest) (int, string, map[string]interface{}, []moderationDiagnoseQuery, error) {
	lookupID := firstNonEmpty(req.RecordID, req.TaskID, req.ID)
	if lookupID == "" {
		return 0, "", nil, nil, errors.New("task record id or task_id is required")
	}

	task, err := findModerationDiagnoseTask(lookupID)
	if err != nil {
		return 0, lookupID, nil, nil, err
	}

	channelID := req.ChannelID
	if channelID == 0 {
		channelID = task.ChannelId
	}

	upstreamGenerationID := extractTaskDataString(task.Data, []string{"id", "Id", "ID"}, "cgt-")
	if upstreamGenerationID != "" && !strings.HasPrefix(upstreamGenerationID, "cgt-") {
		upstreamGenerationID = ""
	}
	if upstreamGenerationID == "" {
		upstreamGenerationID = strings.TrimSpace(task.PrivateData.UpstreamTaskID)
	}
	if upstreamGenerationID != "" && !strings.HasPrefix(upstreamGenerationID, "cgt-") {
		upstreamGenerationID = ""
	}
	requestID := extractTaskDataString(task.Data, []string{"request_id", "requestId", "requestID", "RequestId", "RequestID"}, "")

	queries := make([]moderationDiagnoseQuery, 0, 2)
	if upstreamGenerationID != "" {
		queries = append(queries, moderationDiagnoseQuery{ID: upstreamGenerationID, Type: moderationDiagnoseTypeTaskID})
	}
	if requestID != "" && !queryExists(queries, requestID, moderationDiagnoseTypeRequestID) {
		queries = append(queries, moderationDiagnoseQuery{ID: requestID, Type: moderationDiagnoseTypeRequestID})
	}

	resolved := map[string]interface{}{
		"task_record_id":         task.ID,
		"platform_task_id":       task.TaskID,
		"upstream_generation_id": upstreamGenerationID,
		"request_id":             requestID,
		"channel_id":             channelID,
	}
	return channelID, strconv.FormatInt(task.ID, 10), resolved, queries, nil
}

func resolveModerationDiagnoseLibraryAsset(req moderationDiagnoseRequest) (int, string, map[string]interface{}, []moderationDiagnoseQuery, error) {
	upstreamAssetID := firstNonEmpty(req.AssetID, req.RecordID, req.ID)
	if upstreamAssetID == "" {
		return req.ChannelID, req.RecordID, nil, nil, errors.New("asset record id or upstream asset_id is required")
	}
	if req.ChannelID == 0 {
		return req.ChannelID, upstreamAssetID, nil, nil, errors.New("channel_id is required for library_asset diagnose")
	}

	queries := []moderationDiagnoseQuery{{ID: upstreamAssetID, Type: moderationDiagnoseTypeAssetID}}
	if req.RequestID != "" {
		queries = append(queries, moderationDiagnoseQuery{ID: req.RequestID, Type: moderationDiagnoseTypeRequestID})
	}
	resolved := map[string]interface{}{
		"upstream_asset_id": upstreamAssetID,
		"request_id":        req.RequestID,
		"channel_id":        req.ChannelID,
	}
	return req.ChannelID, upstreamAssetID, resolved, queries, nil
}

func resolveModerationDiagnoseManual(req moderationDiagnoseRequest) (int, string, map[string]interface{}, []moderationDiagnoseQuery, error) {
	if req.ChannelID == 0 {
		return 0, req.RecordID, nil, nil, errors.New("channel_id is required for manual diagnose")
	}
	if req.ID == "" {
		return req.ChannelID, req.RecordID, nil, nil, errors.New("Id is required for manual diagnose")
	}
	queryType := strings.ToLower(strings.TrimSpace(req.Type))
	if !isAllowedModerationDiagnoseType(queryType) {
		return req.ChannelID, req.RecordID, nil, nil, errors.New("Type must be one of task_id, request_id, or asset_id")
	}
	resolved := map[string]interface{}{
		"id":         req.ID,
		"type":       queryType,
		"channel_id": req.ChannelID,
	}
	return req.ChannelID, firstNonEmpty(req.RecordID, req.ID), resolved, []moderationDiagnoseQuery{{ID: req.ID, Type: queryType}}, nil
}

func findModerationDiagnoseTask(lookupID string) (*model.Task, error) {
	if numericID, err := strconv.ParseInt(lookupID, 10, 64); err == nil && numericID > 0 {
		var task model.Task
		if err := model.DB.Where("id = ?", numericID).First(&task).Error; err == nil {
			return &task, nil
		}
	}

	task, exists, err := model.GetByOnlyTaskId(lookupID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func executeModerationDiagnoseQuery(c *gin.Context, query moderationDiagnoseQuery, baseURL string, accessKey string, secretKey string, config *seedanceAssetAdminConfig) moderationDiagnoseAttempt {
	attempt := moderationDiagnoseAttempt{
		ID:   query.ID,
		Type: query.Type,
	}

	payload := map[string]any{
		"ProjectName": config.ProjectName,
		"Id":          query.ID,
		"Type":        query.Type,
	}
	requestBody, err := common.Marshal(payload)
	if err != nil {
		attempt.RawError = "failed to encode request body"
		return attempt
	}
	attempt.RawRequestBody = string(redactSeedanceAssetResponseBody(requestBody, config.ProjectName))

	targetURL, err := buildSeedanceAssetTargetURL(baseURL, seedanceAssetAction{
		Name: seedanceModerationDiagnoseActionName,
		Path: "/",
	})
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
	attempt.Success = resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	return attempt
}

func resolveSeedanceModerationConfigFromChannel(channel *model.Channel) (*seedanceAssetAdminConfig, error) {
	if channel == nil {
		return nil, errors.New("selected channel not found")
	}
	config := &seedanceAssetAdminConfig{}

	channelSetting := channel.GetSetting()
	config.ProjectName = strings.TrimSpace(channelSetting.ByteplusProjectName)
	config.AssetGroupType = strings.TrimSpace(channelSetting.ByteplusAssetGroupType)
	config.Region = strings.TrimSpace(channelSetting.ByteplusRegion)
	config.Proxy = strings.TrimSpace(channelSetting.Proxy)

	channelOtherSettings := channel.GetOtherSettings()
	if config.ProjectName == "" {
		config.ProjectName = strings.TrimSpace(channelOtherSettings.ByteplusProjectName)
	}
	if config.AssetGroupType == "" {
		config.AssetGroupType = strings.TrimSpace(channelOtherSettings.ByteplusAssetGroupType)
	}
	if config.Region == "" {
		config.Region = strings.TrimSpace(channelOtherSettings.ByteplusRegion)
	}
	return config, nil
}

func firstSeedanceChannelKey(channel *model.Channel) (string, error) {
	if channel == nil {
		return "", errors.New("selected channel not found")
	}
	keys := channel.GetKeys()
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			return key, nil
		}
	}
	return "", errors.New("selected channel has no configured key")
}

func recordModerationDiagnoseAudit(c *gin.Context, req moderationDiagnoseRequest, response *moderationDiagnoseResponse, queryTime time.Duration, diagnoseErr error) {
	adminInfo := map[string]interface{}{
		"operator":      c.GetString("username"),
		"operator_id":   c.GetInt("id"),
		"source_type":   req.SourceType,
		"record_id":     firstNonEmpty(req.RecordID, req.TaskID, req.AssetID, req.ID),
		"channel_id":    req.ChannelID,
		"query_time_ms": queryTime.Milliseconds(),
		"success":       diagnoseErr == nil,
	}
	if response != nil {
		adminInfo["record_id"] = firstNonEmpty(response.RecordID, fmt.Sprintf("%v", adminInfo["record_id"]))
		adminInfo["channel_id"] = response.ChannelID
		adminInfo["resolved_id"] = response.ResolvedQuery.ID
		adminInfo["resolved_type"] = response.ResolvedQuery.Type
	}
	if diagnoseErr != nil {
		adminInfo["failure"] = diagnoseErr.Error()
	}

	content := fmt.Sprintf(
		"moderation diagnose source_type=%s record_id=%s channel_id=%v success=%t",
		req.SourceType,
		adminInfo["record_id"],
		adminInfo["channel_id"],
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
