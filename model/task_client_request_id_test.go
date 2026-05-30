package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func insertClientRequestTask(t *testing.T, tokenID int, clientRequestID *string) {
	t.Helper()
	insertTask(t, &Task{
		TaskID:          GenerateTaskID(),
		TokenId:         tokenID,
		ClientRequestID: clientRequestID,
		Status:          TaskStatusNotStart,
		Progress:        "0%",
	})
}

func TestTaskClientRequestIDSameTokenConflicts(t *testing.T) {
	truncateTables(t)

	clientRequestID := "req_seedance_123"
	insertClientRequestTask(t, 101, &clientRequestID)

	err := DB.Create(&Task{
		TaskID:          GenerateTaskID(),
		TokenId:         101,
		ClientRequestID: &clientRequestID,
		Status:          TaskStatusNotStart,
		Progress:        "0%",
	}).Error
	require.Error(t, err)
}

func TestTaskClientRequestIDDifferentTokenAllowed(t *testing.T) {
	truncateTables(t)

	clientRequestID := "req_seedance_456"
	insertClientRequestTask(t, 201, &clientRequestID)

	insertClientRequestTask(t, 202, &clientRequestID)
}

func TestTaskClientRequestIDNullAllowedMultipleRows(t *testing.T) {
	truncateTables(t)

	insertClientRequestTask(t, 301, nil)
	insertClientRequestTask(t, 301, nil)
	insertClientRequestTask(t, 302, nil)
}
