package db

import (
	"database/sql"
	_ "embed"
	"log"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migration.sql
var migrationSQL string

var Conn *sql.DB

func appDbPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "db-viewer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "app.db"), nil
}

func InitDb() (*sql.DB, error) {
	path, err := appDbPath()
	if err != nil {
		return nil, err
	}
	log.Println("using app database:", path)

	// Each window uses a separate process. Wait briefly for writes from other
	// windows instead of failing immediately with SQLITE_BUSY.
	dsn := url.URL{Scheme: "file", Path: path, RawQuery: "_pragma=busy_timeout%285000%29"}
	Conn, err = sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, err
	}

	if err := runMigration(Conn); err != nil {
		return nil, err
	}

	return Conn, nil
}

func runMigration(db *sql.DB) error {
	_, err := db.Exec(migrationSQL)
	if err != nil {
		return err
	}
	// Existing installations need the new persisted setting too.
	rows, err := db.Query("PRAGMA table_info(connections)")
	if err != nil {
		return err
	}
	found := false
	groupFound := false
	sslFound := false
	for rows.Next() {
		var id, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&id, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "group_name" {
			groupFound = true
		}
		if name == "ssl" {
			sslFound = true
		}
		if name == "read_only" {
			found = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		_, err = db.Exec("ALTER TABLE connections ADD COLUMN read_only BOOLEAN NOT NULL DEFAULT false")
		if err != nil {
			return err
		}
	}
	if !groupFound {
		_, err = db.Exec("ALTER TABLE connections ADD COLUMN group_name TEXT NOT NULL DEFAULT ''")
	}
	if err != nil {
		return err
	}
	if !sslFound {
		if _, err = db.Exec("ALTER TABLE connections ADD COLUMN ssl BOOLEAN NOT NULL DEFAULT false"); err != nil {
			return err
		}
	}
	rows, err = db.Query("PRAGMA table_info(saved_consoles)")
	if err != nil {
		return err
	}
	tabStateFound := false
	for rows.Next() {
		var id, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&id, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		tabStateFound = tabStateFound || name == "tab_state"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !tabStateFound {
		_, err = db.Exec("ALTER TABLE saved_consoles ADD COLUMN tab_state TEXT NOT NULL DEFAULT ''")
	}
	return err
}
