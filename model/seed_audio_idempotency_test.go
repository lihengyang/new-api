package model

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func seedAudioPendingParamsForTest(tokenID int, clientRequestID string) SeedAudioIdempotencyPendingParams {
	return SeedAudioIdempotencyPendingParams{
		TokenID:            tokenID,
		ClientRequestID:    clientRequestID,
		RequestHMAC:        "hmac_" + clientRequestID,
		Now:                1000,
		ExpiresAt:          2000,
		TombstoneExpiresAt: 3000,
	}
}

func TestCreateSeedAudioIdempotencyPendingStoresUniqueFields(t *testing.T) {
	truncateTables(t)

	params := seedAudioPendingParamsForTest(101, " req_store ")
	params.RequestHMAC = "hmac_req_store"
	row, err := CreateSeedAudioIdempotencyPending(params)
	require.NoError(t, err)

	var reloaded SeedAudioIdempotency
	require.NoError(t, DB.First(&reloaded, row.ID).Error)
	require.Equal(t, 101, reloaded.TokenID)
	require.Equal(t, "req_store", reloaded.ClientRequestID)
	require.Equal(t, "hmac_req_store", reloaded.RequestHMAC)
	require.Equal(t, SeedAudioIdempotencyStatusPending, reloaded.Status)
	require.EqualValues(t, 1000, reloaded.CreatedAt)
	require.EqualValues(t, 2000, reloaded.ExpiresAt)
	require.EqualValues(t, 3000, reloaded.TombstoneExpiresAt)
}

func TestCreateSeedAudioIdempotencyDuplicateSameTokenDetected(t *testing.T) {
	truncateTables(t)

	_, err := CreateSeedAudioIdempotencyPending(seedAudioPendingParamsForTest(201, "req_duplicate"))
	require.NoError(t, err)
	_, err = CreateSeedAudioIdempotencyPending(seedAudioPendingParamsForTest(201, "req_duplicate"))

	require.Error(t, err)
	require.True(t, IsSeedAudioIdempotencyDuplicateError(err))
}

func TestCreateSeedAudioIdempotencyDifferentTokenAllowed(t *testing.T) {
	truncateTables(t)

	_, err := CreateSeedAudioIdempotencyPending(seedAudioPendingParamsForTest(301, "req_shared"))
	require.NoError(t, err)
	_, err = CreateSeedAudioIdempotencyPending(seedAudioPendingParamsForTest(302, "req_shared"))
	require.NoError(t, err)
}

func TestCompleteSeedAudioIdempotencyStoresReplayRecord(t *testing.T) {
	truncateTables(t)

	row, err := CreateSeedAudioIdempotencyPending(seedAudioPendingParamsForTest(401, "req_complete"))
	require.NoError(t, err)

	record := dto.SeedAudioIdempotencyRecord{
		RequestHMAC:      row.RequestHMAC,
		Status:           SeedAudioIdempotencyStatusCompleted,
		ResponseID:       "aud_cached",
		XTTLogID:         "log_secretish",
		TemporaryURL:     "https://tmp.example.com/audio.mp3",
		URLExpiresAt:     1800,
		Duration:         7.5,
		OriginalDuration: 8.25,
		ActualQuota:      10312,
		CreatedAt:        1200,
	}
	err = CompleteSeedAudioIdempotency(SeedAudioIdempotencyCompleteParams{
		ID:                 row.ID,
		Record:             record,
		UpdatedAt:          1300,
		ExpiresAt:          2000,
		TombstoneExpiresAt: 3000,
	})
	require.NoError(t, err)

	var reloaded SeedAudioIdempotency
	require.NoError(t, DB.First(&reloaded, row.ID).Error)
	require.Equal(t, SeedAudioIdempotencyStatusCompleted, reloaded.Status)
	require.Equal(t, "aud_cached", reloaded.ResponseID)
	require.Equal(t, "https://tmp.example.com/audio.mp3", reloaded.TemporaryURL)
	require.EqualValues(t, 1200, reloaded.CreatedAt)
	require.EqualValues(t, 1300, reloaded.UpdatedAt)
	require.Equal(t, record, reloaded.ToRecord())
}

func TestFailSeedAudioIdempotencyStoresSanitizedError(t *testing.T) {
	truncateTables(t)

	row, err := CreateSeedAudioIdempotencyPending(seedAudioPendingParamsForTest(501, "req_fail"))
	require.NoError(t, err)

	err = FailSeedAudioIdempotency(SeedAudioIdempotencyFailParams{
		ID:                 row.ID,
		RequestHMAC:        row.RequestHMAC,
		XTTLogID:           "log_failure_sanitized",
		ErrorCode:          "seed_audio_upstream_error",
		ErrorStatusCode:    502,
		ErrorDiagnostics:   `{"upstream_http_status":502,"response_class":"5xx"}`,
		UpdatedAt:          1400,
		ExpiresAt:          2000,
		TombstoneExpiresAt: 3000,
	})
	require.NoError(t, err)

	var reloaded SeedAudioIdempotency
	require.NoError(t, DB.First(&reloaded, row.ID).Error)
	require.Equal(t, SeedAudioIdempotencyStatusFailed, reloaded.Status)
	require.Equal(t, "log_failure_sanitized", reloaded.XTTLogID)
	require.Equal(t, "seed_audio_upstream_error", reloaded.ErrorCode)
	require.Equal(t, 502, reloaded.ErrorStatusCode)
	require.Equal(t, `{"upstream_http_status":502,"response_class":"5xx"}`, reloaded.ErrorDiagnostics)
	require.Equal(t, reloaded.ErrorDiagnostics, reloaded.ToRecord().ErrorDiagnostics)
	require.EqualValues(t, 1400, reloaded.UpdatedAt)
}

func TestReclaimSeedAudioIdempotencyPendingAfterTombstoneExpiry(t *testing.T) {
	truncateTables(t)

	row, err := CreateSeedAudioIdempotencyPending(seedAudioPendingParamsForTest(601, "req_reclaim"))
	require.NoError(t, err)
	require.NoError(t, FailSeedAudioIdempotency(SeedAudioIdempotencyFailParams{
		ID:                 row.ID,
		RequestHMAC:        row.RequestHMAC,
		ErrorCode:          "seed_audio_upstream_error",
		ErrorStatusCode:    502,
		UpdatedAt:          1100,
		ExpiresAt:          1200,
		TombstoneExpiresAt: 1300,
	}))

	reloaded, exists, err := GetSeedAudioIdempotencyByTokenClientRequestID(601, "req_reclaim")
	require.NoError(t, err)
	require.True(t, exists)

	reclaimed, err := ReclaimSeedAudioIdempotencyPending(reloaded.ID, reloaded.UpdatedAt, SeedAudioIdempotencyPendingParams{
		TokenID:            601,
		ClientRequestID:    "req_reclaim",
		RequestHMAC:        "new_hmac",
		Now:                1400,
		ExpiresAt:          2400,
		TombstoneExpiresAt: 3400,
	})
	require.NoError(t, err)
	require.Equal(t, SeedAudioIdempotencyStatusPending, reclaimed.Status)
	require.Equal(t, "new_hmac", reclaimed.RequestHMAC)
	require.Empty(t, reclaimed.ErrorCode)
	require.Empty(t, reclaimed.ErrorDiagnostics)
	require.EqualValues(t, 1400, reclaimed.CreatedAt)
}
