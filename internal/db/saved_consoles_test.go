package db

import (
	"context"
	"database/sql"
	"db-lens/internal/types"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedConsolesPersistenceIsolationAndDeletion(t *testing.T) {
	previous := Conn
	defer func() { Conn = previous }()
	path := filepath.Join(t.TempDir(), "consoles.db")
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
	if _, err := Conn.Exec("INSERT INTO connections (id, name) VALUES (1, 'first'), (2, 'second')"); err != nil {
		t.Fatal(err)
	}
	query := "-- Long query with quotes and Unicode: 'नमस्ते'\n" + strings.Repeat("SELECT 'a; b', 42;\n", 10000)
	console := types.SavedConsole{ConnectionID: 1, ID: "console-a", Title: "console_1.sql", SQL: query, CreatedAt: 123, Position: 1}
	if err := SaveConsole(ctx, console); err != nil {
		t.Fatal(err)
	}
	other := console
	other.ConnectionID, other.SQL = 2, "SELECT 2"
	if err := SaveConsole(ctx, other); err != nil {
		t.Fatal(err)
	}
	first := console
	first.ID, first.Position, first.SQL = "console-b", 0, "SELECT 1"
	first.TabState = `{"type":"result","active":true,"table":{"name":"users","schema":"public","database":"app"}}`
	if err := SaveConsole(ctx, first); err != nil {
		t.Fatal(err)
	}
	Conn.Close()
	Conn = open() // Startup migration must preserve saved tabs.
	defer Conn.Close()
	list, err := GetSavedConsoles(ctx, 1)
	if err != nil || len(list) != 2 || list[0] != first || list[1] != console {
		t.Fatalf("restoration failed: count=%d, error=%v", len(list), err)
	}
	console.SQL = "SELECT 'updated'"
	if err := SaveConsole(ctx, console); err != nil {
		t.Fatal(err)
	}
	list, err = GetSavedConsoles(ctx, 1)
	if err != nil || len(list) != 2 || list[1].SQL != console.SQL {
		t.Fatal("saving should update a tab, not duplicate it")
	}
	if err := DeleteSavedConsole(ctx, 1, console.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = GetSavedConsoles(ctx, 2)
	if len(list) != 1 || list[0] != other {
		t.Fatal("closing a tab affected another connection")
	}
	if err := DeleteConnection(1); err != nil {
		t.Fatal(err)
	}
	list, _ = GetSavedConsoles(ctx, 1)
	if len(list) != 0 {
		t.Fatal("deleting connection left saved consoles behind")
	}
	if err := SaveConsole(ctx, console); err == nil {
		t.Fatal("saved a console for a deleted connection")
	}
	console.ConnectionID = -1
	if err := SaveConsole(ctx, console); err == nil {
		t.Fatal("saved a console for a temporary connection")
	}
}

func TestSavedConsoleMigrationPreservesExistingSQL(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	_, err = database.Exec(`CREATE TABLE saved_consoles (
        connection_id INTEGER NOT NULL, id TEXT NOT NULL, title TEXT NOT NULL,
        sql_text TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL,
        PRIMARY KEY (connection_id, id));
        INSERT INTO saved_consoles VALUES (1, 'old', 'console.sql', 'SELECT 42', 0, 123);`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := runMigration(database); err != nil {
			t.Fatal(err)
		}
	}
	var query, state string
	if err := database.QueryRow("SELECT sql_text, tab_state FROM saved_consoles WHERE id = 'old'").Scan(&query, &state); err != nil {
		t.Fatal(err)
	}
	if query != "SELECT 42" || state != "" {
		t.Fatalf("migration changed existing console: query=%q state=%q", query, state)
	}
}
