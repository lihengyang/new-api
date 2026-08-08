package doubao

import (
	"io"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func seedance25TerminalTask(hasVideo bool) *model.Task {
	numerator, denominator := int64(1), int64(1)
	otherRatio := 1.0
	if hasVideo {
		numerator, denominator = 64, 107
		otherRatio = 64.0 / 107.0
	}
	return &model.Task{
		TaskID: "task_public_seedance25",
		Quota:  700,
		Status: model.TaskStatusInProgress,
		Properties: model.Properties{
			OriginModelName: seedance25TenantAliasForDoubaoTest,
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "provider_task_marker",
			BillingContext: &model.TaskBillingContext{
				OriginModelName:       seedance25TenantAliasForDoubaoTest,
				BillingFamily:         relaycommon.Seedance25BillingFamily,
				BillingRuleVersion:    seedance25BillingRuleVersion,
				ModelRatio:            seedance25ModelRatio,
				GroupRatio:            1,
				OtherRatios:           map[string]float64{"seedance_intl_billing": otherRatio},
				OtherRatioNumerator:   numerator,
				OtherRatioDenominator: denominator,
			},
		},
	}
}

func TestResolveSeedance25BillingUsesExactTenantOriginAliasOnly(t *testing.T) {
	for _, origin := range []string{seedance25TenantAliasForDoubaoTest, seedance25SecondTenantAliasForDoubaoTest} {
		t.Run(origin, func(t *testing.T) {
			ctx, matched, err := ResolveSeedanceIntlBilling(
				origin,
				"mapped-provider-model",
				map[string]any{"resolution": "720p", "duration": 4},
			)
			require.NoError(t, err)
			require.True(t, matched)
			require.Equal(t, relaycommon.Seedance25BillingFamily, ctx.Family)
		})
	}

	for _, origin := range []string{"seedance-2.5", "seedance-2.50", "Seedance-2.5", "seedance-2.0"} {
		t.Run(origin, func(t *testing.T) {
			billing, ok, resolveErr := ResolveSeedanceIntlBilling(origin, "mapped-seedance-2.5-provider-name", map[string]any{"resolution": "720p"})
			require.NoError(t, resolveErr)
			if origin == "seedance-2.0" {
				require.True(t, ok)
				require.Equal(t, seedanceBillingFamilyStandard, billing.Family)
				return
			}
			require.False(t, ok)
			require.Nil(t, billing)
		})
	}
}

func TestResolveSeedance25BillingVideoAndNoVideoRatios(t *testing.T) {
	tests := []struct {
		name          string
		metadata      map[string]any
		inputType     string
		expectedRatio float64
	}{
		{name: "text only", metadata: map[string]any{"resolution": "720p", "duration": 4}, inputType: seedanceBillingInputNoVideo, expectedRatio: 1},
		{name: "reference image", metadata: map[string]any{"resolution": "720p", "duration": 4, "content": []any{seedance25Image("reference_image")}}, inputType: seedanceBillingInputNoVideo, expectedRatio: 1},
		{name: "reference video", metadata: map[string]any{"resolution": "720p", "duration": 4, "content": []any{seedance25Video("reference_video")}}, inputType: seedanceBillingInputVideo, expectedRatio: 64.0 / 107.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, matched, err := ResolveSeedanceIntlBilling(seedance25TenantAliasForDoubaoTest, "mapped-provider-model", tt.metadata)
			require.NoError(t, err)
			require.True(t, matched)
			require.Equal(t, tt.inputType, ctx.InputType)
			require.InDelta(t, tt.expectedRatio, ctx.Ratio, 0.000000000001)
			require.InDelta(t, 0.0107*tt.expectedRatio, ctx.UnitPriceUsdPerK, 0.000000000001)
			if tt.inputType == seedanceBillingInputVideo {
				require.InDelta(t, 3.20, seedance25ModelRatio*ctx.Ratio, 0.000000000001)
			}
		})
	}
}

