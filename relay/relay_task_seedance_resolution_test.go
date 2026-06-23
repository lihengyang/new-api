package relay

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRelayTaskSubmitRejectsMappedFastHighResolutionBeforeBilling(t *testing.T) {
	for _, resolution := range []string{"1080p", "4k"} {
		t.Run(resolution, func(t *testing.T) {
			body := fmt.Sprintf(
				`{"prompt":"hello","model":"lsf-seedance-2.0-fast-henrytest","metadata":{"resolution":%q}}`,
				resolution,
			)
			c, info := newTaskClientRequestIDContext(t, body)
			info.OriginModelName = "lsf-seedance-2.0-fast-henrytest"
			c.Set("model_mapping", `{"lsf-seedance-2.0-fast-henrytest":"dreamina-seedance-2-0-fast-260128"}`)

			result, taskErr := RelayTaskSubmit(c, info)

			require.Nil(t, result)
			require.NotNil(t, taskErr)
			require.Equal(t, "invalid_request_error", taskErr.Code)
			require.Contains(t, taskErr.Message, resolution+" is not supported")
			require.Nil(t, info.Billing)
			require.False(t, c.Writer.Written())
		})
	}
}
