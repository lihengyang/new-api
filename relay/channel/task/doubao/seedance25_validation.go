package doubao

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	seedance25InputText           = "text_only"
	seedance25InputFirstFrame     = "first_frame"
	seedance25InputFirstLastFrame = "first_last_frame"
	seedance25InputReferenceImage = "reference_image"
	seedance25InputReferenceVideo = "reference_video"
)

var seedance25AllowedRootFields = map[string]struct{}{
	"metadata": {},
	"model":    {},
	"prompt":   {},
}

var seedance25AllowedMetadataFields = map[string]struct{}{
	"client_request_id": {},
	"content":           {},
	"duration":          {},
	"generate_audio":    {},
	"ratio":             {},
	"resolution":        {},
}

var seedance25ForbiddenFields = map[string]struct{}{
	"callback":                {},
	"callback_events":         {},
	"callback_method":         {},
	"callback_url":            {},
	"camera_fixed":            {},
	"draft":                   {},
	"execution_expires_after": {},
	"frames":                  {},
	"output_format":           {},
	"priority":                {},
	"queue":                   {},
	"queue_id":                {},
	"queue_priority":          {},
	"return_last_frame":       {},
	"seed":                    {},
	"service_tier":            {},
}

func seedance25InvalidRequest(message string) *dto.TaskError {
	return service.TaskErrorWrapperLocal(fmt.Errorf("%s", message), "invalid_request_error", http.StatusBadRequest)
}

func rawJSONIsNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func seedance25RawRequest(c *gin.Context) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, nil, err
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil, nil, err
	}
	root := make(map[string]json.RawMessage)
	if err := common.Unmarshal(body, &root); err != nil {
		return nil, nil, err
	}
	metadata := make(map[string]json.RawMessage)
	rawMetadata, ok := root["metadata"]
	if !ok || len(rawMetadata) == 0 || rawJSONIsNull(rawMetadata) {
		return root, metadata, nil
	}
	if err := common.Unmarshal(rawMetadata, &metadata); err == nil {
		return root, metadata, nil
	}
	var metadataString string
	if err := common.Unmarshal(rawMetadata, &metadataString); err != nil {
		return nil, nil, fmt.Errorf("metadata must be a JSON object")
	}
	if err := common.Unmarshal([]byte(metadataString), &metadata); err != nil {
		return nil, nil, fmt.Errorf("metadata must be a JSON object")
	}
	return root, metadata, nil
}

