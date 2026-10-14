package sqlExecutor

import (
	"context"
	"database/sql"
	"db-lens/internal/engine/entities"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"testing"
	"time"
)

func TestMySQLBoundedPagesAndCancellation(t *testing.T) {
	dsn := os.Getenv("DBVIEWER_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set DBVIEWER_TEST_MYSQL_DSN for an isolated MySQL test database")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "CREATE TEMPORARY TABLE dbv_page_test (id INT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO dbv_page_test WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<500) SELECT x FROM n`); err != nil {
		t.Fatal(err)
	}
	p := NewPager()
	input := entities.QueryInput{Query: "SELECT id, SLEEP(0.001) AS slow FROM dbv_page_test ORDER BY id", PageSize: 100}
	first, err := p.OpenRead(ctx, db, "mysql-test", "mysql", input)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) != 100 || !first.HasMore || first.TotalRows != nil || first.Cursor != "" {
		t.Fatalf("bad page: %+v", first)
	}
	// Would block with one connection if the first page retained an open stream.
	input.Page = 2
	second, err := p.OpenRead(ctx, db, "mysql-test", "mysql", input)
	if err != nil || second.StartRow != 101 {
		t.Fatal("second page failed", err)
	}
	input.Query = "SELECT COUNT(*) FROM dbv_page_test"
	input.Page = 1
	count, err := p.OpenRead(ctx, db, "mysql-test", "mysql", input)
	if err != nil || len(count.Rows) != 1 {
		t.Fatal("explicit count failed", err)
	}
	slow, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	_, err = p.OpenRead(slow, db, "mysql-test", "mysql", entities.QueryInput{Query: "SELECT SUM(SLEEP(0.01)) FROM dbv_page_test"})
	if err == nil || slow.Err() == nil {
		t.Fatal("slow aggregate was not cancelled", err)
	}
	if _, err = db.ExecContext(ctx, "SELECT 1"); err != nil {
		t.Fatal("connection pool unusable after cancellation", err)
	}
}
