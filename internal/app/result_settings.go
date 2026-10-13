package app

import (
	"context"
	"db-lens/internal/db"
	"db-lens/internal/types"
)

func (s *DatabaseService) GetResultSettings(ctx context.Context) (types.ResultSettings, error) {
	return db.GetResultSettings(ctx)
}

func (s *DatabaseService) SaveResultSettings(ctx context.Context, settings types.ResultSettings) error {
	return db.SaveResultSettings(ctx, settings)
}