func firstUnsupportedSeedance25Field(fields map[string]json.RawMessage, allowed map[string]struct{}) string {
	keys := make([]string, 0)
	for key := range fields {
		if _, forbidden := seedance25ForbiddenFields[key]; forbidden {
			keys = append(keys, key)
			continue
		}
		if _, ok := allowed[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func validateSeedance25MediaURL(raw json.RawMessage, field string) error {
	if len(raw) == 0 || rawJSONIsNull(raw) {
		return fmt.Errorf("%s is required", field)
	}
	value := make(map[string]json.RawMessage)
	if err := common.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%s must be an object", field)
	}
	if len(value) != 1 {
		return fmt.Errorf("%s only supports url", field)
	}
	rawURL, ok := value["url"]
	if !ok {
		return fmt.Errorf("%s.url is required", field)
	}
	var url string
	if err := common.Unmarshal(rawURL, &url); err != nil || strings.TrimSpace(url) == "" {
		return fmt.Errorf("%s.url must be a non-empty string", field)
	}
	return nil
}

func validateSeedance25Content(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return seedance25InputText, nil
	}
	if rawJSONIsNull(raw) {
		return "", fmt.Errorf("content must be omitted or an array")
	}
	var items []json.RawMessage
	if err := common.Unmarshal(raw, &items); err != nil {
		return "", fmt.Errorf("content must be an array")
	}
	if len(items) == 0 {
		return "", fmt.Errorf("content must be omitted for text-only requests")
	}
	if len(items) > 2 {
		return "", fmt.Errorf("content exceeds the Seedance 2.5 P0 material limit")
	}

	roleCount := make(map[string]int)
	for _, rawItem := range items {
		item := make(map[string]json.RawMessage)
		if err := common.Unmarshal(rawItem, &item); err != nil {
			return "", fmt.Errorf("content items must be objects")
		}
		var mediaType string
		if err := common.Unmarshal(item["type"], &mediaType); err != nil || mediaType == "" {
			return "", fmt.Errorf("content item type is required")
		}
		var role string
		if err := common.Unmarshal(item["role"], &role); err != nil || role == "" {
			return "", fmt.Errorf("content item role is required")
		}

		allowedFields := map[string]struct{}{"role": {}, "type": {}}
		switch mediaType {
		case "image_url":
			allowedFields["image_url"] = struct{}{}
			if role != seedance25InputFirstFrame && role != "last_frame" && role != seedance25InputReferenceImage {
				return "", fmt.Errorf("image_url role %q is not supported", role)
			}
			if err := validateSeedance25MediaURL(item["image_url"], "image_url"); err != nil {
				return "", err
			}
		case "video_url":
			allowedFields["video_url"] = struct{}{}
			if role != seedance25InputReferenceVideo {
				return "", fmt.Errorf("video_url role must be reference_video")
			}
			if err := validateSeedance25MediaURL(item["video_url"], "video_url"); err != nil {
				return "", err
			}
		case "audio_url":
			return "", fmt.Errorf("reference audio is outside Seedance 2.5 P0")
		default:
			return "", fmt.Errorf("content type %q is not supported", mediaType)
		}
		if unsupported := firstUnsupportedSeedance25Field(item, allowedFields); unsupported != "" {
			return "", fmt.Errorf("content item field %q is not supported", unsupported)
		}
		roleCount[role]++
	}

	for role, count := range roleCount {
		if count != 1 {
			return "", fmt.Errorf("content role %q must appear exactly once", role)
		}
	}
	switch {
	case len(items) == 1 && roleCount[seedance25InputFirstFrame] == 1:
		return seedance25InputFirstFrame, nil
	case len(items) == 2 && roleCount[seedance25InputFirstFrame] == 1 && roleCount["last_frame"] == 1:
		return seedance25InputFirstLastFrame, nil
	case len(items) == 1 && roleCount[seedance25InputReferenceImage] == 1:
		return seedance25InputReferenceImage, nil
	case len(items) == 1 && roleCount[seedance25InputReferenceVideo] == 1:
		return seedance25InputReferenceVideo, nil
	default:
		return "", fmt.Errorf("first/last-frame and reference modes cannot be mixed")
	}
}

func validateSeedance25Request(c *gin.Context, info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) *dto.TaskError {
	originModelName := info.OriginModelName
	if originModelName == "" {
		originModelName = req.Model
	}
	if relaycommon.IsSeedance25AliasLookalike(originModelName) || relaycommon.IsSeedance25AliasLookalike(req.Model) {
		return seedance25InvalidRequest("model must use one exact lsf-seedance-2.5-<tenant> alias")
	}
	if relaycommon.IsSeedance25OriginAlias(originModelName) != relaycommon.IsSeedance25OriginAlias(req.Model) {
		return seedance25InvalidRequest("model must use the exact tenant Seedance 2.5 alias consistently")
	}
	if !relaycommon.IsSeedance25OriginAlias(originModelName) {
		return nil
	}
	if originModelName != req.Model {
		return seedance25InvalidRequest("model must use the exact tenant Seedance 2.5 alias consistently")
	}

	root, metadata, err := seedance25RawRequest(c)
	if err != nil {
		return seedance25InvalidRequest(err.Error())
	}
	var submittedModel string
	if err := common.Unmarshal(root["model"], &submittedModel); err != nil || submittedModel != originModelName {
		return seedance25InvalidRequest("model must use the exact tenant Seedance 2.5 alias")
	}
	if unsupported := firstUnsupportedSeedance25Field(root, seedance25AllowedRootFields); unsupported != "" {
		return seedance25InvalidRequest(fmt.Sprintf("field %q is not supported for seedance-2.5", unsupported))
	}
	if unsupported := firstUnsupportedSeedance25Field(metadata, seedance25AllowedMetadataFields); unsupported != "" {
		return seedance25InvalidRequest(fmt.Sprintf("metadata field %q is not supported for seedance-2.5", unsupported))
	}
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}

	rawDuration, ok := metadata["duration"]
	if !ok || len(rawDuration) == 0 || rawJSONIsNull(rawDuration) {
		return seedance25InvalidRequest("metadata.duration is required and must be an integer from 4 to 30")
	}
	var duration int
	if err := common.Unmarshal(rawDuration, &duration); err != nil || duration < 4 || duration > 30 {
		return seedance25InvalidRequest("metadata.duration must be an integer from 4 to 30")
	}

	resolution := "720p"
	if rawResolution, ok := metadata["resolution"]; ok {
		if rawJSONIsNull(rawResolution) || common.Unmarshal(rawResolution, &resolution) != nil ||
			(resolution != "480p" && resolution != "720p") {
			return seedance25InvalidRequest("metadata.resolution must be 480p or 720p")
		}
	}

	if rawGenerateAudio, ok := metadata["generate_audio"]; ok {
		var generateAudio bool
		if rawJSONIsNull(rawGenerateAudio) || common.Unmarshal(rawGenerateAudio, &generateAudio) != nil {
			return seedance25InvalidRequest("metadata.generate_audio must be true or false")
		}
		req.Metadata["generate_audio"] = generateAudio
	}

	inputMode, err := validateSeedance25Content(metadata["content"])
	if err != nil {
		return seedance25InvalidRequest(err.Error())
	}
	ratio := ""
	if rawRatio, ok := metadata["ratio"]; ok {
		if rawJSONIsNull(rawRatio) || common.Unmarshal(rawRatio, &ratio) != nil {
			return seedance25InvalidRequest("metadata.ratio must be a supported string")
		}
	}
	if inputMode == seedance25InputFirstFrame || inputMode == seedance25InputFirstLastFrame {
		if ratio == "" {
			ratio = "adaptive"
		} else if ratio != "adaptive" {
			return seedance25InvalidRequest("first-frame modes require metadata.ratio=adaptive")
		}
	} else if ratio != "" && ratio != "adaptive" && ratio != "16:9" {
		return seedance25InvalidRequest("metadata.ratio is not supported by the current customer contract")
	}

	req.Metadata["duration"] = duration
	req.Metadata["resolution"] = resolution
	if ratio != "" {
		req.Metadata["ratio"] = ratio
	}
	c.Set("task_request", *req)
	return nil
}
