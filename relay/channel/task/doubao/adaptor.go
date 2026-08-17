package doubao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
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
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content,omitempty"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	OutputFormat          string         `json:"output_format,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
	Ratio       string         `json:"ratio,omitempty"`
	Duration    *dto.IntValue  `json:"duration,omitempty"`
	Frames      *dto.IntValue  `json:"frames,omitempty"`
	Seed        *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark   *dto.BoolValue `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL     string `json:"video_url"`
		LastFrameURL string `json:"last_frame_url"`
	} `json:"content"`
	Seed            int    `json:"seed"`
	Resolution      string `json:"resolution"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	FramesPerSecond int    `json:"framespersecond"`
	ServiceTier     string `json:"service_tier"`
	Tools           []struct {
		Type string `json:"type"`
	} `json:"tools"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		ToolUsage        struct {
			WebSearch int `json:"web_search"`
		} `json:"tool_usage"`
	} `json:"usage"`
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Retryable *bool  `json:"retryable,omitempty"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

const (
	seedanceMiniMaxDurationSeconds = 15
	seedance25MaxDurationSeconds   = 30
	seedanceMiniOutputFPS          = 24
)

const (
	seedance25ReservationFPS            = 24
	seedance25ReservationMaxPixels480p  = 428544
	seedance25ReservationMaxPixels720p  = 927408
	seedance25ReservationMaxPixels1080p = 2086876
)

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	if taskErr := validateSeedance25Request(c, info, &req); taskErr != nil {
		return taskErr
	}
	return validateSeedanceRequestResolution(req, req.Model, info.OriginModelName)
}

// ValidateMappedRequest enforces model-specific resolution constraints after
// tenant aliases have been resolved to the final upstream model.
func (a *TaskAdaptor) ValidateMappedRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if relaycommon.IsSeedance25OriginAlias(info.OriginModelName) {
		mappedModelName, err := seedance25NormalizedMappedModel(info)
		if err != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("server-side model mapping is not configured"), "UPSTREAM_MAPPING_MISSING", http.StatusServiceUnavailable)
		}
		info.UpstreamModelName = mappedModelName
	}
	return validateSeedanceRequestResolution(req, info.OriginModelName, info.UpstreamModelName)
}

func validateSeedanceRequestResolution(req relaycommon.TaskSubmitReq, originModelName, upstreamModelName string) *dto.TaskError {
	if _, ok, err := ResolveSeedanceIntlBilling(originModelName, upstreamModelName, req.Metadata); ok && err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request_error", http.StatusBadRequest)
	}
	return nil
}

func seedance25NormalizedMappedModel(info *relaycommon.RelayInfo) (string, error) {
	if info == nil || info.ChannelMeta == nil {
		return "", fmt.Errorf("server-side model mapping is not configured")
	}
	mappedModelName := strings.TrimSpace(info.UpstreamModelName)
	publicModelName := strings.TrimSpace(info.OriginModelName)
	if !info.IsModelMapped || mappedModelName == "" || strings.EqualFold(mappedModelName, publicModelName) {
		return "", fmt.Errorf("server-side model mapping is not configured")
	}
	return mappedModelName, nil
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateBilling returns Seedance 2.0 international billing ratios.
// For Seedance 2.0 models, ModelRatio should be configured as the no-video
// 480p/720p base price. This resolver then applies video-input and resolution
// adjustments using BytePlus international pricing.
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	if billingCtx, ok, err := ResolveSeedanceIntlBilling(info.OriginModelName, info.UpstreamModelName, req.Metadata); ok {
		if err != nil {
			return nil
		}
		if billingCtx.Family == seedanceBillingFamily25 {
			info.PriceData.BillingFamily = billingCtx.Family
			info.PriceData.BillingRuleVersion = billingCtx.RuleVersion
			numerator, denominator, ok := seedance25BillingRatioFraction(billingCtx.ResolutionGroup, billingCtx.InputType)
			if !ok {
				return nil
			}
			info.PriceData.OtherRatioNumerator = numerator
			info.PriceData.OtherRatioDenominator = denominator
		}
		return map[string]float64{
			"seedance_intl_billing": billingCtx.Ratio,
		}
	}

	if hasVideoInMetadata(req.Metadata) {
		if ratio, ok := GetVideoInputRatio(info.OriginModelName); ok {
			return map[string]float64{"video_input": ratio}
		}
	}
	return nil
}

