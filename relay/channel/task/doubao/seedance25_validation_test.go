package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	seedance25TenantAliasForDoubaoTest       = relaycommon.Seedance25TenantAliasPrefix + "henrytest"
	seedance25SecondTenantAliasForDoubaoTest = relaycommon.Seedance25TenantAliasPrefix + "tenant-two"
)

func seedance25RequestBody(t *testing.T, metadata map[string]any) []byte {
	return seedance25RequestBodyForModel(t, seedance25TenantAliasForDoubaoTest, metadata)
}

func seedance25RequestBodyForModel(t *testing.T, model string, metadata map[string]any) []byte {
	t.Helper()
	body, err := common.Marshal(map[string]any{
		"model":    model,
		"prompt":   "make a short landscape video",
		"metadata": metadata,
	})
	require.NoError(t, err)
	return body
}

func validateSeedance25Body(t *testing.T, body []byte, originModel string) (*gin.Context, *relaycommon.RelayInfo, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		OriginModelName: originModel,
		ChannelMeta:     &relaycommon.ChannelMeta{},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	if taskErr != nil {
		return c, info, fmt.Errorf("%s", taskErr.Message)
	}
	return c, info, nil
}

func validSeedance25Metadata() map[string]any {
	return map[string]any{"duration": 4, "resolution": "720p"}
}

func seedance25Image(role string) map[string]any {
	return seedance25ImageURL(role, "https://example.invalid/reference.png")
}

func seedance25ImageURL(role string, referenceURL string) map[string]any {
	return map[string]any{
		"type":      "image_url",
		"role":      role,
		"image_url": map[string]any{"url": referenceURL},
	}
}

func seedance25Video(role string) map[string]any {
	return seedance25VideoURL(role, "https://example.invalid/reference.mp4")
}

func seedance25VideoURL(role string, referenceURL string) map[string]any {
	return map[string]any{
		"type":      "video_url",
		"role":      role,
		"video_url": map[string]any{"url": referenceURL},
	}
}

func seedance25AudioURL(referenceURL string) map[string]any {
	return map[string]any{
		"type":      "audio_url",
		"role":      "reference_audio",
		"audio_url": map[string]any{"url": referenceURL},
	}
}

func TestValidateSeedance25ExactAliasAndLookalikes(t *testing.T) {
	body := seedance25RequestBody(t, validSeedance25Metadata())
	_, _, err := validateSeedance25Body(t, body, seedance25TenantAliasForDoubaoTest)
	require.NoError(t, err)

	secondTenantBody := seedance25RequestBodyForModel(t, seedance25SecondTenantAliasForDoubaoTest, validSeedance25Metadata())
	_, _, err = validateSeedance25Body(t, secondTenantBody, seedance25SecondTenantAliasForDoubaoTest)
	require.NoError(t, err)

	for _, modelName := range []string{
		"seedance-2.5",
		"lsf-seedance-2.50-henrytest",
		"LSF-Seedance-2.5-HenryTest",
		" lsf-seedance-2.5-henrytest ",
		"seedance-2.5-henrytest",
		"lsf-seedance-2.0-henrytest",
	} {
		t.Run(modelName, func(t *testing.T) {
			_, _, err := validateSeedance25Body(t, body, modelName)
			require.Error(t, err)
		})
	}

	_, _, err = validateSeedance25Body(t, secondTenantBody, seedance25TenantAliasForDoubaoTest)
	require.Error(t, err)

	lookalikeBody := []byte(`{"model":"seedance-2.50","prompt":"p","metadata":{"duration":4,"resolution":"720p"}}`)
	_, _, err = validateSeedance25Body(t, lookalikeBody, "unrelated-video-model")
	require.Error(t, err)
}

func TestValidateSeedance25Resolution(t *testing.T) {
	for _, resolution := range []string{"480p", "720p"} {
		t.Run("accept_"+resolution, func(t *testing.T) {
			metadata := validSeedance25Metadata()
			metadata["resolution"] = resolution
			c, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.NoError(t, err)
			req, getErr := relaycommon.GetTaskRequest(c)
			require.NoError(t, getErr)
			require.Equal(t, resolution, req.Metadata["resolution"])
		})
	}

	t.Run("missing_defaults_to_720p", func(t *testing.T) {
		metadata := map[string]any{"duration": 4}
		c, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
		require.NoError(t, err)
		req, getErr := relaycommon.GetTaskRequest(c)
		require.NoError(t, getErr)
		require.Equal(t, "720p", req.Metadata["resolution"])
	})

	for _, resolution := range []any{"1080p", "4k", "4K", "480", "720", "", nil} {
		t.Run("reject", func(t *testing.T) {
			metadata := validSeedance25Metadata()
			metadata["resolution"] = resolution
			_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.Error(t, err)
		})
	}
}