func TestSeedance25PriceDataValidationAndRatioSnapshot(t *testing.T) {
	c := newDoubaoRequestContext(t, relaycommon.TaskSubmitReq{
		Model:  seedance25TenantAliasForDoubaoTest,
		Prompt: "p",
		Metadata: map[string]any{
			"duration":   4,
			"resolution": "720p",
			"content":    []any{seedance25Video("reference_video")},
		},
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: seedance25TenantAliasForDoubaoTest,
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "mapped-provider-model",
		},
		PriceData: types.PriceData{
			ModelRatio: seedance25ModelRatio,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}

	ratioMap := (&TaskAdaptor{}).EstimateBilling(c, info)
	for key, value := range ratioMap {
		info.PriceData.AddOtherRatio(key, value)
	}
	require.Nil(t, (&TaskAdaptor{}).ValidatePriceData(c, info))
	require.InDelta(t, 64.0/107.0, ratioMap["seedance_intl_billing"], 0.000000000001)
	require.Equal(t, relaycommon.Seedance25BillingFamily, info.PriceData.BillingFamily)
	require.Equal(t, seedance25BillingRuleVersion, info.PriceData.BillingRuleVersion)
	require.EqualValues(t, 64, info.PriceData.OtherRatioNumerator)
	require.EqualValues(t, 107, info.PriceData.OtherRatioDenominator)

	info.PriceData.ModelRatio = 5.34
	require.NotNil(t, (&TaskAdaptor{}).ValidatePriceData(c, info))
	info.PriceData.ModelRatio = seedance25ModelRatio
	info.PriceData.UsePrice = true
	require.NotNil(t, (&TaskAdaptor{}).ValidatePriceData(c, info))
}

func TestSeedance25PriceDataRejectsNonFiniteZeroNegativeAndLooseModelRatios(t *testing.T) {
	base := types.PriceData{
		ModelRatio: seedance25ModelRatio,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio: 1,
		},
		OtherRatios:           map[string]float64{"seedance_intl_billing": 1},
		BillingFamily:         relaycommon.Seedance25BillingFamily,
		BillingRuleVersion:    seedance25BillingRuleVersion,
		OtherRatioNumerator:   1,
		OtherRatioDenominator: 1,
	}
	invalidValues := map[string]float64{
		"nan":      math.NaN(),
		"positive": math.Inf(1),
		"negative": math.Inf(-1),
		"zero":     0,
		"below":    -1,
	}

	for name, invalidValue := range invalidValues {
		t.Run("model_ratio_"+name, func(t *testing.T) {
			priceData := base
			priceData.ModelRatio = invalidValue
			info := &relaycommon.RelayInfo{OriginModelName: seedance25TenantAliasForDoubaoTest, PriceData: priceData}
			require.NotPanics(t, func() {
				require.NotNil(t, (&TaskAdaptor{}).ValidatePriceData(nil, info))
			})
		})
		t.Run("group_ratio_"+name, func(t *testing.T) {
			priceData := base
			priceData.GroupRatioInfo.GroupRatio = invalidValue
			info := &relaycommon.RelayInfo{OriginModelName: seedance25TenantAliasForDoubaoTest, PriceData: priceData}
			require.NotPanics(t, func() {
				require.NotNil(t, (&TaskAdaptor{}).ValidatePriceData(nil, info))
			})
		})
		t.Run("other_ratio_"+name, func(t *testing.T) {
			priceData := base
			priceData.OtherRatios = map[string]float64{"seedance_intl_billing": invalidValue}
			info := &relaycommon.RelayInfo{OriginModelName: seedance25TenantAliasForDoubaoTest, PriceData: priceData}
			require.NotPanics(t, func() {
				require.NotNil(t, (&TaskAdaptor{}).ValidatePriceData(nil, info))
			})
		})
	}

	loose := base
	loose.ModelRatio = math.Nextafter(seedance25ModelRatio, math.Inf(1))
	info := &relaycommon.RelayInfo{OriginModelName: seedance25TenantAliasForDoubaoTest, PriceData: loose}
	require.NotNil(t, (&TaskAdaptor{}).ValidatePriceData(nil, info))
}

