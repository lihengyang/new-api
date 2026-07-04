package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedAudioNormalizePreservesZeroAudioConfig(t *testing.T) {
	body := `{
		"model":"lsf-seed-audio-1.0-tenant-a",
		"input":"hello",
		"metadata":{
			"client_request_id":"req_zero",
			"sample_rate":0,
			"speech_rate":0,
			"loudness_rate":0,
			"pitch_rate":0,
			"references":[
				{"type":"audio_url","url":"https://cdn.example.com/a.wav"},
				{"type":"audio_url","audio_url":"https://cdn.example.com/b.wav"}
			]
		}
	}`
	req, bodyMap := seedAudioTestRequest(t, body)

	normalized, apiErr := buildSeedAudioNormalizedRequest(req, bodyMap)
	require.Nil(t, apiErr)
	require.Equal(t, "mp3", normalized.Format)
	require.Equal(t, "audio_url", normalized.ReferenceMode)
	require.Len(t, normalized.References, 2)
	require.Equal(t, "https://cdn.example.com/a.wav", normalized.References[0].URL)
	require.Equal(t, "https://cdn.example.com/b.wav", normalized.References[1].URL)
	require.NotNil(t, normalized.SampleRate)
	require.NotNil(t, normalized.SpeechRate)
	require.NotNil(t, normalized.LoudnessRate)
	require.NotNil(t, normalized.PitchRate)
	require.Equal(t, 0, *normalized.SampleRate)
	require.Equal(t, 0.0, *normalized.SpeechRate)
	require.Equal(t, 0.0, *normalized.LoudnessRate)
	require.Equal(t, 0.0, *normalized.PitchRate)

	upstreamReq := normalized.toUpstreamRequest()
	require.Equal(t, "hello", upstreamReq.TextPrompt)
	require.Len(t, upstreamReq.References, 2)
	require.Equal(t, "https://cdn.example.com/a.wav", upstreamReq.References[0].AudioURL)
	require.Empty(t, upstreamReq.References[0].ImageURL)
	require.Equal(t, "https://cdn.example.com/b.wav", upstreamReq.References[1].AudioURL)
	require.Empty(t, upstreamReq.References[1].ImageURL)

	upstreamBody, err := common.Marshal(upstreamReq)
	require.NoError(t, err)
	upstreamJSON := string(upstreamBody)
	require.Contains(t, upstreamJSON, `"model":"seed-audio-1.0"`)
	require.Contains(t, upstreamJSON, `"sample_rate":0`)
	require.Contains(t, upstreamJSON, `"speech_rate":0`)
	require.Contains(t, upstreamJSON, `"loudness_rate":0`)
	require.Contains(t, upstreamJSON, `"pitch_rate":0`)
}

func TestSeedAudioTextPromptLimitUsesCombinedTrimmedRunes(t *testing.T) {
	tests := []struct {
		name         string
		instructions string
		input        string
		wantRunes    int
		wantCode     string
	}{
		{
			name:      "2048 chinese characters pass",
			input:     strings.Repeat("界", 2048),
			wantRunes: 2048,
		},
		{
			name:      "2049 chinese characters pass",
			input:     strings.Repeat("界", 2049),
			wantRunes: 2049,
		},
		{
			name:      "2999 chinese characters pass",
			input:     strings.Repeat("界", 2999),
			wantRunes: 2999,
		},
		{
			name:      "3000 chinese characters pass",
			input:     strings.Repeat("界", 3000),
			wantRunes: 3000,
		},
		{
			name:     "3001 chinese characters fail",
			input:    strings.Repeat("界", 3001),
			wantCode: "seed_audio_input_too_long",
		},
		{
			name:         "combined instructions newline input exact limit passes",
			instructions: strings.Repeat("你", 1000),
			input:        strings.Repeat("好", 1999),
			wantRunes:    3000,
		},
		{
			name:         "combined instructions newline input over limit fails",
			instructions: strings.Repeat("你", 1000),
			input:        strings.Repeat("好", 2000),
			wantCode:     "seed_audio_input_too_long",
		},
		{
			name:         "trimmed unicode prompt uses final upstream text_prompt length",
			instructions: "  " + strings.Repeat("声", 1000) + "  ",
			input:        "\n" + strings.Repeat("音", 1999) + "\n",
			wantRunes:    3000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, apiErr := buildSeedAudioNormalizedRequest(&dto.AudioRequest{
				Model:        "lsf-seed-audio-1.0-tenant-a",
				Instructions: tt.instructions,
				Input:        tt.input,
			}, map[string]interface{}{})

			if tt.wantCode != "" {
				require.Nil(t, normalized)
				require.NotNil(t, apiErr)
				require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
				require.Equal(t, tt.wantCode, apiErr.ToOpenAIError().Code)
				return
			}
			require.Nil(t, apiErr)
			require.Equal(t, tt.wantRunes, utf8.RuneCountInString(normalized.TextPrompt))
		})
	}
}

