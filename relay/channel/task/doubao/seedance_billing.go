package doubao

import (
	"fmt"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const (
	seedanceBillingFamilyStandard = "seedance_2_0"
	seedanceBillingFamilyFast     = "seedance_2_0_fast"
	seedanceBillingFamilyMini     = "seedance_2_0_mini"
	seedanceBillingFamily25       = relaycommon.Seedance25BillingFamily

	seedanceBillingResolution480p720p = "480p_720p"
	seedanceBillingResolution1080p    = "1080p"
	seedanceBillingResolution4K       = "4k"

	seedanceBillingInputVideo   = "video_input"
	seedanceBillingInputNoVideo = "no_video_input"

	seedanceBillingRuleVersion       = "byteplus_seedance_2_0_intl_2026_06_4k"
	seedanceMiniBillingRuleVersion   = "byteplus_seedance_2_0_mini_intl_2026_06_rc1"
	seedance25BillingRuleVersion     = "byteplus_seedance_2_5_intl_2026_08_p0"
	seedance25Native1080pRuleVersion = "byteplus_seedance_2_5_intl_2026_08_native_1080p"
)

type SeedanceBillingContext struct {
	Family           string
	InputType        string
	Resolution       string
	ResolutionGroup  string
	UnitPriceUsdPerK float64
	Ratio            float64
	RuleVersion      string
	Reason           string
}

type seedanceBillingProfile struct {
	Family           string
	HasVideoInput    bool
	ResolutionGroup  string
	UnitPriceUsdPerK float64
	Ratio            float64
	RuleVersion      string
}

// Billing profiles store OtherRatio values only.
// Standard, Fast, and Mini all use:
// ModelRatio = family no-video base / repo unit.
// OtherRatio = current tier / family no-video base.
// Family no-video bases are Standard 0.0070, Fast 0.0056, and Mini
// 0.0035 USD / K tokens.
var seedanceIntlBillingProfiles = []seedanceBillingProfile{
	{
		Family:           seedanceBillingFamilyStandard,
		HasVideoInput:    false,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0070,
		Ratio:            1.0,
	},
	{
		Family:           seedanceBillingFamilyStandard,
		HasVideoInput:    true,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0043,
		Ratio:            0.0043 / 0.0070,
	},
	{
		Family:           seedanceBillingFamilyStandard,
		HasVideoInput:    false,
		ResolutionGroup:  seedanceBillingResolution1080p,
		UnitPriceUsdPerK: 0.0077,
		Ratio:            0.0077 / 0.0070,
	},
	{
		Family:           seedanceBillingFamilyStandard,
		HasVideoInput:    true,
		ResolutionGroup:  seedanceBillingResolution1080p,
		UnitPriceUsdPerK: 0.0047,
		Ratio:            0.0047 / 0.0070,
	},
	{
		Family:           seedanceBillingFamilyStandard,
		HasVideoInput:    false,
		ResolutionGroup:  seedanceBillingResolution4K,
		UnitPriceUsdPerK: 0.0040,
		Ratio:            0.0040 / 0.0070,
	},
	{
		Family:           seedanceBillingFamilyStandard,
		HasVideoInput:    true,
		ResolutionGroup:  seedanceBillingResolution4K,
		UnitPriceUsdPerK: 0.0024,
		Ratio:            0.0024 / 0.0070,
	},
	{
		Family:           seedanceBillingFamilyFast,
		HasVideoInput:    false,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0056,
		Ratio:            1.0,
	},
	{
		Family:           seedanceBillingFamilyFast,
		HasVideoInput:    true,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0033,
		Ratio:            0.0033 / 0.0056,
	},
	{
		Family:           seedanceBillingFamilyMini,
		HasVideoInput:    false,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0035,
		Ratio:            1.0,
		RuleVersion:      seedanceMiniBillingRuleVersion,
	},
	{
		Family:           seedanceBillingFamilyMini,
		HasVideoInput:    true,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0021,
		Ratio:            0.0021 / 0.0035,
		RuleVersion:      seedanceMiniBillingRuleVersion,
	},
	{
		Family:           seedanceBillingFamily25,
		HasVideoInput:    false,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0107,
		Ratio:            1.0,
		RuleVersion:      seedance25BillingRuleVersion,
	},
	{
		Family:           seedanceBillingFamily25,
		HasVideoInput:    true,
		ResolutionGroup:  seedanceBillingResolution480p720p,
		UnitPriceUsdPerK: 0.0064,
		Ratio:            64.0 / 107.0,
		RuleVersion:      seedance25BillingRuleVersion,
	},
	{
		Family:           seedanceBillingFamily25,
		HasVideoInput:    false,
		ResolutionGroup:  seedanceBillingResolution1080p,
		UnitPriceUsdPerK: 0.0117,
		Ratio:            117.0 / 107.0,
		RuleVersion:      seedance25Native1080pRuleVersion,
	},
	{
		Family:           seedanceBillingFamily25,
		HasVideoInput:    true,
		ResolutionGroup:  seedanceBillingResolution1080p,
		UnitPriceUsdPerK: 0.0070,
		Ratio:            70.0 / 107.0,
		RuleVersion:      seedance25Native1080pRuleVersion,
	},
}

// resolveSeedanceBillingFamily recognizes released Seedance 2.0 video-generation
// models from either customer-facing aliases or upstream provider model names.
// It deliberately does not match seedance-virtual-asset-admin or
// seedance-real-human-asset-admin, and does not classify Mini as standard.
func resolveSeedance20BillingFamily(modelNames ...string) string {
	for _, modelName := range modelNames {
		normalized := strings.ToLower(strings.TrimSpace(modelName))
		if normalized == "" {
			continue
		}

		if strings.Contains(normalized, "seedance-2.0-mini") ||
			strings.Contains(normalized, "seedance-2-0-mini") {
			return seedanceBillingFamilyMini
		}

		if strings.Contains(normalized, "seedance-2.0-fast") ||
			strings.Contains(normalized, "seedance-2-0-fast") {
			return seedanceBillingFamilyFast
		}

		isSeedance20 := strings.Contains(normalized, "seedance-2.0") ||
			strings.Contains(normalized, "seedance-2-0")
		isMini := strings.Contains(normalized, "seedance-2.0-mini") ||
			strings.Contains(normalized, "seedance-2-0-mini")
		if isSeedance20 && !isMini {
			return seedanceBillingFamilyStandard
		}
	}

	return ""
}

func resolveSeedanceBillingFamily(originModelName, upstreamModelName string) string {
	if relaycommon.IsSeedance25OriginAlias(originModelName) {
		return seedanceBillingFamily25
	}
	return resolveSeedance20BillingFamily(originModelName, upstreamModelName)
}

func normalizeSeedanceBillingResolution(metadata map[string]interface{}) (resolution string, resolutionGroup string, err error) {
	raw := metadataString(metadata, "resolution")
	if raw == "" {
		// BytePlus/OpenAI-compatible requests commonly default to 720p when no
		// explicit resolution is provided. We bill missing resolution as 720p.
		return "720p", seedanceBillingResolution480p720p, nil
	}

	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "480p", "480":
		return "480p", seedanceBillingResolution480p720p, nil
	case "720p", "720":
		return "720p", seedanceBillingResolution480p720p, nil
	case "1080p", "1080":
		return "1080p", seedanceBillingResolution1080p, nil
	case "4k":
		return "4k", seedanceBillingResolution4K, nil
	default:
		return "", "", fmt.Errorf("unsupported Seedance 2.0 resolution %q; supported values are 480p, 720p, 1080p, and 4k", raw)
	}
}