// EstimatePrechargeQuota raises Mini and Seedance 2.5 reservations to the
// existing output-token formula when it exceeds the shared task precharge.
func (a *TaskAdaptor) EstimatePrechargeQuota(c *gin.Context, info *relaycommon.RelayInfo) (int, bool) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return 0, false
	}
	billingCtx, ok, err := ResolveSeedanceIntlBilling(info.OriginModelName, info.UpstreamModelName, req.Metadata)
	if err != nil || !ok || (billingCtx.Family != seedanceBillingFamilyMini && billingCtx.Family != seedanceBillingFamily25) {
		return 0, false
	}
	if info.PriceData.UsePrice || info.PriceData.ModelRatio <= 0 {
		return 0, false
	}
	if billingCtx.Family == seedanceBillingFamily25 {
		expectedOtherRatio, ok := seedance25ExpectedOtherRatio(
			info.PriceData.OtherRatioNumerator,
			info.PriceData.OtherRatioDenominator,
		)
		otherRatio, hasOtherRatio := info.PriceData.OtherRatios["seedance_intl_billing"]
		if !seedance25ApprovedModelRatio(info.PriceData.ModelRatio) ||
			!seedance25FinitePositive(info.PriceData.GroupRatioInfo.GroupRatio) ||
			!ok || !hasOtherRatio || !seedance25FinitePositive(otherRatio) ||
			otherRatio != expectedOtherRatio || !seedance25FinitePositive(billingCtx.Ratio) ||
			billingCtx.Ratio != expectedOtherRatio {
			return 0, false
		}
	}

	maxDuration := seedanceMiniMaxDurationSeconds
	if billingCtx.Family == seedanceBillingFamily25 {
		maxDuration = seedance25MaxDurationSeconds
	}
	outputDuration := seedanceOutputDurationSeconds(req, maxDuration)
	inputDuration := 0
	if billingCtx.InputType == seedanceBillingInputVideo {
		// Current LSF submit path does not inspect remote reference-video
		// duration. Use the family maximum as a conservative input ceiling.
		inputDuration = maxDuration
	}
	width, height := seedanceMiniOutputDimensions(billingCtx.Resolution)
	estimatedTokens := math.Ceil(float64(inputDuration+outputDuration) * float64(width) * float64(height) * seedanceMiniOutputFPS / 1024)
	if billingCtx.Family == seedanceBillingFamily25 {
		maxPixels, ok := seedance25ReservationPixelCeiling(billingCtx.Resolution)
		if !ok {
			return 0, false
		}
		estimatedTokens = math.Ceil(float64(inputDuration+outputDuration) * float64(maxPixels) * seedance25ReservationFPS / 1024)
	}
	quota := math.Ceil(estimatedTokens * info.PriceData.ModelRatio * info.PriceData.GroupRatioInfo.GroupRatio * billingCtx.Ratio)
	if math.IsNaN(quota) || math.IsInf(quota, 0) || quota <= 0 || quota > float64(^uint(0)>>1) {
		return 0, false
	}
	return int(quota), true
}

func seedanceMiniOutputDimensions(resolution string) (int, int) {
	switch resolution {
	case "480p":
		return 854, 480
	default:
		return 1280, 720
	}
}

func seedance25ReservationPixelCeiling(resolution string) (int, bool) {
	switch resolution {
	case "480p":
		return seedance25ReservationMaxPixels480p, true
	case "720p":
		return seedance25ReservationMaxPixels720p, true
	case "1080p":
		return seedance25ReservationMaxPixels1080p, true
	default:
		return 0, false
	}
}

func seedanceOutputDurationSeconds(req relaycommon.TaskSubmitReq, maxDuration int) int {
	if duration, ok := metadataInt(req.Metadata, "duration"); ok {
		return normalizeSeedanceDuration(duration, maxDuration)
	}
	if req.Duration != 0 {
		return normalizeSeedanceDuration(req.Duration, maxDuration)
	}
	if seconds, err := strconv.Atoi(req.Seconds); err == nil && seconds != 0 {
		return normalizeSeedanceDuration(seconds, maxDuration)
	}
	return maxDuration
}

func normalizeSeedanceDuration(duration int, maxDuration int) int {
	if duration == -1 {
		return maxDuration
	}
	if duration > 0 {
		return duration
	}
	return maxDuration
}

func metadataInt(metadata map[string]interface{}, key string) (int, bool) {
	if metadata == nil {
		return 0, false
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		if i, err := strconv.Atoi(v.String()); err == nil {
			return i, true
		}
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	case dto.IntValue:
		return int(v), true
	case *dto.IntValue:
		if v != nil {
			return int(*v), true
		}
	}
	return 0, false
}