func TestSeedAudioUpstreamReferenceMapping(t *testing.T) {
	audioReq, audioBody := seedAudioTestRequest(t, `{
		"model":"lsf-seed-audio-1.0-tenant-a",
		"input":"Please match @Audio1",
		"metadata":{"references":[{"type":"audio_url","url":"https://cdn.example.com/reference.mp3"}]}
	}`)
	normalizedAudio, apiErr := buildSeedAudioNormalizedRequest(audioReq, audioBody)
	require.Nil(t, apiErr)
	upstreamAudio := normalizedAudio.toUpstreamRequest()
	require.Equal(t, "Please match @Audio1", upstreamAudio.TextPrompt)
	require.Len(t, upstreamAudio.References, 1)
	require.Equal(t, "https://cdn.example.com/reference.mp3", upstreamAudio.References[0].AudioURL)
	require.Empty(t, upstreamAudio.References[0].ImageURL)

	imageReq, imageBody := seedAudioTestRequest(t, `{
		"model":"lsf-seed-audio-1.0-tenant-a",
		"input":"Please match image style",
		"metadata":{"references":[{"type":"image_url","url":"https://cdn.example.com/reference.png"}]}
	}`)
	normalizedImage, apiErr := buildSeedAudioNormalizedRequest(imageReq, imageBody)
	require.Nil(t, apiErr)
	upstreamImage := normalizedImage.toUpstreamRequest()
	require.Equal(t, "Please match image style", upstreamImage.TextPrompt)
	require.Len(t, upstreamImage.References, 1)
	require.Empty(t, upstreamImage.References[0].AudioURL)
	require.Equal(t, "https://cdn.example.com/reference.png", upstreamImage.References[0].ImageURL)
}

func TestSeedAudioUpstreamReferenceFetchErrorsMapToInvalidReferenceURL(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "explicit url download failure",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":{"message":"failed to download url"}}`,
		},
		{
			name:       "audio resource download failure",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":{"message":"failed to download audio resource"}}`,
		},
		{
			name:       "media access forbidden",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"message":"cannot fetch media: forbidden"}}`,
		},
		{
			name:       "reference timeout",
			statusCode: http.StatusGatewayTimeout,
			body:       `{"error":{"message":"reference fetch timed out"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := seedAudioUpstreamStatusError(tt.statusCode, []byte(tt.body))
			require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			require.Equal(t, "invalid_reference_url", apiErr.ToOpenAIError().Code)
		})
	}
}

func TestSeedAudioUpstreamServiceErrorsRemainUpstreamError(t *testing.T) {
	apiErr := seedAudioUpstreamStatusError(http.StatusServiceUnavailable, []byte(`{"error":{"message":"upstream overloaded"}}`))
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Equal(t, "seed_audio_upstream_error", apiErr.ToOpenAIError().Code)

	apiErr = seedAudioUpstreamStatusError(http.StatusBadRequest, []byte(`{"error":{"message":"invalid audio_config format"}}`))
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, "seed_audio_upstream_error", apiErr.ToOpenAIError().Code)
}

