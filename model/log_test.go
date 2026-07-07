package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestLogClientRequestIDSchemaAndExactSearch(t *testing.T) {
	truncateTables(t)

	clientRequestID := "req_seed_audio_error"
	upstreamRequestID := "log_seed_audio_error"
	errorCode := "seed_audio_upstream_error"
	httpStatus := 502
	retryable := true
	log := &Log{
		UserId:            1001,
		CreatedAt:         2000,
		Type:              LogTypeError,
		Content:           "Seed Audio request failed",
		Username:          "operator-target-user",
		TokenName:         "seed-audio-token",
		ModelName:         "lsf-seed-audio-1.0-recurve",
		Quota:             0,
		RequestId:         "trace_seed_audio_error",
		ClientRequestID:   &clientRequestID,
		UpstreamRequestID: &upstreamRequestID,
		ErrorCode:         &errorCode,
		HttpStatus:        &httpStatus,
		Retryable:         &retryable,
		Other:             `{"seed_audio_error":true}`,
	}
	require.NoError(t, DB.Create(log).Error)

	logs, total, err := GetAllLogs(LogTypeError, 0, 0, "", "operator-target-user", "seed-audio-token", 0, 20, 0, "", clientRequestID)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, log.Id, logs[0].Id)
	require.NotNil(t, logs[0].ClientRequestID)
	require.Equal(t, clientRequestID, *logs[0].ClientRequestID)

	logs, total, err = GetAllLogs(LogTypeError, 0, 0, "", "operator-target-user", "seed-audio-token", 0, 20, 0, "", upstreamRequestID)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, log.Id, logs[0].Id)

	logs, total, err = GetAllLogs(LogTypeError, 0, 0, "", "operator-target-user", "seed-audio-token", 0, 20, 0, "", "trace_seed_audio_error")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, log.Id, logs[0].Id)

	var indexInfo []struct {
		Seqno int    `gorm:"column:seqno"`
		Cid   int    `gorm:"column:cid"`
		Name  string `gorm:"column:name"`
	}
	require.NoError(t, DB.Raw("PRAGMA index_info('idx_logs_client_request_id_created_at')").Scan(&indexInfo).Error)
	require.Len(t, indexInfo, 2)
	require.Equal(t, "client_request_id", indexInfo[0].Name)
	require.Equal(t, "created_at", indexInfo[1].Name)

	var indexes []struct {
		Name string `gorm:"column:name"`
	}
	require.NoError(t, DB.Raw("PRAGMA index_list('logs')").Scan(&indexes).Error)
	indexNames := make([]string, 0, len(indexes))
	for _, index := range indexes {
		indexNames = append(indexNames, index.Name)
	}
	require.Contains(t, indexNames, "idx_logs_client_request_id_created_at")
	require.Contains(t, indexNames, "idx_logs_upstream_request_id")
	require.NotContains(t, indexNames, "idx_logs_error_code")
	require.NotContains(t, indexNames, "idx_logs_http_status")
}

func TestUserLogsSanitizeSeedAudioErrorDiagnostics(t *testing.T) {
	truncateTables(t)

	clientRequestID := "req_seed_audio_self_hidden"
	upstreamRequestID := "log_seed_audio_self_hidden"
	errorCode := "seed_audio_upstream_error"
	httpStatus := 502
	retryable := true
	log := &Log{
		UserId:            1001,
		CreatedAt:         3000,
		Type:              LogTypeError,
		Content:           "Seed Audio request failed",
		Username:          "operator-target-user",
		TokenName:         "seed-audio-token",
		ModelName:         "lsf-seed-audio-1.0-recurve",
		Quota:             0,
		RequestId:         "trace_seed_audio_self_hidden",
		ClientRequestID:   &clientRequestID,
		UpstreamRequestID: &upstreamRequestID,
		ErrorCode:         &errorCode,
		HttpStatus:        &httpStatus,
		Retryable:         &retryable,
		Other: `{
			"seed_audio":true,
			"seed_audio_error":true,
			"client_request_id":"req_seed_audio_self_hidden",
			"upstream_request_id":"log_seed_audio_self_hidden",
			"error_code":"seed_audio_upstream_error",
			"http_status":502,
			"input_chars":12,
			"request_path":"/v1/audio/speech"
		}`,
	}
	require.NoError(t, DB.Create(log).Error)

	adminLogs, total, err := GetAllLogs(LogTypeError, 0, 0, "", "operator-target-user", "seed-audio-token", 0, 20, 0, "", clientRequestID)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, adminLogs, 1)
	require.NotNil(t, adminLogs[0].ClientRequestID)
	require.Equal(t, clientRequestID, *adminLogs[0].ClientRequestID)
	adminOther, err := common.StrToMap(adminLogs[0].Other)
	require.NoError(t, err)
	require.Equal(t, true, adminOther["seed_audio_error"])
	require.Equal(t, clientRequestID, adminOther["client_request_id"])

	userLogs, total, err := GetUserLogs(1001, LogTypeError, 0, 0, "", "seed-audio-token", 0, 20, "", clientRequestID)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, userLogs, 1)
	require.Nil(t, userLogs[0].ClientRequestID)
	require.Nil(t, userLogs[0].UpstreamRequestID)
	require.Nil(t, userLogs[0].ErrorCode)
	require.Nil(t, userLogs[0].HttpStatus)
	require.Nil(t, userLogs[0].Retryable)

	selfOther, err := common.StrToMap(userLogs[0].Other)
	require.NoError(t, err)
	require.Empty(t, selfOther)
	require.NotContains(t, userLogs[0].Other, "seed_audio_error")
	require.NotContains(t, userLogs[0].Other, "client_request_id")
	require.NotContains(t, userLogs[0].Other, "upstream_request_id")
}
