package ratio_setting

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestSeedance25ModelRatioRequiresExactRuntimeTenantAlias(t *testing.T) {
	tenantAlias := relaycommon.Seedance25TenantAliasPrefix + "henrytest"
	secondTenantAlias := relaycommon.Seedance25TenantAliasPrefix + "tenant-two"

	_, bareDefaultRatio := GetDefaultModelRatioMap()["seedance-2.5"]
	require.False(t, bareDefaultRatio)
	_, tenantDefaultRatio := GetDefaultModelRatioMap()[tenantAlias]
	require.False(t, tenantDefaultRatio)
	_, bareDefaultPrice := GetDefaultModelPriceMap()["seedance-2.5"]
	require.False(t, bareDefaultPrice)
	_, tenantDefaultPrice := GetDefaultModelPriceMap()[tenantAlias]
	require.False(t, tenantDefaultPrice)

	previousRatios := ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateModelRatioByJSONString(previousRatios))
	})
	require.NoError(t, UpdateModelRatioByJSONString(`{"lsf-seedance-2.5-henrytest":5.35}`))

	ratio, ok, matchedName := GetModelRatio(tenantAlias)
	require.True(t, ok)
	require.Equal(t, 5.35, ratio)
	require.Equal(t, tenantAlias, matchedName)
	_, bareConfigured := GetModelRatioCopy()["seedance-2.5"]
	require.False(t, bareConfigured)
	_, secondTenantConfigured := GetModelRatioCopy()[secondTenantAlias]
	require.False(t, secondTenantConfigured)
}
