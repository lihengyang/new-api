package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"gorm.io/gorm"
)

const (
	SeedAudioIdempotencyUniqueIndexName = "idx_seed_audio_idempotencies_token_client_request"

	SeedAudioIdempotencyStatusPending   = "pending"
	SeedAudioIdempotencyStatusCompleted = "completed"
	SeedAudioIdempotencyStatusFailed    = "failed"
)

var ErrSeedAudioIdempotencyNotPending = errors.New("seed audio idempotency record is not pending")

type SeedAudioIdempotency struct {
	ID                 int64   `json:"id" gorm:"primaryKey"`
	CreatedAt          int64   `json:"created_at" gorm:"index"`
	UpdatedAt          int64   `json:"updated_at"`
	TokenID            int     `json:"-" gorm:"column:token_id;not null;default:0;uniqueIndex:idx_seed_audio_idempotencies_token_client_request,priority:1"`
	ClientRequestID    string  `json:"-" gorm:"column:client_request_id;type:varchar(191);not null;uniqueIndex:idx_seed_audio_idempotencies_token_client_request,priority:2"`
	RequestHMAC        string  `json:"-" gorm:"column:request_hmac;type:varchar(128);not null"`
	Status             string  `json:"status" gorm:"type:varchar(20);not null;index"`
	ResponseID         string  `json:"response_id" gorm:"type:varchar(64)"`
	XTTLogID           string  `json:"-" gorm:"column:x_tt_logid;type:varchar(128)"`
	TemporaryURL       string  `json:"-" gorm:"type:text"`
	URLExpiresAt       int64   `json:"url_expires_at" gorm:"index"`
	Duration           float64 `json:"duration"`
	OriginalDuration   float64 `json:"original_duration"`
	ActualQuota        int     `json:"actual_quota"`
	ErrorCode          string  `json:"error_code" gorm:"type:varchar(128)"`
	ErrorStatusCode    int     `json:"error_status_code"`
	ErrorDiagnostics   string  `json:"-" gorm:"type:text"`
	ExpiresAt          int64   `json:"expires_at" gorm:"index"`
	TombstoneExpiresAt int64   `json:"tombstone_expires_at" gorm:"index"`
}

type SeedAudioIdempotencyPendingParams struct {
	TokenID            int
	ClientRequestID    string
	RequestHMAC        string
	Now                int64
	ExpiresAt          int64
	TombstoneExpiresAt int64
}

type SeedAudioIdempotencyCompleteParams struct {
	ID                 int64
	Record             dto.SeedAudioIdempotencyRecord
	UpdatedAt          int64
	ExpiresAt          int64
	TombstoneExpiresAt int64
}

type SeedAudioIdempotencyFailParams struct {
	ID                 int64
	RequestHMAC        string
	XTTLogID           string
	ErrorCode          string
	ErrorStatusCode    int
	ErrorDiagnostics   string
	UpdatedAt          int64
	ExpiresAt          int64
	TombstoneExpiresAt int64
}