// hasVideoInMetadata 直接检查 metadata 的 content 数组是否包含 video_url 条目，
// 避免构建完整的上游 requestPayload。
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return false
	}
	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range contentSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemMap["type"] == "video_url" {
			return true
		}
		if _, has := itemMap["video_url"]; has {
			return true
		}
	}
	return false
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	if relaycommon.IsSeedance25OriginAlias(info.OriginModelName) {
		mappedModelName, err := seedance25NormalizedMappedModel(info)
		if err != nil {
			return nil, err
		}
		info.UpstreamModelName = mappedModelName
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if billingCtx, ok, err := ResolveSeedanceIntlBilling(info.OriginModelName, info.UpstreamModelName, req.Metadata); ok {
		if err != nil {
			return nil, err
		}
		if metadataString(req.Metadata, "resolution") != "" {
			body.Resolution = billingCtx.Resolution
		}
	}
	if info.IsModelMapped {
		mappedModelName := info.UpstreamModelName
		body.Model = mappedModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	taskID, taskData, submitResponse, taskErr := a.DoResponseNoWrite(c, resp, info)
	if taskErr != nil {
		return "", nil, taskErr
	}
	c.JSON(submitResponse.StatusCode, submitResponse.Body)
	return taskID, taskData, nil
}

func (a *TaskAdaptor) DoResponseNoWrite(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, response *channel.TaskSubmitResponse, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		if relaycommon.IsSeedance25OriginAlias(info.OriginModelName) {
			taskErr = service.TaskErrorWrapper(errors.Wrap(err, "invalid upstream submit response"), "unmarshal_response_body_failed", http.StatusInternalServerError)
		} else {
			taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		}
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	taskData = responseBody
	return dResp.ID, taskData, &channel.TaskSubmitResponse{
		StatusCode: http.StatusOK,
		Body:       ov,
	}, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	// Add images if present
	if req.HasImage() {
		for _, imgURL := range req.Images {
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &MediaURL{
					URL: imgURL,
				},
			})
		}
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	if sec, _ := strconv.Atoi(req.Seconds); sec > 0 {
		r.Duration = lo.ToPtr(dto.IntValue(sec))
	}

	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	r.Content = append(r.Content, ContentItem{
		Type: "text",
		Text: req.Prompt,
	})

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}
	return newDoubaoTaskInfo(
		resTask.Status,
		resTask.Content.VideoURL,
		resTask.Content.LastFrameURL,
		resTask.Error.Code,
		resTask.Error.Message,
		resTask.Usage.CompletionTokens,
		resTask.Usage.CompletionTokens > 0,
		resTask.Usage.TotalTokens,
	), nil
}

func newDoubaoTaskInfo(status, videoURL, lastFrameURL, errorCode, errorMessage string, completionTokens int, completionTokensValid bool, totalTokens int) *relaycommon.TaskInfo {
	taskResult := relaycommon.TaskInfo{
		Code:                  0,
		CompletionTokens:      completionTokens,
		CompletionTokensValid: completionTokensValid,
		TotalTokens:           totalTokens,
		UpstreamErrorCode:     errorCode,
		LastFrameURL:          lastFrameURL,
	}

	// Map Doubao status to internal status
	switch status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = videoURL
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = errorMessage
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	if !isSeedance25Task(originTask) {
		openAIVideo.SetMetadata("url", dResp.Content.VideoURL)
	} else if originTask.Status == model.TaskStatusSuccess && strings.TrimSpace(dResp.Content.VideoURL) != "" {
		openAIVideo.SetMetadata("url", strings.TrimSpace(dResp.Content.VideoURL))
	}
	if isSeedance25Task(originTask) && originTask.Status == model.TaskStatusSuccess && strings.TrimSpace(dResp.Content.LastFrameURL) != "" {
		openAIVideo.SetMetadata("last_frame_url", strings.TrimSpace(dResp.Content.LastFrameURL))
	}
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if dResp.Usage.CompletionTokens > 0 || dResp.Usage.TotalTokens > 0 {
		openAIVideo.Usage = &dto.OpenAIVideoUsage{
			CompletionTokens: dResp.Usage.CompletionTokens,
			TotalTokens:      dResp.Usage.TotalTokens,
		}
	}

	if dResp.Status == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message:   dResp.Error.Message,
			Code:      dResp.Error.Code,
			Retryable: dResp.Error.Retryable,
		}
	}

	return common.Marshal(openAIVideo)
}
