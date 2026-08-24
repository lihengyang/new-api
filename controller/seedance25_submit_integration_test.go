package controller

import (
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	seedance25SubmitTestBalance      = 20_000_000
	seedance25MappedModelPlaceholder = "placeholder-upstream-model"
	seedance25TenantAliasForTest     = relaycommon.Seedance25TenantAliasPrefix + "henrytest"
)

var seedance25SubmitSequence atomic.Int64

type seedance25SubmitFixture struct {
	db       *gorm.DB
	context  *gin.Context
	recorder *httptest.ResponseRecorder
	info     *relaycommon.RelayInfo
	userID   int
	tokenID  int
}

func setupSeedance25SubmitFixture(t *testing.T, requestBody []byte, upstreamURL, modelMapping string) *seedance25SubmitFixture {
	t.Helper()

	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldUsingSQLite, oldUsingMySQL, oldUsingPostgreSQL := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldLogConsumeEnabled, oldDataExportEnabled := common.LogConsumeEnabled, common.DataExportEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	oldSQLitePath, oldIsMasterNode := common.SQLitePath, common.IsMasterNode

	id := 50_000 + int(seedance25SubmitSequence.Add(1))
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.LogConsumeEnabled = true
	common.DataExportEnabled = false
	common.BatchUpdateEnabled = false
	common.IsMasterNode = false
	common.SQLitePath = "file:seedance25_submit_" + strconv.Itoa(id) + "?mode=memory&cache=shared"
	t.Setenv("SQL_DSN", "local")
	require.NoError(t, model.InitDB())
	db := model.DB
	model.LOG_DB = db
	ratio_setting.InitRatioSettings()
	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	runtimeModelRatios := ratio_setting.GetModelRatioCopy()
	runtimeModelRatios[seedance25TenantAliasForTest] = 5.35
	runtimeModelRatiosJSON, err := common.Marshal(runtimeModelRatios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(runtimeModelRatiosJSON)))
	service.InitHttpClient()
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.Channel{}, &model.User{}, &model.Token{}, &model.UserSubscription{}, &model.Log{}))

	user := &model.User{
		Id:       id,
		Username: "seedance25-submit-user-placeholder-" + strconv.Itoa(id),
		Password: "placeholder-password",
		Status:   common.UserStatusEnabled,
		Quota:    seedance25SubmitTestBalance,
		Group:    "default",
	}
	token := &model.Token{
		Id:          id,
		UserId:      id,
		Key:         "seedance25-submit-token-placeholder-" + strconv.Itoa(id),
		Name:        "seedance25-submit-token-placeholder",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: seedance25SubmitTestBalance,
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(token).Error)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(string(requestBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("model_mapping", modelMapping)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeDoubaoVideo)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstreamURL)
	common.SetContextKey(c, constant.ContextKeyChannelId, id)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "placeholder-channel-key")

	info := &relaycommon.RelayInfo{
		UserId:          id,
		UserGroup:       "default",
		UsingGroup:      "default",
		TokenId:         id,
		TokenKey:        token.Key,
		OriginModelName: seedance25TenantAliasForTest,
		UserSetting: dto.UserSetting{
			BillingPreference: "wallet_only",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_seedance25_submit_" + strconv.Itoa(id),
		},
	}

	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldUsingSQLite, oldUsingMySQL, oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		common.LogConsumeEnabled, common.DataExportEnabled = oldLogConsumeEnabled, oldDataExportEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
		common.SQLitePath, common.IsMasterNode = oldSQLitePath, oldIsMasterNode
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
	})

	return &seedance25SubmitFixture{
		db:       db,
		context:  c,
		recorder: recorder,
		info:     info,
		userID:   id,
		tokenID:  id,
	}
}

func seedance25SubmitRequestBody(t *testing.T, metadata map[string]any) []byte {
	return seedance25SubmitRequestBodyWithPrompt(t, "safe integration prompt", metadata)
}