func TestValidateSeedance25DurationStrictJSONInteger(t *testing.T) {
	for _, duration := range []int{-1, 4, 30} {
		t.Run("accept_boundary", func(t *testing.T) {
			metadata := validSeedance25Metadata()
			metadata["duration"] = duration
			_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.NoError(t, err)
		})
	}

	invalidBodies := map[string]string{
		"missing": `{ "model":"lsf-seedance-2.5-henrytest", "prompt":"p", "metadata":{"resolution":"720p"} }`,
		"null":    `{ "model":"lsf-seedance-2.5-henrytest", "prompt":"p", "metadata":{"duration":null,"resolution":"720p"} }`,
		"string":  `{ "model":"lsf-seedance-2.5-henrytest", "prompt":"p", "metadata":{"duration":"4","resolution":"720p"} }`,
		"float":   `{ "model":"lsf-seedance-2.5-henrytest", "prompt":"p", "metadata":{"duration":4.0,"resolution":"720p"} }`,
		"three":   `{ "model":"lsf-seedance-2.5-henrytest", "prompt":"p", "metadata":{"duration":3,"resolution":"720p"} }`,
		"thirty1": `{ "model":"lsf-seedance-2.5-henrytest", "prompt":"p", "metadata":{"duration":31,"resolution":"720p"} }`,
	}
	for name, body := range invalidBodies {
		t.Run(name, func(t *testing.T) {
			_, _, err := validateSeedance25Body(t, []byte(body), seedance25TenantAliasForDoubaoTest)
			require.Error(t, err)
		})
	}
}

func TestValidateSeedance25SupportedInputModes(t *testing.T) {
	tests := []struct {
		name            string
		content         []any
		ratio           string
		expectedDefault string
	}{
		{name: "text only"},
		{name: "first frame", content: []any{seedance25Image("first_frame")}, expectedDefault: "adaptive"},
		{name: "first and last", content: []any{seedance25Image("first_frame"), seedance25Image("last_frame")}, expectedDefault: "adaptive"},
		{name: "single reference image", content: []any{seedance25Image("reference_image")}, ratio: "16:9"},
		{name: "single reference video", content: []any{seedance25Video("reference_video")}, ratio: "adaptive"},
		{name: "pure reference audio", content: []any{seedance25AudioURL("asset://audio/reference-one")}, ratio: "adaptive"},
		{name: "mixed references", content: []any{seedance25Image("reference_image"), seedance25Video("reference_video"), seedance25AudioURL("https://example.invalid/reference.wav")}, ratio: "adaptive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := validSeedance25Metadata()
			if tt.content != nil {
				metadata["content"] = tt.content
			}
			if tt.ratio != "" {
				metadata["ratio"] = tt.ratio
			}
			c, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.NoError(t, err)
			if tt.expectedDefault != "" {
				req, getErr := relaycommon.GetTaskRequest(c)
				require.NoError(t, getErr)
				require.Equal(t, tt.expectedDefault, req.Metadata["ratio"])
			}
		})
	}
}

func TestValidateSeedance25RejectsInvalidInputCombinations(t *testing.T) {
	tests := []struct {
		name    string
		content any
		ratio   any
	}{
		{name: "empty content", content: []any{}},
		{name: "last frame only", content: []any{seedance25Image("last_frame")}},
		{name: "duplicate first role", content: []any{seedance25Image("first_frame"), seedance25Image("first_frame")}},
		{name: "mixed first and reference", content: []any{seedance25Image("first_frame"), seedance25Video("reference_video")}},
		{name: "reference image video role", content: []any{seedance25Image("reference_video")}},
		{name: "reference video image role", content: []any{seedance25Video("reference_image")}},
		{name: "missing role", content: []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.invalid/a.png"}}}},
		{name: "wrong audio role", content: []any{map[string]any{"type": "audio_url", "role": "reference_video", "audio_url": map[string]any{"url": "https://example.invalid/a.wav"}}}},
		{name: "first fixed ratio", content: []any{seedance25Image("first_frame")}, ratio: "16:9"},
		{name: "unsupported reference ratio", content: []any{seedance25Image("reference_image")}, ratio: "9:16"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := validSeedance25Metadata()
			metadata["content"] = tt.content
			if tt.ratio != nil {
				metadata["ratio"] = tt.ratio
			}
			_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.Error(t, err)
		})
	}
}

