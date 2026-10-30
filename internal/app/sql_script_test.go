package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"db-lens/internal/db"
	"db-lens/internal/engine/entities"
	"db-lens/internal/types"
)

func TestScriptStatementResultsAndPreviewLimits(t *testing.T) {
	s, table, conn := editService(t)
	result, err := s.ExecuteConsoleScript(context.Background(), "results", "1", table.ConnectionID, `
CREATE TEMP TABLE imported(id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO imported VALUES(1, 'one;two') RETURNING id, name;
SELECT name, NULL AS empty, 9007199254740993 AS big FROM imported;
WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x < 1100) SELECT x FROM n;
INSERT INTO imported VALUES(2, 'after preview');
SELECT count(*) AS total FROM imported;
SELECT * FROM missing;
INSERT INTO imported VALUES(3, 'must not run');`)
	if err != nil || result.Completed != 6 || result.Total != 8 || len(result.Statements) != 7 || result.Error == "" {
		t.Fatalf("script summary: %+v %v", result, err)
	}
	returning := result.Statements[1].Results[0]
	if len(returning.Rows) != 1 || returning.Rows[0][1] != "one;two" {
		t.Fatalf("RETURNING result: %+v", returning)
	}
	selected := result.Statements[2].Results[0]
	if selected.Columns[0] != "name" || selected.Rows[0][1] != nil || selected.Rows[0][2] != "9007199254740993" {
		t.Fatalf("query values: %+v", selected)
	}
	limited := result.Statements[3].Results[0]
	if !limited.Truncated || len(limited.Rows) != 1000 {
		t.Fatalf("preview limit: %+v", limited)
	}
	if count := result.Statements[4].RowsAffected; count == nil || *count != 1 {
		t.Fatalf("affected rows: %v", count)
	}
	if count := result.Statements[5].Results[0].Rows[0][0]; count != int64(2) {
		t.Fatalf("RETURNING ran more than once or later insert failed: %v", count)
	}
	if result.Statements[6].Error == "" || result.Statements[6].Line != 8 {
		t.Fatalf("failed statement: %+v", result.Statements[6])
	}
	if s.GetConsoleScriptProgress("results", "1") != nil {
		t.Fatal("completed progress was not cleaned up")
	}
	// The script's temporary table must not leak onto a pooled connection.
	if _, err := conn.DB().Exec("SELECT * FROM imported"); err == nil {
		t.Fatal("script session returned to pool")
	}
}

func TestScriptBatchMultipleResultSets(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	mock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"first"}).AddRow([]byte("one")),
		sqlmock.NewRows([]string{"second"}).AddRow(2),
	).RowsWillBeClosed()
	session, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	entry := ScriptStatementResult{}
	remaining := 1
	if err := executeScriptStatement(context.Background(), session, "sqlserver", "SELECT 1; SELECT 2;", &entry, &remaining); err != nil {
		t.Fatal(err)
	}
	if len(entry.Results) != 2 || entry.Results[0].Rows[0][0] != "one" || !entry.Results[1].Truncated || len(entry.Results[1].Rows) != 0 {
		t.Fatalf("batch results: %+v", entry.Results)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

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
	for {
		progress := s.GetConsoleScriptProgress("c", "long")
		if progress != nil && progress.Current == 1 && progress.Total == 2 && progress.Completed == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("live progress: %+v", progress)
		}
		time.Sleep(time.Millisecond)
	}
	if s.GetConsoleScriptProgress("c", "other") != nil {
		t.Fatal("progress leaked across requests")
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
