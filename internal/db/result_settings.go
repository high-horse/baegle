package db

import (
	"context"
	"database/sql"
	"db-lens/internal/types"
	"encoding/json"
	"errors"
	"fmt"
)

func GetResultSettings(ctx context.Context) (types.ResultSettings, error) {
	settings := types.DefaultResultSettings()
	var value string
	err := Conn.QueryRowContext(ctx, "SELECT value FROM app_preferences WHERE key = 'result_grid'").Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return types.DefaultResultSettings(), err
	}
	return settings, validateResultSettings(settings)
}

func SaveResultSettings(ctx context.Context, settings types.ResultSettings) error {
	if err := validateResultSettings(settings); err != nil {
		return err
	}
	value, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = Conn.ExecContext(ctx, `INSERT INTO app_preferences (key, value) VALUES ('result_grid', ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(value))
	return err
}

func validateResultSettings(settings types.ResultSettings) error {
	if settings.RowsPerPage < 1 || settings.RowsPerPage > 500 {
		return fmt.Errorf("rows per page must be between 1 and 500")
	}
	if settings.FontSize < 10 || settings.FontSize > 18 {
		return fmt.Errorf("font size must be between 10 and 18")
	}
	return nil
}