func TestValidateSeedance25RejectsEveryDisabledFieldByPresence(t *testing.T) {
	for field := range seedance25ForbiddenFields {
		field := field
		t.Run("metadata_"+field, func(t *testing.T) {
			metadata := validSeedance25Metadata()
			metadata[field] = false
			_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.Error(t, err)
		})
		t.Run("root_"+field, func(t *testing.T) {
			root := map[string]any{
				"model":    seedance25TenantAliasForDoubaoTest,
				"prompt":   "p",
				"metadata": validSeedance25Metadata(),
				field:      0,
			}
			body, marshalErr := common.Marshal(root)
			require.NoError(t, marshalErr)
			_, _, err := validateSeedance25Body(t, body, seedance25TenantAliasForDoubaoTest)
			require.Error(t, err)
		})
	}

	metadata := validSeedance25Metadata()
	metadata["future_queue_control"] = nil
	_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
	require.Error(t, err)
}

func TestValidateSeedance25RejectsCanonicalDisabledValuesAndProjectOverride(t *testing.T) {
	tests := map[string]any{
		"camera_fixed": false,
		"draft":        false,
		"frames":       0,
		"priority":     0,
		"seed":         0,
		"service_tier": "flex",
	}
	for field, value := range tests {
		t.Run(field, func(t *testing.T) {
			metadata := validSeedance25Metadata()
			metadata[field] = value
			_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.Error(t, err)
		})
	}

	for _, field := range []string{"ProjectName", "project_name"} {
		t.Run(field, func(t *testing.T) {
			root := map[string]any{
				"model":    seedance25TenantAliasForDoubaoTest,
				"prompt":   "p",
				"metadata": validSeedance25Metadata(),
				field:      "client-value-marker",
			}
			body, err := common.Marshal(root)
			require.NoError(t, err)
			_, _, err = validateSeedance25Body(t, body, seedance25TenantAliasForDoubaoTest)
			require.Error(t, err)
		})
	}
}

