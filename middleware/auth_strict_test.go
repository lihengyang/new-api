package middleware

import (
	"net/http"
	"net/http/httptest"
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
	router.POST("/diagnose", ModerationDiagnoseRateLimit(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for i := 0; i < 10; i++ {
		request := httptest.NewRequest(http.MethodPost, "/diagnose", nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/diagnose", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
}
