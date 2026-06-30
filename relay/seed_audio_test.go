package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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

	upstreamBody, err := common.Marshal(normalized.toUpstreamRequest())
	require.NoError(t, err)
	upstreamJSON := string(upstreamBody)
	require.Contains(t, upstreamJSON, `"model":"seed-audio-1.0"`)
	require.Contains(t, upstreamJSON, `"sample_rate":0`)
	require.Contains(t, upstreamJSON, `"speech_rate":0`)
	require.Contains(t, upstreamJSON, `"loudness_rate":0`)
	require.Contains(t, upstreamJSON, `"pitch_rate":0`)
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
			code:       "invalid_request_error",
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
	})
	body, err := common.Marshal(response)
	require.NoError(t, err)
	bodyString := string(body)
	require.Contains(t, bodyString, "lsf-seed-audio-1.0-tenant-a")
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

func TestSeedAudioIdempotencyFailsClosedWhenRedisUnavailable(t *testing.T) {
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
	})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	idem, cached, apiErr := seedAudioPrepareIdempotency(c, &relaycommon.RelayInfo{TokenId: 123}, &seedAudioNormalizedRequest{
		TextPrompt:      "hello",
		Format:          seedAudioDefaultFormat,
		ReferenceMode:   "text_only",
		ClientRequestID: "req_1",
	})
	require.Nil(t, idem)
	require.Nil(t, cached)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Equal(t, "seed_audio_idempotency_unavailable", apiErr.ToOpenAIError().Code)
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
