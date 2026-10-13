package app

import (
	"context"
	"db-lens/internal/db"
	"db-lens/internal/types"
)

func (s *DatabaseService) GetSavedConsoles(ctx context.Context, connectionID int) ([]types.SavedConsole, error) {
	return db.GetSavedConsoles(ctx, connectionID)
}

func (s *DatabaseService) SaveConsole(ctx context.Context, console types.SavedConsole) error {
	return db.SaveConsole(ctx, console)
}

func (s *DatabaseService) DeleteSavedConsole(ctx context.Context, connectionID int, id string) error {
	return db.DeleteSavedConsole(ctx, connectionID, id)
}