func TestSeedance25ReservationUsesConservativeThirtySecondVideoCeilingAndCeil(t *testing.T) {
	tests := []struct {
		name           string
		resolution     string
		duration       int
		video          bool
		expectedTokens int
		expectedQuota  int
	}{
		{name: "480p four seconds rounds up", resolution: "480p", duration: 4, expectedTokens: 40176, expectedQuota: 214942},
		{name: "720p four seconds", resolution: "720p", duration: 4, expectedTokens: 86945, expectedQuota: 465156},
		{name: "720p thirty seconds", resolution: "720p", duration: 30, expectedTokens: 652084, expectedQuota: 3488650},
		{name: "720p video reserves thirty second input", resolution: "720p", duration: 4, video: true, expectedTokens: 739029, expectedQuota: 2364893},
		{name: "720p video plus thirty second output", resolution: "720p", duration: 30, video: true, expectedTokens: 1304168, expectedQuota: 4173338},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := map[string]any{"resolution": tt.resolution, "duration": tt.duration}
			if tt.video {
				metadata["content"] = []any{seedance25Video("reference_video")}
			}
			c := newDoubaoRequestContext(t, relaycommon.TaskSubmitReq{Model: seedance25TenantAliasForDoubaoTest, Prompt: "p", Metadata: metadata})
			info := &relaycommon.RelayInfo{
				OriginModelName: seedance25TenantAliasForDoubaoTest,
				ChannelMeta: &relaycommon.ChannelMeta{
					IsModelMapped:     true,
					UpstreamModelName: "mapped-provider-model",
				},
				PriceData: types.PriceData{
					ModelRatio: seedance25ModelRatio,
					GroupRatioInfo: types.GroupRatioInfo{
						GroupRatio: 1,
					},
				},
			}
			adaptor := &TaskAdaptor{}
			for key, value := range adaptor.EstimateBilling(c, info) {
				info.PriceData.AddOtherRatio(key, value)
			}
			maxPixels, ok := seedance25ReservationPixelCeiling(tt.resolution)
			require.True(t, ok)
			inputDuration := 0
			if tt.video {
				inputDuration = seedance25MaxDurationSeconds
			}
			estimatedTokens := int(math.Ceil(float64(inputDuration+tt.duration) * float64(maxPixels) * seedance25ReservationFPS / 1024))
			require.Equal(t, tt.expectedTokens, estimatedTokens)
			quota, ok := adaptor.EstimatePrechargeQuota(c, info)
			require.True(t, ok)
			require.Equal(t, tt.expectedQuota, quota)
		})
	}
}

func TestSeedance25ReservationPixelCeilingsCoverOfficialDimensions(t *testing.T) {
	tests := []struct {
		resolution string
		dimensions [][2]int
		ceiling    int
	}{
		{
			resolution: "480p",
			dimensions: [][2]int{{854, 480}, {752, 560}, {640, 640}, {560, 752}, {480, 854}, {992, 432}},
			ceiling:    428544,
		},
		{
			resolution: "720p",
			dimensions: [][2]int{{1280, 720}, {1112, 834}, {960, 960}, {834, 1112}, {720, 1280}, {1470, 630}},
			ceiling:    927408,
		},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			ceiling, ok := seedance25ReservationPixelCeiling(tt.resolution)
			require.True(t, ok)
			require.Equal(t, tt.ceiling, ceiling)
			maximumPixels := 0
			for _, dimensions := range tt.dimensions {
				pixels := dimensions[0] * dimensions[1]
				require.LessOrEqual(t, pixels, ceiling)
				if pixels > maximumPixels {
					maximumPixels = pixels
				}
			}
			require.Equal(t, ceiling, maximumPixels)
		})
	}

	_, ok := seedance25ReservationPixelCeiling("1080p")
	require.False(t, ok)
}

// This test proves that the current reservation code uses the same conservative
// ceiling for adaptive and 16:9. It does not prove the actual upstream output
// geometry for arbitrary first-frame or first+last source-image ratios.
func TestSeedance25AdaptiveAnd16By9UseSameReservationCeiling(t *testing.T) {
	tests := []struct {
		resolution  string
		fixedWidth  int
		fixedHeight int
	}{
		{resolution: "480p", fixedWidth: 854, fixedHeight: 480},
		{resolution: "720p", fixedWidth: 1280, fixedHeight: 720},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			ceiling, ok := seedance25ReservationPixelCeiling(tt.resolution)
			require.True(t, ok)
			require.GreaterOrEqual(t, ceiling, tt.fixedWidth*tt.fixedHeight)

			reservationForRatio := func(ratio string) int {
				t.Helper()
				c := newDoubaoRequestContext(t, relaycommon.TaskSubmitReq{
					Model:  seedance25TenantAliasForDoubaoTest,
					Prompt: "p",
					Metadata: map[string]any{
						"duration":   4,
						"resolution": tt.resolution,
						"ratio":      ratio,
					},
				})
				info := &relaycommon.RelayInfo{
					OriginModelName: seedance25TenantAliasForDoubaoTest,
					ChannelMeta: &relaycommon.ChannelMeta{
						IsModelMapped:     true,
						UpstreamModelName: "placeholder-upstream-model",
					},
					PriceData: types.PriceData{
						ModelRatio: seedance25ModelRatio,
						GroupRatioInfo: types.GroupRatioInfo{
							GroupRatio: 1,
						},
					},
				}
				adaptor := &TaskAdaptor{}
				for key, value := range adaptor.EstimateBilling(c, info) {
					info.PriceData.AddOtherRatio(key, value)
				}
				quota, ok := adaptor.EstimatePrechargeQuota(c, info)
				require.True(t, ok)
				return quota
			}

			fixedQuota := reservationForRatio("16:9")
			adaptiveQuota := reservationForRatio("adaptive")
			require.Equal(t, fixedQuota, adaptiveQuota)
		})
	}
}