func TestSeedAudioUpstreamDiagnosticsClassifyFailures(t *testing.T) {
	t.Run("4xx json", func(t *testing.T) {
		body := []byte(`{"ResponseMetadata":{"RequestId":"req-json-4xx"},"error":{"message":"bad request"}}`)
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type":  []string{"application/json"},
				"X-Tt-Logid":    []string{"log-4xx"},
				"X-Tt-Trace-Id": []string{"trace-4xx"},
				"Request-Id":    []string{"request-4xx"},
				"X-Request-Id":  []string{"x-request-4xx"},
			},
		}
		diagnostics := seedAudioBuildUpstreamDiagnostics(resp, body, 23*time.Millisecond)
		apiErr := seedAudioUpstreamStatusErrorWithDiagnostics(resp.StatusCode, body, diagnostics)

		require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		require.Equal(t, "json", diagnostics.ContentTypeClass)
		require.Equal(t, "4xx", diagnostics.ResponseClass)
		require.Equal(t, "upstream_http_4xx_json", diagnostics.ErrorClass)
		require.Equal(t, "log-4xx", diagnostics.XTTLogID)
		require.Equal(t, "trace-4xx", diagnostics.XTTTraceID)
		require.Equal(t, "request-4xx", diagnostics.RequestID)
		require.Equal(t, "x-request-4xx", diagnostics.XRequestID)
		require.Equal(t, "req-json-4xx", diagnostics.ResponseMetadataRequestID)
		require.Equal(t, int64(23), diagnostics.LatencyMS)
		require.Equal(t, "le_1kb", diagnostics.BodySizeBucket)
	})

	t.Run("5xx json", func(t *testing.T) {
		body := []byte(`{"ResponseMetadata":{"RequestId":"req-json-5xx"},"error":{"message":"service unavailable"}}`)
		resp := &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		}
		diagnostics := seedAudioBuildUpstreamDiagnostics(resp, body, 41*time.Millisecond)
		apiErr := seedAudioUpstreamStatusErrorWithDiagnostics(resp.StatusCode, body, diagnostics)

		require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
		require.Equal(t, "5xx", diagnostics.ResponseClass)
		require.Equal(t, "upstream_http_5xx_json", diagnostics.ErrorClass)
		require.Equal(t, "req-json-5xx", diagnostics.ResponseMetadataRequestID)
	})

	t.Run("invalid json", func(t *testing.T) {
		body := []byte(`{"error":`)
		resp := &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}
		diagnostics := seedAudioBuildUpstreamDiagnostics(resp, body, 10*time.Millisecond)
		apiErr := seedAudioUpstreamStatusErrorWithDiagnostics(resp.StatusCode, body, diagnostics)

		require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
		require.True(t, diagnostics.InvalidJSON)
		require.Equal(t, "upstream_http_5xx_invalid_json", diagnostics.ErrorClass)
	})

	t.Run("html cloudflare marker", func(t *testing.T) {
		body := []byte(`<!doctype html><html><title>Attention Required!</title><body>Cloudflare cf-ray</body></html>`)
		resp := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     http.Header{"Content-Type": []string{"text/html"}},
		}
		diagnostics := seedAudioBuildUpstreamDiagnostics(resp, body, 15*time.Millisecond)
		apiErr := seedAudioUpstreamStatusErrorWithDiagnostics(resp.StatusCode, body, diagnostics)
		diagnosticsJSON := seedAudioDiagnosticsJSON(diagnostics)

		require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
		require.Equal(t, "html", diagnostics.ContentTypeClass)
		require.True(t, diagnostics.HTML)
		require.True(t, diagnostics.CloudflareLike)
		require.Equal(t, "upstream_http_5xx_cloudflare", diagnostics.ErrorClass)
		require.NotContains(t, diagnosticsJSON, "Attention Required")
		require.NotContains(t, diagnosticsJSON, "cf-ray")
	})

	t.Run("timeout", func(t *testing.T) {
		diagnostics := seedAudioUpstreamDiagnosticsForError(nil, 1500*time.Millisecond, "timeout")

		require.Equal(t, "timeout", diagnostics.ResponseClass)
		require.Equal(t, "timeout", diagnostics.ErrorClass)
		require.Equal(t, int64(1500), diagnostics.LatencyMS)
		require.Equal(t, "empty", diagnostics.BodySizeBucket)
	})
}

