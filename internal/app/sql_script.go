package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/queryExecutor/detector"
	"db-lens/internal/engine/sqlscript"
	"github.com/jackc/pgx/v5/stdlib"
)

type ScriptResult struct {
	Completed  int                     `json:"completed"`
	Total      int                     `json:"total"`
	Error      string                  `json:"error"`
	ErrorLine  int                     `json:"errorLine"`
	Cancelled  bool                    `json:"cancelled"`
	Duration   int64                   `json:"duration"`
	Statements []ScriptStatementResult `json:"statements"`
}

type ScriptResultSet struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
}

type ScriptStatementResult struct {
	Statement    int               `json:"statement"`
	Line         int               `json:"line"`
	Preview      string            `json:"preview"`
	Duration     int64             `json:"duration"`
	RowsAffected *int64            `json:"rowsAffected"`
	Results      []ScriptResultSet `json:"results"`
	Error        string            `json:"error"`
}

type ScriptProgress struct {
	RequestID string `json:"requestID"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Current   int    `json:"current"`
	Line      int    `json:"line"`
	Duration  int64  `json:"duration"`
}

// GetConsoleScriptProgress only exposes the requested execution; late polls
// cannot attach progress from a newer script in the same console.
func (s *DbService) GetConsoleScriptProgress(consoleID, requestID string) *ScriptProgress {
	s.consoleMu.Lock()
	defer s.consoleMu.Unlock()
	progress, ok := s.scriptProgress[consoleID]
	if !ok || progress.RequestID != requestID {
		return nil
	}
	return &progress
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
	progress := func(current, line int) {
		s.consoleMu.Lock()
		defer s.consoleMu.Unlock()
		if s.scriptProgress == nil {
			s.scriptProgress = make(map[string]ScriptProgress)
		}
		s.scriptProgress[consoleID] = ScriptProgress{RequestID: requestID, Completed: result.Completed, Total: result.Total, Current: current, Line: line, Duration: time.Since(start).Milliseconds()}
	}
	defer func() { s.consoleMu.Lock(); delete(s.scriptProgress, consoleID); s.consoleMu.Unlock() }()
	progress(0, 0)
	session, err := sqlConn.DB().Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	// Never return script session settings, locks, or an unfinished transaction to the pool.
	defer session.Raw(func(any) error { return driver.ErrBadConn })
	// Keep previews bounded while still consuming every result, so subsequent
	// statements execute even when an earlier SELECT returns a large data set.
	remainingRows := 10000
	for index, statement := range statements {
		progress(index+1, statement.Line)
		statementStart := time.Now()
		preview := []rune(strings.Join(strings.Fields(statement.SQL), " "))
		if len(preview) > 160 {
			preview = append(preview[:160], '…')
		}
		entry := ScriptStatementResult{Statement: index + 1, Line: statement.Line, Preview: string(preview), Results: []ScriptResultSet{}}
		if err = ctx.Err(); err == nil {
			if statement.CopyData != nil {
				err = session.Raw(func(raw any) error {
					pg, ok := raw.(*stdlib.Conn)
					if !ok {
						return fmt.Errorf("COPY FROM STDIN requires a PostgreSQL connection")
					}
					tag, err := pg.Conn().PgConn().CopyFrom(ctx, strings.NewReader(*statement.CopyData), statement.SQL)
					if err == nil {
						count := tag.RowsAffected()
						entry.RowsAffected = &count
					}
					return err
				})
			} else {
				err = executeScriptStatement(ctx, session, conn.Type(), statement.SQL, &entry, &remainingRows)
			}
		}
		entry.Duration = time.Since(statementStart).Milliseconds()
		if err != nil {
			if ctx.Err() != nil {
				err = context.Cause(ctx)
			}
			result.Error = fmt.Sprintf("Statement %d (line %d): %v", result.Completed+1, statement.Line, err)
			result.ErrorLine = statement.Line
			result.Cancelled = ctx.Err() != nil
			entry.Error = result.Error
			result.Statements = append(result.Statements, entry)
			return result, nil
		}
		result.Statements = append(result.Statements, entry)
		result.Completed++
		progress(index+1, statement.Line)
	}
	return result, nil
}

var scriptReturning = regexp.MustCompile(`(?i)\b(RETURNING|OUTPUT)\b`)

func executeScriptStatement(ctx context.Context, session *sql.Conn, dialect, text string, entry *ScriptStatementResult, remaining *int) error {
	if dialect == "pgx" {
		dialect = "postgres"
	}
	// Never retry an execution through a different API: a failed statement may
	// already have changed data. RETURNING/OUTPUT must use the row-returning API.
	if !scriptReturning.MatchString(text) && detector.For(dialect).Detect(text) == detector.KindExec {
		result, err := session.ExecContext(ctx, text)
		if err != nil {
			return err
		}
		if count, err := result.RowsAffected(); err == nil {
			entry.RowsAffected = &count
		}
		return nil
	}
	rows, err := session.QueryContext(ctx, text)
	if err != nil {
		return err
	}
	defer rows.Close()
	for {
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		set := ScriptResultSet{Columns: columns, Rows: [][]any{}}
		for rows.Next() {
			if len(columns) == 0 {
				continue
			}
			if len(set.Rows) >= 1000 || *remaining <= 0 {
				set.Truncated = true
				continue
			}
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				return err
			}
			for i, value := range values {
				if bytes, ok := value.([]byte); ok {
					values[i] = string(bytes)
				} else if integer, ok := value.(int64); ok && (integer > 9007199254740991 || integer < -9007199254740991) {
					values[i] = fmt.Sprintf("%d", integer)
				}
			}
			set.Rows = append(set.Rows, values)
			*remaining -= 1
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(columns) > 0 {
			entry.Results = append(entry.Results, set)
		}
		if !rows.NextResultSet() {
			return rows.Err()
		}
	}
}