func TestSeedance25ReservationRejectsNonFiniteZeroAndNegativeRatiosWithoutPanic(t *testing.T) {
	c := newDoubaoRequestContext(t, relaycommon.TaskSubmitReq{
		Model:  seedance25TenantAliasForDoubaoTest,
		Prompt: "p",
		Metadata: map[string]any{
			"duration":   4,
			"resolution": "720p",
		},
	})
	newInfo := func() *relaycommon.RelayInfo {
		info := &relaycommon.RelayInfo{
			OriginModelName: seedance25TenantAliasForDoubaoTest,
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: "placeholder-upstream-model",
				IsModelMapped:     true,
			},
			PriceData: types.PriceData{
				ModelRatio: seedance25ModelRatio,
				GroupRatioInfo: types.GroupRatioInfo{
					GroupRatio: 1,
				},
			},
		}
		adaptor := &TaskAdaptor{}
		for key, value := range adaptor.EstimateBilling(c, info) {
			info.PriceData.AddOtherRatio(key, value)
		}
		return info
	}
	invalidValues := map[string]float64{
		"nan":          math.NaN(),
		"positive_inf": math.Inf(1),
		"negative_inf": math.Inf(-1),
		"zero":         0,
		"negative":     -1,
	}
	for name, invalidValue := range invalidValues {
		t.Run("model_ratio_"+name, func(t *testing.T) {
			info := newInfo()
			info.PriceData.ModelRatio = invalidValue
			require.NotPanics(t, func() {
				quota, ok := (&TaskAdaptor{}).EstimatePrechargeQuota(c, info)
				require.False(t, ok)
				require.Zero(t, quota)
			})
		})
		t.Run("group_ratio_"+name, func(t *testing.T) {
			info := newInfo()
			info.PriceData.GroupRatioInfo.GroupRatio = invalidValue
			require.NotPanics(t, func() {
				quota, ok := (&TaskAdaptor{}).EstimatePrechargeQuota(c, info)
				require.False(t, ok)
				require.Zero(t, quota)
			})
		})
		t.Run("other_ratio_"+name, func(t *testing.T) {
			info := newInfo()
			info.PriceData.OtherRatios["seedance_intl_billing"] = invalidValue
			require.NotPanics(t, func() {
				quota, ok := (&TaskAdaptor{}).EstimatePrechargeQuota(c, info)
				require.False(t, ok)
				require.Zero(t, quota)
			})
		})
	}
}

func TestSeedance25ActualQuotaUsesCompletionTokensAndSavedExactRatios(t *testing.T) {
	noVideo := seedance25TerminalTask(false)
	quota, ok := seedance25ActualQuota(noVideo, 100)
	require.True(t, ok)
	require.Equal(t, 535, quota)

	video := seedance25TerminalTask(true)
	quota, ok = seedance25ActualQuota(video, 100)
	require.True(t, ok)
	require.Equal(t, 320, quota)

	noVideo.PrivateData.BillingContext.GroupRatio = 2
	quota, ok = seedance25ActualQuota(noVideo, 100)
	require.True(t, ok)
	require.Equal(t, 1070, quota)

	noVideo.PrivateData.BillingContext.ModelRatio = 5.34
	_, ok = seedance25ActualQuota(noVideo, 100)
	require.False(t, ok)
}