func TestSeedAudioDispatchInvalidJSONDiagnostics(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":`))
	}))
	defer upstream.Close()

	c := newSeedAudioIdempotencyTestContext()
	result, diagnostics, apiErr := seedAudioDispatchUpstream(c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: upstream.URL,
			ApiKey:         "test-upstream-key",
		},
	}, &seedAudioNormalizedRequest{
		TextPrompt:    "hello",
		Format:        seedAudioDefaultFormat,
		ReferenceMode: "text_only",
	})

	require.Nil(t, result)
	require.NotNil(t, apiErr)
	require.NotNil(t, diagnostics)
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.Equal(t, "seed_audio_upstream_error", apiErr.ToOpenAIError().Code)
	require.Equal(t, "2xx", diagnostics.ResponseClass)
	require.True(t, diagnostics.InvalidJSON)
	require.Equal(t, "invalid_json", diagnostics.ErrorClass)
}

func TestSeedAudioValidationRejectsP0ExcludedInputs(t *testing.T) {
	tooLong := strings.Repeat("界", seedAudioMaxTextRunes+1)
	tests := []struct {
		name       string
		body       string
		statusCode int
		code       string
	}{
		{
			name:       "project name",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"project_name":"hidden"}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "audio data",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"audio_data":"abc"}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "base64 variant field",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"audio_base64":"abc"}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "generic data url",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"references":[{"type":"audio_url","url":"data:application/octet-stream;base64,abc"}]}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "voice",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","voice":"clone-me"}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "non mp3",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","response_format":"pcm"}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "too long",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"` + tooLong + `"}`,
			statusCode: http.StatusBadRequest,
			code:       "seed_audio_input_too_long",
		},
		{
			name:       "private ip reference",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"references":[{"type":"audio_url","url":"https://10.0.0.8/a.wav"}]}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_reference_url",
		},
		{
			name:       "localhost reference",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"references":[{"type":"audio_url","url":"https://localhost/a.wav"}]}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_reference_url",
		},
		{
			name:       "non https reference",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"references":[{"type":"audio_url","url":"http://cdn.example.com/a.wav"}]}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_reference_url",
		},
		{
			name:       "mixed references",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"references":[{"type":"audio_url","url":"https://cdn.example.com/a.wav"},{"type":"image_url","url":"https://cdn.example.com/a.png"}]}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "too many audio references",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"references":[{"type":"audio_url","url":"https://cdn.example.com/1.wav"},{"type":"audio_url","url":"https://cdn.example.com/2.wav"},{"type":"audio_url","url":"https://cdn.example.com/3.wav"},{"type":"audio_url","url":"https://cdn.example.com/4.wav"}]}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
		{
			name:       "too many image references",
			body:       `{"model":"lsf-seed-audio-1.0-a","input":"hello","metadata":{"references":[{"type":"image_url","url":"https://cdn.example.com/1.png"},{"type":"image_url","url":"https://cdn.example.com/2.png"}]}}`,
			statusCode: http.StatusBadRequest,
			code:       "invalid_request_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, bodyMap := seedAudioTestRequest(t, tt.body)
			_, apiErr := buildSeedAudioNormalizedRequest(req, bodyMap)
			require.NotNil(t, apiErr)
			require.Equal(t, tt.statusCode, apiErr.StatusCode)
			require.Equal(t, tt.code, apiErr.ToOpenAIError().Code)
		})
	}
}

func TestSeedAudioTooLongRequestStopsBeforeIdempotencyBillingAndUpstream(t *testing.T) {
	setupRelayTaskTestDB(t)
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	body := `{
		"model":"lsf-seed-audio-1.0-tenant-a",
		"input":"` + strings.Repeat("界", seedAudioMaxTextRunes+1) + `",
		"metadata":{"client_request_id":"req_too_long"}
	}`
	c, info, _ := newSeedAudioHelperTestContext(t, body, upstream.URL)

	apiErr := SeedAudioHelper(c, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "seed_audio_input_too_long", apiErr.ToOpenAIError().Code)
	require.EqualValues(t, 0, atomic.LoadInt32(&upstreamCalls))
	require.Nil(t, info.Billing)

	var count int64
	require.NoError(t, model.DB.Model(&model.SeedAudioIdempotency{}).
		Where("client_request_id = ?", "req_too_long").
		Count(&count).Error)
	require.Zero(t, count)

	var user model.User
	require.NoError(t, model.DB.First(&user, 1001).Error)
	require.Equal(t, 1_000_000, user.Quota)
}

func TestSeedAudioUpstreamFailureRefundsAndStoresDiagnostics(t *testing.T) {
	setupRelayTaskTestDB(t)
	var upstreamCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Tt-Logid", "log-refund-test")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-refund-test"},"error":{"message":"temporary service failure"}}`))
	}))
	defer upstream.Close()

	body := `{
		"model":"lsf-seed-audio-1.0-tenant-a",
		"input":"hello",
		"metadata":{"client_request_id":"req_refund_failure"}
	}`
	c, info, _ := newSeedAudioHelperTestContext(t, body, upstream.URL)

	apiErr := SeedAudioHelper(c, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Equal(t, "seed_audio_upstream_error", apiErr.ToOpenAIError().Code)
	require.EqualValues(t, 1, atomic.LoadInt32(&upstreamCalls))
	require.NotNil(t, info.Billing)

	require.Eventually(t, func() bool {
		var user model.User
		if err := model.DB.First(&user, 1001).Error; err != nil {
			return false
		}
		return user.Quota == 1_000_000
	}, 2*time.Second, 10*time.Millisecond)

	var row model.SeedAudioIdempotency
	require.NoError(t, model.DB.Where("client_request_id = ?", "req_refund_failure").First(&row).Error)
	require.Equal(t, model.SeedAudioIdempotencyStatusFailed, row.Status)
	require.Equal(t, "seed_audio_upstream_error", row.ErrorCode)
	require.Equal(t, http.StatusServiceUnavailable, row.ErrorStatusCode)
	require.NotEmpty(t, row.ErrorDiagnostics)

	var diagnostics dto.SeedAudioUpstreamDiagnostics
	require.NoError(t, common.Unmarshal([]byte(row.ErrorDiagnostics), &diagnostics))
	require.Equal(t, http.StatusInternalServerError, diagnostics.UpstreamHTTPStatus)
	require.Equal(t, "5xx", diagnostics.ResponseClass)
	require.Equal(t, "json", diagnostics.ContentTypeClass)
	require.Equal(t, "upstream_http_5xx_json", diagnostics.ErrorClass)
	require.Equal(t, "log-refund-test", diagnostics.XTTLogID)
	require.Equal(t, "req-refund-test", diagnostics.ResponseMetadataRequestID)
	require.Equal(t, "le_1kb", diagnostics.BodySizeBucket)
	require.NotContains(t, row.ErrorDiagnostics, "temporary service failure")
	require.NotContains(t, row.ErrorDiagnostics, "hello")
	require.NotContains(t, row.ErrorDiagnostics, "https://")
}

