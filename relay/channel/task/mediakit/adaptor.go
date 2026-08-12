package mediakit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	CanonicalModel = "mediakit-video-enhancement"
	ChannelName    = "BytePlus AI MediaKit"

	normalizedContextKey = "mediakit_video_enhancement_normalized_request"
)

var modelList = []string{CanonicalModel}

type normalizedRequest struct {
	VideoURL       string
	ToolVersion    string
	ResolutionTier string
	Duration       float64
	FPS            float64
	FPSProvided    bool
	ClientToken    string
	Forward        map[string]any
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
}

func (a *TaskAdaptor) Init(_ *relaycommon.RelayInfo) {}

func invalidRequest(message string) *dto.TaskError {
	return service.TaskErrorWrapperLocal(errors.New(message), "invalid_request", http.StatusBadRequest)
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var raw map[string]any
	if err := common.UnmarshalBodyReusable(c, &raw); err != nil {
		return invalidRequest("invalid JSON request")
	}
	for key := range raw {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
		if normalized == "projectname" || normalized == "sourcetaskid" || normalized == "clienttoken" {
			return invalidRequest("unsupported MediaKit request field")
		}
	}

	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return invalidRequest("invalid task request")
	}
	if strings.TrimSpace(req.Model) == "" {
		return invalidRequest("model is required")
	}
	info.Action = constant.TaskActionGenerate
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) ValidateMappedRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if !info.IsModelMapped || strings.TrimSpace(info.UpstreamModelName) != CanonicalModel {
		return invalidRequest("MediaKit model must map to the canonical video enhancement model")
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return invalidRequest("task request is unavailable")
	}
	normalized, validationErr := normalizeMetadata(req.Metadata)
	if validationErr != nil {
		return validationErr
	}
	if normalized.ClientToken != info.ClientRequestID {
		return invalidRequest("metadata.client_request_id is inconsistent with the validated request")
	}
	c.Set(normalizedContextKey, normalized)
	return nil
}

