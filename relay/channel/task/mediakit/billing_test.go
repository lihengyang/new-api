package mediakit

import (
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestMediaKitRateCardAllP0Combinations(t *testing.T) {
	tests := []struct {
		tool       string
		resolution string
		fps        float64
		expected   string
	}{
		{"standard", "1080p", 30, "0.4132"},
		{"standard", "1080p", 60, "0.8264"},
		{"standard", "1080p", 120, "1.6528"},
		{"standard", "4k", 30, "1.6528"},
		{"standard", "4k", 60, "3.3056"},
		{"standard", "4k", 120, "6.6112"},
		{"professional", "1080p", 30, "4.1322"},
		{"professional", "1080p", 60, "8.2644"},
		{"professional", "1080p", 120, "16.5288"},
		{"professional", "4k", 30, "16.5288"},
		{"professional", "4k", 60, "33.0576"},
		{"professional", "4k", 120, "66.1152"},
	}
	for _, test := range tests {
		rate, ok := rateFor(test.tool, test.resolution, test.fps)
		require.True(t, ok)
		require.Equal(t, test.expected, rate.StringFixed(4))
	}
}

func TestMediaKitFPSTierBoundaries(t *testing.T) {
	tests := []struct {
		fps      float64
		tier     int
		accepted bool
	}{
		{30, 0, true}, {30.168, 1, true}, {60, 1, true},
		{60.001, 2, true}, {120, 2, true}, {120.001, 0, false},
	}
	for _, test := range tests {
		tier, accepted := fpsTier(test.fps)
		require.Equal(t, test.accepted, accepted)
		if accepted {
			require.Equal(t, test.tier, tier)
		}
	}
}

func TestMediaKitDeclaredDurationRoundsUpForPrecharge(t *testing.T) {
	rate, ok := rateFor("standard", "1080p", 30)
	require.True(t, ok)
	for _, test := range []struct {
		duration float64
		seconds  int64
	}{{0.1, 1}, {1.0, 1}, {1.01, 2}, {5.967, 6}} {
		quota, ok := quotaFor(rate, int64(math.Ceil(test.duration)), 1)
		require.True(t, ok)
		expected := rate.Div(decimal.NewFromInt(60)).Mul(decimal.NewFromInt(test.seconds)).Mul(decimal.NewFromInt(int64(common.QuotaPerUnit))).Floor().IntPart()
		require.EqualValues(t, expected, quota)
	}
}

func TestMediaKitMissingFPSUsesHighestPrechargeTier(t *testing.T) {
	provided, ok := rateFor("standard", "1080p", 30.168)
	require.True(t, ok)
	missing, ok := rateFor("standard", "1080p", 120)
	require.True(t, ok)
	require.Equal(t, "0.8264", provided.StringFixed(4))
	require.Equal(t, "1.6528", missing.StringFixed(4))
}

func TestMediaKitGroupRatioAppliedExactlyOnce(t *testing.T) {
	rate, ok := rateFor("professional", "4k", 60.001)
	require.True(t, ok)
	quota, ok := quotaFor(rate, 6, 1.75)
	require.True(t, ok)
	expected := rate.Div(decimal.NewFromInt(60)).Mul(decimal.NewFromInt(6)).Mul(decimal.NewFromFloat(1.75)).Mul(decimal.NewFromInt(int64(common.QuotaPerUnit))).Floor().IntPart()
	require.EqualValues(t, expected, quota)
}

func mediaKitBillingTask(precharge int, declaredDuration float64) *model.Task {
	return &model.Task{
		Quota:      precharge,
		Properties: model.Properties{UpstreamModelName: CanonicalModel},
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
			GroupRatio:         1,
			BillingFamily:      BillingFamily,
			BillingRuleVersion: BillingRuleVersion,
			BillingMetadata: map[string]any{
				"declared_duration": declaredDuration,
				"tool_version":      "standard",
				"resolution_tier":   "1080p",
			},
		}},
	}
}

