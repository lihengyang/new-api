package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	seedanceAssetAdminDefaultGroupType   = "AIGC"
	seedanceAssetAdminRealHumanGroupType = "LivenessFace"
	seedanceAssetAdminVersion            = "2024-01-01"
	seedanceAssetAdminService            = "ark"
)

type seedanceAssetAdminConfig struct {
	ProjectName    string
	AssetGroupType string
	Region         string
	Proxy          string
}

type seedanceAssetAction struct {
	Name string
	Path string
}

var (
	seedanceAssetChannelLoader = model.CacheGetChannel
	seedanceAssetActions       = map[string]seedanceAssetAction{
		"/v1/seedance/virtual/asset-groups/create":        {Name: "CreateAssetGroup", Path: "/"},
		"/v1/seedance/virtual/asset-groups/list":          {Name: "ListAssetGroups", Path: "/"},
		"/v1/seedance/virtual/asset-groups/get":           {Name: "GetAssetGroup", Path: "/"},
		"/v1/seedance/virtual/asset-groups/update":        {Name: "UpdateAssetGroup", Path: "/"},
		"/v1/seedance/virtual/asset-groups/delete":        {Name: "DeleteAssetGroup", Path: "/"},
		"/v1/seedance/virtual/assets/create":              {Name: "CreateAsset", Path: "/"},
		"/v1/seedance/virtual/assets/list":                {Name: "ListAssets", Path: "/"},
		"/v1/seedance/virtual/assets/get":                 {Name: "GetAsset", Path: "/"},
		"/v1/seedance/virtual/assets/update":              {Name: "UpdateAsset", Path: "/"},
		"/v1/seedance/virtual/assets/delete":              {Name: "DeleteAsset", Path: "/"},
		"/v1/seedance/real-human/validate-session/create": {Name: "CreateVisualValidateSession", Path: "/"},
		"/v1/seedance/real-human/validate-result/get":     {Name: "GetVisualValidateResult", Path: "/"},
		"/v1/seedance/real-human/asset-groups/list":       {Name: "ListAssetGroups", Path: "/"},
		"/v1/seedance/real-human/asset-groups/get":        {Name: "GetAssetGroup", Path: "/"},
		"/v1/seedance/real-human/asset-groups/update":     {Name: "UpdateAssetGroup", Path: "/"},
		"/v1/seedance/real-human/asset-groups/delete":     {Name: "DeleteAssetGroup", Path: "/"},
		"/v1/seedance/real-human/assets/create":           {Name: "CreateAsset", Path: "/"},
		"/v1/seedance/real-human/assets/list":             {Name: "ListAssets", Path: "/"},
		"/v1/seedance/real-human/assets/get":              {Name: "GetAsset", Path: "/"},
		"/v1/seedance/real-human/assets/update":           {Name: "UpdateAsset", Path: "/"},
		"/v1/seedance/real-human/assets/delete":           {Name: "DeleteAsset", Path: "/"},
	}
)

func seedanceAssetError(c *gin.Context, status int, errType string, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func seedanceAssetActionForPath(path string) (seedanceAssetAction, bool) {
	action, ok := seedanceAssetActions[path]
	return action, ok
}

func resolveSeedanceAssetAdminConfig(c *gin.Context) (*seedanceAssetAdminConfig, error) {
	config := &seedanceAssetAdminConfig{}
	if setting, ok := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting); ok {
		config.ProjectName = strings.TrimSpace(setting.ByteplusProjectName)
		config.AssetGroupType = strings.TrimSpace(setting.ByteplusAssetGroupType)
		config.Region = strings.TrimSpace(setting.ByteplusRegion)
		config.Proxy = strings.TrimSpace(setting.Proxy)
	}
	if setting, ok := common.GetContextKeyType[dto.ChannelOtherSettings](c, constant.ContextKeyChannelOtherSetting); ok {
		if config.ProjectName == "" {
			config.ProjectName = strings.TrimSpace(setting.ByteplusProjectName)
		}
		if config.AssetGroupType == "" {
			config.AssetGroupType = strings.TrimSpace(setting.ByteplusAssetGroupType)
		}
		if config.Region == "" {
			config.Region = strings.TrimSpace(setting.ByteplusRegion)
		}
	}
	if config.ProjectName != "" && config.Region != "" {
		return config, nil
	}

	channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	if channelID == 0 {
		return nil, errors.New("channel_id is missing")
	}
	channel, err := seedanceAssetChannelLoader(channelID)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, errors.New("selected channel not found")
	}

	channelSetting := channel.GetSetting()
	if config.ProjectName == "" {
		config.ProjectName = strings.TrimSpace(channelSetting.ByteplusProjectName)
	}
	if config.AssetGroupType == "" {
		config.AssetGroupType = strings.TrimSpace(channelSetting.ByteplusAssetGroupType)
	}
	if config.Region == "" {
		config.Region = strings.TrimSpace(channelSetting.ByteplusRegion)
	}
	if config.Proxy == "" {
		config.Proxy = strings.TrimSpace(channelSetting.Proxy)
	}

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

