package enterprise

import (
	"context"
	"database/sql"
	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

type testConnection struct {
	manager.Connection
	db *sql.DB
}

func (c testConnection) DB() *sql.DB          { return c.db }
func (c testConnection) DatabaseName() string { return "test" }
func TestCatalogMapping(t *testing.T) {
	for _, dialect := range []string{"oracle", "sqlserver", "db2"} {
		t.Run(dialect, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			conn := testConnection{db: db}
			inspector := New(dialect)
			mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"schema", "table", "type"}).AddRow("APP", "ITEMS", "TABLE").AddRow("APP", "ITEM_VIEW", "VIEW"))
			tables, err := inspector.ListTables(context.Background(), conn)
			if err != nil || len(tables) != 2 || tables[0].Schema != "APP" || tables[1].Type != "VIEW" {
				t.Fatalf("tables: %+v %v", tables, err)
			}
			mock.ExpectQuery("SELECT").WithArgs("APP", "ITEMS").WillReturnRows(sqlmock.NewRows([]string{"name", "type", "nullable", "default", "pk", "generated", "identity", "length"}).AddRow("ID", "INTEGER", 0, nil, 1, 0, 1, 4).AddRow("NAME", "VARCHAR", 1, "'x'", 0, 0, 0, 100).AddRow("CALCULATED", "INTEGER", 1, nil, 0, 1, 0, 4))
			columns, err := inspector.ListColumns(context.Background(), conn, tables[0])
			if err != nil || len(columns) != 3 || !columns[0].PrimaryKey || !columns[0].AutoIncrement || !columns[1].Nullable || columns[1].DefaultValue != "'x'" || !columns[2].Generated {
				t.Fatalf("columns: %+v %v", columns, err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestOracleNativeDDL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT object_type").WithArgs("APP", "ITEMS").WillReturnRows(sqlmock.NewRows([]string{"type"}).AddRow("TABLE"))
	mock.ExpectQuery("SELECT DBMS_METADATA.GET_DDL").WithArgs("TABLE", "ITEMS", "APP").WillReturnRows(sqlmock.NewRows([]string{"ddl"}).AddRow("CREATE TABLE ITEMS(ID NUMBER)"))
	ddl, err := New("oracle").GetTableDDL(context.Background(), testConnection{db: db}, entities.TableRef{Name: "ITEMS", Schema: "APP"})
	if err != nil || ddl != "CREATE TABLE ITEMS(ID NUMBER)" {
		t.Fatalf("DDL: %s %v", ddl, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
