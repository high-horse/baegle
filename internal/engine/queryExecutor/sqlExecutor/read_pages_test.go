package sqlExecutor

import (
	"context"
	"testing"
	"time"

	"db-lens/internal/engine/entities"
)

func TestResultExpressionsApplyBeforePagination(t *testing.T) {
	p, db := testPager(t)
	ctx := context.Background()
	_, err := db.Exec(`CREATE TABLE filtered_items (id INTEGER PRIMARY KEY, score INTEGER);
	WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<250)
	INSERT INTO filtered_items SELECT x, x % 3 FROM n`)
	if err != nil {
		t.Fatal(err)
	}
	input := entities.QueryInput{Query: "SELECT id, score FROM filtered_items ORDER BY id; -- original", PageSize: 10, Where: "id > 200", OrderBy: "2 ASC, 1 DESC"}
	first, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) != 10 || first.Rows[0][0] != int64(249) || first.Rows[9][0] != int64(222) || !first.HasMore {
		t.Fatalf("filter/order must include rows beyond original first page: %+v", first)
	}
	input.Page = 2
	second, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil {
		t.Fatal(err)
	}
	if second.Rows[0][0] != int64(219) || second.StartRow != 11 {
		t.Fatalf("wrong second page: %+v", second)
	}
	filtered, err := FilterReadSQL(input.Query, input.Where, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	total, err := CountRead(ctx, db, filtered)
	if err != nil || total != 50 {
		t.Fatalf("filtered count = %d, %v", total, err)
	}
	input.Page = 5
	last, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || last.HasMore || last.TotalRows == nil || *last.TotalRows != 50 {
		t.Fatalf("wrong last page: %+v, %v", last, err)
	}
	input.Page = 1
	input.Where = "id > 999"
	empty, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || len(empty.Rows) != 0 || *empty.TotalRows != 0 {
		t.Fatalf("wrong empty page: %+v, %v", empty, err)
	}
	input.Where = ""
	input.OrderBy = "1 DESC"
	sorted, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || sorted.Rows[0][0] != int64(250) {
		t.Fatalf("ORDER BY alone is not global: %+v, %v", sorted, err)
	}
	input.OrderBy = ""
	reset, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || reset.Rows[0][0] != int64(1) {
		t.Fatalf("reset failed: %+v, %v", reset, err)
	}
	input.Query = "SELECT id, score FROM filtered_items ORDER BY id LIMIT 205"
	input.Where = "id > 200"
	limited, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || len(limited.Rows) != 5 {
		t.Fatalf("original limit lost: %+v, %v", limited, err)
	}
	input.Where = "id > 1); DELETE FROM filtered_items; --"
	if _, err := p.OpenRead(ctx, db, "test", "sqlite", input); err == nil {
		t.Fatal("accepted multiple statements in WHERE")
	}
	input.Where = ""
	input.OrderBy = "1 DESC; DELETE FROM filtered_items"
	if _, err := p.OpenRead(ctx, db, "test", "sqlite", input); err == nil {
		t.Fatal("accepted invalid ORDER BY")
	}
}

func TestReadPagesBoundedJumpAndGlobalSort(t *testing.T) {
	p, db := testPager(t)
	ctx := context.Background()
	if _, err := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, score INTEGER); WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<250) INSERT INTO items SELECT x, 251-x FROM n"); err != nil {
		t.Fatal(err)
	}
	input := entities.QueryInput{Query: "SELECT id, score FROM items ORDER BY id; -- keep comment", PageSize: 100, Page: 1}
	first, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil {
		t.Fatal(err)
	}
	if !first.CanNavigate || first.TotalRows != nil || first.Cursor != "" || !first.HasMore || first.Rows[0][0] != int64(1) {
		t.Fatalf("bad first page: %+v", first)
	}
	input.Page = 2
	second, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || second.StartRow != 101 || second.TotalRows != nil || second.Cursor != "" {
		t.Fatal("stream next/count failed", err)
	}
	p.Close(first.Cursor, "test")
	input.Page = 3
	last, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || last.StartRow != 201 || len(last.Rows) != 50 || last.HasMore || last.Rows[0][0] != int64(201) {
		t.Fatal("direct last page failed", err)
	}
	input.Page = 2
	back, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || back.StartRow != 101 {
		t.Fatal("backwards offset jump failed", err)
	}
	input.Page = 3
	last, err = p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || last.StartRow != 201 {
		t.Fatal("stream sequence after jump failed", err)
	}
	input.Page = 1
	input.SortColumn = 2
	input.SortDirection = "asc"
	sorted, err := p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil {
		t.Fatal(err)
	}
	// The lowest scores originate beyond the first unsorted page.
	if sorted.Rows[0][0] != int64(250) || sorted.Rows[99][0] != int64(151) {
		t.Fatal("sort only applied to first page")
	}
	p.Close(sorted.Cursor, "test")
	input.Query = "SELECT id, score FROM items ORDER BY id LIMIT 205"
	input.Page = 3
	last, err = p.OpenRead(ctx, db, "test", "sqlite", input)
	if err != nil || *last.TotalRows != 205 || len(last.Rows) != 5 || last.Rows[0][0] != int64(5) {
		t.Fatal("original limit not preserved with global sort", err)
	}
}

