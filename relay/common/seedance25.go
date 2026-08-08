package common

import "strings"

const (
	Seedance25TenantAliasPrefix = "lsf-seedance-2.5-"
	Seedance25BillingFamily     = "seedance_2_5"
)

const seedance25FamilyAliasToken = "seedance-2.5"

func isSeedance25TenantSuffix(value string) bool {
	if value == "" {
		return false
	}
	isAlphaNumeric := func(char byte) bool {
		return char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
	}
	if !isAlphaNumeric(value[0]) || !isAlphaNumeric(value[len(value)-1]) {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if isAlphaNumeric(char) || char == '-' || char == '_' || char == '.' {
			continue
		}
		return false
	}
	return true
}

// IsSeedance25OriginAlias recognizes exact tenant-facing aliases in the form
// lsf-seedance-2.5-<tenant>. Billing and validation must never infer this
// family from a channel, mapped upstream model, or endpoint identifier.
func IsSeedance25OriginAlias(modelName string) bool {
	if !strings.HasPrefix(modelName, Seedance25TenantAliasPrefix) {
		return false
	}
	return isSeedance25TenantSuffix(strings.TrimPrefix(modelName, Seedance25TenantAliasPrefix))
}

// IsSeedance25AliasLookalike identifies names that could otherwise be mistaken
// for a released tenant alias while preserving exact family classification.
func IsSeedance25AliasLookalike(modelName string) bool {
	if IsSeedance25OriginAlias(modelName) {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(normalized, seedance25FamilyAliasToken) ||
		strings.Contains(normalized, "seedance-2-5") ||
		strings.Contains(normalized, "seedance_2.5")
}
