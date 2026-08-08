package ratio_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestDefaultSeedance25ModelRatio(t *testing.T) {
	ratio, ok := GetDefaultModelRatioMap()[common.Seedance25PublicAlias]
	require.True(t, ok)
	require.Equal(t, 5.35, ratio)
	_, fixedPrice := GetDefaultModelPriceMap()[common.Seedance25PublicAlias]
	require.False(t, fixedPrice)
}
