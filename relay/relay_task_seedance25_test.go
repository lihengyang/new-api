package relay

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	corecommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

const seedance25TenantAliasForRelayTest = relaycommon.Seedance25TenantAliasPrefix + "henrytest"

func TestRelayTaskSubmitRejectsSeedance25InvalidRequestsBeforeBilling(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing duration", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"resolution":"720p"}}`},
		{name: "unsupported resolution", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"1080p"}}`},
		{name: "disabled false field", body: `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p","camera_fixed":false}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, info := newTaskClientRequestIDContext(t, tt.body)
			info.OriginModelName = seedance25TenantAliasForRelayTest
			c.Set("model_mapping", `{"`+seedance25TenantAliasForRelayTest+`":"mapped-provider-model-placeholder"}`)

			result, taskErr := RelayTaskSubmit(c, info)

			require.Nil(t, result)
			require.NotNil(t, taskErr)
			require.Equal(t, "invalid_request_error", taskErr.Code)
			require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			require.Nil(t, info.Billing)
			require.False(t, c.Writer.Written())
		})
	}
}

func TestRelayTaskSubmitRejectsSeedance25MissingMappingBeforeBilling(t *testing.T) {
	body := `{"prompt":"p","model":"` + seedance25TenantAliasForRelayTest + `","metadata":{"duration":4,"resolution":"720p"}}`
	c, info := newTaskClientRequestIDContext(t, body)
	info.OriginModelName = seedance25TenantAliasForRelayTest

	result, taskErr := RelayTaskSubmit(c, info)

	require.Nil(t, result)
	require.NotNil(t, taskErr)
	require.Equal(t, "UPSTREAM_MAPPING_MISSING", taskErr.Code)
	require.Equal(t, http.StatusServiceUnavailable, taskErr.StatusCode)
	require.Nil(t, info.Billing)
	require.False(t, c.Writer.Written())
}

func TestSeedance25VideoFetchDoesNotExposePrivateRoutingOrBillingContext(t *testing.T) {
	setupRelayTaskTestDB(t)
	now := time.Now().Unix()
	task := &model.Task{
		TaskID:    "task_public_seedance25",
		UserId:    1001,
		Group:     "test-group",
		ChannelId: 77,
		Platform:  constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeDoubaoVideo)),
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		CreatedAt: now,
		UpdatedAt: now,
		Properties: model.Properties{
			OriginModelName:   seedance25TenantAliasForRelayTest,
			UpstreamModelName: "provider_model_marker",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "provider_task_marker",
			BillingContext: &model.TaskBillingContext{
				OriginModelName:       seedance25TenantAliasForRelayTest,
				BillingFamily:         relaycommon.Seedance25BillingFamily,
				BillingRuleVersion:    "private_rule_marker",
				ModelRatio:            5.35,
				GroupRatio:            1,
				OtherRatioNumerator:   64,
				OtherRatioDenominator: 107,
			},
		},
	}
	var err error
	task.Data, err = corecommon.Marshal(map[string]any{
		"status":  "succeeded",
		"content": map[string]any{"video_url": "https://example.invalid/result.mp4"},
		"usage":   map[string]any{"completion_tokens": 100},
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(task).Error)

	body, taskErr := videoFetchByIDRespBodyBuilder(newVideoFetchContext(1001, "test-group", task.TaskID))
	require.Nil(t, taskErr)
	require.Contains(t, string(body), seedance25TenantAliasForRelayTest)
	require.Contains(t, string(body), `"completion_tokens":100`)
	for _, forbidden := range []string{
		"provider_model_marker",
		"provider_task_marker",
		"private_rule_marker",
		"billing_context",
		"billing_family",
		"channel_id",
		"group_ratio",
		"model_ratio",
		"private_data",
		"ProjectName",
		"upstream_model_name",
	} {
		require.NotContains(t, string(body), forbidden)
	}
}
