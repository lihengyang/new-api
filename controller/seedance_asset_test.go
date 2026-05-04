package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSeedanceAssetTestContext(t *testing.T, target string, body []byte) *gin.Context {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	return ctx
}

func TestResolveSeedanceAssetAdminConfigPrefersContext(t *testing.T) {
	ctx := newSeedanceAssetTestContext(t, "/v1/seedance/virtual/assets/list", []byte(`{}`))
	common.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		ByteplusProjectName:    "project-from-setting",
		ByteplusAssetGroupType: "AIGC",
		ByteplusRegion:         "ap-southeast-1",
		Proxy:                  "http://proxy.example.com",
	})
	common.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		ByteplusProjectName:    "project-from-other",
		ByteplusAssetGroupType: "IGNORED",
		ByteplusRegion:         "cn-beijing",
	})

	config, err := resolveSeedanceAssetAdminConfig(ctx)

	require.NoError(t, err)
	require.Equal(t, "project-from-setting", config.ProjectName)
	require.Equal(t, "AIGC", config.AssetGroupType)
	require.Equal(t, "ap-southeast-1", config.Region)
	require.Equal(t, "http://proxy.example.com", config.Proxy)
}

func TestResolveSeedanceAssetAdminConfigFallsBackToChannelLookup(t *testing.T) {
	ctx := newSeedanceAssetTestContext(t, "/v1/seedance/virtual/assets/list", []byte(`{}`))
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 42)

	originalLoader := seedanceAssetChannelLoader
	t.Cleanup(func() {
		seedanceAssetChannelLoader = originalLoader
	})

	channel := &model.Channel{}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		ByteplusProjectName:    "project-from-cache",
		ByteplusAssetGroupType: "AIGC",
		ByteplusRegion:         "ap-southeast-1",
	})
	seedanceAssetChannelLoader = func(id int) (*model.Channel, error) {
		require.Equal(t, 42, id)
		return channel, nil
	}

	config, err := resolveSeedanceAssetAdminConfig(ctx)

	require.NoError(t, err)
	require.Equal(t, "project-from-cache", config.ProjectName)
	require.Equal(t, "AIGC", config.AssetGroupType)
	require.Equal(t, "ap-southeast-1", config.Region)
}

func TestParseSeedanceAssetAdminKeyRequiresAKSK(t *testing.T) {
	_, _, err := parseSeedanceAssetAdminKey("invalid-key")

	require.EqualError(t, err, "asset admin channel key must use AK|SK format")
}

func TestPrepareSeedanceAssetPayloadUsesChannelProjectNameForClientVariants(t *testing.T) {
	testCases := []struct {
		name            string
		key             string
		expectAliasGone bool
	}{
		{name: "ProjectName", key: "ProjectName", expectAliasGone: false},
		{name: "project_name", key: "project_name", expectAliasGone: true},
		{name: "projectName", key: "projectName", expectAliasGone: true},
		{name: "projectname", key: "projectname", expectAliasGone: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{
				tc.key:    "client-project",
				"GroupId": "group-id",
			}

			err := prepareSeedanceAssetPayload("GetAsset", payload, &seedanceAssetAdminConfig{
				ProjectName: "server-project",
			})

			require.NoError(t, err)
			require.Equal(t, "server-project", payload["ProjectName"])
			if tc.expectAliasGone {
				_, exists := payload[tc.key]
				require.False(t, exists)
			}
		})
	}
}

func TestPrepareSeedanceAssetPayloadRealHumanValidateUsesChannelProjectName(t *testing.T) {
	payload := map[string]any{
		"projectName": "client-project",
		"SessionId":   "session-id",
	}

	err := prepareSeedanceAssetPayloadForRoute(
		"/v1/seedance/real-human/validate-session/create",
		"CreateVisualValidateSession",
		payload,
		&seedanceAssetAdminConfig{
			ProjectName: "server-project",
		},
	)

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	_, exists := payload["projectName"]
	require.False(t, exists)
}

func TestPrepareSeedanceAssetPayloadRequiresProjectName(t *testing.T) {
	payload := map[string]any{
		"ProjectName": "client-project",
	}

	err := prepareSeedanceAssetPayload("GetAsset", payload, &seedanceAssetAdminConfig{})

	require.EqualError(t, err, "byteplus_project_name is required on the selected channel")
}