func TestSeedAudioQuotaMathUsesOriginalDurationAndGroupRatio(t *testing.T) {
	require.Equal(t, 150000, seedAudioPrechargeQuota(1))
	require.Equal(t, 225000, seedAudioPrechargeQuota(1.5))
	require.Equal(t, 15425, seedAudioActualQuota(12.34, 1))
	require.Equal(t, 23137, seedAudioActualQuota(12.34, 1.5))
	require.Equal(t, 0, seedAudioActualQuota(0, 1))
}

func TestSeedAudioTimeoutClamp(t *testing.T) {
	t.Setenv("SEED_AUDIO_UPSTREAM_TIMEOUT_SECONDS", "10")
	require.Equal(t, 60*time.Second, seedAudioTimeout())

	t.Setenv("SEED_AUDIO_UPSTREAM_TIMEOUT_SECONDS", "150")
	require.Equal(t, 150*time.Second, seedAudioTimeout())

	t.Setenv("SEED_AUDIO_UPSTREAM_TIMEOUT_SECONDS", "999")
	require.Equal(t, 180*time.Second, seedAudioTimeout())
}

func TestSeedAudioResponseExtractionAndReplayRedaction(t *testing.T) {
	upstream := map[string]interface{}{
		"data": map[string]interface{}{
			"audio": map[string]interface{}{
				"url":               "https://tmp.example.com/audio.mp3",
				"duration":          7.5,
				"original_duration": "8.25",
				"audio_data":        "base64-secret",
			},
		},
	}
	require.Equal(t, "https://tmp.example.com/audio.mp3", seedAudioFindStringField(upstream, "url"))
	require.Equal(t, 7.5, seedAudioFindFloatField(upstream, "duration"))
	require.Equal(t, 8.25, seedAudioFindFloatField(upstream, "original_duration"))

	response := seedAudioResponseFromRecord("lsf-seed-audio-1.0-tenant-a", dto.SeedAudioIdempotencyRecord{
		ResponseID:       "aud_cached",
		TemporaryURL:     "https://tmp.example.com/audio.mp3",
		URLExpiresAt:     1234,
		Duration:         7.5,
		OriginalDuration: 8.25,
		CreatedAt:        1000,
		ActualQuota:      10312,
	}, "req_cached")
	require.Equal(t, "req_cached", response.Metadata["client_request_id"])
	body, err := common.Marshal(response)
	require.NoError(t, err)
	bodyString := string(body)
	require.Contains(t, bodyString, "lsf-seed-audio-1.0-tenant-a")
	require.Contains(t, bodyString, `"client_request_id":"req_cached"`)
	require.NotContains(t, bodyString, `"model":"`+seedAudioUpstreamModel+`"`)
	require.NotContains(t, bodyString, "channel_id")
	require.NotContains(t, bodyString, "group")
	require.NotContains(t, bodyString, "ProjectName")
	require.NotContains(t, bodyString, "X-Api-Key")
	require.NotContains(t, bodyString, "base64-secret")
}

