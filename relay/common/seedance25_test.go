package common

import "testing"

func TestSeedance25TenantAliasClassificationIsExact(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		exact     bool
		lookalike bool
	}{
		{name: "henrytest tenant", model: "lsf-seedance-2.5-henrytest", exact: true},
		{name: "second tenant", model: "lsf-seedance-2.5-tenant-two", exact: true},
		{name: "bare global alias", model: "seedance-2.5", lookalike: true},
		{name: "empty tenant", model: "lsf-seedance-2.5-", lookalike: true},
		{name: "surrounding whitespace", model: " lsf-seedance-2.5-henrytest ", lookalike: true},
		{name: "case lookalike", model: "LSF-Seedance-2.5-HenryTest", lookalike: true},
		{name: "missing lsf prefix", model: "seedance-2.5-henrytest", lookalike: true},
		{name: "numeric lookalike", model: "seedance-2.50", lookalike: true},
		{name: "wrong family", model: "lsf-seedance-2.0-henrytest"},
		{name: "unrelated", model: "video-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSeedance25OriginAlias(tt.model); got != tt.exact {
				t.Fatalf("IsSeedance25OriginAlias(%q) = %t, want %t", tt.model, got, tt.exact)
			}
			if got := IsSeedance25AliasLookalike(tt.model); got != tt.lookalike {
				t.Fatalf("IsSeedance25AliasLookalike(%q) = %t, want %t", tt.model, got, tt.lookalike)
			}
		})
	}
}