func TestValidateSeedance25ReferenceCountBoundaries(t *testing.T) {
	materials := func(images, videos, audios int) []any {
		items := make([]any, 0, images+videos+audios)
		for i := 0; i < images; i++ {
			items = append(items, seedance25ImageURL("reference_image", fmt.Sprintf("https://example.invalid/image-%02d.png", i)))
		}
		for i := 0; i < videos; i++ {
			items = append(items, seedance25VideoURL("reference_video", fmt.Sprintf("https://example.invalid/video-%02d.mp4", i)))
		}
		for i := 0; i < audios; i++ {
			items = append(items, seedance25AudioURL(fmt.Sprintf("https://example.invalid/audio-%02d.wav", i)))
		}
		return items
	}

	tests := []struct {
		name    string
		content []any
		valid   bool
	}{
		{name: "30 images", content: materials(30, 0, 0), valid: true},
		{name: "31 images", content: materials(31, 0, 0)},
		{name: "10 videos", content: materials(0, 10, 0), valid: true},
		{name: "11 videos", content: materials(0, 11, 0)},
		{name: "10 audios", content: materials(0, 0, 10), valid: true},
		{name: "11 audios", content: materials(0, 0, 11)},
		{name: "50 mixed materials", content: materials(30, 10, 10), valid: true},
		{name: "51 materials", content: materials(31, 10, 10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := validSeedance25Metadata()
			metadata["content"] = tt.content
			_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestSeedance25ReferenceOrderAndP1ParametersReachUpstreamUnchanged(t *testing.T) {
	content := []any{
		seedance25ImageURL("reference_image", "https://example.invalid/shared-reference"),
		seedance25VideoURL("reference_video", "https://example.invalid/video-one.mp4"),
		seedance25AudioURL("asset://audio/reference-one"),
		seedance25ImageURL("reference_image", "https://example.invalid/shared-reference"),
		seedance25AudioURL("https://example.invalid/audio-two.wav"),
	}
	metadata := map[string]any{
		"duration":          -1,
		"resolution":        "720p",
		"ratio":             "adaptive",
		"content":           content,
		"output_format":     "mov",
		"return_last_frame": false,
		"watermark":         true,
	}
	c, info, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
	require.NoError(t, err)
	info.ChannelMeta = &relaycommon.ChannelMeta{IsModelMapped: true, UpstreamModelName: "mapped-provider-model"}
	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, float64(-1), payload["duration"])
	require.Equal(t, "mov", payload["output_format"])
	require.Equal(t, false, payload["return_last_frame"])
	require.Equal(t, true, payload["watermark"])

	upstreamContent := payload["content"].([]any)
	require.Len(t, upstreamContent, len(content)+1)
	for i, expected := range content {
		expectedItem := expected.(map[string]any)
		actualItem := upstreamContent[i].(map[string]any)
		require.Equal(t, expectedItem["type"], actualItem["type"])
		require.Equal(t, expectedItem["role"], actualItem["role"])
		mediaField := strings.TrimSuffix(expectedItem["type"].(string), "_url") + "_url"
		require.Equal(t, expectedItem[mediaField], actualItem[mediaField])
	}
	require.Equal(t, "text", upstreamContent[len(content)].(map[string]any)["type"])
}

func TestValidateSeedance25P1ParameterTypesAndMediaSchemes(t *testing.T) {
	for _, outputFormat := range []string{"mp4", "mov"} {
		metadata := validSeedance25Metadata()
		metadata["output_format"] = outputFormat
		_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
		require.NoError(t, err)
	}
	for _, field := range []string{"return_last_frame", "watermark"} {
		metadata := validSeedance25Metadata()
		metadata[field] = "false"
		_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
		require.Error(t, err)
	}
	for _, outputFormat := range []any{"MP4", "avi", false, nil} {
		metadata := validSeedance25Metadata()
		metadata["output_format"] = outputFormat
		_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
		require.Error(t, err)
	}
	for _, referenceURL := range []string{"http://example.invalid/a.png", "data:image/png;base64,AAAA", "file:///tmp/a.png", "https://user:pass@example.invalid/a.png"} {
		metadata := validSeedance25Metadata()
		metadata["content"] = []any{seedance25ImageURL("reference_image", referenceURL)}
		_, _, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
		require.Error(t, err)
	}
}

func TestValidateSeedance25GenerateAudioPreservesExplicitFalse(t *testing.T) {
	tests := []struct {
		name          string
		include       bool
		value         bool
		expectPresent bool
	}{
		{name: "omitted"},
		{name: "true", include: true, value: true, expectPresent: true},
		{name: "false", include: true, value: false, expectPresent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := validSeedance25Metadata()
			if tt.include {
				metadata["generate_audio"] = tt.value
			}
			c, info, err := validateSeedance25Body(t, seedance25RequestBody(t, metadata), seedance25TenantAliasForDoubaoTest)
			require.NoError(t, err)
			info.ChannelMeta = &relaycommon.ChannelMeta{
				IsModelMapped:     true,
				UpstreamModelName: "mapped-provider-model",
			}
			reader, buildErr := (&TaskAdaptor{}).BuildRequestBody(c, info)
			require.NoError(t, buildErr)
			body, readErr := io.ReadAll(reader)
			require.NoError(t, readErr)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(body, &payload))
			got, present := payload["generate_audio"]
			require.Equal(t, tt.expectPresent, present)
			if present {
				require.Equal(t, tt.value, got)
			}
		})
	}
}

func TestValidateSeedance25RequiresServerSideModelMapping(t *testing.T) {
	c, info, err := validateSeedance25Body(t, seedance25RequestBody(t, validSeedance25Metadata()), seedance25TenantAliasForDoubaoTest)
	require.NoError(t, err)

	taskErr := (&TaskAdaptor{}).ValidateMappedRequest(c, info)
	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusServiceUnavailable, taskErr.StatusCode)
	require.Equal(t, "UPSTREAM_MAPPING_MISSING", taskErr.Code)

	info.ChannelMeta = &relaycommon.ChannelMeta{
		IsModelMapped:     true,
		UpstreamModelName: "   ",
	}
	taskErr = (&TaskAdaptor{}).ValidateMappedRequest(c, info)
	require.NotNil(t, taskErr)
	require.Equal(t, "UPSTREAM_MAPPING_MISSING", taskErr.Code)

	info.UpstreamModelName = "  mapped-provider-model-placeholder  "
	require.Nil(t, (&TaskAdaptor{}).ValidateMappedRequest(c, info))
	require.Equal(t, "mapped-provider-model-placeholder", info.UpstreamModelName)

	info.UpstreamModelName = "  " + seedance25TenantAliasForDoubaoTest + "  "
	taskErr = (&TaskAdaptor{}).ValidateMappedRequest(c, info)
	require.NotNil(t, taskErr)
	require.Equal(t, "UPSTREAM_MAPPING_MISSING", taskErr.Code)

	info.UpstreamModelName = "  " + strings.ToUpper(seedance25TenantAliasForDoubaoTest) + "  "
	taskErr = (&TaskAdaptor{}).ValidateMappedRequest(c, info)
	require.NotNil(t, taskErr)
	require.Equal(t, "UPSTREAM_MAPPING_MISSING", taskErr.Code)
}

