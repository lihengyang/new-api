package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertPollingTask(t *testing.T, taskID string, status TaskStatus, progress string) {
	t.Helper()
	insertTask(t, &Task{
		TaskID:   taskID,
		Status:   status,
		Progress: progress,
	})
}

func taskIDs(tasks []*Task) map[string]bool {
	ids := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		ids[task.TaskID] = true
	}
	return ids
}

func TestTaskStatusIsUpstreamPollable(t *testing.T) {
	assert.False(t, TaskStatusReserved.IsUpstreamPollable())
	assert.False(t, TaskStatus(TaskStatusFailure).IsUpstreamPollable())
	assert.False(t, TaskStatus(TaskStatusSuccess).IsUpstreamPollable())
	assert.True(t, TaskStatus(TaskStatusNotStart).IsUpstreamPollable())
	assert.True(t, TaskStatus(TaskStatusSubmitted).IsUpstreamPollable())
	assert.True(t, TaskStatus(TaskStatusQueued).IsUpstreamPollable())
	assert.True(t, TaskStatus(TaskStatusInProgress).IsUpstreamPollable())
	assert.True(t, TaskStatus(TaskStatusUnknown).IsUpstreamPollable())
}

func TestGetAllUnFinishSyncTasksSkipsReserved(t *testing.T) {
	truncateTables(t)

	insertPollingTask(t, "task_reserved", TaskStatusReserved, "0%")
	insertPollingTask(t, "task_not_start", TaskStatusNotStart, "0%")

	ids := taskIDs(GetAllUnFinishSyncTasks(10))
	assert.False(t, ids["task_reserved"])
	assert.True(t, ids["task_not_start"])
}

func TestGetAllUnFinishSyncTasksIncludesNormalUnfinishedStatuses(t *testing.T) {
	truncateTables(t)

	expected := []struct {
		taskID string
		status TaskStatus
	}{
		{taskID: "task_not_start", status: TaskStatusNotStart},
		{taskID: "task_submitted", status: TaskStatusSubmitted},
		{taskID: "task_queued", status: TaskStatusQueued},
		{taskID: "task_in_progress", status: TaskStatusInProgress},
		{taskID: "task_unknown", status: TaskStatusUnknown},
	}
	for _, item := range expected {
		insertPollingTask(t, item.taskID, item.status, "50%")
	}

	ids := taskIDs(GetAllUnFinishSyncTasks(10))
	for _, item := range expected {
		require.True(t, ids[item.taskID], "expected %s to remain pollable", item.status)
	}
}

func TestGetAllUnFinishSyncTasksExcludesTerminalStatuses(t *testing.T) {
	truncateTables(t)

	insertPollingTask(t, "task_failure", TaskStatusFailure, "50%")
	insertPollingTask(t, "task_success", TaskStatusSuccess, "50%")
	insertPollingTask(t, "task_progress_complete", TaskStatusInProgress, "100%")
	insertPollingTask(t, "task_in_progress", TaskStatusInProgress, "50%")

	ids := taskIDs(GetAllUnFinishSyncTasks(10))
	assert.False(t, ids["task_failure"])
	assert.False(t, ids["task_success"])
	assert.False(t, ids["task_progress_complete"])
	assert.True(t, ids["task_in_progress"])
}

func TestGetTimedOutUnfinishedTasksExcludesReservedAndTerminalStatuses(t *testing.T) {
	truncateTables(t)

	for _, item := range []struct {
		taskID string
		status TaskStatus
	}{
		{taskID: "task_reserved", status: TaskStatusReserved},
		{taskID: "task_failure", status: TaskStatusFailure},
		{taskID: "task_success", status: TaskStatusSuccess},
		{taskID: "task_in_progress", status: TaskStatusInProgress},
	} {
		insertTask(t, &Task{
			TaskID:     item.taskID,
			Status:     item.status,
			Progress:   "50%",
			SubmitTime: 999,
		})
	}

	ids := taskIDs(GetTimedOutUnfinishedTasks(1000, 10))
	assert.False(t, ids["task_reserved"])
	assert.False(t, ids["task_failure"])
	assert.False(t, ids["task_success"])
	assert.True(t, ids["task_in_progress"])
}
