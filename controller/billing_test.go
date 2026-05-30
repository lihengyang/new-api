package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type billingBalanceTestResponse struct {
	Object   string `json:"object"`
	Currency string `json:"currency"`
	Balance  struct {
		Available *float64 `json:"available"`
		Used      float64  `json:"used"`
		Unlimited bool     `json:"unlimited"`
	} `json:"balance"`
	UpdatedAt int64 `json:"updated_at"`
}

func setupBillingBalanceRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	wasMasterNode := common.IsMasterNode
	sqlitePath := common.SQLitePath
	common.IsMasterNode = false

	quotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() {
		common.QuotaPerUnit = quotaPerUnit
		common.SQLitePath = sqlitePath
		common.IsMasterNode = wasMasterNode
	})

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	common.SQLitePath = dsn
	t.Setenv("SQL_DSN", "local")
	require.NoError(t, model.InitDB())

	db := model.DB
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))

	router := gin.New()
	router.GET("/v1/billing/balance", middleware.TokenAuth(), GetBillingBalance)

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return router, db
}

func seedBillingBalanceToken(t *testing.T, db *gorm.DB, key string, remainQuota int, usedQuota int, unlimited bool) {
	t.Helper()

	user := &model.User{
		Id:       1001,
		Username: "billing-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    999999999,
	}
	require.NoError(t, db.Create(user).Error)

	token := &model.Token{
		UserId:         user.Id,
		Key:            key,
		Name:           "billing-token",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    remainQuota,
		UsedQuota:      usedQuota,
		UnlimitedQuota: unlimited,
	}
	require.NoError(t, db.Create(token).Error)
}

func requestBillingBalance(router *gin.Engine, tokenKey string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/balance", nil)
	if tokenKey != "" {
		req.Header.Set("Authorization", "Bearer sk-"+tokenKey)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestGetBillingBalanceValidTokenResponse(t *testing.T) {
	router, db := setupBillingBalanceRouter(t)
	seedBillingBalanceToken(t, db, "balancevalidtoken", 4406550, 17688350, false)

	recorder := requestBillingBalance(router, "balancevalidtoken")

	require.Equal(t, http.StatusOK, recorder.Code)

	var response billingBalanceTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "billing.balance", response.Object)
	require.Equal(t, "USD", response.Currency)
	require.NotNil(t, response.Balance.Available)
	require.InDelta(t, 8.8131, *response.Balance.Available, 0.0000001)
	require.InDelta(t, 35.3767, response.Balance.Used, 0.0000001)
	require.False(t, response.Balance.Unlimited)
	require.Greater(t, response.UpdatedAt, int64(0))
}

func TestGetBillingBalanceUnlimitedTokenResponse(t *testing.T) {
	router, db := setupBillingBalanceRouter(t)
	seedBillingBalanceToken(t, db, "balanceunlimitedtoken", 4406550, 17688350, true)

	recorder := requestBillingBalance(router, "balanceunlimitedtoken")

	require.Equal(t, http.StatusOK, recorder.Code)

	var response billingBalanceTestResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "billing.balance", response.Object)
	require.Equal(t, "USD", response.Currency)
	require.Nil(t, response.Balance.Available)
	require.InDelta(t, 35.3767, response.Balance.Used, 0.0000001)
	require.True(t, response.Balance.Unlimited)
	require.Greater(t, response.UpdatedAt, int64(0))
}

func TestGetBillingBalanceDoesNotExposeInternalFields(t *testing.T) {
	router, db := setupBillingBalanceRouter(t)
	seedBillingBalanceToken(t, db, "balancenointernalfields", 4406550, 17688350, false)

	recorder := requestBillingBalance(router, "balancenointernalfields")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	for _, field := range []string{
		"remain_quota",
		"used_quota",
		"quota_per_unit",
		"token_id",
		"user_id",
		"group",
		"channel_id",
		"modelRatio",
		"groupRatio",
		"otherMultiplier",
		"pre-deduct",
		"refund",
		"top-up",
	} {
		require.NotContains(t, body, field)
	}
}

func TestGetBillingBalanceAuthErrorsUseTokenAuth(t *testing.T) {
	router, _ := setupBillingBalanceRouter(t)

	missingAuth := requestBillingBalance(router, "")
	require.Equal(t, http.StatusUnauthorized, missingAuth.Code)
	require.Contains(t, missingAuth.Body.String(), `"error"`)

	invalidAuth := requestBillingBalance(router, "invalidtoken")
	require.Equal(t, http.StatusUnauthorized, invalidAuth.Code)
	require.Contains(t, invalidAuth.Body.String(), `"error"`)
}
