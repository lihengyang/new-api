package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStrictAdminAuthRejectsCommonUserWithForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("strict-admin-test", cookie.NewStore([]byte("test-secret"))))
	router.Use(func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "common-user")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 1)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		c.Next()
	})
	router.GET("/admin-only", StrictAdminAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	request.Header.Set("New-Api-User", "1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestStrictAdminAuthRejectsUnauthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("strict-admin-test", cookie.NewStore([]byte("test-secret"))))
	router.GET("/admin-only", StrictAdminAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestModerationDiagnoseRateLimitAllowsTenRequestsPerMinute(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 424242)
		c.Next()
	})
	handlerCalls := 0
	router.POST("/diagnose", ModerationDiagnoseRateLimit(), func(c *gin.Context) {
		var payload map[string]any
		require.NoError(t, common.DecodeJson(c.Request.Body, &payload))
		require.Equal(t, "manual", payload["source_type"])
		handlerCalls++
		c.Status(http.StatusNoContent)
	})

	for i := 0; i < 10; i++ {
		request := httptest.NewRequest(
			http.MethodPost,
			"/diagnose",
			strings.NewReader(`{"source_type":"manual"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	}
	require.Equal(t, 10, handlerCalls)

	request := httptest.NewRequest(
		http.MethodPost,
		"/diagnose",
		strings.NewReader(`{"source_type":"manual"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Equal(t, 10, handlerCalls)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	data, ok := response["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rate_limited", data["result_status"])
}

func TestModerationDiagnoseRateLimitDoesNotAffectOtherAPIs(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 434343)
		c.Next()
	})
	router.POST("/diagnose", ModerationDiagnoseRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.POST("/other-admin-api", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for i := 0; i < 11; i++ {
		diagnoseRequest := httptest.NewRequest(
			http.MethodPost,
			"/diagnose",
			strings.NewReader(`{"source_type":"manual"}`),
		)
		diagnoseRequest.Header.Set("Content-Type", "application/json")
		diagnoseRecorder := httptest.NewRecorder()
		router.ServeHTTP(diagnoseRecorder, diagnoseRequest)

		otherRequest := httptest.NewRequest(http.MethodPost, "/other-admin-api", nil)
		otherRecorder := httptest.NewRecorder()
		router.ServeHTTP(otherRecorder, otherRequest)
		require.Equal(t, http.StatusNoContent, otherRecorder.Code)
	}
}

func TestModerationDiagnoseLibraryBoundaryDoesNotConsumeRateLimit(t *testing.T) {
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("id", 444444)
		c.Next()
	})
	router.POST("/diagnose", ModerationDiagnoseRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for i := 0; i < 20; i++ {
		request := httptest.NewRequest(
			http.MethodPost,
			"/diagnose",
			strings.NewReader(`{"source_type":"library_asset","asset_id":"asset-placeholder"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	}

	for i := 0; i < 10; i++ {
		request := httptest.NewRequest(
			http.MethodPost,
			"/diagnose",
			strings.NewReader(`{"source_type":"manual"}`),
		)
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	}
}