func CreateSeedAudioIdempotencyPending(params SeedAudioIdempotencyPendingParams) (*SeedAudioIdempotency, error) {
	clientRequestID := strings.TrimSpace(params.ClientRequestID)
	if clientRequestID == "" {
		return nil, errors.New("client_request_id is required")
	}
	if strings.TrimSpace(params.RequestHMAC) == "" {
		return nil, errors.New("request_hmac is required")
	}
	now := seedAudioIdempotencyNow(params.Now)
	row := &SeedAudioIdempotency{
		CreatedAt:          now,
		UpdatedAt:          now,
		TokenID:            params.TokenID,
		ClientRequestID:    clientRequestID,
		RequestHMAC:        params.RequestHMAC,
		Status:             SeedAudioIdempotencyStatusPending,
		ExpiresAt:          params.ExpiresAt,
		TombstoneExpiresAt: params.TombstoneExpiresAt,
	}
	if err := DB.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func GetSeedAudioIdempotencyByTokenClientRequestID(tokenID int, clientRequestID string) (*SeedAudioIdempotency, bool, error) {
	if strings.TrimSpace(clientRequestID) == "" {
		return nil, false, nil
	}
	var row SeedAudioIdempotency
	err := DB.Where("token_id = ? and client_request_id = ?", tokenID, strings.TrimSpace(clientRequestID)).First(&row).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return &row, exist, nil
}

func ReclaimSeedAudioIdempotencyPending(id int64, expectedUpdatedAt int64, params SeedAudioIdempotencyPendingParams) (*SeedAudioIdempotency, error) {
	clientRequestID := strings.TrimSpace(params.ClientRequestID)
	if clientRequestID == "" {
		return nil, errors.New("client_request_id is required")
	}
	now := seedAudioIdempotencyNow(params.Now)
	result := DB.Model(&SeedAudioIdempotency{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(map[string]any{
			"created_at":           now,
			"updated_at":           now,
			"token_id":             params.TokenID,
			"client_request_id":    clientRequestID,
			"request_hmac":         params.RequestHMAC,
			"status":               SeedAudioIdempotencyStatusPending,
			"response_id":          "",
			"x_tt_logid":           "",
			"temporary_url":        "",
			"url_expires_at":       0,
			"duration":             0,
			"original_duration":    0,
			"actual_quota":         0,
			"error_code":           "",
			"error_status_code":    0,
			"error_diagnostics":    "",
			"expires_at":           params.ExpiresAt,
			"tombstone_expires_at": params.TombstoneExpiresAt,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrSeedAudioIdempotencyNotPending
	}
	var row SeedAudioIdempotency
	if err := DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func CompleteSeedAudioIdempotency(params SeedAudioIdempotencyCompleteParams) error {
	updatedAt := seedAudioIdempotencyNow(params.UpdatedAt)
	createdAt := params.Record.CreatedAt
	if createdAt == 0 {
		createdAt = updatedAt
	}
	result := DB.Model(&SeedAudioIdempotency{}).
		Where("id = ? AND status = ?", params.ID, SeedAudioIdempotencyStatusPending).
		Updates(map[string]any{
			"created_at":           createdAt,
			"updated_at":           updatedAt,
			"request_hmac":         params.Record.RequestHMAC,
			"status":               SeedAudioIdempotencyStatusCompleted,
			"response_id":          params.Record.ResponseID,
			"x_tt_logid":           params.Record.XTTLogID,
			"temporary_url":        params.Record.TemporaryURL,
			"url_expires_at":       params.Record.URLExpiresAt,
			"duration":             params.Record.Duration,
			"original_duration":    params.Record.OriginalDuration,
			"actual_quota":         params.Record.ActualQuota,
			"error_code":           "",
			"error_status_code":    0,
			"error_diagnostics":    "",
			"expires_at":           params.ExpiresAt,
			"tombstone_expires_at": params.TombstoneExpiresAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSeedAudioIdempotencyNotPending
	}
	return nil
}

func FailSeedAudioIdempotency(params SeedAudioIdempotencyFailParams) error {
	updatedAt := seedAudioIdempotencyNow(params.UpdatedAt)
	result := DB.Model(&SeedAudioIdempotency{}).
		Where("id = ? AND status = ?", params.ID, SeedAudioIdempotencyStatusPending).
		Updates(map[string]any{
			"updated_at":           updatedAt,
			"request_hmac":         params.RequestHMAC,
			"status":               SeedAudioIdempotencyStatusFailed,
			"x_tt_logid":           params.XTTLogID,
			"error_code":           params.ErrorCode,
			"error_status_code":    params.ErrorStatusCode,
			"error_diagnostics":    params.ErrorDiagnostics,
			"expires_at":           params.ExpiresAt,
			"tombstone_expires_at": params.TombstoneExpiresAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSeedAudioIdempotencyNotPending
	}
	return nil
}

func DeleteSeedAudioIdempotency(id int64) error {
	return DB.Delete(&SeedAudioIdempotency{}, id).Error
}

func IsSeedAudioIdempotencyDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	errText := strings.ToLower(err.Error())
	indexName := strings.ToLower(SeedAudioIdempotencyUniqueIndexName)
	if strings.Contains(errText, indexName) {
		return true
	}
	return strings.Contains(errText, "unique") &&
		strings.Contains(errText, "token_id") &&
		strings.Contains(errText, "client_request_id")
}

func (row SeedAudioIdempotency) ToRecord() dto.SeedAudioIdempotencyRecord {
	return dto.SeedAudioIdempotencyRecord{
		RequestHMAC:      row.RequestHMAC,
		Status:           row.Status,
		ResponseID:       row.ResponseID,
		XTTLogID:         row.XTTLogID,
		TemporaryURL:     row.TemporaryURL,
		URLExpiresAt:     row.URLExpiresAt,
		Duration:         row.Duration,
		OriginalDuration: row.OriginalDuration,
		ActualQuota:      row.ActualQuota,
		CreatedAt:        row.CreatedAt,
		ErrorCode:        row.ErrorCode,
		ErrorStatusCode:  row.ErrorStatusCode,
		ErrorDiagnostics: row.ErrorDiagnostics,
	}
}

func seedAudioIdempotencyNow(value int64) int64 {
	if value != 0 {
		return value
	}
	return time.Now().Unix()
}