func TestPrepareSeedanceAssetPayloadCreateAssetGroupDefaultsAIGC(t *testing.T) {
	payload := map[string]any{
		"Name":      "group-name",
		"GroupType": "CLIENT",
	}

	err := prepareSeedanceAssetPayload("CreateAssetGroup", payload, &seedanceAssetAdminConfig{
		ProjectName: "server-project",
	})

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	require.Equal(t, seedanceAssetAdminDefaultGroupType, payload["GroupType"])
}

func TestPrepareSeedanceAssetPayloadListOverridesFilterGroupType(t *testing.T) {
	payload := map[string]any{
		"Filter": map[string]any{
			"GroupType": "client-value",
		},
	}

	err := prepareSeedanceAssetPayload("ListAssets", payload, &seedanceAssetAdminConfig{
		ProjectName:    "server-project",
		AssetGroupType: "AIGC",
	})

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	filter := payload["Filter"].(map[string]any)
	require.Equal(t, seedanceAssetAdminDefaultGroupType, filter["GroupType"])
}

func TestPrepareSeedanceAssetPayloadCreateAssetRequiresURLFields(t *testing.T) {
	payload := map[string]any{
		"GroupId":   "group-id",
		"AssetType": "IMAGE",
	}

	err := prepareSeedanceAssetPayload("CreateAsset", payload, &seedanceAssetAdminConfig{
		ProjectName: "server-project",
	})

	require.EqualError(t, err, "URL is required; base64 upload is not supported")
}

func TestPrepareSeedanceAssetPayloadCreateAssetRejectsBase64Fields(t *testing.T) {
	payload := map[string]any{
		"GroupId":          "group-id",
		"URL":              "https://example.com/image.png",
		"AssetType":        "IMAGE",
		"BinaryDataBase64": "ZmFrZQ==",
	}

	err := prepareSeedanceAssetPayload("CreateAsset", payload, &seedanceAssetAdminConfig{
		ProjectName: "server-project",
	})

	require.EqualError(t, err, "base64 upload is not supported; URL is required")
}

func TestPrepareSeedanceAssetPayloadCreateAssetDoesNotInjectGroupType(t *testing.T) {
	payload := map[string]any{
		"GroupId":   "group-id",
		"URL":       "https://example.com/image.png",
		"AssetType": "IMAGE",
		"GroupType": "client-value",
	}

	err := prepareSeedanceAssetPayload("CreateAsset", payload, &seedanceAssetAdminConfig{
		ProjectName: "server-project",
	})

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	_, exists := payload["GroupType"]
	require.False(t, exists)
}

func TestBuildSeedanceAssetTargetURLUsesActionStyle(t *testing.T) {
	targetURL, err := buildSeedanceAssetTargetURL("https://ark.ap-southeast-1.byteplusapi.com", seedanceAssetAction{
		Name: "ListAssets",
		Path: "/",
	})

	require.NoError(t, err)
	require.Equal(t, "https://ark.ap-southeast-1.byteplusapi.com/?Action=ListAssets&Version=2024-01-01", targetURL)
}

func TestSeedanceAssetActionForRealHumanValidateRoutes(t *testing.T) {
	action, ok := seedanceAssetActionForPath("/v1/seedance/real-human/validate-session/create")
	require.True(t, ok)
	require.Equal(t, "CreateVisualValidateSession", action.Name)

	action, ok = seedanceAssetActionForPath("/v1/seedance/real-human/validate-result/get")
	require.True(t, ok)
	require.Equal(t, "GetVisualValidateResult", action.Name)
}

func TestPrepareSeedanceAssetPayloadForRealHumanListRoutesForcesLivenessFace(t *testing.T) {
	payload := map[string]any{
		"Filter": map[string]any{
			"GroupType": "client-value",
		},
	}

	err := prepareSeedanceAssetPayloadForRoute(
		"/v1/seedance/real-human/assets/list",
		"ListAssets",
		payload,
		&seedanceAssetAdminConfig{
			ProjectName:    "server-project",
			AssetGroupType: "AIGC",
		},
	)

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	filter := payload["Filter"].(map[string]any)
	require.Equal(t, seedanceAssetAdminRealHumanGroupType, filter["GroupType"])
}

