package doubao

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newDoubaoRequestContext(t *testing.T, req relaycommon.TaskSubmitReq) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	c.Set("task_request", req)
	return c
}

func TestTaskAdaptorBuildRequestBodySerializes4KAndExplicitFalse(t *testing.T) {
	c := newDoubaoRequestContext(t, relaycommon.TaskSubmitReq{
		Model:  "tenant-standard-alias",
		Prompt: "test prompt",
		Metadata: map[string]interface{}{
			"resolution":     "4k",
			"generate_audio": false,
		},
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "tenant-standard-alias",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "dreamina-seedance-2-0-260128",
			IsModelMapped:     true,
		},
	}

	bodyReader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(bodyReader)
	require.NoError(t, err)

	var body map[string]interface{}
	require.NoError(t, common.Unmarshal(bodyBytes, &body))
	require.Equal(t, "dreamina-seedance-2-0-260128", body["model"])
	require.Equal(t, "4k", body["resolution"])
	generateAudio, exists := body["generate_audio"]
	require.True(t, exists)
	require.Equal(t, false, generateAudio)
}

func TestTaskAdaptorBuildRequestBodyCanonicalizes4K(t *testing.T) {
	c := newDoubaoRequestContext(t, relaycommon.TaskSubmitReq{
		Model:  "tenant-standard-alias",
		Prompt: "test prompt",
		Metadata: map[string]interface{}{
			"resolution": "4K",
		},
	})
	info := &relaycommon.RelayInfo{
		OriginModelName: "tenant-standard-alias",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "dreamina-seedance-2-0-260128",
			IsModelMapped:     true,
		},
	}

	bodyReader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	bodyBytes, err := io.ReadAll(bodyReader)
	require.NoError(t, err)

	var body map[string]interface{}
	require.NoError(t, common.Unmarshal(bodyBytes, &body))
	require.Equal(t, "4k", body["resolution"])
}

func TestTaskAdaptorValidateMappedRequestRejectsFastHighResolution(t *testing.T) {
	for _, resolution := range []string{"1080p", "4k"} {
		t.Run(resolution, func(t *testing.T) {
			c := newDoubaoRequestContext(t, relaycommon.TaskSubmitReq{
				Model:  "opaque-tenant-alias",
				Prompt: "test prompt",
				Metadata: map[string]interface{}{
					"resolution": resolution,
				},
			})
			info := &relaycommon.RelayInfo{
				OriginModelName: "lsf-seedance-2.0-fast-henrytest",
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: "dreamina-seedance-2-0-fast-260128",
				},
			}

			taskErr := (&TaskAdaptor{}).ValidateMappedRequest(c, info)
			require.NotNil(t, taskErr)
			require.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			require.Contains(t, taskErr.Message, resolution+" is not supported")
		})
	}
}

func TestTaskAdaptorModelListIncludesDreaminaSeedance20Models(t *testing.T) {
	require.Contains(t, (&TaskAdaptor{}).GetModelList(), "dreamina-seedance-2-0-260128")
	require.Contains(t, (&TaskAdaptor{}).GetModelList(), "dreamina-seedance-2-0-fast-260128")
}