func normalizeMetadata(metadata map[string]any) (*normalizedRequest, *dto.TaskError) {
	if metadata == nil {
		return nil, invalidRequest("metadata is required")
	}
	allowed := map[string]struct{}{
		"video_url": {}, "tool_version": {}, "scene": {}, "enhance_style": {},
		"resolution": {}, "resolution_limit": {}, "fps": {}, "bitrate_level": {},
		"bitrate": {}, "bit_depth": {}, "client_request_id": {}, "duration": {},
	}
	for key := range metadata {
		if _, ok := allowed[key]; !ok {
			return nil, invalidRequest("unsupported MediaKit metadata field: " + key)
		}
	}

	videoURL, ok := requiredString(metadata, "video_url")
	if !ok || !isPublicHTTPURL(videoURL) {
		return nil, invalidRequest("metadata.video_url must be a public HTTP/HTTPS URL")
	}
	duration, ok := numberValue(metadata, "duration")
	if !ok || !finitePositive(duration) {
		return nil, invalidRequest("metadata.duration must be a finite number greater than 0")
	}

	toolVersion := "standard"
	toolVersionProvided := false
	if raw, exists := metadata["tool_version"]; exists {
		value, valid := stringValue(raw)
		if !valid {
			return nil, invalidRequest("metadata.tool_version must be a string")
		}
		toolVersion = strings.ToLower(value)
		toolVersionProvided = true
	}
	if toolVersion != "standard" && toolVersion != "professional" {
		return nil, invalidRequest("metadata.tool_version must be standard or professional")
	}

	resolutionRaw, hasResolution := metadata["resolution"]
	limitRaw, hasLimit := metadata["resolution_limit"]
	if hasResolution == hasLimit {
		return nil, invalidRequest("exactly one of metadata.resolution or metadata.resolution_limit is required")
	}
	resolutionTier := ""
	forward := map[string]any{"video_url": videoURL}
	if hasResolution {
		resolution, valid := stringValue(resolutionRaw)
		if !valid {
			return nil, invalidRequest("metadata.resolution must be a string")
		}
		resolution = strings.ToLower(resolution)
		if resolution != "1080p" && resolution != "4k" {
			return nil, invalidRequest("metadata.resolution must be 1080p or 4k")
		}
		resolutionTier = resolution
		forward["resolution"] = resolution
	} else {
		limit, valid := integerValue(limitRaw)
		if !valid || (limit != 1080 && limit != 2160) {
			return nil, invalidRequest("metadata.resolution_limit must be 1080 or 2160")
		}
		if limit == 1080 {
			resolutionTier = "1080p"
		} else {
			resolutionTier = "4k"
		}
		forward["resolution_limit"] = limit
	}
	if toolVersionProvided {
		forward["tool_version"] = toolVersion
	}

	if raw, exists := metadata["scene"]; exists {
		value, valid := stringValue(raw)
		if !valid || !oneOf(value, "aigc", "short_series", "ugc", "old_film") {
			return nil, invalidRequest("metadata.scene is invalid")
		}
		forward["scene"] = value
	}
	if raw, exists := metadata["enhance_style"]; exists {
		value, valid := stringValue(raw)
		if !valid || !oneOf(value, "hd", "natural") {
			return nil, invalidRequest("metadata.enhance_style must be hd or natural")
		}
		forward["enhance_style"] = value
	}

	fps := float64(0)
	fpsProvided := false
	if raw, exists := metadata["fps"]; exists {
		value, valid := numeric(raw)
		if !valid || !finitePositive(value) || value > 120 {
			return nil, invalidRequest("metadata.fps must be a finite number greater than 0 and at most 120")
		}
		fps, fpsProvided = value, true
		forward["fps"] = value
	}
	if raw, exists := metadata["bitrate_level"]; exists {
		value, valid := stringValue(raw)
		if !valid || !oneOf(value, "low", "medium", "high") {
			return nil, invalidRequest("metadata.bitrate_level must be low, medium, or high")
		}
		forward["bitrate_level"] = value
	}
	if raw, exists := metadata["bitrate"]; exists {
		value, valid := integerValue(raw)
		if !valid || value < 10 || value > 150000 {
			return nil, invalidRequest("metadata.bitrate must be an integer from 10 to 150000")
		}
		forward["bitrate"] = value
	}
	if raw, exists := metadata["bit_depth"]; exists {
		value, valid := integerValue(raw)
		if !valid || (value != 8 && value != 10 && value != 12) {
			return nil, invalidRequest("metadata.bit_depth must be 8, 10, or 12")
		}
		if toolVersion != "professional" {
			return nil, invalidRequest("metadata.bit_depth is supported only for professional tool_version")
		}
		forward["bit_depth"] = value
	}

	clientToken := ""
	if raw, exists := metadata["client_request_id"]; exists {
		value, valid := stringValue(raw)
		if !valid || !validMediaKitClientToken(value) {
			return nil, invalidRequest("metadata.client_request_id must contain 1-64 printable ASCII characters")
		}
		clientToken = value
	}

	return &normalizedRequest{
		VideoURL: videoURL, ToolVersion: toolVersion, ResolutionTier: resolutionTier,
		Duration: duration, FPS: fps, FPSProvided: fpsProvided,
		ClientToken: clientToken, Forward: forward,
	}, nil
}

func normalizedFromContext(c *gin.Context) (*normalizedRequest, error) {
	value, ok := c.Get(normalizedContextKey)
	if !ok {
		return nil, errors.New("normalized MediaKit request is unavailable")
	}
	req, ok := value.(*normalizedRequest)
	if !ok || req == nil {
		return nil, errors.New("normalized MediaKit request is invalid")
	}
	return req, nil
}