func TestMediaKitCompletionPolicyAuditsActualDurationAndSanitizes(t *testing.T) {
	task := mediaKitBillingTask(13773, 1.01)
	result := &relaycommon.TaskInfo{
		Status: string(model.TaskStatusSuccess), Duration: 1.2, FPS: 30.168,
		Resolution: "1080p", ToolVersion: "standard", Url: "https://signed.example/result?signature=secret", ExpiresAt: 2_000_000_000,
	}
	persisted, err := (&TaskAdaptor{}).ApplyTaskResultPolicy(task, result, []byte(`{"raw":"must-not-persist"}`))
	require.NoError(t, err)
	require.NotContains(t, string(persisted), "signed.example")
	require.NotContains(t, string(persisted), "must-not-persist")
	metadata := task.PrivateData.BillingContext.BillingMetadata
	require.Equal(t, 1.2, metadata["actual_duration"])
	require.InDelta(t, 0.19, metadata["duration_difference"], 1e-9)
	require.EqualValues(t, 2, metadata["actual_seconds"])
	require.Equal(t, 30.168, metadata["actual_fps"])
	actualQuota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, result)
	require.Greater(t, actualQuota, 0)
}

func TestMediaKitCompletionPolicyRejectsCriticalTierConflict(t *testing.T) {
	task := mediaKitBillingTask(1000, 1)
	result := &relaycommon.TaskInfo{Status: string(model.TaskStatusSuccess), Duration: 1, FPS: 30, Resolution: "4k", ToolVersion: "standard"}
	_, err := (&TaskAdaptor{}).ApplyTaskResultPolicy(task, result, nil)
	require.Error(t, err)
}

func TestMediaKitFailureDetailPersistsAndReturnsInStableErrorShape(t *testing.T) {
	task := mediaKitBillingTask(1000, 1)
	task.TaskID = "task_public"
	result := &relaycommon.TaskInfo{
		Status: string(model.TaskStatusFailure), UpstreamErrorCode: "INVALID_ARGUMENT",
		Reason: "resolution 1920 is unsupported; source=https://private.example/input.mp4?token=secret",
	}
	persisted, err := (&TaskAdaptor{}).ApplyTaskResultPolicy(task, result, []byte(`{"raw":"must-not-persist"}`))
	require.NoError(t, err)
	require.Contains(t, string(persisted), "resolution 1920 is unsupported")
	require.NotContains(t, string(persisted), "private.example")
	require.NotContains(t, string(persisted), "must-not-persist")

	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.Data = persisted
	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &video))
	require.NotNil(t, video.Error)
	require.Equal(t, "INVALID_ARGUMENT", video.Error.Code)
	require.Equal(t, result.Reason, video.Error.Message)
	require.NotContains(t, string(body), "private.example")
}

func TestMediaKitCompletedResponseReturnsURLOnlyBeforeExpiry(t *testing.T) {
	task := mediaKitBillingTask(1000, 1)
	task.TaskID = "task_public"
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.Properties.OriginModelName = "lsf-video-enhancement-tenant"
	adaptor := &TaskAdaptor{}
	for _, test := range []struct {
		name      string
		expiresAt int64
		hasURL    bool
	}{
		{name: "active", expiresAt: time.Now().Add(time.Hour).Unix(), hasURL: true},
		{name: "expired", expiresAt: time.Now().Add(-time.Hour).Unix(), hasURL: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := adaptor.ConvertToOpenAIVideoWithResult(task, &relaycommon.TaskInfo{
				Duration: 1, FPS: 30, Resolution: "1080p", ToolVersion: "standard",
				Url: "https://signed.example/result?signature=secret", ExpiresAt: test.expiresAt,
			})
			require.NoError(t, err)
			var video dto.OpenAIVideo
			require.NoError(t, common.Unmarshal(body, &video))
			require.Equal(t, test.expiresAt, video.ExpiresAt)
			_, hasURL := video.Metadata["video_url"]
			require.Equal(t, test.hasURL, hasURL)
		})
	}
}
