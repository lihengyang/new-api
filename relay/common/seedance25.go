package common

import "strings"

const (
	Seedance25PublicAlias   = "seedance-2.5"
	Seedance25BillingFamily = "seedance_2_5"
)

// IsSeedance25OriginAlias recognizes only the exact tenant-facing alias. Billing
// and validation must never infer this family from a mapped upstream model.
func IsSeedance25OriginAlias(modelName string) bool {
	return modelName == Seedance25PublicAlias
}

// IsSeedance25AliasLookalike identifies names that could otherwise be mistaken
// for the released alias while preserving exact-match family classification.
func IsSeedance25AliasLookalike(modelName string) bool {
	if IsSeedance25OriginAlias(modelName) {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(normalized, Seedance25PublicAlias) ||
		strings.Contains(normalized, "seedance-2-5") ||
		strings.Contains(normalized, "seedance_2.5")
}
