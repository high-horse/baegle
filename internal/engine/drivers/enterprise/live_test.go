package enterprise

import (
	"context"
	"database/sql"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/queryExecutor/sqlExecutor"
	"os"
	"strings"
	"testing"
	"time"
)

// Opt-in read-only smoke tests. No credentials are committed or logged, and no
// objects are created. Use the driver's DSN syntax described in database-drivers.md.
func TestLiveEnterpriseDatabase(t *testing.T) {
	for _, dialect := range []string{"oracle", "sqlserver", "db2"} {
		t.Run(dialect, func(t *testing.T) {
			prefix := "DB_VIEWER_" + strings.ToUpper(dialect)
			dsn := os.Getenv(prefix + "_DSN")
			if dsn == "" {
				t.Skip("no live database DSN configured")
			}
			if dialect == "db2" && !db2Enabled {
				t.Fatal("Db2 smoke tests require -tags db2 and IBM CLI libraries")
			}
			driver := dialect
			if dialect == "db2" {
				driver = "go_ibm_db"
			}
			db, err := sql.Open(driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err = db.PingContext(ctx); err != nil {
				t.Fatal(err)
			}
			query := "SELECT 42 AS answer"
			if dialect == "oracle" {
				query += " FROM dual"
			}
			if dialect == "db2" {
				query += " FROM SYSIBM.SYSDUMMY1"
			}
			result, err := sqlExecutor.NewPager().OpenRead(ctx, db, "smoke", dialect, entities.QueryInput{Query: query, PageSize: 10, Page: 1})
			if err != nil || result == nil || len(result.Rows) != 1 {
				t.Fatalf("query smoke test: %v", err)
			}
			total, err := sqlExecutor.CountReadDialect(ctx, db, query, dialect)
			if err != nil || total != 1 {
				t.Fatalf("count: %d %v", total, err)
			}
			conn := &Connection{config: entities.ConnectionConfig{ID: "smoke", Type: dialect}, db: db}
			tables, err := New(dialect).Inspector().ListTables(ctx, conn)
			if err != nil {
				t.Fatal(err)
			}
			if tableName := os.Getenv(prefix + "_TABLE"); tableName != "" {
				table := entities.InspectTableInfo{Name: tableName, Schema: os.Getenv(prefix + "_SCHEMA")}
				columns, err := New(dialect).Inspector().ListColumns(ctx, conn, table)
				if err != nil || len(columns) == 0 {
					t.Fatalf("columns: %v", err)
				}
			}
			t.Logf("Connected; paged query and count passed; %d accessible tables/views", len(tables))
		})
	}
}
