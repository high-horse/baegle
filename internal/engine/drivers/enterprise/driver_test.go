package enterprise

import (
	"context"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
	"net/url"
	"strings"
	"testing"
)

func TestConnectionStrings(t *testing.T) {
	config := entities.ConnectionConfig{Type: "sqlserver", User: "user@domain", Password: "p;@:/?# %", Database: "db name", SSL: true}
	name, dsn, err := ConnectionString(config, "127.0.0.1:41234")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	if name != "sqlserver" || parsed.Host != "127.0.0.1:41234" || parsed.User.Username() != config.User || password != config.Password || parsed.Query().Get("database") != config.Database || parsed.Query().Get("encrypt") != "true" {
		t.Fatal("SQL Server connection options did not round-trip")
	}
	config.Type = "oracle"
	config.Database = "FREEPDB1"
	name, dsn, err = ConnectionString(config, "[::1]:1521")
	if err != nil || name != "oracle" {
		t.Fatalf("Oracle DSN: %v", err)
	}
	parsed, err = url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ = parsed.User.Password()
	if password != config.Password || !strings.Contains(dsn, "FREEPDB1") || !strings.Contains(dsn, "1521") {
		t.Fatal("Oracle DSN lost connection fields")
	}
	config.Type = "db2"
	config.Password = "password"
	config.Database = "SAMPLE"
	name, dsn, err = ConnectionString(config, "127.0.0.1:41234")
	if err != nil || name != "go_ibm_db" || !strings.Contains(dsn, "PORT=41234;") || !strings.Contains(dsn, "SECURITY=SSL;") {
		t.Fatalf("Db2 DSN: %v", err)
	}
	config.Password = "x;DATABASE=other"
	if _, _, err = ConnectionString(config, "localhost:50000"); err == nil {
		t.Fatal("Db2 option injection accepted")
	}
	if _, _, err = ConnectionString(config, "localhost"); err == nil {
		t.Fatal("invalid endpoint accepted")
	}
}

func TestParserKeepsRoutineAndBatchBoundaries(t *testing.T) {
	for _, tt := range []struct{ dialect, query, want string }{
		{"oracle", "SELECT 1 FROM dual;", "SELECT 1 FROM dual"},
		{"oracle", "CREATE TABLE t (type VARCHAR2(20));", "CREATE TABLE t (type VARCHAR2(20))"},
		{"oracle", "BEGIN NULL; NULL; END;\n/", "BEGIN NULL; NULL; END;"},
		{"sqlserver", "DECLARE @n int = 1; SELECT @n;\nGO", "DECLARE @n int = 1; SELECT @n;"},
		{"db2", "SELECT 1 FROM SYSIBM.SYSDUMMY1;", "SELECT 1 FROM SYSIBM.SYSDUMMY1"},
	} {
		t.Run(tt.dialect+tt.want, func(t *testing.T) {
			result, err := New(tt.dialect).Parser().Parse(tt.query)
			if err != nil || result.RawSQL != tt.want {
				t.Fatalf("parsed %v, %v", result, err)
			}
		})
	}
	if _, err := New("oracle").Parser().Parse("SELECT 1 FROM dual; DELETE FROM t;"); err == nil {
		t.Fatal("multiple statements should use Run script")
	}
	if !db2Enabled {
		if _, err := New("db2").Create(context.Background(), entities.ConnectionConfig{Type: "db2"}, transports.NewDirect("localhost", 50000)); err == nil {
			t.Fatal("native-client-free build silently accepted Db2")
		}
	}
}
