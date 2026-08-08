package common

import "testing"

func TestSeedance25AliasClassificationIsExact(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		exact     bool
		lookalike bool
	}{
		{name: "exact", model: "seedance-2.5", exact: true},
		{name: "surrounding whitespace", model: " seedance-2.5 ", lookalike: true},
		{name: "case lookalike", model: "Seedance-2.5", lookalike: true},
		{name: "suffix lookalike", model: "seedance-2.5-fast", lookalike: true},
		{name: "numeric lookalike", model: "seedance-2.50", lookalike: true},
		{name: "seedance 2.0", model: "seedance-2.0"},
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
