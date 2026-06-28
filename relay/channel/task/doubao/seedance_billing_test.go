package doubao

import (
	"math"
	"strings"
	"testing"
)

func assertRatio(t *testing.T, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 0.0000001 {
		t.Fatalf("ratio = %f, want %f", actual, expected)
	}
}

func seedanceFinalQuotaForTest(totalTokens int, modelRatio, groupRatio, otherRatio float64) int {
	return int(float64(totalTokens) * modelRatio * groupRatio * otherRatio)
}

func videoMetadata(resolution string) map[string]interface{} {
	metadata := map[string]interface{}{
		"content": []interface{}{
			map[string]interface{}{
				"type":      "video_url",
				"video_url": map[string]interface{}{"url": "https://example.com/input.mp4"},
			},
		},
	}
	if resolution != "" {
		metadata["resolution"] = resolution
	}
	return metadata
}

func noVideoMetadata(resolution string) map[string]interface{} {
	metadata := map[string]interface{}{}
	if resolution != "" {
		metadata["resolution"] = resolution
	}
	return metadata
}

func mediaMetadata(resolution, mediaType string) map[string]interface{} {
	metadata := noVideoMetadata(resolution)
	metadata["content"] = []interface{}{
		map[string]interface{}{
			"type": mediaType,
			mediaType: map[string]interface{}{
				"url": "https://example.com/input",
			},
		},
	}
	return metadata
}

func durationMetadata(resolution string, duration int, hasVideo bool) map[string]interface{} {
	metadata := noVideoMetadata(resolution)
	metadata["duration"] = duration
	if hasVideo {
		metadata["content"] = []interface{}{
			map[string]interface{}{
				"type":      "video_url",
				"video_url": map[string]interface{}{"url": "https://example.com/input.mp4"},
				"role":      "reference_video",
			},
		}
	}
	return metadata
}

func textMetadata(resolution string) map[string]interface{} {
	metadata := noVideoMetadata(resolution)
	metadata["content"] = []interface{}{
		map[string]interface{}{
			"type": "text",
			"text": "reference text only",
		},
	}
	return metadata
}

func TestResolveSeedanceIntlBillingStandardNoVideo720p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-aivision",
		"",
		noVideoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.Family != seedanceBillingFamilyStandard {
		t.Fatalf("family = %s", ctx.Family)
	}
	if ctx.InputType != seedanceBillingInputNoVideo {
		t.Fatalf("input type = %s", ctx.InputType)
	}
	if ctx.ResolutionGroup != seedanceBillingResolution480p720p {
		t.Fatalf("resolution group = %s", ctx.ResolutionGroup)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0070)
	assertRatio(t, ctx.Ratio, 1.0)
}

func TestResolveSeedanceIntlBillingStandardVideo720p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-xiangpai",
		"",
		videoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.InputType != seedanceBillingInputVideo {
		t.Fatalf("input type = %s", ctx.InputType)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0043)
	assertRatio(t, ctx.Ratio, 0.0043/0.0070)
}

func TestResolveSeedanceIntlBillingStandardNoVideo480p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-aivision",
		"",
		noVideoMetadata("480p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.Resolution != "480p" {
		t.Fatalf("resolution = %s", ctx.Resolution)
	}
	if ctx.ResolutionGroup != seedanceBillingResolution480p720p {
		t.Fatalf("resolution group = %s", ctx.ResolutionGroup)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0070)
	assertRatio(t, ctx.Ratio, 1.0)
}

func TestResolveSeedanceIntlBillingStandardVideo480p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-xiangpai",
		"",
		videoMetadata("480p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.Resolution != "480p" {
		t.Fatalf("resolution = %s", ctx.Resolution)
	}
	if ctx.InputType != seedanceBillingInputVideo {
		t.Fatalf("input type = %s", ctx.InputType)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0043)
	assertRatio(t, ctx.Ratio, 0.0043/0.0070)
}

func TestResolveSeedanceIntlBillingFastNoVideo480p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-aivision",
		"",
		noVideoMetadata("480p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.Resolution != "480p" {
		t.Fatalf("resolution = %s", ctx.Resolution)
	}
	if ctx.Family != seedanceBillingFamilyFast {
		t.Fatalf("family = %s", ctx.Family)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0056)
	assertRatio(t, ctx.Ratio, 1.0)
}