func TestSeedAudioRequestHMACIsStableAndSecretBacked(t *testing.T) {
	req, bodyMap := seedAudioTestRequest(t, `{
		"model":"lsf-seed-audio-1.0-a",
		"input":"hello",
		"metadata":{"client_request_id":"req_1","references":[{"type":"audio_url","url":"https://cdn.example.com/a.wav"}]}
	}`)
	normalized, apiErr := buildSeedAudioNormalizedRequest(req, bodyMap)
	require.Nil(t, apiErr)

	first, err := seedAudioRequestHMAC(normalized)
	require.NoError(t, err)
	second, err := seedAudioRequestHMAC(normalized)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Len(t, first, 64)
	require.NotContains(t, first, "hello")
	require.NotContains(t, first, "cdn.example.com")

	normalized.TextPrompt = "hello again"
	changed, err := seedAudioRequestHMAC(normalized)
	require.NoError(t, err)
	require.NotEqual(t, first, changed)
}

func TestSeedAudioIdempotencyUsesDBWhenRedisUnavailable(t *testing.T) {
	setupRelayTaskTestDB(t)
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})

	c := newSeedAudioIdempotencyTestContext()

	idem, cached, apiErr := seedAudioPrepareIdempotency(c, &relaycommon.RelayInfo{TokenId: 123}, &seedAudioNormalizedRequest{
		TextPrompt:      "hello",
		Format:          seedAudioDefaultFormat,
		ReferenceMode:   "text_only",
		ClientRequestID: "req_1",
	})
	require.NotNil(t, idem)
	require.Nil(t, cached)
	require.Nil(t, apiErr)

	var row model.SeedAudioIdempotency
	require.NoError(t, model.DB.Where("token_id = ? AND client_request_id = ?", 123, "req_1").First(&row).Error)
	require.Equal(t, model.SeedAudioIdempotencyStatusPending, row.Status)
	require.NotEmpty(t, row.RequestHMAC)
}