func requiredString(metadata map[string]any, key string) (string, bool) {
	raw, ok := metadata[key]
	if !ok {
		return "", false
	}
	return stringValue(raw)
}

func stringValue(raw any) (string, bool) {
	value, ok := raw.(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func numeric(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case int32:
		return float64(value), true
	default:
		return 0, false
	}
}

func numberValue(metadata map[string]any, key string) (float64, bool) {
	raw, ok := metadata[key]
	if !ok {
		return 0, false
	}
	return numeric(raw)
}

func integerValue(raw any) (int, bool) {
	value, ok := numeric(raw)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
		return 0, false
	}
	if value < float64(math.MinInt) || value > float64(math.MaxInt) {
		return 0, false
	}
	return int(value), true
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validMediaKitClientToken(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func isPublicHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap()
		if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast() {
			return false
		}
	}
	return true
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return strings.TrimRight(info.ChannelBaseUrl, "/") + "/api/v1/tools/enhance-video", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, _ *relaycommon.RelayInfo) (io.Reader, error) {
	normalized, err := normalizedFromContext(c)
	if err != nil {
		return nil, err
	}
	payload := make(map[string]any, len(normalized.Forward)+1)
	for key, value := range normalized.Forward {
		payload[key] = value
	}
	if normalized.ClientToken != "" {
		payload["client_token"] = normalized.ClientToken
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	requestURL, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, requestURL, requestBody)
	if err != nil {
		return nil, errors.New("failed to build MediaKit request")
	}
	if err := a.BuildRequestHeader(c, req, info); err != nil {
		return nil, errors.New("failed to build MediaKit request headers")
	}
	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, errors.New("failed to initialize MediaKit client")
	}
	if client == nil {
		client = http.DefaultClient
	}
	// Do not let net/http replay the create POST on 307/308 redirects. A create
	// response of any kind is handled exactly once by the relay controller.
	singleShotClient := *client
	singleShotClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := singleShotClient.Do(req)
	if err != nil {
		return nil, errors.New("MediaKit task submission failed")
	}
	return resp, nil
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	taskID, taskData, response, taskErr := a.DoResponseNoWrite(c, resp, info)
	if taskErr == nil {
		c.JSON(response.StatusCode, response.Body)
	}
	return taskID, taskData, taskErr
}

func (a *TaskAdaptor) DoResponseNoWrite(_ *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *channel.TaskSubmitResponse, *dto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return "", nil, nil, service.TaskErrorWrapper(errors.New("failed to read upstream submission response"), "read_response_body_failed", http.StatusBadGateway)
	}
	var envelope map[string]any
	if err := common.Unmarshal(body, &envelope); err != nil {
		return "", nil, nil, service.TaskErrorWrapper(errors.New("invalid upstream submission response"), "invalid_response", http.StatusBadGateway)
	}
	if success, ok := envelope["success"].(bool); ok && !success {
		return "", nil, nil, service.TaskErrorWrapper(errors.New("upstream task submission failed"), "upstream_submission_failed", http.StatusBadGateway)
	}
	result := nestedMap(envelope, "result")
	if result == nil {
		result = nestedMap(envelope, "output")
	}
	taskID := firstString(result, "task_id", "id")
	if taskID == "" {
		taskID = firstString(envelope, "task_id", "id")
	}
	if taskID == "" {
		return "", nil, nil, service.TaskErrorWrapper(errors.New("upstream submission result was uncertain"), "mediakit_submission_uncertain", http.StatusBadGateway)
	}
	taskData, _ := common.Marshal(map[string]any{"status": "queued"})
	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	video.CreatedAt = time.Now().Unix()
	return taskID, taskData, &channel.TaskSubmitResponse{StatusCode: http.StatusOK, Body: video}, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, errors.New("invalid task_id")
	}
	requestURL := strings.TrimRight(baseURL, "/") + "/api/v1/tasks/" + url.PathEscape(taskID)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, errors.New("failed to initialize upstream client")
	}
	if client == nil {
		client = http.DefaultClient
	}
	queryClient := *client
	queryClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := queryClient.Do(req)
	if err != nil {
		return nil, errors.New("MediaKit task query failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("MediaKit task query failed with status %d", resp.StatusCode)
	}
	return resp, nil
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	return parseTaskResult(body)
}