func TestSeedance25ActualQuotaRejectsNonFiniteZeroAndNegativeSnapshotRatiosWithoutPanic(t *testing.T) {
	invalidValues := map[string]float64{
		"nan":          math.NaN(),
		"positive_inf": math.Inf(1),
		"negative_inf": math.Inf(-1),
		"zero":         0,
		"negative":     -1,
	}
	for name, invalidValue := range invalidValues {
		t.Run("model_ratio_"+name, func(t *testing.T) {
			task := seedance25TerminalTask(false)
			task.PrivateData.BillingContext.ModelRatio = invalidValue
			require.NotPanics(t, func() {
				quota, ok := seedance25ActualQuota(task, 100)
				require.False(t, ok)
				require.Zero(t, quota)
			})
		})
		t.Run("group_ratio_"+name, func(t *testing.T) {
			task := seedance25TerminalTask(false)
			task.PrivateData.BillingContext.GroupRatio = invalidValue
			require.NotPanics(t, func() {
				quota, ok := seedance25ActualQuota(task, 100)
				require.False(t, ok)
				require.Zero(t, quota)
			})
		})
		t.Run("other_ratio_"+name, func(t *testing.T) {
			task := seedance25TerminalTask(false)
			task.PrivateData.BillingContext.OtherRatios["seedance_intl_billing"] = invalidValue
			require.NotPanics(t, func() {
				quota, ok := seedance25ActualQuota(task, 100)
				require.False(t, ok)
				require.Zero(t, quota)
			})
		})
	}
}

func TestSeedance25TerminalSuccessPersistsOnlyCompletionTokenUsage(t *testing.T) {
	const responseBody = `{
		"id":"provider_task_marker",
		"model":"provider_model_marker",
		"status":"succeeded",
		"content":{"video_url":"https://example.invalid/result.mp4"},
		"usage":{"completion_tokens":100,"total_tokens":999}
	}`
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResultForTask(seedance25TerminalTask(false), []byte(responseBody))
	require.NoError(t, err)
	require.True(t, result.CompletionTokensValid)
	require.Equal(t, 100, result.CompletionTokens)
	require.Equal(t, 999, result.TotalTokens)

	task := seedance25TerminalTask(false)
	persisted, err := adaptor.ApplyTaskResultPolicy(task, result, []byte(responseBody))
	require.NoError(t, err)
	require.EqualValues(t, model.TaskStatusSuccess, result.Status)
	require.Equal(t, 535, adaptor.AdjustBillingOnComplete(task, result))
	require.Contains(t, string(persisted), `"completion_tokens":100`)
	require.NotContains(t, string(persisted), "total_tokens")
	require.NotContains(t, string(persisted), "provider_task_marker")
	require.NotContains(t, string(persisted), "provider_model_marker")
}

func TestSeedance25MalformedOrMissingSuccessUsageFailsSafe(t *testing.T) {
	usageCases := map[string]string{
		"missing":  `"total_tokens":999`,
		"null":     `"completion_tokens":null,"total_tokens":999`,
		"string":   `"completion_tokens":"100","total_tokens":999`,
		"float":    `"completion_tokens":100.5,"total_tokens":999`,
		"zero":     `"completion_tokens":0,"total_tokens":999`,
		"negative": `"completion_tokens":-1,"total_tokens":999`,
	}

	for name, usage := range usageCases {
		t.Run(name, func(t *testing.T) {
			body := `{"id":"provider_task_marker","model":"provider_model_marker","status":"succeeded","content":{"video_url":"https://example.invalid/result.mp4"},"usage":{` + usage + `}}`
			adaptor := &TaskAdaptor{}
			result, err := adaptor.ParseTaskResultForTask(seedance25TerminalTask(false), []byte(body))
			require.NoError(t, err)
			require.False(t, result.CompletionTokensValid)
			persisted, policyErr := adaptor.ApplyTaskResultPolicy(seedance25TerminalTask(false), result, []byte(body))
			require.NoError(t, policyErr)
			require.EqualValues(t, model.TaskStatusFailure, result.Status)
			require.Empty(t, result.Url)
			require.Equal(t, 0, adaptor.AdjustBillingOnComplete(seedance25TerminalTask(false), result))
			require.Contains(t, string(persisted), "invalid_upstream_usage")
			require.NotContains(t, string(persisted), "provider_task_marker")
			require.NotContains(t, string(persisted), "provider_model_marker")
			require.NotContains(t, string(persisted), "total_tokens")
		})
	}
}

