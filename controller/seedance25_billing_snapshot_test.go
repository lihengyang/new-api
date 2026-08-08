package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestApplyTaskPrivateDataPersistsSeedance25BillingSnapshot(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: relaycommon.Seedance25PublicAlias,
		BillingSource:   "wallet",
		TokenId:         12,
		PriceData: types.PriceData{
			ModelRatio:            5.35,
			BillingFamily:         relaycommon.Seedance25BillingFamily,
			BillingRuleVersion:    "seedance25_test_rule",
			OtherRatioNumerator:   64,
			OtherRatioDenominator: 107,
			OtherRatios: map[string]float64{
				"seedance_intl_billing": 64.0 / 107.0,
			},
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1.25,
			},
		},
	}
	privateData := model.TaskPrivateData{}
	applyTaskPrivateData(&privateData, info, &relay.TaskSubmitResult{UpstreamTaskID: "provider_task_marker"})

	require.Equal(t, "provider_task_marker", privateData.UpstreamTaskID)
	require.NotNil(t, privateData.BillingContext)
	require.Equal(t, relaycommon.Seedance25PublicAlias, privateData.BillingContext.OriginModelName)
	require.Equal(t, relaycommon.Seedance25BillingFamily, privateData.BillingContext.BillingFamily)
	require.Equal(t, "seedance25_test_rule", privateData.BillingContext.BillingRuleVersion)
	require.Equal(t, 5.35, privateData.BillingContext.ModelRatio)
	require.Equal(t, 1.25, privateData.BillingContext.GroupRatio)
	require.EqualValues(t, 64, privateData.BillingContext.OtherRatioNumerator)
	require.EqualValues(t, 107, privateData.BillingContext.OtherRatioDenominator)

	serialized, err := privateData.Value()
	require.NoError(t, err)
	var roundTrip model.TaskPrivateData
	require.NoError(t, roundTrip.Scan(serialized))
	require.NotNil(t, roundTrip.BillingContext)
	require.Equal(t, relaycommon.Seedance25BillingFamily, roundTrip.BillingContext.BillingFamily)
	require.Equal(t, 5.35, roundTrip.BillingContext.ModelRatio)
	require.Equal(t, 1.25, roundTrip.BillingContext.GroupRatio)
	require.EqualValues(t, 64, roundTrip.BillingContext.OtherRatioNumerator)
	require.EqualValues(t, 107, roundTrip.BillingContext.OtherRatioDenominator)
}
