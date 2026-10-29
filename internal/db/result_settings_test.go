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
	// Older installations have no sidebar or console keys in their saved JSON.
	if _, err := Conn.Exec(`INSERT INTO app_preferences (key, value) VALUES ('result_grid',
		'{"rowsPerPage":250,"fontSize":14,"stripedRows":false,"showNulls":true}')`); err != nil {
		t.Fatal(err)
	}
	legacy, err := GetResultSettings(ctx)
	if err != nil || legacy.RowsPerPage != 250 || legacy.FontSize != 14 || legacy.StripedRows || !legacy.ShowNulls ||
		legacy.AccentColor != defaults.AccentColor || legacy.SidebarFontSize != defaults.SidebarFontSize || legacy.ConsoleFontSize != defaults.ConsoleFontSize || legacy.ImportLimitMiB != 0 || legacy.QueryTimeoutSeconds != 60 {
		t.Fatalf("legacy preferences lost values or new defaults: %+v %v", legacy, err)
	}
	saved := types.ResultSettings{AccentColor: "#b7a5e8", QueryTimeoutSeconds: 30, RowsPerPage: 250, FontSize: 14, SidebarFontSize: 16, ConsoleFontSize: 20, ImportLimitMiB: 64, StripedRows: false, ShowNulls: true}
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
	for _, invalidate := range []func(*types.ResultSettings){
		func(s *types.ResultSettings) { s.AccentColor = "red" },
		func(s *types.ResultSettings) { s.AccentColor = "#fff" },
		func(s *types.ResultSettings) { s.AccentColor = "#zzzzzz" },
		func(s *types.ResultSettings) { s.QueryTimeoutSeconds = -1 },
		func(s *types.ResultSettings) { s.QueryTimeoutSeconds = 86401 },
		func(s *types.ResultSettings) { s.RowsPerPage = 0 },
		func(s *types.ResultSettings) { s.RowsPerPage = 501 },
		func(s *types.ResultSettings) { s.FontSize = 9 },
		func(s *types.ResultSettings) { s.FontSize = 19 },
		func(s *types.ResultSettings) { s.SidebarFontSize = 9 },
		func(s *types.ResultSettings) { s.SidebarFontSize = 25 },
		func(s *types.ResultSettings) { s.ConsoleFontSize = 9 },
		func(s *types.ResultSettings) { s.ConsoleFontSize = 25 },
		func(s *types.ResultSettings) { s.ImportLimitMiB = -1 },
	} {
		invalid := saved
		invalidate(&invalid)
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