func normalizeSeedance25BillingResolution(metadata map[string]interface{}) (resolution string, resolutionGroup string, err error) {
	raw := metadataString(metadata, "resolution")
	if raw == "" {
		return "720p", seedanceBillingResolution480p720p, nil
	}
	switch raw {
	case "480p", "720p":
		return raw, seedanceBillingResolution480p720p, nil
	case "1080p":
		return raw, seedanceBillingResolution1080p, nil
	default:
		return "", "", fmt.Errorf("unsupported Seedance 2.5 resolution %q; supported values are 480p, 720p, and 1080p", raw)
	}
}

func seedance25BillingRatioFraction(resolutionGroup, inputType string) (int64, int64, bool) {
	switch resolutionGroup {
	case seedanceBillingResolution480p720p:
		if inputType == seedanceBillingInputVideo {
			return 64, 107, true
		}
		if inputType == seedanceBillingInputNoVideo {
			return 1, 1, true
		}
	case seedanceBillingResolution1080p:
		if inputType == seedanceBillingInputVideo {
			return 70, 107, true
		}
		if inputType == seedanceBillingInputNoVideo {
			return 117, 107, true
		}
	}
	return 0, 0, false
}

func metadataString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}

	raw, ok := metadata[key]
	if !ok || raw == nil {
		return ""
	}

	switch v := raw.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func findSeedanceBillingProfile(family string, hasVideoInput bool, resolutionGroup string) (seedanceBillingProfile, bool) {
	for _, profile := range seedanceIntlBillingProfiles {
		if profile.Family == family &&
			profile.HasVideoInput == hasVideoInput &&
			profile.ResolutionGroup == resolutionGroup {
			return profile, true
		}
	}
	return seedanceBillingProfile{}, false
}

