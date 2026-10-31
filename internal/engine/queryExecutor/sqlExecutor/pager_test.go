package sqlExecutor

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	_ "modernc.org/sqlite"
)

func testPager(t *testing.T) (*Pager, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	p := NewPager()
	t.Cleanup(func() { p.CloseConnection("test"); db.Close() })
	return p, db
}

func TestCursorStreamsLargeResultAndSurvivesRequest(t *testing.T) {
	p, db := testPager(t)
	ctx, cancel := context.WithCancel(context.Background())
	first, err := p.Open(ctx, db, "test", "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1000000) SELECT x FROM n", 100)
	if err != nil {
		t.Fatal(err)
	}
	cancel() // The Wails call has returned; subsequent pages must still work.
	if len(first.Rows) != 100 || !first.HasMore || first.StartRow != 1 {
		t.Fatalf("bad first page: %+v", first)
	}
	second, err := p.Fetch(first.Cursor, "test", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Rows) != 100 || second.StartRow != 101 || second.Rows[0][0] != int64(101) {
		t.Fatalf("bad second page: %+v", second)
	}
	replay, err := p.Fetch(first.Cursor, "test", 2)
	if err != nil || replay != second {
		t.Fatal("retry advanced the cursor", err)
	}
	if _, err := p.Fetch(first.Cursor, "other", 3); err == nil {
		t.Fatal("accepted another connection's cursor")
	}
	p.Close(first.Cursor, "test")
	if db.Stats().InUse != 0 {
		t.Fatal("cursor retained a connection")
	}
	if _, err := p.Fetch(first.Cursor, "test", 3); err == nil {
		t.Fatal("accepted closed cursor")
	}
}

func TestCursorBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, 100, 101, 200, 201} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			p, db := testPager(t)
			result, err := p.Open(context.Background(), db, "test", "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<201) SELECT x, NULL, CAST('text' AS BLOB) FROM n WHERE x <= "+strconv.Itoa(count), 100)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			for page := 2; ; page++ {
				for _, row := range result.Rows {
					seen++
					if row[0] != int64(seen) || row[1] != nil || row[2] != "text" {
						t.Fatalf("unexpected row: %v", row)
					}
				}
				if !result.HasMore {
					break
				}
				result, err = p.Fetch(result.Cursor, "test", page)
				if err != nil {
					t.Fatal(err)
				}
			}
			if seen != count || len(p.cursors) != 0 || db.Stats().InUse != 0 {
				t.Fatalf("seen=%d, expected=%d, cursors=%d", seen, count, len(p.cursors))
			}
		})
	}
}

func TestCursorPreservesSQLAndExecutesOnce(t *testing.T) {
	p, db := testPager(t)
	ctx := context.Background()
	if _, err := p.Open(ctx, db, "test", "CREATE TABLE data (id INTEGER)", 100); err != nil {
		t.Fatal(err)
	}
	result, err := p.Open(ctx, db, "test", "INSERT INTO data VALUES (1), (2), (3) RETURNING id", 1)
	if err != nil {
		t.Fatal(err)
	}
	for page := 2; result.HasMore; page++ {
		result, err = p.Fetch(result.Cursor, "test", page)
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM data").Scan(&count); err != nil || count != 3 {
		t.Fatal("statement re-executed", count, err)
	}
	result, err = p.Open(ctx, db, "test", "SELECT id FROM data ORDER BY id DESC LIMIT 2", 1)
	if err != nil || result.Rows[0][0] != int64(3) {
		t.Fatal("SQL changed", err)
	}
	result, err = p.Fetch(result.Cursor, "test", 2)
	if err != nil || result.HasMore || result.Rows[0][0] != int64(2) {
		t.Fatal("explicit limit changed", err)
	}
	if _, err := p.Open(ctx, db, "test", "invalid SQL", 100); err == nil {
		t.Fatal("missing SQL error")
	}
	if len(p.cursors) != 0 {
		t.Fatal("failed query leaked cursor")
	}
}

func TestCursorValidationAndConnectionCleanup(t *testing.T) {
	p, db := testPager(t)
	for _, size := range []int{-1, -100} {
		if _, err := p.Open(context.Background(), db, "test", "SELECT 1", size); err == nil {
			t.Fatal("accepted invalid size")
		}
	}
	result, err := p.Open(context.Background(), db, "test", "SELECT 1 UNION ALL SELECT 2", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Fetch(result.Cursor, "test", 9); err == nil {
		t.Fatal("accepted skipped page")
	}
	p.CloseConnection("test")
	if len(p.cursors) != 0 || db.Stats().InUse != 0 {
		t.Fatal("connection cleanup leaked stream")
	}
}

func TestCursorReadFailureClosesStream(t *testing.T) {
	p, db := testPager(t)
	result, err := p.Open(context.Background(), db, "test", "SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT abs(-9223372036854775808)", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Fetch(result.Cursor, "test", 2); err == nil {
		t.Fatal("missing driver error from lookahead")
	}
	if len(p.cursors) != 0 || db.Stats().InUse != 0 {
		t.Fatal("read error leaked cursor")
	}
}

func TestAlreadyCancelledRequestDoesNotExecute(t *testing.T) {
	p, db := testPager(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Open(ctx, db, "test", "CREATE TABLE should_not_exist (id INT)", 100); err == nil {
		t.Fatal("executed cancelled request")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name = 'should_not_exist'").Scan(&count); err != nil || count != 0 {
		t.Fatal("cancelled statement ran", err)
	}
}