func TestResolveSeedanceIntlBillingFastVideo480p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-xiangpai",
		"",
		videoMetadata("480p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.Resolution != "480p" {
		t.Fatalf("resolution = %s", ctx.Resolution)
	}
	if ctx.InputType != seedanceBillingInputVideo {
		t.Fatalf("input type = %s", ctx.InputType)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0033)
	assertRatio(t, ctx.Ratio, 0.0033/0.0056)
}

func TestResolveSeedanceIntlBillingStandardNoVideo1080p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-aivision",
		"",
		noVideoMetadata("1080p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.ResolutionGroup != seedanceBillingResolution1080p {
		t.Fatalf("resolution group = %s", ctx.ResolutionGroup)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0077)
	assertRatio(t, ctx.Ratio, 0.0077/0.0070)
}

func TestResolveSeedanceIntlBillingStandardVideo1080p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-xiangpai",
		"",
		videoMetadata("1080p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.InputType != seedanceBillingInputVideo {
		t.Fatalf("input type = %s", ctx.InputType)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0047)
	assertRatio(t, ctx.Ratio, 0.0047/0.0070)
}

func TestResolveSeedanceIntlBillingFastNoVideo720p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-aivision",
		"",
		noVideoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.Family != seedanceBillingFamilyFast {
		t.Fatalf("family = %s", ctx.Family)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0056)
	assertRatio(t, ctx.Ratio, 1.0)
}

func TestResolveSeedanceIntlBillingFastVideo720p(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-xiangpai",
		"",
		videoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if ctx.InputType != seedanceBillingInputVideo {
		t.Fatalf("input type = %s", ctx.InputType)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0033)
	assertRatio(t, ctx.Ratio, 0.0033/0.0056)
}

func TestResolveSeedanceIntlBillingMiniPricing(t *testing.T) {
	tests := []struct {
		name              string
		metadata          map[string]interface{}
		expectedInputType string
		expectedPrice     float64
		expectedRatio     float64
	}{
		{
			name:              "480p no video duration 4s",
			metadata:          durationMetadata("480p", 4, false),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0035,
			expectedRatio:     1.0,
		},
		{
			name:              "720p no video duration 15s",
			metadata:          durationMetadata("720p", 15, false),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0035,
			expectedRatio:     1.0,
		},
		{
			name:              "480p with video duration 4s",
			metadata:          durationMetadata("480p", 4, true),
			expectedInputType: seedanceBillingInputVideo,
			expectedPrice:     0.0021,
			expectedRatio:     0.0021 / 0.0035,
		},
		{
			name:              "720p with video duration 15s",
			metadata:          durationMetadata("720p", 15, true),
			expectedInputType: seedanceBillingInputVideo,
			expectedPrice:     0.0021,
			expectedRatio:     0.0021 / 0.0035,
		},
		{
			name:              "missing resolution defaults to 720p",
			metadata:          durationMetadata("", 4, false),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0035,
			expectedRatio:     1.0,
		},
		{
			name:              "image input remains no video",
			metadata:          mediaMetadata("720p", "image_url"),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0035,
			expectedRatio:     1.0,
		},
		{
			name:              "audio input remains no video",
			metadata:          mediaMetadata("720p", "audio_url"),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0035,
			expectedRatio:     1.0,
		},
		{
			name:              "text input remains no video",
			metadata:          textMetadata("720p"),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0035,
			expectedRatio:     1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, ok, err := ResolveSeedanceIntlBilling(
				"lsf-seedance-2.0-mini-henrytest",
				"",
				tt.metadata,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatal("expected Mini billing to match")
			}
			if ctx.Family != seedanceBillingFamilyMini {
				t.Fatalf("family = %s", ctx.Family)
			}
			if ctx.InputType != tt.expectedInputType {
				t.Fatalf("input type = %s, want %s", ctx.InputType, tt.expectedInputType)
			}
			if ctx.ResolutionGroup != seedanceBillingResolution480p720p {
				t.Fatalf("resolution group = %s", ctx.ResolutionGroup)
			}
			if ctx.RuleVersion != seedanceMiniBillingRuleVersion {
				t.Fatalf("rule version = %s", ctx.RuleVersion)
			}
			assertRatio(t, ctx.UnitPriceUsdPerK, tt.expectedPrice)
			assertRatio(t, ctx.Ratio, tt.expectedRatio)
		})
	}
}

