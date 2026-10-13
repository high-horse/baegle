package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"db-lens/internal/db"
	"db-lens/internal/engine/entities"
	"db-lens/internal/types"
)

func TestScriptExecutionAndTransactionCleanup(t *testing.T) {
	s, table, conn := editService(t)
	ctx := context.Background()
	result, err := s.ExecuteConsoleScript(ctx, "c", "1", table.ConnectionID, "CREATE TABLE items(id INTEGER PRIMARY KEY, name TEXT);\nINSERT INTO items VALUES(1,'one; two');\nINSERT INTO items VALUES(1,'duplicate');\nINSERT INTO items VALUES(2,'not run');")
	if err != nil || result.Completed != 2 || result.ErrorLine != 3 || result.Error == "" {
		t.Fatalf("script error: %+v %v", result, err)
	}
	var count int
	if err = conn.DB().QueryRow("SELECT count(*) FROM items").Scan(&count); err != nil || count != 1 {
		t.Fatalf("autocommitted rows: %d %v", count, err)
	}
	result, err = s.ExecuteConsoleScript(ctx, "c", "2", table.ConnectionID, "BEGIN; INSERT INTO items VALUES(2,'rollback'); INSERT INTO missing VALUES(1); COMMIT;")
	if err != nil || result.Error == "" {
		t.Fatalf("transaction error: %+v %v", result, err)
	}
	if err = conn.DB().QueryRow("SELECT count(*) FROM items").Scan(&count); err != nil || count != 1 {
		t.Fatalf("unfinished transaction leaked: %d %v", count, err)
	}
	result, err = s.ExecuteConsoleScript(ctx, "c", "3", table.ConnectionID, "BEGIN; INSERT INTO items VALUES(2,'commit'); COMMIT;")
	if err != nil || result.Error != "" || result.Completed != 3 {
		t.Fatalf("transaction success: %+v %v", result, err)
	}
	if _, err = s.ExecuteConsoleScript(ctx, "c", "4", "other", "DELETE FROM items"); err == nil {
		t.Fatal("cross connection execution accepted")
	}
	config := conn.Config()
	config.ReadOnly = true
	if _, err = s.Connect(ctx, config); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExecuteConsoleScript(ctx, "c", "5", table.ConnectionID, "DELETE FROM items"); err == nil {
		t.Fatal("read-only execution accepted")
	}
}

func TestScriptCancellationAndSharedConsoleLock(t *testing.T) {
	s, table, conn := editService(t)
	ctx := context.Background()
	done := make(chan *ScriptResult, 1)
	errs := make(chan error, 1)
	go func() {
		result, err := s.ExecuteConsoleScript(ctx, "c", "long", table.ConnectionID, "WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x < 1000000000) SELECT sum(x) FROM n; CREATE TABLE should_not_run(id INTEGER);")
		if err != nil {
			errs <- err
		} else {
			done <- result
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for conn.DB().Stats().InUse == 0 {
		if time.Now().After(deadline) {
			t.Fatal("script did not start")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := s.ExecuteConsoleQuery(ctx, "c", "query", entities.QueryInput{Query: "SELECT 1"}); err == nil {
		t.Fatal("query overlapped script")
	}
	if !s.CancelConsoleQuery("c", "long") {
		t.Fatal("script not cancellable")
	}
	select {
	case result := <-done:
		if !result.Cancelled || result.Error == "" {
			t.Fatalf("cancel: %+v", result)
		}
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not cancel script")
	}
	if _, err := conn.DB().Exec("SELECT * FROM should_not_run"); err == nil {
		t.Fatal("statement after Stop ran")
	}
}

func TestConfigurableImportSizeAndSQLReadOnlyLoading(t *testing.T) {
	previous := db.Conn
	settingsDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "preferences.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.Conn = settingsDB
	defer func() { settingsDB.Close(); db.Conn = previous }()
	if _, err = settingsDB.Exec("CREATE TABLE app_preferences(key TEXT PRIMARY KEY,value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "large.sql")
	text := "--" + strings.Repeat("x", 33<<20) + "\nCREATE TABLE never_executed(id INTEGER);"
	if err = os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	s, _, conn := editService(t)
	loaded, err := s.ReadSQLScript(context.Background(), path)
	if err != nil || loaded.SQL != text {
		t.Fatalf("unlimited default rejected >32 MiB: %v", err)
	}
	if _, err = conn.DB().Exec("SELECT * FROM never_executed"); err == nil {
		t.Fatal("opening SQL executed it")
	}
	settings := types.DefaultResultSettings()
	settings.ImportLimitMiB = 1
	if err = db.SaveResultSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadSQLScript(context.Background(), path); err == nil || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("configured limit ignored: %v", err)
	}
	settings.ImportLimitMiB = 64
	db.SaveResultSettings(context.Background(), settings)
	if _, err = s.ReadSQLScript(context.Background(), path); err != nil {
		t.Fatalf("increased limit: %v", err)
	}
	settings.ImportLimitMiB = 0
	db.SaveResultSettings(context.Background(), settings)
	if _, err = readTransferFile(context.Background(), path); err != nil {
		t.Fatalf("unlimited: %v", err)
	}
}