func TestSeedance25BuildRequestBodyDirectlyRejectsUnsafeMappings(t *testing.T) {
	tests := []struct {
		name        string
		channelMeta *relaycommon.ChannelMeta
	}{
		{name: "missing mapping metadata", channelMeta: nil},
		{name: "mapping flag false", channelMeta: &relaycommon.ChannelMeta{IsModelMapped: false, UpstreamModelName: "placeholder-upstream-model"}},
		{name: "empty mapping", channelMeta: &relaycommon.ChannelMeta{IsModelMapped: true, UpstreamModelName: ""}},
		{name: "blank mapping", channelMeta: &relaycommon.ChannelMeta{IsModelMapped: true, UpstreamModelName: "  \t "}},
		{name: "public alias mapping", channelMeta: &relaycommon.ChannelMeta{IsModelMapped: true, UpstreamModelName: seedance25TenantAliasForDoubaoTest}},
		{name: "trimmed public alias mapping", channelMeta: &relaycommon.ChannelMeta{IsModelMapped: true, UpstreamModelName: "  " + seedance25TenantAliasForDoubaoTest + "  "}},
		{name: "case folded public alias mapping", channelMeta: &relaycommon.ChannelMeta{IsModelMapped: true, UpstreamModelName: "  " + strings.ToUpper(seedance25TenantAliasForDoubaoTest) + "  "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, info, err := validateSeedance25Body(t, seedance25RequestBody(t, validSeedance25Metadata()), seedance25TenantAliasForDoubaoTest)
			require.NoError(t, err)
			info.ChannelMeta = tt.channelMeta
			require.NotPanics(t, func() {
				reader, buildErr := (&TaskAdaptor{}).BuildRequestBody(c, info)
				require.Error(t, buildErr)
				require.Nil(t, reader)
				require.Contains(t, buildErr.Error(), "server-side model mapping is not configured")
			})
		})
	}
}

func TestSeedance25BuildRequestBodyDirectlyTrimsValidMappings(t *testing.T) {
	validMappings := []string{
		"  placeholder-upstream-model  ",
		"  provider-seedance-2.5-endpoint-placeholder  ",
	}

	for _, mapping := range validMappings {
		t.Run(mapping, func(t *testing.T) {
			c, info, err := validateSeedance25Body(t, seedance25RequestBody(t, validSeedance25Metadata()), seedance25TenantAliasForDoubaoTest)
			require.NoError(t, err)
			info.ChannelMeta = &relaycommon.ChannelMeta{
				IsModelMapped:     true,
				UpstreamModelName: mapping,
			}

			reader, buildErr := (&TaskAdaptor{}).BuildRequestBody(c, info)
			require.NoError(t, buildErr)
			body, readErr := io.ReadAll(reader)
			require.NoError(t, readErr)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(body, &payload))
			expectedMapping := strings.TrimSpace(mapping)
			require.Equal(t, expectedMapping, info.UpstreamModelName)
			require.Equal(t, expectedMapping, payload["model"])
			require.NotEqual(t, seedance25TenantAliasForDoubaoTest, payload["model"])
		})
	}
}