func TestResolveSeedanceIntlBillingEffectiveRatioInvariants(t *testing.T) {
	tests := []struct {
		name           string
		originModel    string
		metadata       map[string]interface{}
		modelRatio     float64
		expectedOther  float64
		expectedFinal  float64
		expectedFamily string
	}{
		{
			name:           "mini no video",
			originModel:    "lsf-seedance-2.0-mini-henrytest",
			metadata:       noVideoMetadata("720p"),
			modelRatio:     0.0035 / 0.0020,
			expectedOther:  1.0,
			expectedFinal:  1.75,
			expectedFamily: seedanceBillingFamilyMini,
		},
		{
			name:           "mini video",
			originModel:    "lsf-seedance-2.0-mini-henrytest",
			metadata:       videoMetadata("720p"),
			modelRatio:     0.0035 / 0.0020,
			expectedOther:  0.0021 / 0.0035,
			expectedFinal:  1.05,
			expectedFamily: seedanceBillingFamilyMini,
		},
		{
			name:           "fast video",
			originModel:    "lsf-seedance-2.0-fast-xiangpai",
			metadata:       videoMetadata("720p"),
			modelRatio:     0.0056 / 0.0020,
			expectedOther:  0.0033 / 0.0056,
			expectedFinal:  1.65,
			expectedFamily: seedanceBillingFamilyFast,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, ok, err := ResolveSeedanceIntlBilling(tt.originModel, "", tt.metadata)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatal("expected Seedance billing to match")
			}
			if ctx.Family != tt.expectedFamily {
				t.Fatalf("family = %s, want %s", ctx.Family, tt.expectedFamily)
			}
			assertRatio(t, ctx.Ratio, tt.expectedOther)
			assertRatio(t, tt.modelRatio*ctx.Ratio, tt.expectedFinal)
		})
	}
}

func TestResolveSeedanceIntlBillingMiniVideoFinalQuotaLowerThanFastVideo(t *testing.T) {
	const totalTokens = 100000
	const groupRatio = 1.0

	miniCtx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-mini-henrytest",
		"",
		videoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Mini billing to match")
	}

	fastCtx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-xiangpai",
		"",
		videoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Fast billing to match")
	}

	miniFinalQuota := seedanceFinalQuotaForTest(totalTokens, 0.0035/0.0020, groupRatio, miniCtx.Ratio)
	fastFinalQuota := seedanceFinalQuotaForTest(totalTokens, 0.0056/0.0020, groupRatio, fastCtx.Ratio)

	if miniFinalQuota != 105000 {
		t.Fatalf("mini final quota = %d, want 105000", miniFinalQuota)
	}
	if fastFinalQuota != 165000 {
		t.Fatalf("fast final quota = %d, want 165000", fastFinalQuota)
	}
	if miniFinalQuota >= fastFinalQuota {
		t.Fatalf("mini final quota = %d, want lower than fast final quota %d", miniFinalQuota, fastFinalQuota)
	}
}

