package app

import (
	"context"
	"database/sql"
	"db-lens/internal/db"
	"db-lens/internal/engine/entities"
	"db-lens/internal/types"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSavedQueryTimeoutCancelsAndAllowsNextQuery(t *testing.T) {
	previous := db.Conn
	settingsDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.Conn = settingsDB
	t.Cleanup(func() { db.Conn = previous; settingsDB.Close() })
	if _, err := settingsDB.Exec("CREATE TABLE app_preferences (key TEXT PRIMARY KEY, value TEXT)"); err != nil {
		t.Fatal(err)
	}
	settings := types.DefaultResultSettings()
	settings.QueryTimeoutSeconds = 1
	if err := db.SaveResultSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	service, _, _ := editService(t)
	outer, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started := time.Now()
	_, err = service.ExecuteConsoleQuery(outer, "timeout-console", "slow", entities.QueryInput{
		Query: "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT SUM(x) FROM n",
	})
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timed out after 1 seconds") {
		t.Fatalf("expected saved timeout, got %v", err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("timeout did not interrupt the database")
	}
	if _, err := service.ExecuteConsoleQuery(outer, "timeout-console", "next", entities.QueryInput{Query: "SELECT 42"}); err != nil {
		t.Fatal("query stayed locked after timeout", err)
	}
	script, err := service.ExecuteConsoleScript(outer, "timeout-console", "script", "test",
		"CREATE TABLE timeout_test (value INTEGER); INSERT INTO timeout_test WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT SUM(x) FROM n;")
	if err != nil || script == nil || !script.Cancelled || script.Completed != 1 || !strings.Contains(script.Error, "timed out after 1 seconds") {
		t.Fatalf("script timeout did not preserve completion/error details: %+v %v", script, err)
	}
	settings.QueryTimeoutSeconds = 0
	if err := db.SaveResultSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	unlimited, finish, err := queryTimeoutContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if _, limited := unlimited.Deadline(); limited {
		t.Fatal("zero must disable the timeout")
	}
}