func seedance25SubmitRequestBodyWithPrompt(t *testing.T, prompt string, metadata map[string]any) []byte {
	t.Helper()
	body, err := common.Marshal(map[string]any{
		"model":    seedance25TenantAliasForTest,
		"prompt":   prompt,
		"metadata": metadata,
	})
	require.NoError(t, err)
	return body
}

func TestSeedance25RealDoubaoSubmitPreservesPromptlessMediaAndOmittedDefaults(t *testing.T) {
	tests := []struct {
		name              string
		prompt            string
		metadata          map[string]any
		expectDuration    bool
		expectRatio       bool
		expectedMediaType string
	}{
		{
			name:   "promptless explicit audio reference",
			prompt: "",
			metadata: map[string]any{
				"duration": 4, "resolution": "720p", "ratio": "21:9",
				"omni_reference_task_type": "reference",
				"content":                  []any{seedance25SubmitAudio("reference_audio")},
			},
			expectDuration: true, expectRatio: true, expectedMediaType: "audio_url",
		},
		{
			name: "edit omitted duration and ratio",
			metadata: map[string]any{
				"resolution": "720p", "omni_reference_task_type": "edit",
				"content": []any{seedance25SubmitVideo("reference_video")},
			},
			expectedMediaType: "video_url",
		},
		{
			name: "extend omitted ratio",
			metadata: map[string]any{
				"duration": 5, "resolution": "720p", "omni_reference_task_type": "extend",
				"content": []any{seedance25SubmitVideo("reference_video")},
			},
			expectDuration: true, expectedMediaType: "video_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var upstreamBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				var err error
				upstreamBody, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"placeholder-upstream-task"}`)
			}))
			defer server.Close()

			requestBody := seedance25SubmitRequestBodyWithPrompt(t, tt.prompt, tt.metadata)
			mapping := `{"` + seedance25TenantAliasForTest + `":"` + seedance25MappedModelPlaceholder + `"}`
			fixture := setupSeedance25SubmitFixture(t, requestBody, server.URL, mapping)
			result, taskErr := relay.RelayTaskSubmit(fixture.context, fixture.info)
			require.Nil(t, taskErr)
			require.NotNil(t, result)

			payload := decodeSeedance25SubmitPayload(t, upstreamBody)
			_, durationPresent := payload["duration"]
			_, ratioPresent := payload["ratio"]
			require.Equal(t, tt.expectDuration, durationPresent)
			require.Equal(t, tt.expectRatio, ratioPresent)
			content := payload["content"].([]any)
			require.Equal(t, tt.expectedMediaType, content[0].(map[string]any)["type"])
			if tt.prompt == "" {
				require.Len(t, content, 1)
			} else {
				require.Equal(t, "text", content[len(content)-1].(map[string]any)["type"])
			}
		})
	}
}

func seedance25SubmitImage(role string) map[string]any {
	return map[string]any{
		"type":      "image_url",
		"role":      role,
		"image_url": map[string]any{"url": "https://example.invalid/placeholder.png"},
	}
}

func seedance25SubmitVideo(role string) map[string]any {
	return map[string]any{
		"type":      "video_url",
		"role":      role,
		"video_url": map[string]any{"url": "https://example.invalid/placeholder.mp4"},
	}
}

func seedance25SubmitAudio(role string) map[string]any {
	return map[string]any{
		"type":      "audio_url",
		"role":      role,
		"audio_url": map[string]any{"url": "https://example.invalid/placeholder.wav"},
	}
}

func decodeSeedance25SubmitPayload(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, common.Unmarshal(body, &payload))
	return payload
}

func requireSeedance25SubmitQuotaUnchanged(t *testing.T, fixture *seedance25SubmitFixture) {
	t.Helper()
	var user model.User
	var token model.Token
	require.NoError(t, fixture.db.First(&user, fixture.userID).Error)
	require.NoError(t, fixture.db.First(&token, fixture.tokenID).Error)
	require.Equal(t, seedance25SubmitTestBalance, user.Quota)
	require.Equal(t, seedance25SubmitTestBalance, token.RemainQuota)
	require.Zero(t, token.UsedQuota)
}

func TestSeedance25RealDoubaoSubmitChainPersistsUpstreamIDAndReturnsPublicIdentity(t *testing.T) {
	tests := []struct {
		name                string
		metadata            map[string]any
		expectedRoles       []string
		expectedRatio       string
		expectedNumerator   int64
		expectedDenominator int64
	}{
		{
			name:          "text only",
			metadata:      map[string]any{"duration": 4, "resolution": "480p", "ratio": "16:9", "generate_audio": false},
			expectedRatio: "16:9", expectedNumerator: 1, expectedDenominator: 1,
		},
		{
			name:          "native 1080p text only",
			metadata:      map[string]any{"duration": 4, "resolution": "1080p", "ratio": "16:9", "generate_audio": false},
			expectedRatio: "16:9", expectedNumerator: 117, expectedDenominator: 107,
		},
		{
			name:          "first frame",
			metadata:      map[string]any{"duration": 4, "resolution": "720p", "content": []any{seedance25SubmitImage("first_frame")}, "generate_audio": false},
			expectedRoles: []string{"first_frame"}, expectedRatio: "adaptive", expectedNumerator: 1, expectedDenominator: 1,
		},
		{
			name:          "first and last frame",
			metadata:      map[string]any{"duration": 30, "resolution": "720p", "content": []any{seedance25SubmitImage("first_frame"), seedance25SubmitImage("last_frame")}, "generate_audio": false},
			expectedRoles: []string{"first_frame", "last_frame"}, expectedRatio: "adaptive", expectedNumerator: 1, expectedDenominator: 1,
		},
		{
			name:          "reference image",
			metadata:      map[string]any{"duration": 4, "resolution": "480p", "ratio": "16:9", "content": []any{seedance25SubmitImage("reference_image")}, "generate_audio": false},
			expectedRoles: []string{"reference_image"}, expectedRatio: "16:9", expectedNumerator: 1, expectedDenominator: 1,
		},
		{
			name:          "reference video",
			metadata:      map[string]any{"duration": 30, "resolution": "720p", "ratio": "adaptive", "content": []any{seedance25SubmitVideo("reference_video")}, "generate_audio": false},
			expectedRoles: []string{"reference_video"}, expectedRatio: "adaptive", expectedNumerator: 64, expectedDenominator: 107,
		},
		{
			name:          "native 1080p reference video",
			metadata:      map[string]any{"duration": 4, "resolution": "1080p", "ratio": "adaptive", "content": []any{seedance25SubmitVideo("reference_video")}, "generate_audio": false},
			expectedRoles: []string{"reference_video"}, expectedRatio: "adaptive", expectedNumerator: 70, expectedDenominator: 107,
		},
		{
			name: "mixed editing references with p1 output controls",
			metadata: map[string]any{
				"duration": -1, "resolution": "1080p", "ratio": "adaptive", "omni_reference_task_type": "edit",
				"content": []any{
					seedance25SubmitImage("reference_image"),
					seedance25SubmitVideo("reference_video"),
					seedance25SubmitAudio("reference_audio"),
				},
				"generate_audio": false, "output_format": "mov", "return_last_frame": true, "watermark": false,
			},
			expectedRoles: []string{"reference_image", "reference_video", "reference_audio"}, expectedRatio: "adaptive", expectedNumerator: 70, expectedDenominator: 107,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var upstreamBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/api/v3/contents/generations/tasks", r.URL.Path)
				var err error
				upstreamBody, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"placeholder-upstream-task"}`)
			}))
			defer server.Close()

			requestBody := seedance25SubmitRequestBody(t, tt.metadata)
			mapping := `{"` + seedance25TenantAliasForTest + `":"  ` + seedance25MappedModelPlaceholder + `  "}`
			fixture := setupSeedance25SubmitFixture(t, requestBody, server.URL, mapping)

			result, taskErr := relay.RelayTaskSubmit(fixture.context, fixture.info)
			require.Nil(t, taskErr)
			require.NotNil(t, result)
			require.Nil(t, handleTaskSubmitSuccess(fixture.context, fixture.info, result))

			payload := decodeSeedance25SubmitPayload(t, upstreamBody)
			require.Equal(t, seedance25MappedModelPlaceholder, payload["model"])
			require.NotEqual(t, seedance25TenantAliasForTest, payload["model"])
			require.Equal(t, tt.metadata["resolution"], payload["resolution"])
			require.Equal(t, false, payload["generate_audio"])
			require.Equal(t, tt.expectedRatio, payload["ratio"])
			require.Equal(t, float64(tt.metadata["duration"].(int)), payload["duration"])
			require.NotContains(t, payload, "seed")
			require.NotContains(t, payload, "camera_fixed")
			require.NotContains(t, payload, "frames")
			require.NotContains(t, payload, "draft")
			require.NotContains(t, payload, "service_tier")
			for _, field := range []string{"omni_reference_task_type", "output_format", "return_last_frame", "watermark"} {
				expected, present := tt.metadata[field]
				if present {
					require.Equal(t, expected, payload[field])
				} else {
					require.NotContains(t, payload, field)
				}
			}
			require.NotContains(t, payload, "priority")

			content, ok := payload["content"].([]any)
			require.True(t, ok)
			require.Len(t, content, len(tt.expectedRoles)+1)
			for index, role := range tt.expectedRoles {
				item, itemOK := content[index].(map[string]any)
				require.True(t, itemOK)
				require.Equal(t, role, item["role"])
			}
			textItem, textOK := content[len(content)-1].(map[string]any)
			require.True(t, textOK)
			require.Equal(t, "text", textItem["type"])

			var task model.Task
			require.NoError(t, fixture.db.Where("task_id = ?", fixture.info.PublicTaskID).First(&task).Error)
			require.Equal(t, result.Quota, task.Quota)
			require.Equal(t, seedance25TenantAliasForTest, task.Properties.OriginModelName)
			require.NotNil(t, task.PrivateData.BillingContext)
			require.Equal(t, 5.35, task.PrivateData.BillingContext.ModelRatio)
			require.Equal(t, 1.0, task.PrivateData.BillingContext.GroupRatio)
			require.Equal(t, relaycommon.Seedance25BillingFamily, task.PrivateData.BillingContext.BillingFamily)
			require.Equal(t, tt.expectedNumerator, task.PrivateData.BillingContext.OtherRatioNumerator)
			require.Equal(t, tt.expectedDenominator, task.PrivateData.BillingContext.OtherRatioDenominator)
			require.Equal(t, "placeholder-upstream-task", task.PrivateData.UpstreamTaskID)
			var submittedTaskData map[string]any
			require.NoError(t, common.Unmarshal(task.Data, &submittedTaskData))
			require.Equal(t, "placeholder-upstream-task", submittedTaskData["id"])
			require.NotContains(t, string(task.Data), seedance25MappedModelPlaceholder)
			require.NotContains(t, fixture.recorder.Body.String(), "placeholder-upstream-task")
			require.NotContains(t, fixture.recorder.Body.String(), seedance25MappedModelPlaceholder)
			require.Contains(t, fixture.recorder.Body.String(), seedance25TenantAliasForTest)

			var logs []model.Log
			require.NoError(t, fixture.db.Where("user_id = ?", fixture.userID).Find(&logs).Error)
			require.Len(t, logs, 1)
			require.Equal(t, seedance25TenantAliasForTest, logs[0].ModelName)
			require.NotContains(t, logs[0].Content, seedance25MappedModelPlaceholder)
			require.NotContains(t, logs[0].Other, seedance25MappedModelPlaceholder)

			var user model.User
			var token model.Token
			require.NoError(t, fixture.db.First(&user, fixture.userID).Error)
			require.NoError(t, fixture.db.First(&token, fixture.tokenID).Error)
			require.Equal(t, seedance25SubmitTestBalance-result.Quota, user.Quota)
			require.Equal(t, seedance25SubmitTestBalance-result.Quota, token.RemainQuota)
			require.Equal(t, result.Quota, token.UsedQuota)
		})
	}
}

