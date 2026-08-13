package mediakit

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func mediaKitContext(t *testing.T, body string) (*gin.Context, *relaycommon.RelayInfo, *TaskAdaptor) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
	req, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	clientRequestID, openAIError := relaycommon.ExtractClientRequestIDFromTaskRequest(req)
	require.Nil(t, openAIError)
	if clientRequestID != nil {
		info.ClientRequestID = *clientRequestID
	}
	info.ChannelMeta = &relaycommon.ChannelMeta{IsModelMapped: true}
	info.UpstreamModelName = CanonicalModel
	return c, info, adaptor
}

func validateBody(t *testing.T, body string) (*normalizedRequest, *gin.Context, *relaycommon.RelayInfo, *TaskAdaptor) {
	t.Helper()
	c, info, adaptor := mediaKitContext(t, body)
	require.Nil(t, adaptor.ValidateMappedRequest(c, info))
	normalized, err := normalizedFromContext(c)
	require.NoError(t, err)
	return normalized, c, info, adaptor
}

func TestMediaKitDefaultsAndDoesNotForwardEstimationMetadata(t *testing.T) {
	normalized, c, info, adaptor := validateBody(t, `{
		"model":"lsf-video-enhancement-acme",
		"metadata":{"video_url":"https://example.com/source.mp4?signature=secret","resolution":"1080p","duration":1.01,"client_request_id":"order-1"}
	}`)
	require.Equal(t, "standard", normalized.ToolVersion)
	require.False(t, normalized.FPSProvided)
	require.NotContains(t, normalized.Forward, "tool_version")
	require.NotContains(t, normalized.Forward, "fps")
	require.NotContains(t, normalized.Forward, "client_token")

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	forwarded, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(forwarded, &payload))
	require.NotContains(t, payload, "duration")
	require.NotContains(t, payload, "client_request_id")
	require.NotContains(t, payload, "metadata")
	require.Equal(t, "order-1", payload["client_token"])
	require.Equal(t, "https://example.com/source.mp4?signature=secret", payload["video_url"])
}

func TestMediaKitClientRequestIDMapsToClientToken(t *testing.T) {
	_, c, info, adaptor := validateBody(t, `{
		"model":"lsf-video-enhancement-acme",
		"metadata":{
			"video_url":"https://example.com/source.mp4",
			"tool_version":"standard",
			"scene":"aigc",
			"resolution":"1080p",
			"duration":6,
			"client_request_id":"mediakit-p0-preflight-20260812-02"
		}
	}`)

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	forwarded, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(forwarded, &payload))
	require.Equal(t, "mediakit-p0-preflight-20260812-02", payload["client_token"])
	require.Equal(t, "https://example.com/source.mp4", payload["video_url"])
	require.Equal(t, "standard", payload["tool_version"])
	require.Equal(t, "aigc", payload["scene"])
	require.Equal(t, "1080p", payload["resolution"])
	require.NotContains(t, payload, "duration")
	require.NotContains(t, payload, "client_request_id")
	require.NotContains(t, payload, "metadata")
	require.NotContains(t, payload, "fps")
}

func TestMediaKitRejectsDirectClientTokenOverride(t *testing.T) {
	for _, field := range []string{"client_token", "client-token", "ClientToken"} {
		t.Run(field, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			body := `{"model":"lsf-video-enhancement-acme","` + field + `":"customer-override","metadata":{"video_url":"https://example.com/source.mp4","resolution":"1080p","duration":6,"client_request_id":"request-02"}}`
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
			require.NotNil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))
		})
	}
}

func TestMediaKitClientTokenValidation(t *testing.T) {
	base := map[string]any{
		"video_url":  "https://example.com/source.mp4",
		"resolution": "1080p",
		"duration":   6.0,
	}
	tests := []struct {
		name  string
		value any
	}{
		{name: "too long", value: strings.Repeat("a", 65)},
		{name: "non ascii", value: "request-测试"},
		{name: "non printable", value: "request\n02"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := make(map[string]any, len(base)+1)
			for key, value := range base {
				metadata[key] = value
			}
			metadata["client_request_id"] = test.value
			_, taskErr := normalizeMetadata(metadata)
			require.NotNil(t, taskErr)
		})
	}

	valid := make(map[string]any, len(base)+1)
	for key, value := range base {
		valid[key] = value
	}
	valid["client_request_id"] = strings.Repeat("a", 64)
	normalized, taskErr := normalizeMetadata(valid)
	require.Nil(t, taskErr)
	require.Equal(t, strings.Repeat("a", 64), normalized.ClientToken)
}

