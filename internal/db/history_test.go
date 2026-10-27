package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryReadsStoredExecutionsNewestFirst(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = runMigration(database); err != nil {
		t.Fatal(err)
	}
	repo := NewHistoryRepository(database)
	ctx := context.Background()
	for _, query := range []string{"SELECT 1", "SELECT 2"} {
		if err = repo.Log(ctx, QueryHistoryEntity{ConnectionId: "1", DatabaseName: "test", QueryText: query, Duration: 12, Status: "SUCCESS"}); err != nil {
			t.Fatal(err)
		}
	}
	history, err := repo.GetHistory(ctx, 1, time.Unix(0, 0))
	if err != nil || len(history) != 1 {
		t.Fatalf("history: %+v %v", history, err)
	}
	if history[0].QueryText != "SELECT 2" || history[0].ConnectionId != "1" || history[0].Duration != 12 || history[0].ExecutedAt == 0 {
		t.Fatalf("stored execution: %+v", history[0])
	}
	history, err = repo.GetHistory(ctx, 100, time.Now().Add(time.Hour))
	if err != nil || len(history) != 0 {
		t.Fatalf("since filter: %+v %v", history, err)
	}
}
