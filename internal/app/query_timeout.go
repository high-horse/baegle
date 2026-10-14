package app

import (
	"context"
	"db-lens/internal/db"
	"db-lens/internal/types"
	"fmt"
	"time"
)

func queryTimeoutContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	settings := types.DefaultResultSettings()
	if db.Conn != nil {
		var err error
		settings, err = db.GetResultSettings(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("load query timeout setting: %w", err)
		}
	}
	if settings.QueryTimeoutSeconds == 0 {
		operation, cancel := context.WithCancel(ctx)
		return operation, cancel, nil
	}
	operation, cancel := context.WithTimeoutCause(ctx, time.Duration(settings.QueryTimeoutSeconds)*time.Second,
		fmt.Errorf("query timed out after %d seconds: %w", settings.QueryTimeoutSeconds, context.DeadlineExceeded))
	return operation, cancel, nil
}