func TestSeedAudioIdempotencyDBCompletedReplay(t *testing.T) {
	setupRelayTaskTestDB(t)
	c := newSeedAudioIdempotencyTestContext()
	info := &relaycommon.RelayInfo{TokenId: 123}
	normalized := &seedAudioNormalizedRequest{
		TextPrompt:      "hello",
		Format:          seedAudioDefaultFormat,
		ReferenceMode:   "text_only",
		ClientRequestID: "req_replay",
	}

	idem, cached, apiErr := seedAudioPrepareIdempotency(c, info, normalized)
	require.Nil(t, apiErr)
	require.Nil(t, cached)
	require.NotNil(t, idem)
	require.NoError(t, idem.storeCompleted(c, dto.SeedAudioIdempotencyRecord{
		RequestHMAC:      idem.requestHMAC,
		Status:           model.SeedAudioIdempotencyStatusCompleted,
		ResponseID:       "aud_cached",
		TemporaryURL:     "https://tmp.example.com/audio.mp3",
		URLExpiresAt:     time.Now().Add(time.Hour).Unix(),
		Duration:         7.5,
		OriginalDuration: 8.25,
		ActualQuota:      10312,
		CreatedAt:        1234,
	}))

	replayed, cached, apiErr := seedAudioPrepareIdempotency(c, info, normalized)
	require.Nil(t, apiErr)
	require.NotNil(t, replayed)
	require.NotNil(t, cached)
	require.Equal(t, "aud_cached", cached.ResponseID)
	require.Equal(t, "https://tmp.example.com/audio.mp3", cached.TemporaryURL)
	require.EqualValues(t, 1234, cached.CreatedAt)

	response := seedAudioResponseFromRecord("lsf-seed-audio-1.0-a", *cached, normalized.ClientRequestID)
	require.Equal(t, normalized.ClientRequestID, response.Metadata["client_request_id"])
}

func TestSeedAudioRecordConsumeLogIsSanitized(t *testing.T) {
	setupRelayTaskTestDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Log{}))
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldDataExportEnabled := common.DataExportEnabled
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	t.Cleanup(func() {
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.DataExportEnabled = oldDataExportEnabled
	})

	c := newSeedAudioIdempotencyTestContext()
	c.Set("username", "relay-task-test-user")
	c.Set("token_name", "relay-task-test-token")
	c.Set(common.RequestIdKey, "req_log_visible")
	info := &relaycommon.RelayInfo{
		UserId:          1001,
		TokenId:         501,
		OriginModelName: "lsf-seed-audio-1.0-tenant-a",
		UsingGroup:      "henrytest",
		StartTime:       time.Now().Add(-2 * time.Second),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 77},
	}
	info.PriceData.GroupRatioInfo.GroupRatio = 2

	seedAudioRecordConsumeLog(c, info, &seedAudioNormalizedRequest{
		TextPrompt:      "secret customer prompt",
		ReferenceMode:   "audio_url",
		ClientRequestID: "req_log",
	}, &seedAudioUpstreamResult{
		URL:              "https://tmp.example.com/audio.mp3?token=secret",
		Duration:         2.8,
		OriginalDuration: 3.2,
	}, 8000, 300000)

	var log model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeConsume).First(&log).Error)
	require.Equal(t, "lsf-seed-audio-1.0-tenant-a", log.ModelName)
	require.Equal(t, 8000, log.Quota)
	require.Equal(t, 0, log.PromptTokens)
	require.Equal(t, 0, log.CompletionTokens)

	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	require.Equal(t, true, other["seed_audio"])
	require.Equal(t, "seconds", other["usage_type"])
	require.Equal(t, "audio_url", other["reference_mode"])
	require.Equal(t, true, other["client_request_id_present"])
	require.Equal(t, true, other["output_url_present"])
	require.EqualValues(t, 8000, other["actual_quota"])
	require.EqualValues(t, 3.2, other["original_duration"])

	adminLogs, total, err := model.GetAllLogs(model.LogTypeConsume, 0, 0, "lsf-seed-audio-1.0-tenant-a", "relay-task-test-user", "relay-task-test-token", 0, 10, 0, "", "req_log_visible")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, adminLogs, 1)
	require.Equal(t, log.Id, adminLogs[0].Id)
	require.Equal(t, "henrytest", adminLogs[0].Group)

	userLogs, total, err := model.GetUserLogs(1001, model.LogTypeConsume, 0, 0, "lsf-seed-audio-1.0-tenant-a", "relay-task-test-token", 0, 10, "", "req_log_visible")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, userLogs, 1)
	require.Equal(t, log.Id, userLogs[0].Id)
	require.Equal(t, "henrytest", userLogs[0].Group)

	combined := log.Content + log.Other
	require.NotContains(t, combined, "secret customer prompt")
	require.NotContains(t, combined, "tmp.example.com")
	require.NotContains(t, combined, "audio.mp3")
	require.NotContains(t, combined, "token=secret")
	require.NotContains(t, combined, "temporary_url")
}

