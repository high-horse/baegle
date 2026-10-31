package pgxInspector

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type metadataTestConnection struct {
	manager.Connection
	database *sql.DB
}

func (c metadataTestConnection) DB() *sql.DB { return c.database }

// Run against a disposable database as a role allowed to CREATE ROLE and schema objects.
// Each restricted pool logs in as a separate user; fixtures are removed after the test.
func TestLivePostgresMetadataWithoutOwnership(t *testing.T) {
	dsn := os.Getenv("DB_VIEWER_METADATA_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DB_VIEWER_METADATA_POSTGRES_DSN to run the live PostgreSQL test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := fmt.Sprint(time.Now().UnixNano())
	schema, noAccess, reader := "metadata_"+suffix, "noaccess_"+suffix, "reader_"+suffix
	password := rand.Text()
	setup := fmt.Sprintf(`
 CREATE ROLE %s LOGIN PASSWORD '%s';
 CREATE ROLE %s LOGIN PASSWORD '%s';
 CREATE SCHEMA %s;
 CREATE TABLE %s.items (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name varchar(40) NOT NULL DEFAULT 'untitled',
  doubled bigint GENERATED ALWAYS AS (id * 2) STORED,
  note text
 );
 INSERT INTO %s.items(name) VALUES ('sample');
 COMMENT ON TABLE %s.items IS 'metadata test';
 CREATE VIEW %s.item_names AS SELECT name FROM %s.items;
 ALTER TABLE %s.items ADD COLUMN removed text;
 ALTER TABLE %s.items DROP COLUMN removed;
 GRANT USAGE ON SCHEMA %s TO %s;
 GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s;
 `, noAccess, password, reader, password, schema, schema, schema, schema, schema, schema, schema, schema, schema, reader, schema, reader)
	if _, err := db.ExecContext(ctx, setup); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := db.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE; DROP ROLE "+noAccess+"; DROP ROLE "+reader)
		if err != nil {
			t.Error("cleanup fixtures:", err)
		}
	}()
	inspector := NewInspector()
	for _, role := range []string{noAccess, reader} {
		t.Run(role[:8], func(t *testing.T) {
			config, err := pgx.ParseConfig(dsn)
			if err != nil {
				t.Fatal(err)
			}
			config.User = role
			config.Password = password
			restricted := sql.OpenDB(stdlib.GetConnector(*config))
			defer restricted.Close()
			conn := metadataTestConnection{database: restricted}
			var effective string
			if err := restricted.QueryRowContext(ctx, "SELECT current_user").Scan(&effective); err != nil || effective != role {
				t.Fatalf("effective role: %s %v", effective, err)
			}
			tables, err := inspector.ListTables(ctx, conn)
			if err != nil {
				t.Fatal(err)
			}
			found := map[string]entities.InspectTableInfo{}
			for _, table := range tables {
				if table.Schema == schema {
					found[table.Name] = table
				}
			}
			for name, kind := range map[string]string{"items": "TABLE", "item_names": "VIEW"} {
				if found[name].Type != kind {
					t.Errorf("%s: expected %s, got %+v", name, kind, found[name])
				}
			}
			if found["items"].Comment != "metadata test" {
				t.Error("table comment missing")
			}
			// Use an explicit reference even if discovery failed, to independently check columns.
			var database string
			if err := restricted.QueryRowContext(ctx, "SELECT current_database()").Scan(&database); err != nil {
				t.Fatal(err)
			}
			columns, err := inspector.ListColumns(ctx, conn, entities.InspectTableInfo{Name: "items", Schema: schema, Database: database})
			if err != nil {
				t.Fatal(err)
			}
			if len(columns) != 4 {
				t.Errorf("expected 4 visible catalog columns, got %+v", columns)
			} else {
				if columns[0].Name != "id" || !columns[0].PrimaryKey || !columns[0].AutoIncrement || columns[0].Nullable {
					t.Errorf("identity primary key metadata: %+v", columns[0])
				}
				if columns[1].Name != "name" || columns[1].Nullable || columns[1].DefaultValue == nil {
					t.Errorf("default metadata: %+v", columns[1])
				}
				if !columns[2].Generated || !columns[3].Nullable {
					t.Errorf("generated/nullable metadata: %+v", columns)
				}
			}
			var count int
			readErr := restricted.QueryRowContext(ctx, "SELECT count(*) FROM "+schema+".items").Scan(&count)
			if role == noAccess && readErr == nil {
				t.Error("role without permissions could read table data")
			}
			if role == reader && (readErr != nil || count != 1) {
				t.Errorf("reader cannot read sample: %d %v", count, readErr)
			}

		})
	}
}
