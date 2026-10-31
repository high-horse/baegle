package nosql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
)

type fakeBackend struct {
	backend
	calls int
	fail  bool
}

func (b *fakeBackend) query(_ context.Context, q string, size int, state []byte) (*entities.QueryResult, []byte, error) {
	b.calls++
	if b.fail {
		return nil, nil, errors.New("network error")
	}
	if q != "original" {
		return nil, nil, fmt.Errorf("query was replaced")
	}
	result := valueResult("ok")
	// Empty intermediate pages and variable-sized pages are valid for SCAN.
	switch string(state) {
	case "":
		result.Rows = [][]any{}
		return result, []byte("next"), nil
	case "next":
		result.Rows = [][]any{{"a"}, {"b"}, {"c"}}
		return result, []byte("last"), nil
	default:
		return result, nil, nil
	}
}
func (b *fakeBackend) close() error { return nil }
func TestNativeCursorLifecycle(t *testing.T) {
	backend := &fakeBackend{}
	c := &Connection{cfg: entities.ConnectionConfig{ID: "one"}, backend: backend, transport: transports.NewDirect("localhost", 1), connected: true, cursors: map[string]*cursor{}}
	ctx := context.Background()
	first, err := c.ExecuteInput(ctx, entities.QueryInput{Query: "original", PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.StartRow != 1 || !first.HasMore || first.Cursor == "" {
		t.Fatalf("first: %+v", first)
	}
	id := first.Cursor
	input := entities.QueryInput{Type: entities.QueryExecutionFetchPaged, Cursor: id, Page: 2, Query: "replacement"}
	second, err := c.ExecuteInput(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if second.StartRow != 1 || len(second.Rows) != 3 {
		t.Fatalf("second: %+v", second)
	}
	retry, err := c.ExecuteInput(ctx, input)
	if err != nil || retry != second || backend.calls != 2 {
		t.Fatal("retry reexecuted a page", err)
	}
	input.Page = 4
	if _, err = c.ExecuteInput(ctx, input); err == nil {
		t.Fatal("accepted skipped page")
	}
	input.Page = 3
	backend.fail = true
	if _, err = c.ExecuteInput(ctx, input); err == nil {
		t.Fatal("missing failure")
	}
	backend.fail = false
	last, err := c.ExecuteInput(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if last.HasMore || last.Cursor != "" || last.StartRow != 4 {
		t.Fatalf("last: %+v", last)
	}
	other := &Connection{connected: true, cursors: map[string]*cursor{}}
	if _, err = other.ExecuteInput(ctx, input); err == nil {
		t.Fatal("accepted foreign cursor")
	}
	c.cursors[id].used = time.Now().Add(-11 * time.Minute)
	if _, err = c.ExecuteInput(ctx, input); err == nil {
		t.Fatal("accepted expired cursor")
	}
	if err = c.Disconnect(); err != nil || len(c.cursors) != 0 {
		t.Fatal("disconnect failed", err)
	}
}
func TestNativeCursorBoundsAndClose(t *testing.T) {
	c := &Connection{connected: true, backend: &fakeBackend{}, cursors: map[string]*cursor{}}
	for i := 0; i < 20; i++ {
		result, err := c.ExecuteInput(context.Background(), entities.QueryInput{Query: "original", PageSize: 1000})
		if err != nil {
			t.Fatal(err)
		}
		if result.PageSize != 1000 {
			t.Fatal("requested page size was changed")
		}
	}
	if len(c.cursors) != 16 {
		t.Fatal("cursor limit", len(c.cursors))
	}
	for id := range c.cursors {
		_, err := c.ExecuteInput(context.Background(), entities.QueryInput{Type: entities.QueryExecutionClose, Cursor: id})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(c.cursors) != 0 {
		t.Fatal("close retained cursors")
	}
}
func TestCQLValidation(t *testing.T) {
	for _, q := range []string{"SELECT * FROM t", "-- comment\nSELECT * FROM t;", "/* write? */ // note\n SELECT * FROM t WHERE k='a;b'"} {
		_, read, err := cqlStatement(q)
		if err != nil || !read {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for _, q := range []string{"USE other", "SELECT * FROM t; DELETE FROM t WHERE k=1", "", "/* unclosed", "BEGIN BATCH INSERT INTO t (k) VALUES (1); APPLY BATCH"} {
		if _, _, err := cqlStatement(q); err == nil {
			t.Fatalf("accepted %s", q)
		}
	}
	b := &cassandraBackend{cfg: entities.ConnectionConfig{ReadOnly: true}}
	if _, _, err := b.query(context.Background(), "/* prefix */ DELETE FROM t WHERE k=1", 100, nil); err == nil {
		t.Fatal("read-only write accepted")
	}
	if got := cqlValue(int64(9223372036854775807)); got != "9223372036854775807" {
		t.Fatal("lost bigint precision", got)
	}
}
