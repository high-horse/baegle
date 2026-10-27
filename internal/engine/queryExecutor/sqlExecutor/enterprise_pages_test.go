package sqlExecutor

import (
	"context"
	"db-lens/internal/engine/entities"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func TestEnterprisePageQueries(t *testing.T) {
	for _, dialect := range []string{"oracle", "sqlserver", "db2"} {
		t.Run(dialect, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			alias := " AS _dbv_result"
			if dialect == "oracle" {
				alias = " dbv_result"
			}
			if dialect == "db2" {
				alias = " AS dbv_result"
			}
			query := "SELECT * FROM (\nSELECT id FROM items\n)" + alias + " ORDER BY 1 DESC\nOFFSET 2 ROWS FETCH NEXT 3 ROWS ONLY"
			mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8).AddRow(7).AddRow(6))
			result, err := NewPager().OpenRead(context.Background(), db, "test", dialect, entities.QueryInput{Query: "SELECT id FROM items;", Page: 2, PageSize: 2, SortColumn: 1, SortDirection: "desc"})
			if err != nil {
				t.Fatal(err)
			}
			if !result.HasMore || result.StartRow != 3 || len(result.Rows) != 2 || !result.CanNavigate {
				t.Fatalf("page: %+v", result)
			}
			count := "COUNT(*)"
			alias = " AS dbv_count"
			if dialect == "oracle" {
				alias = " dbv_count"
			}
			if dialect == "sqlserver" {
				count = "COUNT_BIG(*)"
			}
			mock.ExpectQuery("SELECT " + count + " FROM (\nSELECT id FROM items\n)" + alias).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3000000000)))
			total, err := CountReadDialect(context.Background(), db, "SELECT id FROM items;", dialect)
			if err != nil || total != 3000000000 {
				t.Fatalf("count %d: %v", total, err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestSQLServerComplexSelectKeepsStreamSemantics(t *testing.T) {
	for _, query := range []string{"WITH c AS (SELECT 1 AS n) SELECT * FROM c", "SELECT id FROM items ORDER BY id", "SELECT * FROM items OPTION(RECOMPILE)"} {
		if _, ok := PageableDialectSQL(query, "sqlserver"); ok {
			t.Fatalf("rewrapped %s", query)
		}
	}
	if _, ok := PageableDialectSQL("SELECT id FROM items", "sqlserver"); !ok {
		t.Fatal("simple SELECT should page")
	}
	if base, ok := PageableDialectSQL("SELECT id FROM items;", "oracle"); !ok || strings.HasSuffix(base, ";") {
		t.Fatal("invalid Oracle base SQL")
	}
}
