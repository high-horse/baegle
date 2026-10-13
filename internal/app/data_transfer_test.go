package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"db-lens/internal/engine/datatransfer"
	"db-lens/internal/engine/entities"
)

func TestTransferTableRoundTrip(t *testing.T) {
	for _, format := range []string{"csv", "json", "sql", "xlsx"} {
		t.Run(format, func(t *testing.T) {
			s, table, conn := editService(t)
			ctx := context.Background()
			if _, err := conn.DB().Exec(`CREATE TABLE items(id INTEGER PRIMARY KEY,name TEXT,note TEXT,blob BLOB); INSERT INTO items VALUES(9007199254740993,'O''Reilly, 😀',NULL,X'0001FF'),(2,'','\N',X''); CREATE TABLE target(id INTEGER PRIMARY KEY,name TEXT,note TEXT,blob BLOB)`); err != nil {
				t.Fatal(err)
			}
			options := datatransfer.Options{Format: format, Delimiter: ",", NullToken: `\N`}
			var file bytes.Buffer
			rows, err := s.writeExport(ctx, &file, ExportRequest{ConnectionID: table.ConnectionID, Scope: "table", Table: table, Options: options})
			if err != nil || rows != 2 {
				t.Fatalf("export: %d %v", rows, err)
			}
			path := filepath.Join(t.TempDir(), "export."+format)
			if err = os.WriteFile(path, file.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			target := table
			target.Name = "target"
			input := ImportRequest{Path: path, Table: target, Options: options, Mapping: map[string]string{"id": "id", "name": "name", "note": "note", "blob": "blob"}}
			preview, err := s.PreviewImport(ctx, input)
			if err != nil || preview.Total != 2 {
				t.Fatalf("preview: %+v %v", preview, err)
			}
			input.SHA = preview.SHA
			result, err := s.ImportData(ctx, input)
			if err != nil || result.Applied != 2 {
				t.Fatalf("import: %+v %v", result, err)
			}
			var differences int
			if err = conn.DB().QueryRow(`SELECT COUNT(*) FROM (SELECT * FROM items EXCEPT SELECT * FROM target)`).Scan(&differences); err != nil || differences != 0 {
				t.Fatalf("roundtrip changed values: %d %v", differences, err)
			}
		})
	}
}
func TestTransferValidationAndRollback(t *testing.T) {
	s, table, conn := editService(t)
	ctx := context.Background()
	if _, err := conn.DB().Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY,name TEXT NOT NULL UNIQUE, note TEXT DEFAULT 'default')`); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	os.WriteFile(path, []byte(`[{"name":"same"},{"name":"same"}]`), 0600)
	input := ImportRequest{Path: path, Table: table, Options: datatransfer.Options{Format: "json"}, Mapping: map[string]string{"name": "name"}}
	preview, err := s.PreviewImport(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.SHA = preview.SHA
	if _, err = s.ImportData(ctx, input); err == nil {
		t.Fatal("expected unique failure")
	}
	var count int
	conn.DB().QueryRow(`SELECT count(*) FROM items`).Scan(&count)
	if count != 0 {
		t.Fatal("partial import was committed")
	}
	os.WriteFile(path, []byte(`[{"name":"new"}]`), 0600)
	if _, err = s.ImportData(ctx, input); err == nil || !strings.Contains(err.Error(), "file changed") {
		t.Fatalf("changed file accepted: %v", err)
	}
	preview, _ = s.PreviewImport(ctx, input)
	input.SHA = preview.SHA
	input.Mapping = map[string]string{"name": "unknown"}
	if _, err = s.ImportData(ctx, input); err == nil {
		t.Fatal("unknown column accepted")
	}
	input.Mapping = map[string]string{"name": "name"}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.ImportData(cancelled, input); err == nil {
		t.Fatal("cancelled import committed")
	}
	input.Table.ConnectionID = "other"
	if _, err = s.ImportData(ctx, input); err == nil {
		t.Fatal("cross connection import accepted")
	}
	input.Table = table
	config := conn.Config()
	config.ReadOnly = true
	if _, err = s.Connect(ctx, config); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportData(ctx, input); err == nil {
		t.Fatal("read-only import accepted")
	}
}
func TestTransferAllQueryRowsAndPageScope(t *testing.T) {
	s, table, conn := editService(t)
	ctx := context.Background()
	if _, err := conn.DB().Exec(`CREATE TABLE items(id INTEGER); INSERT INTO items VALUES(1),(2),(3)`); err != nil {
		t.Fatal(err)
	}
	page, err := s.ExecuteQuery(ctx, entities.QueryInput{Query: `SELECT * FROM items`, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	input := ExportRequest{ConnectionID: table.ConnectionID, Scope: "query", Query: `SELECT * FROM items WHERE id > 1`, Options: datatransfer.Options{Format: "json"}}
	count, err := s.writeExport(ctx, &b, input)
	if err != nil || count != 2 {
		t.Fatalf("all rows: %d %v", count, err)
	}
	if page.Cursor != "" {
		if _, err = s.pager.Fetch(page.Cursor, table.ConnectionID, 2); err != nil {
			t.Fatalf("export disturbed result cursor: %v", err)
		}
	}
	input.Query = `SELECT * FROM items; DELETE FROM items`
	if _, err = s.writeExport(ctx, &b, input); err == nil {
		t.Fatal("export accepted multiple statements")
	}
	input.Scope = "page"
	input.Columns = []string{"id"}
	input.Rows = [][]json.RawMessage{{raw(42)}}
	count, err = s.writeExport(ctx, &b, input)
	if err != nil || count != 1 {
		t.Fatalf("page scope: %d %v", count, err)
	}
}

func TestImportBinaryValidation(t *testing.T) {
	for _, input := range []string{`true`, `42`, `[]`, `{"$binary":{}}`, `"not base64!"`} {
		if _, err := importValue(json.RawMessage(input), "BLOB"); err == nil {
			t.Fatalf("accepted invalid binary value %s", input)
		}
	}
}