func TestMediaKitMissingClientRequestIDPreservesExistingForwarding(t *testing.T) {
	normalized, c, info, adaptor := validateBody(t, `{
		"model":"lsf-video-enhancement-acme",
		"metadata":{"video_url":"https://example.com/source.mp4","resolution":"1080p","duration":6}
	}`)
	require.Empty(t, normalized.ClientToken)
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	forwarded, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(forwarded, &payload))
	require.NotContains(t, payload, "client_token")
	require.NotContains(t, payload, "fps")
}

func TestMediaKitForwardsAllAllowedProvidedFields(t *testing.T) {
	normalized, _, _, _ := validateBody(t, `{
		"model":"lsf-video-enhancement-acme",
		"metadata":{
			"video_url":"http://media.example.com/source",
			"tool_version":"professional",
			"enhance_style":"natural",
			"resolution_limit":2160,
			"fps":30.168,
			"bitrate_level":"high",
			"bitrate":150000,
			"bit_depth":10,
			"client_request_id":"allowlist-forward-map",
			"duration":5.967
		}
	}`)
	require.Equal(t, "professional", normalized.ToolVersion)
	require.Equal(t, "4k", normalized.ResolutionTier)
	require.Equal(t, 30.168, normalized.FPS)
	require.Equal(t, "allowlist-forward-map", normalized.ClientToken)
	require.Equal(t, map[string]any{
		"video_url": "http://media.example.com/source", "tool_version": "professional",
		"enhance_style": "natural", "resolution_limit": 2160,
		"fps": 30.168, "bitrate_level": "high", "bitrate": 150000, "bit_depth": 10,
	}, normalized.Forward)
}

func TestMediaKitStandardCommonSceneAcceptedAndForwarded(t *testing.T) {
	for _, scene := range []string{"common", "ugc", "short_series", "aigc", "old_film"} {
		t.Run(scene, func(t *testing.T) {
			normalized, taskErr := normalizeMetadata(map[string]any{
				"video_url": "https://media.example.com/source", "tool_version": "standard",
				"scene": scene, "resolution": "1080p", "duration": 1.0,
			})
			require.Nil(t, taskErr)
			require.Equal(t, scene, normalized.Forward["scene"])
		})
	}
	_, c, info, adaptor := validateBody(t, `{
		"model":"lsf-video-enhancement-acme",
		"metadata":{"video_url":"https://media.example.com/source","scene":"common","resolution":"1080p","duration":1}
	}`)
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	forwarded, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(forwarded, &payload))
	require.Equal(t, "common", payload["scene"])
}

func TestMediaKitFPSExplicitRange(t *testing.T) {
	for _, test := range []struct {
		name     string
		fps      float64
		accepted bool
	}{
		{name: "below 15", fps: 14.999, accepted: false},
		{name: "15", fps: 15, accepted: true},
		{name: "120", fps: 120, accepted: true},
		{name: "above 120", fps: 120.001, accepted: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			normalized, taskErr := normalizeMetadata(map[string]any{
				"video_url": "https://example.com/a", "resolution": "1080p", "duration": 1.0, "fps": test.fps,
			})
			require.Equal(t, test.accepted, taskErr == nil)
			if test.accepted {
				require.Equal(t, test.fps, normalized.Forward["fps"])
			}
		})
	}
}

func TestMediaKitResolutionLimitContract(t *testing.T) {
	for _, test := range []struct {
		limit      int
		accepted   bool
		resolution string
	}{
		{limit: 1080, accepted: true, resolution: "1080p"},
		{limit: 2160, accepted: true, resolution: "4k"},
		{limit: 1920, accepted: false},
	} {
		t.Run(strconv.Itoa(test.limit), func(t *testing.T) {
			normalized, taskErr := normalizeMetadata(map[string]any{
				"video_url": "https://example.com/a", "resolution_limit": test.limit, "duration": 1.0,
			})
			require.Equal(t, test.accepted, taskErr == nil)
			if test.accepted {
				require.Equal(t, test.resolution, normalized.ResolutionTier)
				require.Equal(t, test.limit, normalized.Forward["resolution_limit"])
			}
		})
	}
	_, c, info, adaptor := validateBody(t, `{
		"model":"lsf-video-enhancement-acme",
		"metadata":{"video_url":"https://media.example.com/source","resolution_limit":1080,"duration":1}
	}`)
	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	forwarded, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(forwarded, &payload))
	require.EqualValues(t, 1080, payload["resolution_limit"])
}

