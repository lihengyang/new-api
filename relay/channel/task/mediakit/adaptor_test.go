package mediakit

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
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

	reader, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	forwarded, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(forwarded, &payload))
	require.NotContains(t, payload, "duration")
	require.NotContains(t, payload, "client_request_id")
	require.Equal(t, "https://example.com/source.mp4?signature=secret", payload["video_url"])
}

func TestMediaKitForwardsAllAllowedProvidedFields(t *testing.T) {
	normalized, _, _, _ := validateBody(t, `{
		"model":"lsf-video-enhancement-acme",
		"metadata":{
			"video_url":"http://media.example.com/source",
			"tool_version":"professional",
			"scene":"ugc",
			"enhance_style":"natural",
			"resolution_limit":2160,
			"fps":30.168,
			"bitrate_level":"high",
			"bitrate":150000,
			"bit_depth":10,
			"duration":5.967
		}
	}`)
	require.Equal(t, "professional", normalized.ToolVersion)
	require.Equal(t, "4k", normalized.ResolutionTier)
	require.Equal(t, 30.168, normalized.FPS)
	require.Equal(t, map[string]any{
		"video_url": "http://media.example.com/source", "tool_version": "professional",
		"scene": "ugc", "enhance_style": "natural", "resolution_limit": 2160,
		"fps": 30.168, "bitrate_level": "high", "bitrate": 150000, "bit_depth": 10,
	}, normalized.Forward)
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
		{name: "source task", mutate: func(m map[string]any) { m["source_task_id"] = "task" }},
		{name: "project", mutate: func(m map[string]any) { m["ProjectName"] = "project" }},
		{name: "both resolutions", mutate: func(m map[string]any) { m["resolution_limit"] = 1080 }},
		{name: "no resolution", mutate: func(m map[string]any) { delete(m, "resolution") }},
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
	require.Nil(t, taskErr)
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

func TestMediaKitUnknownStatusRetainsCurrentTaskState(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResultForTask(&model.Task{Status: model.TaskStatusInProgress, Progress: "30%"}, []byte(`{"success":true,"result":{"status":"provider_new_state"}}`))
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