func TestPrepareSeedanceAssetPayloadForRealHumanAssetGroupListForcesLivenessFace(t *testing.T) {
	payload := map[string]any{
		"Filter": map[string]any{
			"GroupType": "client-value",
		},
	}

	err := prepareSeedanceAssetPayloadForRoute(
		"/v1/seedance/real-human/asset-groups/list",
		"ListAssetGroups",
		payload,
		&seedanceAssetAdminConfig{
			ProjectName: "server-project",
		},
	)

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	filter := payload["Filter"].(map[string]any)
	require.Equal(t, seedanceAssetAdminRealHumanGroupType, filter["GroupType"])
}

func TestPrepareSeedanceAssetPayloadForRealHumanCreateDoesNotInjectGroupType(t *testing.T) {
	payload := map[string]any{
		"GroupId":   "group-id",
		"URL":       "https://example.com/image.png",
		"AssetType": "IMAGE",
	}

	err := prepareSeedanceAssetPayloadForRoute(
		"/v1/seedance/real-human/assets/create",
		"CreateAsset",
		payload,
		&seedanceAssetAdminConfig{
			ProjectName: "server-project",
		},
	)

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	_, exists := payload["GroupType"]
	require.False(t, exists)
}

func TestPrepareSeedanceAssetPayloadForRealHumanValidateSessionDoesNotInjectGroupType(t *testing.T) {
	payload := map[string]any{
		"SessionName": "session-1",
	}

	err := prepareSeedanceAssetPayloadForRoute(
		"/v1/seedance/real-human/validate-session/create",
		"CreateVisualValidateSession",
		payload,
		&seedanceAssetAdminConfig{
			ProjectName: "server-project",
		},
	)

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	_, exists := payload["GroupType"]
	require.False(t, exists)
	filter, hasFilter := payload["Filter"]
	require.False(t, hasFilter)
	require.Nil(t, filter)
}

func TestPrepareSeedanceAssetPayloadForRealHumanValidateResultDoesNotInjectGroupType(t *testing.T) {
	payload := map[string]any{
		"TaskId": "task-1",
	}

	err := prepareSeedanceAssetPayloadForRoute(
		"/v1/seedance/real-human/validate-result/get",
		"GetVisualValidateResult",
		payload,
		&seedanceAssetAdminConfig{
			ProjectName: "server-project",
		},
	)

	require.NoError(t, err)
	require.Equal(t, "server-project", payload["ProjectName"])
	_, exists := payload["GroupType"]
	require.False(t, exists)
	filter, hasFilter := payload["Filter"]
	require.False(t, hasFilter)
	require.Nil(t, filter)
}

func TestRedactSeedanceAssetResponseBodyRemovesProjectNameVariants(t *testing.T) {
	body := []byte(`{
		"ProjectName": "HenryAPItest",
		"Items": [
			{
				"projectName": "HenryAPItest",
				"Message": "created in HenryAPItest"
			},
			{
				"project_name": "HenryAPItest",
				"Nested": {
					"projectname": "HenryAPItest",
					"Note": "HenryAPItest asset"
				}
			}
		]
	}`)

	redacted := string(redactSeedanceAssetResponseBody(body, "HenryAPItest"))

	require.NotContains(t, redacted, "ProjectName")
	require.NotContains(t, redacted, "projectName")
	require.NotContains(t, redacted, "project_name")
	require.NotContains(t, redacted, "projectname")
	require.NotContains(t, redacted, "HenryAPItest")
	require.Contains(t, redacted, "[REDACTED]")
}

func TestRedactSeedanceAssetResponseBodyReplacesProjectNameInMessages(t *testing.T) {
	body := []byte(`{"Message":"HenryAPItest validation completed","Code":"OK"}`)

	redacted := string(redactSeedanceAssetResponseBody(body, "HenryAPItest"))

	require.NotContains(t, redacted, "HenryAPItest")
	require.Contains(t, redacted, "[REDACTED] validation completed")
	require.Contains(t, redacted, `"Code":"OK"`)
}

func TestRedactSeedanceAssetResponseBodyFallbackRedactsNonJSON(t *testing.T) {
	body := []byte(`upstream error for HenryAPItest`)

	redacted := string(redactSeedanceAssetResponseBody(body, "HenryAPItest"))

	require.Equal(t, "upstream error for [REDACTED]", redacted)
}