func TestMediaKitDurationValidation(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{name: "missing", metadata: map[string]any{"video_url": "https://example.com/a", "resolution": "1080p"}},
		{name: "string", metadata: map[string]any{"video_url": "https://example.com/a", "resolution": "1080p", "duration": "1"}},
		{name: "zero", metadata: map[string]any{"video_url": "https://example.com/a", "resolution": "1080p", "duration": 0.0}},
		{name: "negative", metadata: map[string]any{"video_url": "https://example.com/a", "resolution": "1080p", "duration": -1.0}},
		{name: "nan", metadata: map[string]any{"video_url": "https://example.com/a", "resolution": "1080p", "duration": math.NaN()}},
		{name: "infinite", metadata: map[string]any{"video_url": "https://example.com/a", "resolution": "1080p", "duration": math.Inf(1)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, taskErr := normalizeMetadata(test.metadata)
			require.NotNil(t, taskErr)
		})
	}
}

func TestMediaKitRejectsInvalidFieldsAndCombinations(t *testing.T) {
	base := map[string]any{"video_url": "https://example.com/a", "resolution": "1080p", "duration": 1.0}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "unknown", mutate: func(m map[string]any) { m["mystery"] = true }},
		{name: "direct client token", mutate: func(m map[string]any) { m["client_token"] = "forbidden" }},
		{name: "source task", mutate: func(m map[string]any) { m["source_task_id"] = "task" }},
		{name: "project", mutate: func(m map[string]any) { m["ProjectName"] = "project" }},
		{name: "both resolutions", mutate: func(m map[string]any) { m["resolution_limit"] = 1080 }},
		{name: "no resolution", mutate: func(m map[string]any) { delete(m, "resolution") }},
		{name: "numeric resolution", mutate: func(m map[string]any) { m["resolution"] = 1080 }},
		{name: "standard bit depth", mutate: func(m map[string]any) { m["bit_depth"] = 8 }},
		{name: "fps above max", mutate: func(m map[string]any) { m["fps"] = 120.001 }},
		{name: "private URL", mutate: func(m map[string]any) { m["video_url"] = "http://127.0.0.1/video" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := make(map[string]any, len(base)+1)
			for key, value := range base {
				metadata[key] = value
			}
			test.mutate(metadata)
			_, taskErr := normalizeMetadata(metadata)
			require.NotNil(t, taskErr)
		})
	}

	professional := map[string]any{"video_url": "https://example.com/a", "resolution": "4k", "duration": 1.0, "tool_version": "professional", "bit_depth": 12, "scene": "old_film"}
	_, taskErr := normalizeMetadata(professional)
	require.NotNil(t, taskErr)
}