func TestReadPageBoundaryAndValidation(t *testing.T) {
	p, db := testPager(t)
	for _, query := range []string{"SELECT 1 AS value WHERE false", "SELECT 1 AS value"} {
		result, err := p.OpenRead(context.Background(), db, "test", "sqlite", entities.QueryInput{Query: query, Page: 1, PageSize: 100})
		if err != nil || result.StartRow != 1 || result.HasMore || result.TotalRows == nil {
			t.Fatal("empty/single page failed", err)
		}
	}
	for _, input := range []entities.QueryInput{
		{Query: "DELETE FROM items RETURNING id"},
		{Query: "SELECT 1", SortColumn: 1, SortDirection: "desc; DROP TABLE items"},
		{Query: "SELECT 1", SortColumn: -1},
		{Query: "SELECT 1", SortColumn: 2, SortDirection: "asc"},
		{Query: "SELECT 1", PageSize: 501},
	} {
		if _, err := p.OpenRead(context.Background(), db, "test", "sqlite", input); err == nil {
			t.Fatalf("accepted invalid input: %+v", input)
		}
	}
}

func TestPageableSQL(t *testing.T) {
	for _, query := range []string{
		"SELECT * FROM items; -- comment", "/* hi */ SELECT 'a;b'", "SELECT \"update\" FROM t",
		"WITH n AS (SELECT 1) SELECT * FROM n", "SELECT 'it''s fine'", "SELECT `odd;name` FROM t",
	} {
		if _, ok := PageableSQL(query); !ok {
			t.Errorf("rejected read: %s", query)
		}
	}
	for _, query := range []string{
		"INSERT INTO t VALUES(1) RETURNING id", "SELECT 1; DELETE FROM t", "SELECT * INTO temp FROM t",
		"WITH deleted AS (DELETE FROM t RETURNING *) SELECT * FROM deleted", "SELECT * FROM t FOR UPDATE",
		"SELECT * FROM t LOCK IN SHARE MODE", "SELECT 1 /*! INTO OUTFILE 'x' */", "SELECT $$text$$",
		"SELECT 'unterminated", "/* unterminated", "SELECT @value := 1", "SELECT 1 /* nested /* comment */ */",
	} {
		if _, ok := PageableSQL(query); ok {
			t.Errorf("accepted non-pageable statement: %s", query)
		}
	}
}

func TestReadFirstPageDoesNotCountOrHoldConnection(t *testing.T) {
	p, db := testPager(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// An unbounded recursive result cannot be counted. The first page must still
	// return promptly, and the one-connection pool must be immediately reusable.
	result, err := p.OpenRead(ctx, db, "test", "sqlite", entities.QueryInput{
		Query: "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT x FROM n", PageSize: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 100 || !result.HasMore || result.TotalRows != nil || result.Cursor != "" {
		t.Fatalf("unexpected page: %+v", result)
	}
	var value int
	if err := db.QueryRowContext(ctx, "SELECT 42").Scan(&value); err != nil || value != 42 {
		t.Fatal("page retained the database connection", err)
	}
}

func TestReadExactBoundaryAndAggregate(t *testing.T) {
	p, db := testPager(t)
	ctx := context.Background()
	for _, query := range []string{
		"WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<100) SELECT x FROM n",
		"SELECT count(*) FROM (SELECT 1 UNION ALL SELECT 2)",
	} {
		result, err := p.OpenRead(ctx, db, "test", "sqlite", entities.QueryInput{Query: query, PageSize: 100})
		if err != nil {
			t.Fatal(err)
		}
		if result.HasMore || result.Cursor != "" || result.TotalRows == nil || *result.TotalRows != int64(len(result.Rows)) {
			t.Fatalf("bad exact result: %+v", result)
		}
		if len(result.Rows) == 1 && result.Rows[0][0] != int64(2) {
			t.Fatal("aggregate result changed")
		}
	}
}

func TestExplicitCountHonorsResultShapeAndCancellation(t *testing.T) {
	p, db := testPager(t)
	_ = p
	ctx := context.Background()
	for _, tc := range []struct {
		query string
		total int64
	}{
		{"SELECT 1 WHERE false", 0},
		{"SELECT x FROM (SELECT 1 AS x UNION ALL SELECT 2 UNION ALL SELECT 2) WHERE x = 2", 2},
		{"SELECT x FROM (SELECT 1 AS x UNION ALL SELECT 2 UNION ALL SELECT 2) GROUP BY x", 2},
		{"SELECT 1 UNION ALL SELECT 2 LIMIT 1", 1},
		{"SELECT COUNT(*) FROM (SELECT 1 UNION ALL SELECT 2)", 1},
	} {
		total, err := CountRead(ctx, db, tc.query)
		if err != nil || total != tc.total {
			t.Fatalf("%s: got %d, %v", tc.query, total, err)
		}
	}
	if _, err := CountRead(ctx, db, "CREATE TABLE should_not_run (id INT)"); err == nil {
		t.Fatal("accepted a command")
	}
	timed, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := CountRead(timed, db, "WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n) SELECT x FROM n"); err == nil || timed.Err() == nil {
		t.Fatal("count did not cancel", err)
	}
	if _, err := CountRead(ctx, db, "SELECT 1"); err != nil {
		t.Fatal("connection unusable after count cancellation", err)
	}
}
