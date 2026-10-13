package db

import (
	"context"
	"database/sql"
	"db-lens/internal/types"
	"path/filepath"
	"testing"
)

func TestResultSettingsSurviveDatabaseReopen(t *testing.T) {
	previous := Conn
	defer func() { Conn = previous }()
	path := filepath.Join(t.TempDir(), "settings.db")
	open := func() *sql.DB {
		t.Helper()
		database, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if err := runMigration(database); err != nil {
			database.Close()
			t.Fatal(err)
		}
		return database
	}
	Conn = open()
	ctx := context.Background()
	defaults, err := GetResultSettings(ctx)
	if err != nil || defaults != types.DefaultResultSettings() {
		t.Fatalf("defaults: %+v %v", defaults, err)
	}
	saved := types.ResultSettings{RowsPerPage: 250, FontSize: 14, StripedRows: false, ShowNulls: true}
	if err := SaveResultSettings(ctx, saved); err != nil {
		t.Fatal(err)
	}
	Conn.Close()
	Conn = open() // Startup migration must preserve preferences across restarts.
	defer Conn.Close()
	actual, err := GetResultSettings(ctx)
	if err != nil || actual != saved {
		t.Fatalf("settings did not persist: %+v %v", actual, err)
	}
	if _, err := Conn.Exec("INSERT INTO connections (name) VALUES ('one'), ('two')"); err != nil {
		t.Fatal(err)
	}
	actual, err = GetResultSettings(ctx)
	if err != nil || actual != saved {
		t.Fatalf("connections changed preferences: %+v %v", actual, err)
	}
	for _, invalid := range []types.ResultSettings{
		{RowsPerPage: 0, FontSize: 11}, {RowsPerPage: 501, FontSize: 11},
		{RowsPerPage: 100, FontSize: 9}, {RowsPerPage: 100, FontSize: 19},
	} {
		if err := SaveResultSettings(ctx, invalid); err == nil {
			t.Fatalf("accepted invalid settings: %+v", invalid)
		}
	}
	actual, _ = GetResultSettings(ctx)
	if actual != saved {
		t.Fatal("invalid settings overwrote saved preferences")
	}
	if err := SaveResultSettings(ctx, types.DefaultResultSettings()); err != nil {
		t.Fatal(err)
	}
	actual, _ = GetResultSettings(ctx)
	if actual != types.DefaultResultSettings() {
		t.Fatal("restore defaults failed")
	}
}