func TestMediaKitFailureDetailsAreActionableAndSanitized(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"success":true,
		"status":"failed",
		"result":{"error":{"code":"INVALID_ARGUMENT","message":"resolution 1920 is unsupported; source=https://private.example/input.mp4?token=secret Authorization=Bearer private-token ProjectName=private-project queue_id=private-queue channel=private-channel group=private-group endpoint=https://internal.example/task"}}
	}`))
	require.NoError(t, err)
	require.Equal(t, "INVALID_ARGUMENT", result.UpstreamErrorCode)
	require.Contains(t, result.Reason, "resolution 1920 is unsupported")
	for _, forbidden := range []string{
		"private.example", "private-token", "ProjectName", "private-project", "queue_id", "private-queue",
		"private-channel", "private-group", "internal.example", "Authorization", "Bearer",
	} {
		require.NotContains(t, result.Reason, forbidden)
	}
}

func TestMediaKitRejectsForbiddenTopLevelFields(t *testing.T) {
	for _, field := range []string{"ProjectName", "source_task_id"} {
		t.Run(field, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			body := `{"model":"lsf-video-enhancement-acme","` + field + `":"forbidden","metadata":{"video_url":"https://example.com/a","resolution":"1080p","duration":1}}`
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
			require.NotNil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))
		})
	}
}

func TestMediaKitRequiresServerSideCanonicalMapping(t *testing.T) {
	c, info, adaptor := mediaKitContext(t, `{"model":"mediakit-video-enhancement","metadata":{"video_url":"https://example.com/a","resolution":"1080p","duration":1}}`)
	info.ChannelMeta = &relaycommon.ChannelMeta{IsModelMapped: false}
	info.UpstreamModelName = CanonicalModel
	require.NotNil(t, adaptor.ValidateMappedRequest(c, info))
}

func TestMediaKitParsesTopLevelCompletedWithNestedResultOutput(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResultForTask(
		&model.Task{Status: model.TaskStatusInProgress, Progress: "30%"},
		[]byte(`{
			"success":true,
			"status":"completed",
			"result":{
				"video_url":"https://signed.example/result?signature=secret",
				"duration":10,
				"fps":30,
				"resolution":"1080p",
				"tool_version":"standard"
			}
		}`),
	)
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusSuccess), result.Status)
	require.Equal(t, "100%", result.Progress)
	require.Equal(t, "https://signed.example/result?signature=secret", result.Url)
	require.Equal(t, 10.0, result.Duration)
	require.Equal(t, 30.0, result.FPS)
	require.Equal(t, "1080p", result.Resolution)
	require.Equal(t, "standard", result.ToolVersion)
}

func TestMediaKitParsesTopLevelNonSuccessStatusesWithoutNestedStatus(t *testing.T) {
	tests := []struct {
		name             string
		upstreamStatus   string
		expectedStatus   model.TaskStatus
		expectedProgress string
	}{
		{name: "processing", upstreamStatus: "processing", expectedStatus: model.TaskStatusInProgress, expectedProgress: "30%"},
		{name: "failed", upstreamStatus: "failed", expectedStatus: model.TaskStatusFailure, expectedProgress: "100%"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `{"success":true,"status":"` + test.upstreamStatus + `","result":{"task_id":"provider-task-placeholder"}}`
			result, err := (&TaskAdaptor{}).ParseTaskResultForTask(
				&model.Task{Status: model.TaskStatusInProgress, Progress: "30%"},
				[]byte(body),
			)
			require.NoError(t, err)
			require.Equal(t, string(test.expectedStatus), result.Status)
			require.Equal(t, test.expectedProgress, result.Progress)
		})
	}
}

func TestMediaKitUnknownStatusRetainsCurrentTaskState(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResultForTask(&model.Task{Status: model.TaskStatusInProgress, Progress: "30%"}, []byte(`{"success":true,"result":{"status":"provider_new_state"}}`))
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusInProgress), result.Status)
	require.Equal(t, "30%", result.Progress)
}

func TestMediaKitMissingStatusRetainsCurrentTaskState(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResultForTask(
		&model.Task{Status: model.TaskStatusInProgress, Progress: "30%"},
		[]byte(`{"success":true,"result":{"task_id":"provider-task-placeholder"}}`),
	)
	require.NoError(t, err)
	require.Equal(t, string(model.TaskStatusInProgress), result.Status)
	require.Equal(t, "30%", result.Progress)
}

func TestMediaKitMissingExpiryUsesStableTwentyFourHourDefault(t *testing.T) {
	finishTime := time.Now().Add(-time.Hour).Unix()
	task := &model.Task{Status: model.TaskStatusSuccess, FinishTime: finishTime, Data: []byte(`{"status":"SUCCESS"}`)}
	result, err := (&TaskAdaptor{}).ParseTaskResultForTask(task, []byte(`{"success":true,"result":{"status":"completed","video_url":"https://signed.example/result"}}`))
	require.NoError(t, err)
	require.Equal(t, finishTime+int64(24*time.Hour/time.Second), result.ExpiresAt)
}

func TestMediaKitCreateDoesNotFollowRedirect(t *testing.T) {
	service.InitHttpClient()
	var redirectedCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectedCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	c, info, adaptor := mediaKitContext(t, `{"model":"alias","metadata":{"video_url":"https://example.com/a","resolution":"1080p","duration":1}}`)
	info.ChannelBaseUrl = source.URL
	info.ApiKey = "placeholder-key"
	resp, err := adaptor.DoRequest(c, info, bytes.NewBufferString(`{"safe":true}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	_ = resp.Body.Close()
	require.Zero(t, redirectedCalls.Load())
}

func TestMediaKitUncertainCreateResponseIsSanitized(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(`{"success":true,"result":{"video_url":"https://signed.example/result?secret=value"}}`))}
	_, taskData, submitResponse, taskErr := (&TaskAdaptor{}).DoResponseNoWrite(c, response, &relaycommon.RelayInfo{})
	require.NotNil(t, taskErr)
	require.Equal(t, "mediakit_submission_uncertain", taskErr.Code)
	require.NotContains(t, taskErr.Message, "signed.example")
	require.Nil(t, taskData)
	require.Nil(t, submitResponse)
}

func TestMediaKitSubmissionErrorDetailIsActionableAndSanitized(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	response := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewBufferString(`{
		"success":false,
		"error":{"code":"INVALID_ARGUMENT","message":"bitrate is out of range; source=https://private.example/input.mp4 Authorization=Bearer private-token queue_id=private-queue"}
	}`))}
	_, taskData, submitResponse, taskErr := (&TaskAdaptor{}).DoResponseNoWrite(c, response, &relaycommon.RelayInfo{})
	require.NotNil(t, taskErr)
	require.Equal(t, "INVALID_ARGUMENT", taskErr.Code)
	require.Contains(t, taskErr.Message, "bitrate is out of range")
	for _, forbidden := range []string{"private.example", "private-token", "private-queue", "Authorization", "Bearer", "queue_id"} {
		require.NotContains(t, taskErr.Message, forbidden)
	}
	require.Nil(t, taskData)
	require.Nil(t, submitResponse)
}