func (a *TaskAdaptor) ParseTaskResultForTask(task *model.Task, body []byte) (*relaycommon.TaskInfo, error) {
	result, err := parseTaskResult(body)
	if err == nil && result != nil && task != nil {
		if result.Status == "" {
			result.Status = string(task.Status)
			result.Progress = task.Progress
		}
		if result.Url != "" && result.ExpiresAt == 0 {
			var stored map[string]any
			if common.Unmarshal(task.Data, &stored) == nil {
				result.ExpiresAt = parseExpiresAt(stored["expires_at"])
			}
			if result.ExpiresAt == 0 && task.FinishTime > 0 {
				result.ExpiresAt = task.FinishTime + int64(24*time.Hour/time.Second)
			}
			if result.ExpiresAt == 0 {
				result.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
			}
		}
	}
	return result, err
}

func parseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var envelope map[string]any
	if err := common.Unmarshal(body, &envelope); err != nil {
		return nil, errors.New("invalid MediaKit task response")
	}
	if success, ok := envelope["success"].(bool); ok && !success {
		return nil, errors.New("MediaKit task query returned an error")
	}
	result := nestedMap(envelope, "result")
	if result == nil {
		result = nestedMap(envelope, "output")
	}
	if result == nil {
		result = envelope
	}
	info := &relaycommon.TaskInfo{
		TaskID:      firstString(result, "task_id", "id"),
		Url:         firstString(result, "video_url", "url"),
		Resolution:  firstString(result, "resolution"),
		ToolVersion: strings.ToLower(firstString(result, "tool_version")),
		ExpiresAt:   parseExpiresAt(result["expires_at"]),
	}
	info.Duration, _ = numeric(result["duration"])
	info.FPS, _ = numeric(result["fps"])
	status := strings.ToLower(firstString(envelope, "status"))
	if status == "" {
		status = strings.ToLower(firstString(result, "status"))
	}
	switch status {
	case "pending", "submitted", "queued", "created", "waiting":
		info.Status, info.Progress = string(model.TaskStatusQueued), taskcommon.ProgressQueued
	case "processing", "running", "in_progress":
		info.Status, info.Progress = string(model.TaskStatusInProgress), taskcommon.ProgressInProgress
	case "completed", "complete", "succeeded", "success":
		info.Status, info.Progress = string(model.TaskStatusSuccess), taskcommon.ProgressComplete
	case "failed", "failure", "error", "cancelled", "canceled":
		info.Status, info.Progress = string(model.TaskStatusFailure), taskcommon.ProgressComplete
		info.Reason = "video enhancement failed"
	default:
		// An undocumented provider status is deliberately left unmapped. The
		// task-aware parser retains the current non-terminal state.
	}
	return info, nil
}

func nestedMap(value map[string]any, key string) map[string]any {
	if value == nil {
		return nil
	}
	nested, _ := value[key].(map[string]any)
	return nested
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := value[key]; ok {
			if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func parseExpiresAt(raw any) int64 {
	if value, ok := numeric(raw); ok && finitePositive(value) {
		if value > 1e12 {
			value /= 1000
		}
		return int64(value)
	}
	text, ok := raw.(string)
	if !ok {
		return 0
	}
	if unix, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64); err == nil {
		if unix > 1e12 {
			unix /= 1000
		}
		return unix
	}
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(text)); err == nil {
		return parsed.Unix()
	}
	return 0
}

func (a *TaskAdaptor) GetModelList() []string { return modelList }
func (a *TaskAdaptor) GetChannelName() string { return ChannelName }