func ResolveSeedanceIntlBilling(originModelName, upstreamModelName string, metadata map[string]interface{}) (*SeedanceBillingContext, bool, error) {
	family := resolveSeedanceBillingFamily(originModelName, upstreamModelName)
	if family == "" {
		return nil, false, nil
	}

	resolution, resolutionGroup, err := normalizeSeedanceBillingResolution(metadata)
	if family == seedanceBillingFamily25 {
		resolution, resolutionGroup, err = normalizeSeedance25BillingResolution(metadata)
	}
	if err != nil {
		return nil, true, err
	}

	if family == seedanceBillingFamilyFast {
		switch resolutionGroup {
		case seedanceBillingResolution1080p:
			return nil, true, fmt.Errorf("1080p is not supported for Dreamina Seedance 2.0 fast; please use 480p or 720p")
		case seedanceBillingResolution4K:
			return nil, true, fmt.Errorf("4k is not supported for Dreamina Seedance 2.0 fast; please use 480p or 720p")
		}
	}
	if family == seedanceBillingFamilyMini {
		switch resolutionGroup {
		case seedanceBillingResolution1080p:
			return nil, true, fmt.Errorf("1080p is not supported for Dreamina Seedance 2.0 mini; please use 480p or 720p")
		case seedanceBillingResolution4K:
			return nil, true, fmt.Errorf("4k is not supported for Dreamina Seedance 2.0 mini; please use 480p or 720p")
		}
	}

	hasVideoInput := hasVideoInMetadata(metadata)
	profile, ok := findSeedanceBillingProfile(family, hasVideoInput, resolutionGroup)
	if !ok {
		return nil, true, fmt.Errorf("Seedance billing profile not configured for family=%s, video_input=%t, resolution=%s", family, hasVideoInput, resolutionGroup)
	}

	inputType := seedanceBillingInputNoVideo
	if hasVideoInput {
		inputType = seedanceBillingInputVideo
	}

	reason := "no_video_detected"
	if hasVideoInput {
		reason = "metadata.content.video_url_detected"
	}

	ruleVersion := profile.RuleVersion
	if ruleVersion == "" {
		ruleVersion = seedanceBillingRuleVersion
	}

	return &SeedanceBillingContext{
		Family:           family,
		InputType:        inputType,
		Resolution:       resolution,
		ResolutionGroup:  resolutionGroup,
		UnitPriceUsdPerK: profile.UnitPriceUsdPerK,
		Ratio:            profile.Ratio,
		RuleVersion:      ruleVersion,
		Reason:           reason,
	}, true, nil
}