func TestSeedance25MissingOrBlankSuccessVideoURLFailsSafe(t *testing.T) {
	urlCases := map[string]string{
		"missing": "",
		"empty":   `"content":{"video_url":""},`,
		"blank":   `"content":{"video_url":"  \t "},`,
	}
	for name, content := range urlCases {
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"status":"succeeded",` + content + `"usage":{"completion_tokens":100}}`)
			adaptor := &TaskAdaptor{}
			task := seedance25TerminalTask(false)
			result, err := adaptor.ParseTaskResultForTask(task, body)
			require.NoError(t, err)
			persisted, err := adaptor.ApplyTaskResultPolicy(task, result, body)
			require.NoError(t, err)
			require.EqualValues(t, model.TaskStatusFailure, result.Status)
			require.Empty(t, result.Url)
			require.Zero(t, adaptor.AdjustBillingOnComplete(task, result))
			require.Contains(t, string(persisted), `"code":"invalid_upstream_output"`)
			require.NotContains(t, string(persisted), "provider")
		})
	}
}

func TestSeedance25StrictUsageParserDoesNotChangeLegacyParser(t *testing.T) {
	const body = `{"status":"succeeded","content":{"video_url":"https://example.invalid/result.mp4"},"usage":{"completion_tokens":"100","total_tokens":999}}`
	adaptor := &TaskAdaptor{}
	_, legacyErr := adaptor.ParseTaskResult([]byte(body))
	require.Error(t, legacyErr)

	result, err := adaptor.ParseTaskResultForTask(seedance25TerminalTask(false), []byte(body))
	require.NoError(t, err)
	require.False(t, result.CompletionTokensValid)
	_, err = adaptor.ApplyTaskResultPolicy(seedance25TerminalTask(false), result, []byte(body))
	require.NoError(t, err)
	require.EqualValues(t, model.TaskStatusFailure, result.Status)
}

func TestSeedance25TaskTypeConstraintIsSanitizedAndNonRetryable(t *testing.T) {
	const body = `{
		"id":"provider_task_marker",
		"model":"provider_model_marker",
		"status":"failed",
		"error":{"code":"InvalidParameter.TaskTypeConstraint","message":"raw provider diagnostic with internal-project-marker"}
	}`
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResultForTask(seedance25TerminalTask(false), []byte(body))
	require.NoError(t, err)
	persisted, err := adaptor.ApplyTaskResultPolicy(seedance25TerminalTask(false), result, []byte(body))
	require.NoError(t, err)
	require.EqualValues(t, model.TaskStatusFailure, result.Status)
	require.Equal(t, "request parameters are not supported for seedance-2.5", result.Reason)
	require.Contains(t, string(persisted), `"code":"invalid_request_error"`)
	require.NotContains(t, string(persisted), "InvalidParameter.TaskTypeConstraint")
	require.NotContains(t, string(persisted), "internal-project-marker")
	require.NotContains(t, string(persisted), "provider_task_marker")
	require.NotContains(t, string(persisted), "provider_model_marker")
}

func TestSeedance25TaskTypeConstraintWithoutStatusStillTerminatesSafely(t *testing.T) {
	const body = `{"error":{"code":"InvalidParameter.TaskTypeConstraint","message":"raw internal diagnostic marker"}}`
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResultForTask(seedance25TerminalTask(false), []byte(body))
	require.NoError(t, err)
	require.EqualValues(t, model.TaskStatusFailure, result.Status)
	persisted, err := adaptor.ApplyTaskResultPolicy(seedance25TerminalTask(false), result, []byte(body))
	require.NoError(t, err)
	require.Contains(t, string(persisted), `"code":"invalid_request_error"`)
	require.NotContains(t, string(persisted), "raw internal diagnostic marker")
}

func TestSeedance25SubmitResponseUsesOnlyPublicIdentity(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		OriginModelName: seedance25TenantAliasForDoubaoTest,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_seedance25",
		},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"provider_task_marker"}`)),
	}
	upstreamID, taskData, publicResponse, taskErr := adaptor.DoResponseNoWrite(nil, resp, info)
	require.Nil(t, taskErr)
	require.Equal(t, "provider_task_marker", upstreamID)
	require.NotContains(t, string(taskData), "provider_task_marker")
	publicBody, err := common.Marshal(publicResponse.Body)
	require.NoError(t, err)
	require.Contains(t, string(publicBody), "task_public_seedance25")
	require.Contains(t, string(publicBody), seedance25TenantAliasForDoubaoTest)
	require.NotContains(t, string(publicBody), "provider_task_marker")
}

func TestSeedance25ModelRatioConstantIsStable(t *testing.T) {
	require.Equal(t, 5.35, seedance25ModelRatio)
}