func parseSeedanceAssetAdminKey(apiKey string) (string, string, error) {
	parts := strings.Split(apiKey, "|")
	if len(parts) != 2 {
		return "", "", errors.New("asset admin channel key must use AK|SK format")
	}
	accessKey := strings.TrimSpace(parts[0])
	secretKey := strings.TrimSpace(parts[1])
	if accessKey == "" || secretKey == "" {
		return "", "", errors.New("asset admin channel key must use AK|SK format")
	}
	return accessKey, secretKey, nil
}

func seedanceAssetAdminBaseURL(channelBaseURL string, region string) (string, error) {
	if trimmed := strings.TrimSpace(channelBaseURL); trimmed != "" {
		return trimmed, nil
	}
	if strings.TrimSpace(region) == "" {
		return "", errors.New("byteplus_region is required on the selected channel")
	}
	return fmt.Sprintf("https://ark.%s.byteplusapi.com", region), nil
}

func parseSeedanceAssetRequestBody(c *gin.Context) (map[string]any, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := storage.Bytes()
	if err != nil {
		return nil, err
	}

	payload := make(map[string]any)
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		if err = common.Unmarshal(bodyBytes, &payload); err != nil {
			return nil, err
		}
	}
	if _, seekErr := storage.Seek(0, io.SeekStart); seekErr == nil {
		c.Request.Body = io.NopCloser(storage)
	}
	return payload, nil
}

func forceStringField(payload map[string]any, key string, value string) {
	delete(payload, strings.ToLower(key))
	payload[key] = value
}

func seedanceAssetFilter(payload map[string]any) map[string]any {
	raw, ok := payload["Filter"]
	if !ok || raw == nil {
		filter := make(map[string]any)
		payload["Filter"] = filter
		return filter
	}
	if filter, ok := raw.(map[string]any); ok {
		return filter
	}
	filter := make(map[string]any)
	payload["Filter"] = filter
	return filter
}

func hasSeedanceBase64UploadField(payload map[string]any) bool {
	for key := range payload {
		if strings.Contains(strings.ToLower(strings.TrimSpace(key)), "base64") {
			return true
		}
	}
	return false
}

func isSeedanceRealHumanRoute(path string) bool {
	return strings.HasPrefix(path, "/v1/seedance/real-human/")
}

func prepareSeedanceAssetPayload(actionName string, payload map[string]any, config *seedanceAssetAdminConfig) error {
	delete(payload, "project_name")
	forceStringField(payload, "ProjectName", config.ProjectName)

	switch actionName {
	case "CreateAssetGroup":
		payload["GroupType"] = seedanceAssetAdminDefaultGroupType
	case "ListAssetGroups", "ListAssets":
		filter := seedanceAssetFilter(payload)
		filter["GroupType"] = seedanceAssetAdminDefaultGroupType
	case "CreateAsset":
		groupID, _ := payload["GroupId"].(string)
		resourceURL, _ := payload["URL"].(string)
		assetType, _ := payload["AssetType"].(string)
		if hasSeedanceBase64UploadField(payload) {
			return errors.New("base64 upload is not supported; URL is required")
		}
		if strings.TrimSpace(groupID) == "" {
			return errors.New("GroupId is required")
		}
		if strings.TrimSpace(resourceURL) == "" {
			return errors.New("URL is required; base64 upload is not supported")
		}
		if strings.TrimSpace(assetType) == "" {
			return errors.New("AssetType is required")
		}
		delete(payload, "GroupType")
	case "GetAssetGroup", "UpdateAssetGroup", "DeleteAssetGroup", "GetAsset", "UpdateAsset", "DeleteAsset":
		// ProjectName override is enough for the first backend-only slice.
	}
	return nil
}

func prepareSeedanceAssetPayloadForRoute(path string, actionName string, payload map[string]any, config *seedanceAssetAdminConfig) error {
	if err := prepareSeedanceAssetPayload(actionName, payload, config); err != nil {
		return err
	}
	if isSeedanceRealHumanRoute(path) {
		switch path {
		case "/v1/seedance/real-human/asset-groups/list", "/v1/seedance/real-human/assets/list":
			filter := seedanceAssetFilter(payload)
			filter["GroupType"] = seedanceAssetAdminRealHumanGroupType
		}
	}
	return nil
}

