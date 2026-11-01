package sqlExecutor

import (
	"context"
	"encoding/json"
	"testing"

	"db-lens/internal/engine/entities"
)

func TestBinaryValuesPreserveBytesAcrossJSON(t *testing.T) {
	columns := []entities.ColumnInfo{{DatabaseType: "BLOB"}, {DatabaseType: "TEXT"}}
	result := normalizeRow([]interface{}{[]byte{0, 65, 255}, []byte("hello")}, columns...)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `["0x0041ff","hello"]` {
		t.Fatalf("unexpected JSON: %s", encoded)
	}
	for _, name := range []string{"bytea", "VARBINARY(50)", "LONG RAW", "image"} {
		if !binaryColumn(name) {
			t.Fatalf("missed binary type %q", name)
		}
	}
	if binaryColumn("VARCHAR") {
		t.Fatal("text classified as binary")
	}
}

func TestCursorPreservesBinaryBytes(t *testing.T) {
	pager, db := testPager(t)
	if _, err := db.Exec(`CREATE TABLE blobs (payload BLOB); INSERT INTO blobs VALUES (X'0041FF'), (X'')`); err != nil {
		t.Fatal(err)
	}
	result, err := pager.Open(context.Background(), db, "test", "SELECT payload FROM blobs", 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows[0][0] != "0x0041ff" {
		t.Fatalf("unexpected bytes: %#v", result.Rows)
	}

	next, err := pager.Fetch(result.Cursor, "test", 2)
	if err != nil {
		t.Fatal(err)
	}
	if next.Rows[0][0] != "0x" {
		t.Fatalf("empty binary changed: %#v", next.Rows)
	}
}
