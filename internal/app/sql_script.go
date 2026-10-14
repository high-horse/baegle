package app

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/sqlscript"
	"github.com/jackc/pgx/v5/stdlib"
)

type ScriptResult struct {
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Error     string `json:"error"`
	ErrorLine int    `json:"errorLine"`
	Cancelled bool   `json:"cancelled"`
	Duration  int64  `json:"duration"`
}

// ExecuteConsoleScript runs the edited text on one session. Explicit transaction
// statements in the script control commits; autocommitted statements remain on error.
func (s *DbService) ExecuteConsoleScript(ctx context.Context, consoleID, requestID, connectionID, text string) (*ScriptResult, error) {
	ctx, finish, err := s.beginConsoleQuery(ctx, consoleID, requestID)
	if err != nil {
		return nil, err
	}
	defer finish()
	ctx, cancel, err := queryTimeoutContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	conn, ok := s.manager.Active()
	if !ok || conn.ID() != connectionID {
		return nil, fmt.Errorf("the console's connection is no longer active")
	}
	if conn.Config().ReadOnly {
		return nil, fmt.Errorf("scripts cannot run on a read-only connection")
	}
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("SQL scripts are not supported on this connection")
	}
	start := time.Now()
	result := &ScriptResult{}
	defer func() { result.Duration = time.Since(start).Milliseconds() }()
	statements, err := sqlscript.Split(text, conn.Type())
	if err != nil {
		return nil, err
	}
	result.Total = len(statements)
	if len(statements) == 0 {
		return nil, fmt.Errorf("the script contains no SQL statements")
	}
	session, err := sqlConn.DB().Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	// Never return script session settings, locks, or an unfinished transaction to the pool.
	defer session.Raw(func(any) error { return driver.ErrBadConn })
	for _, statement := range statements {
		if err = ctx.Err(); err == nil {
			if statement.CopyData != nil {
				err = session.Raw(func(raw any) error {
					pg, ok := raw.(*stdlib.Conn)
					if !ok {
						return fmt.Errorf("COPY FROM STDIN requires a PostgreSQL connection")
					}
					_, err := pg.Conn().PgConn().CopyFrom(ctx, strings.NewReader(*statement.CopyData), statement.SQL)
					return err
				})
			} else {
				_, err = session.ExecContext(ctx, statement.SQL)
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				err = context.Cause(ctx)
			}
			result.Error = fmt.Sprintf("Statement %d (line %d): %v", result.Completed+1, statement.Line, err)
			result.ErrorLine = statement.Line
			result.Cancelled = ctx.Err() != nil
			return result, nil
		}
		result.Completed++
	}
	return result, nil
}