func TestSeedance25RealDoubaoSubmitRejects4KBeforeTaskBillingAndUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	requestBody := seedance25SubmitRequestBody(t, map[string]any{"duration": 4, "resolution": "4k"})
	mapping := `{"` + seedance25TenantAliasForTest + `":"` + seedance25MappedModelPlaceholder + `"}`
	fixture := setupSeedance25SubmitFixture(t, requestBody, server.URL, mapping)

	result, taskErr := relay.RelayTaskSubmit(fixture.context, fixture.info)
	require.Nil(t, result)
	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_request_error", taskErr.Code)
	require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	require.Nil(t, fixture.info.Billing)
	require.Zero(t, upstreamCalls.Load())
	requireSeedance25SubmitQuotaUnchanged(t, fixture)

	var taskCount int64
	var logCount int64
	require.NoError(t, fixture.db.Model(&model.Task{}).Count(&taskCount).Error)
	require.NoError(t, fixture.db.Model(&model.Log{}).Count(&logCount).Error)
	require.Zero(t, taskCount)
	require.Zero(t, logCount)
}

func TestSeedance25RealDoubaoSubmitRejectsMissingOrBlankMappingBeforeBilling(t *testing.T) {
	tests := []struct {
		name    string
		mapping string
	}{
		{name: "missing", mapping: ""},
		{name: "bare global key", mapping: `{"seedance-2.5":"placeholder-upstream-model"}`},
		{name: "blank", mapping: `{"lsf-seedance-2.5-henrytest":"   "}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var upstreamCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamCalls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			requestBody := seedance25SubmitRequestBody(t, map[string]any{"duration": 4, "resolution": "720p"})
			fixture := setupSeedance25SubmitFixture(t, requestBody, server.URL, tt.mapping)

			result, taskErr := relay.RelayTaskSubmit(fixture.context, fixture.info)
			require.Nil(t, result)
			require.NotNil(t, taskErr)
			require.Equal(t, "UPSTREAM_MAPPING_MISSING", taskErr.Code)
			require.Equal(t, http.StatusServiceUnavailable, taskErr.StatusCode)
			require.Nil(t, fixture.info.Billing)
			require.Zero(t, upstreamCalls.Load())
			requireSeedance25SubmitQuotaUnchanged(t, fixture)
		})
	}
}

func TestSeedance25RealDoubaoSubmitRejectsNonFiniteGroupRatioBeforeBilling(t *testing.T) {
	groupRatios := ratio_setting.GetGroupRatioSetting().GroupRatio
	previous, existed := groupRatios.Get("default")
	require.True(t, existed)
	groupRatios.Set("default", math.NaN())
	t.Cleanup(func() { groupRatios.Set("default", previous) })

	var upstreamCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	requestBody := seedance25SubmitRequestBody(t, map[string]any{"duration": 4, "resolution": "720p"})
	mapping := `{"` + seedance25TenantAliasForTest + `":"` + seedance25MappedModelPlaceholder + `"}`
	fixture := setupSeedance25SubmitFixture(t, requestBody, server.URL, mapping)

	result, taskErr := relay.RelayTaskSubmit(fixture.context, fixture.info)
	require.Nil(t, result)
	require.NotNil(t, taskErr)
	require.Equal(t, "billing_configuration_error", taskErr.Code)
	require.Nil(t, fixture.info.Billing)
	require.Zero(t, upstreamCalls.Load())
	requireSeedance25SubmitQuotaUnchanged(t, fixture)
}
