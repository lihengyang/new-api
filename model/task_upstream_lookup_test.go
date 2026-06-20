package model

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTaskCreatedAtIndexExistsInSQLiteTestSchema(t *testing.T) {
	require.True(t, DB.Migrator().HasIndex(&Task{}, "CreatedAt"))
}

func TestGetByUpstreamTaskIDUsesNarrowCreatedAtRangeAndExactJSONMatch(t *testing.T) {
	truncateTables(t)

	taskTime := time.Now().UTC().Truncate(time.Second)
	upstreamTaskID := "cgt-" + taskTime.Format("20060102150405") + "-abc12"
	task := &Task{
		CreatedAt: taskTime.Unix(),
		TaskID:    GenerateTaskID(),
		PrivateData: TaskPrivateData{
			UpstreamTaskID: upstreamTaskID,
		},
	}
	require.NoError(t, DB.Create(task).Error)

	similar := &Task{
		CreatedAt: taskTime.Unix(),
		TaskID:    GenerateTaskID(),
		PrivateData: TaskPrivateData{
			UpstreamTaskID: upstreamTaskID + "x",
		},
	}
	require.NoError(t, DB.Create(similar).Error)

	found, exists, err := GetByUpstreamTaskID(
		upstreamTaskID,
		taskTime.Add(-15*time.Minute).Unix(),
		taskTime.Add(15*time.Minute).Unix(),
	)

	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, task.ID, found.ID)
}

func TestGetByUpstreamTaskIDMissingReturnsNotFound(t *testing.T) {
	truncateTables(t)

	taskTime := time.Now().UTC().Truncate(time.Second)
	found, exists, err := GetByUpstreamTaskID(
		"cgt-"+taskTime.Format("20060102150405")+"-none1",
		taskTime.Add(-15*time.Minute).Unix(),
		taskTime.Add(15*time.Minute).Unix(),
	)

	require.NoError(t, err)
	require.False(t, exists)
	require.Nil(t, found)
}

func TestUpstreamTaskIDQueryUsesCreatedAtIndexInSQLite(t *testing.T) {
	taskTime := time.Now().UTC().Truncate(time.Second)
	start := taskTime.Add(-15 * time.Minute).Unix()
	end := taskTime.Add(15 * time.Minute).Unix()
	upstreamTaskID := "cgt-" + taskTime.Format("20060102150405") + "-plan1"

	dryRun := buildUpstreamTaskIDQuery(
		DB.Session(&gorm.Session{DryRun: true}),
		upstreamTaskID,
		start,
		end,
	).Find(&[]Task{})
	require.NoError(t, dryRun.Error)
	require.Contains(t, dryRun.Statement.SQL.String(), "created_at >= ?")
	require.Contains(t, dryRun.Statement.SQL.String(), "created_at <= ?")
	require.Contains(t, dryRun.Statement.SQL.String(), "LIMIT 2")
	require.Equal(t, []interface{}{start, end, upstreamTaskID}, dryRun.Statement.Vars)
	require.LessOrEqual(t, end-start, int64((30*time.Minute)/time.Second))

	var planRows []struct {
		Detail string `gorm:"column:detail"`
	}
	require.NoError(t, DB.Raw(
		"EXPLAIN QUERY PLAN "+dryRun.Statement.SQL.String(),
		dryRun.Statement.Vars...,
	).Scan(&planRows).Error)
	require.NotEmpty(t, planRows)

	plan := make([]string, 0, len(planRows))
	for _, row := range planRows {
		plan = append(plan, row.Detail)
	}
	planText := strings.Join(plan, "\n")
	require.Contains(t, planText, "USING INDEX idx_tasks_created_at")
	require.NotContains(t, planText, "SCAN tasks")
}

func TestUpstreamTaskIDQueryKeepsDatabaseSpecificJSONPredicatesParameterized(t *testing.T) {
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgreSQL
	})

	testCases := []struct {
		name       string
		mysql      bool
		postgresql bool
		predicate  string
	}{
		{
			name:      "mysql",
			mysql:     true,
			predicate: "JSON_UNQUOTE(JSON_EXTRACT(private_data, '$.upstream_task_id')) = ?",
		},
		{
			name:       "postgresql",
			postgresql: true,
			predicate:  "private_data ->> 'upstream_task_id' = ?",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			common.UsingSQLite = false
			common.UsingMySQL = tc.mysql
			common.UsingPostgreSQL = tc.postgresql

			query := buildUpstreamTaskIDQuery(
				DB.Session(&gorm.Session{DryRun: true}),
				"cgt-20260620000000-sql01",
				100,
				200,
			).Find(&[]Task{})
			require.NoError(t, query.Error)
			require.Contains(t, query.Statement.SQL.String(), tc.predicate)
			require.Contains(t, query.Statement.SQL.String(), "LIMIT 2")
			require.Equal(t, []interface{}{int64(100), int64(200), "cgt-20260620000000-sql01"}, query.Statement.Vars)
			require.NotContains(t, query.Statement.SQL.String(), "cgt-20260620000000-sql01")
		})
	}
}

func TestGetByUpstreamTaskIDRejectsInvalidRange(t *testing.T) {
	_, _, err := GetByUpstreamTaskID("cgt-20260620000000-range", 200, 100)
	require.EqualError(t, err, "invalid upstream task lookup time range")
}