func TestSeedAudioIdempotencyDBConflictDifferentRequest(t *testing.T) {
	setupRelayTaskTestDB(t)
	c := newSeedAudioIdempotencyTestContext()
	info := &relaycommon.RelayInfo{TokenId: 123}
	normalized := &seedAudioNormalizedRequest{
		TextPrompt:      "hello",
		Format:          seedAudioDefaultFormat,
		ReferenceMode:   "text_only",
		ClientRequestID: "req_conflict",
	}
	idem, cached, apiErr := seedAudioPrepareIdempotency(c, info, normalized)
	require.Nil(t, apiErr)
	require.Nil(t, cached)
	require.NotNil(t, idem)

	normalized.TextPrompt = "hello changed"
	replayed, cached, apiErr := seedAudioPrepareIdempotency(c, info, normalized)
	require.Nil(t, replayed)
	require.Nil(t, cached)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusConflict, apiErr.StatusCode)
	require.Equal(t, "idempotency_conflict", apiErr.ToOpenAIError().Code)
}

func TestSeedAudioIdempotencyDBExpiredResultReturnsGone(t *testing.T) {
	setupRelayTaskTestDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.SeedAudioIdempotency{
		CreatedAt:          now - 7200,
		UpdatedAt:          now - 7200,
		TokenID:            123,
		ClientRequestID:    "req_expired",
		RequestHMAC:        "unused",
		Status:             model.SeedAudioIdempotencyStatusCompleted,
		ResponseID:         "aud_expired",
		TemporaryURL:       "https://tmp.example.com/expired.mp3",
		URLExpiresAt:       now - 1,
		ExpiresAt:          now - 1,
		TombstoneExpiresAt: now + 3600,
	}).Error)

	c := newSeedAudioIdempotencyTestContext()
	idem, cached, apiErr := seedAudioPrepareIdempotency(c, &relaycommon.RelayInfo{TokenId: 123}, &seedAudioNormalizedRequest{
		TextPrompt:      "hello",
		Format:          seedAudioDefaultFormat,
		ReferenceMode:   "text_only",
		ClientRequestID: "req_expired",
	})
	require.Nil(t, idem)
	require.Nil(t, cached)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusGone, apiErr.StatusCode)
	require.Equal(t, "idempotency_result_expired", apiErr.ToOpenAIError().Code)
}

func TestSeedAudioAliasGate(t *testing.T) {
	require.True(t, IsSeedAudioAlias("lsf-seed-audio-1.0-tenant-a"))
	require.True(t, IsSeedAudioAlias("lsf-seed-audio-1.0"))
	require.False(t, IsSeedAudioAlias(seedAudioUpstreamModel))
	require.False(t, IsSeedAudioAlias("tts-1"))
}

func seedAudioTestRequest(t *testing.T, body string) (*dto.AudioRequest, map[string]interface{}) {
	t.Helper()
	var req dto.AudioRequest
	require.NoError(t, common.Unmarshal([]byte(body), &req))
	var bodyMap map[string]interface{}
	require.NoError(t, common.Unmarshal([]byte(body), &bodyMap))
	return &req, bodyMap
}

func newSeedAudioHelperTestContext(t *testing.T, body string, upstreamURL string) (*gin.Context, *relaycommon.RelayInfo, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("token_name", "relay-task-test-token")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstreamURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-upstream-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 77)

	var req dto.AudioRequest
	require.NoError(t, common.Unmarshal([]byte(body), &req))
	info := &relaycommon.RelayInfo{
		UserId:          1001,
		TokenId:         501,
		TokenKey:        "test-token",
		UsingGroup:      "test-group",
		UserGroup:       "test-group",
		OriginModelName: "lsf-seed-audio-1.0-tenant-a",
		StartTime:       time.Now(),
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
		Request: &req,
	}
	return c, info, recorder
}

func newSeedAudioIdempotencyTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	return c
}
