package db

import (
	"context"
	"db-lens/internal/types"
	"fmt"
	"strings"
)

func GetSavedConsoles(ctx context.Context, connectionID int) ([]types.SavedConsole, error) {
	if connectionID < 1 {
		return nil, fmt.Errorf("save the connection before saving consoles")
	}
	rows, err := Conn.QueryContext(ctx, `SELECT connection_id, id, title, sql_text, position, created_at
        FROM saved_consoles WHERE connection_id = ? ORDER BY position, created_at, id`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []types.SavedConsole{}
	for rows.Next() {
		var console types.SavedConsole
		if err := rows.Scan(&console.ConnectionID, &console.ID, &console.Title, &console.SQL, &console.Position, &console.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, console)
	}
	return result, rows.Err()
}

func SaveConsole(ctx context.Context, console types.SavedConsole) error {
	if console.ConnectionID < 1 {
		return fmt.Errorf("save the connection before saving consoles")
	}
	if strings.TrimSpace(console.ID) == "" || strings.TrimSpace(console.Title) == "" || console.Position < 0 {
		return fmt.Errorf("invalid console identity or position")
	}
	result, err := Conn.ExecContext(ctx, `INSERT INTO saved_consoles (connection_id, id, title, sql_text, position, created_at)
        SELECT ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM connections WHERE id = ?)
        ON CONFLICT(connection_id, id) DO UPDATE SET title = excluded.title, sql_text = excluded.sql_text, position = excluded.position`,
		console.ConnectionID, console.ID, console.Title, console.SQL, console.Position, console.CreatedAt, console.ConnectionID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("saved connection no longer exists")
	}
	return nil
}

func DeleteSavedConsole(ctx context.Context, connectionID int, id string) error {
	if connectionID < 1 || id == "" {
		return fmt.Errorf("invalid saved console identity")
	}
	_, err := Conn.ExecContext(ctx, "DELETE FROM saved_consoles WHERE connection_id = ? AND id = ?", connectionID, id)
	return err
}