func TestResolveSeedanceIntlBillingMiniHighResolutionRejected(t *testing.T) {
	for _, resolution := range []string{"1080p", "4k"} {
		t.Run(resolution, func(t *testing.T) {
			_, ok, err := ResolveSeedanceIntlBilling(
				"lsf-seedance-2.0-mini-henrytest",
				"",
				noVideoMetadata(resolution),
			)
			if !ok {
				t.Fatal("expected Mini billing to match")
			}
			if err == nil {
				t.Fatalf("expected Mini %s to be rejected", resolution)
			}
			if !strings.Contains(err.Error(), resolution+" is not supported") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveSeedanceIntlBillingFast1080pRejected(t *testing.T) {
	_, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-aivision",
		"",
		noVideoMetadata("1080p"),
	)
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if err == nil {
		t.Fatal("expected fast 1080p to be rejected")
	}
	if !strings.Contains(err.Error(), "1080p is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveSeedanceIntlBillingStandard4KPricing(t *testing.T) {
	tests := []struct {
		name              string
		metadata          map[string]interface{}
		expectedInputType string
		expectedPrice     float64
		expectedRatio     float64
	}{
		{
			name:              "no video",
			metadata:          noVideoMetadata("4k"),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0040,
			expectedRatio:     0.0040 / 0.0070,
		},
		{
			name:              "image only",
			metadata:          mediaMetadata("4k", "image_url"),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0040,
			expectedRatio:     0.0040 / 0.0070,
		},
		{
			name:              "audio only",
			metadata:          mediaMetadata("4k", "audio_url"),
			expectedInputType: seedanceBillingInputNoVideo,
			expectedPrice:     0.0040,
			expectedRatio:     0.0040 / 0.0070,
		},
		{
			name:              "video",
			metadata:          videoMetadata("4k"),
			expectedInputType: seedanceBillingInputVideo,
			expectedPrice:     0.0024,
			expectedRatio:     0.0024 / 0.0070,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, ok, err := ResolveSeedanceIntlBilling(
				"lsf-seedance-2.0-aivision",
				"",
				tt.metadata,
			)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatal("expected Seedance billing to match")
			}
			if ctx.Resolution != "4k" {
				t.Fatalf("resolution = %s", ctx.Resolution)
			}
			if ctx.ResolutionGroup != seedanceBillingResolution4K {
				t.Fatalf("resolution group = %s", ctx.ResolutionGroup)
			}
			if ctx.InputType != tt.expectedInputType {
				t.Fatalf("input type = %s, want %s", ctx.InputType, tt.expectedInputType)
			}
			assertRatio(t, ctx.UnitPriceUsdPerK, tt.expectedPrice)
			assertRatio(t, ctx.Ratio, tt.expectedRatio)
		})
	}
}

func TestResolveSeedanceIntlBillingFast4KRejected(t *testing.T) {
	_, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-aivision",
		"",
		noVideoMetadata("4k"),
	)
	if !ok {
		t.Fatal("expected Seedance billing to match")
	}
	if err == nil {
		t.Fatal("expected fast 4k to be rejected")
	}
	if !strings.Contains(err.Error(), "4k is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveSeedanceIntlBillingAssetAdminNotMatched(t *testing.T) {
	_, ok, err := ResolveSeedanceIntlBilling(
		"seedance-virtual-asset-admin",
		"",
		noVideoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("asset admin model must not match Seedance video billing")
	}

	_, ok, err = ResolveSeedanceIntlBilling(
		"seedance-real-human-asset-admin",
		"",
		noVideoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("real-human asset admin model must not match Seedance video billing")
	}
}

func TestResolveSeedanceIntlBillingDreaminaFastUpstreamModelMatched(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"lsf-seedance-2.0-fast-henrytest",
		"dreamina-seedance-2-0-fast-260128",
		videoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected upstream model name to match Seedance billing")
	}
	if ctx.Family != seedanceBillingFamilyFast {
		t.Fatalf("family = %s", ctx.Family)
	}
	assertRatio(t, ctx.Ratio, 0.0033/0.0056)
}

func TestResolveSeedanceIntlBillingLegacyDoubaoFastUpstreamModelMatched(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"custom-customer-alias",
		"doubao-seedance-2-0-fast-260128",
		videoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected legacy upstream model name to match Seedance billing")
	}
	if ctx.Family != seedanceBillingFamilyFast {
		t.Fatalf("family = %s", ctx.Family)
	}
	assertRatio(t, ctx.Ratio, 0.0033/0.0056)
}

func TestResolveSeedanceIntlBillingDreaminaStandardUpstreamModelMatched(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"tenant-standard-alias",
		"dreamina-seedance-2-0-260128",
		noVideoMetadata("4k"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected upstream model name to match Seedance billing")
	}
	if ctx.Family != seedanceBillingFamilyStandard {
		t.Fatalf("family = %s", ctx.Family)
	}
	assertRatio(t, ctx.Ratio, 0.0040/0.0070)
}

func TestResolveSeedanceIntlBillingDreaminaMiniUpstreamModelMatched(t *testing.T) {
	ctx, ok, err := ResolveSeedanceIntlBilling(
		"tenant-mini-alias",
		"dreamina-seedance-2-0-mini-260615",
		videoMetadata("720p"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected upstream Mini model name to match Seedance billing")
	}
	if ctx.Family != seedanceBillingFamilyMini {
		t.Fatalf("family = %s", ctx.Family)
	}
	assertRatio(t, ctx.UnitPriceUsdPerK, 0.0021)
	assertRatio(t, ctx.Ratio, 0.0021/0.0035)
}
