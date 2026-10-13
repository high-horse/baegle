package db

import (
	"context"
	"database/sql"
	"runtime"
	"testing"
)

func TestIndependentWindowsShareSavedConnections(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses XDG_CONFIG_HOME to isolate the app database")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	previous := Conn
	defer func() { Conn = previous }()
	first, err := InitDb()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := InitDb()
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := first.Exec(`INSERT INTO connections (name) VALUES ('first window')`); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := second.QueryRow(`SELECT name FROM connections`).Scan(&name); err != nil || name != "first window" {
		t.Fatalf("saved connection not shared: %q, %v", name, err)
	}
	// Check separate pooled connections: the timeout must apply to every handle.
	for _, database := range []*sql.DB{first, second} {
		one, err := database.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		two, err := database.Conn(context.Background())
		if err != nil {
			one.Close()
			t.Fatal(err)
		}
		for _, connection := range []*sql.Conn{one, two} {
			var timeout int
			err := connection.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout)
			connection.Close()
			if err != nil || timeout != 5000 {
				t.Fatalf("busy timeout = %d: %v", timeout, err)
			}
		}
	}
}
