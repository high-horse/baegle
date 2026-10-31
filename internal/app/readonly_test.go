package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/datatransfer"
	"db-lens/internal/engine/entities"
)

func TestReadOnlyConnectionQueriesAndNativeProtection(t *testing.T) {
	s, table, conn := editService(t)
	ctx := context.Background()
	if _, err := conn.DB().Exec("CREATE TABLE items(id INTEGER PRIMARY KEY, name TEXT); INSERT INTO items VALUES(1,'original'),(2,'second')"); err != nil {
		t.Fatal(err)
	}
	config := conn.Config()
	config.ReadOnly = true
	if _, err := s.Connect(ctx, config); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"UPDATE items SET name='changed' RETURNING *", "DELETE FROM items", "INSERT INTO items VALUES(3,'new')",
		"CREATE TABLE another(id INT)", "DROP TABLE items", "SELECT 1; DELETE FROM items",
		"PRAGMA query_only=OFF", "ATTACH DATABASE ':memory:' AS other", "SELECT load_extension('missing')",
	} {
		if _, err := s.ExecuteConsoleQuery(ctx, "readonly", "request", entities.QueryInput{Query: query}); err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Errorf("write not blocked: %s: %v", query, err)
		}
	}
	page, err := s.ExecuteQuery(ctx, entities.QueryInput{Query: "SELECT * FROM items ORDER BY id", PageSize: 1})
	if err != nil || len(page.Rows) != 1 || !page.HasMore {
		t.Fatalf("read failed: %+v %v", page, err)
	}
	next, err := s.ExecuteQuery(ctx, entities.QueryInput{Query: "SELECT * FROM items ORDER BY id", Type: entities.QueryExecutionNavigate, Page: 2, PageSize: 1})
	if err != nil || len(next.Rows) != 1 || next.Rows[0][0] != int64(2) {
		t.Fatalf("page failed: %+v %v", next, err)
	}
	count, err := s.ExecuteQuery(ctx, entities.QueryInput{Query: "SELECT * FROM items", Type: entities.QueryExecutionCount})
	if err != nil || count.TotalRows == nil || *count.TotalRows != 2 {
		t.Fatalf("count failed: %+v %v", count, err)
	}
	if _, err := s.ExecuteQuery(ctx, entities.QueryInput{Query: "PRAGMA table_info(items)"}); err != nil {
		t.Fatal(err)
	}
	info, err := s.DescribeTableEdit(ctx, table)
	if err != nil || info.CanInsert || info.CanModify {
		t.Fatalf("read-only editor: %+v %v", info, err)
	}
	reads, err := s.ExecuteConsoleScript(ctx, "readonly", "script", config.ID, "SELECT * FROM items; SELECT count(*) FROM items;")
	if err != nil || reads.Completed != 2 || reads.Error != "" {
		t.Fatalf("read script failed: %+v %v", reads, err)
	}
	if _, err := s.ExecuteConsoleScript(ctx, "readonly", "mixed", config.ID, "SELECT * FROM items; DELETE FROM items;"); err == nil {
		t.Fatal("mixed script accepted")
	}
	if _, err := s.SaveTableChanges(ctx, entities.TableChanges{Table: table, Changes: []entities.RowChange{{Operation: "delete"}}}); err == nil {
		t.Fatal("row deletion accepted")
	}
	var output bytes.Buffer
	rows, err := s.writeExport(ctx, &output, ExportRequest{ConnectionID: config.ID, Scope: "query", Query: "SELECT * FROM items", Options: datatransfer.Options{Format: "csv"}})
	if err != nil || rows != 2 || !strings.Contains(output.String(), "original") {
		t.Fatalf("export failed: %d %v %s", rows, err, output.String())
	}
	active, _ := s.manager.Active()
	readDB := active.(manager.SQLConnection).DB()
	// Even code bypassing the UI/query guard cannot mutate this SQLite file.
	for _, query := range []string{"INSERT INTO items VALUES(3,'bypass')", "DROP TABLE items"} {
		if _, err := readDB.ExecContext(ctx, query); err == nil {
			t.Fatalf("native read-only mode allowed %s", query)
		}
	}
	config.ReadOnly = false
	if _, err := s.Connect(ctx, config); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecuteQuery(ctx, entities.QueryInput{Query: "UPDATE items SET name='allowed' WHERE id=1"}); err != nil {
		t.Fatalf("writable reconnect failed: %v", err)
	}
}
