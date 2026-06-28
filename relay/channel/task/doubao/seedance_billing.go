package doubao

import (
	"fmt"
	"strings"
)

const (
	seedanceBillingFamilyStandard = "seedance_2_0"
	seedanceBillingFamilyFast     = "seedance_2_0_fast"
	seedanceBillingFamilyMini     = "seedance_2_0_mini"

	seedanceBillingResolution480p720p = "480p_720p"
	seedanceBillingResolution1080p    = "1080p"
	seedanceBillingResolution4K       = "4k"

	seedanceBillingInputVideo   = "video_input"
	seedanceBillingInputNoVideo = "no_video_input"

	seedanceBillingRuleVersion     = "byteplus_seedance_2_0_intl_2026_06_4k"
	seedanceMiniBillingRuleVersion = "byteplus_seedance_2_0_mini_intl_2026_06_rc1"
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
}

// resolveSeedanceBillingFamily recognizes released Seedance 2.0 video-generation
// models from either customer-facing aliases or upstream provider model names.
// It deliberately does not match seedance-virtual-asset-admin or
// seedance-real-human-asset-admin, and does not classify Mini as standard.
func resolveSeedanceBillingFamily(modelNames ...string) string {
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