func buildSeedanceAssetTargetURL(baseURL string, action seedanceAssetAction) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if u.Path == "" {
		u.Path = action.Path
	} else {
		u.Path = strings.TrimRight(u.Path, "/") + action.Path
	}
	query := u.Query()
	query.Set("Action", action.Name)
	query.Set("Version", seedanceAssetAdminVersion)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func signSeedanceAssetAdminRequest(req *http.Request, accessKey string, secretKey string, region string) error {
	var bodyBytes []byte
	var err error
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		if err = req.Body.Close(); err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	payloadHash := sha256.Sum256(bodyBytes)
	hexPayloadHash := hex.EncodeToString(payloadHash[:])

	now := time.Now().UTC()
	xDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")

	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Date", xDate)
	req.Header.Set("X-Content-Sha256", hexPayloadHash)

	queryParams := req.URL.Query()
	keys := make([]string, 0, len(queryParams))
	for key := range queryParams {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	queryParts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := queryParams[key]
		sort.Strings(values)
		for _, value := range values {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", url.QueryEscape(key), url.QueryEscape(value)))
		}
	}
	canonicalQueryString := strings.Join(queryParts, "&")

	signedHeaders := "host;x-content-sha256;x-date"
	canonicalHeaders := fmt.Sprintf(
		"host:%s\nx-content-sha256:%s\nx-date:%s\n",
		req.URL.Host,
		hexPayloadHash,
		xDate,
	)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hexPayloadHash,
	)
	canonicalRequestHash := sha256.Sum256([]byte(canonicalRequest))
	hexCanonicalRequestHash := hex.EncodeToString(canonicalRequestHash[:])

	credentialScope := fmt.Sprintf("%s/%s/%s/request", shortDate, region, seedanceAssetAdminService)
	stringToSign := fmt.Sprintf("HMAC-SHA256\n%s\n%s\n%s",
		xDate,
		credentialScope,
		hexCanonicalRequestHash,
	)

	kDate := seedanceAssetHMACSHA256([]byte(secretKey), []byte(shortDate))
	kRegion := seedanceAssetHMACSHA256(kDate, []byte(region))
	kService := seedanceAssetHMACSHA256(kRegion, []byte(seedanceAssetAdminService))
	kSigning := seedanceAssetHMACSHA256(kService, []byte("request"))
	signature := hex.EncodeToString(seedanceAssetHMACSHA256(kSigning, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey,
		credentialScope,
		signedHeaders,
		signature,
	))
	return nil
}

func seedanceAssetHMACSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func RelaySeedanceAsset(c *gin.Context) {
	action, ok := seedanceAssetActionForPath(c.Request.URL.Path)
	if !ok {
		seedanceAssetError(c, http.StatusNotFound, "invalid_request_error", "unsupported seedance asset admin route")
		return
	}

	config, err := resolveSeedanceAssetAdminConfig(c)
	if err != nil {
		seedanceAssetError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if strings.TrimSpace(config.ProjectName) == "" {
		seedanceAssetError(c, http.StatusBadRequest, "invalid_request_error", "byteplus_project_name is required on the selected channel")
		return
	}
	if strings.TrimSpace(config.Region) == "" {
		seedanceAssetError(c, http.StatusBadRequest, "invalid_request_error", "byteplus_region is required on the selected channel")
		return
	}

	apiKey := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyChannelKey))
	accessKey, secretKey, err := parseSeedanceAssetAdminKey(apiKey)
	if err != nil {
		seedanceAssetError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	payload, err := parseSeedanceAssetRequestBody(c)
	if err != nil {
		seedanceAssetError(c, http.StatusBadRequest, "invalid_request_error", "invalid JSON request body")
		return
	}
	if err = prepareSeedanceAssetPayloadForRoute(c.Request.URL.Path, action.Name, payload, config); err != nil {
		seedanceAssetError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	baseURL, err := seedanceAssetAdminBaseURL(common.GetContextKeyString(c, constant.ContextKeyChannelBaseUrl), config.Region)
	if err != nil {
		seedanceAssetError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	targetURL, err := buildSeedanceAssetTargetURL(baseURL, action)
	if err != nil {
		seedanceAssetError(c, http.StatusInternalServerError, "server_error", "failed to build upstream url")
		return
	}

	requestBody, err := common.Marshal(payload)
	if err != nil {
		seedanceAssetError(c, http.StatusInternalServerError, "server_error", "failed to encode request body")
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		seedanceAssetError(c, http.StatusInternalServerError, "server_error", "failed to create upstream request")
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if err = signSeedanceAssetAdminRequest(req, accessKey, secretKey, config.Region); err != nil {
		seedanceAssetError(c, http.StatusInternalServerError, "server_error", "failed to sign upstream request")
		return
	}

	client, err := service.GetHttpClientWithProxy(config.Proxy)
	if err != nil {
		seedanceAssetError(c, http.StatusInternalServerError, "server_error", "failed to create proxy client")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		seedanceAssetError(c, http.StatusBadGateway, "server_error", "upstream request failed")
		return
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		seedanceAssetError(c, http.StatusBadGateway, "server_error", "failed to read upstream response")
		return
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
}
