package sqlExecutor

import (
	"context"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"fmt"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/queryExecutor/detector"
)

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

func (e *Executor) ExecuteWithDetector(
	ctx context.Context,
	conn manager.Connection,
	query string,
	args ...any,
) (*entities.QueryResult, error) {
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("connection %q is not a SQL connection", conn.ID())
	}

	db := sqlConn.DB()
	if db == nil {
		return nil, fmt.Errorf("connection %q is not connected", conn.ID())
	}

	kind := detector.For(conn.Type()).Detect(query)
	start := time.Now()

	if kind == detector.KindExec {
		res, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			rowsAffected, _ := res.RowsAffected()
			lastInsertId, _ := res.LastInsertId()
			return &entities.QueryResult{
				RowsAffected: rowsAffected,
				LastInsertId: lastInsertId,
				Duration:     time.Since(start),
				IsQuery:      false,
			}, nil
		}
		// Detector guessed wrong (statement actually returns rows) — fall through to QueryContext.
	}

	return e.runQuery(ctx, db, query, start, args...)

}

func (e *Executor) Execute(
	ctx context.Context,
	conn manager.Connection,
	query string,
	args ...any,
) (*entities.QueryResult, error) {
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("connection %q is not a SQL connection", conn.ID())
	}

	db := sqlConn.DB()
	if db == nil {
		return nil, fmt.Errorf("connection %q is not connected", conn.ID())
	}

	start := time.Now()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	// No columns => it wasn't a row-returning statement (INSERT/UPDATE/DDL/etc).
	// It already ran, exactly once, via QueryContext above.
	if len(colTypes) == 0 {
		return &entities.QueryResult{
			Duration: time.Since(start),
			IsQuery:  false,
			// RowsAffected intentionally omitted — not obtainable from sql.Rows.
			// See note below if you need it.
		}, nil
	}

	columns := make([]entities.ColumnInfo, len(colTypes))
	for i, ct := range colTypes {
		nullable, hasNullable := ct.Nullable()
		columns[i] = entities.ColumnInfo{
			Name:         ct.Name(),
			DatabaseType: ct.DatabaseTypeName(),
			Nullable:     nullable && hasNullable,
		}
	}

	var result [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(colTypes))
		ptrs := make([]interface{}, len(colTypes))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		result = append(result, normalizeRow(values, columns...))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &entities.QueryResult{
		Columns:  columns,
		Rows:     result,
		Duration: time.Since(start),
		IsQuery:  true,
	}, nil
}

func prepareIndex() string {
	return ""
}

func (e *Executor) ExecuteOld(
	ctx context.Context,
	conn manager.Connection,
	query string,
	args ...any,
) (*entities.QueryResult, error) {

	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("connection %q is not a SQL connection", conn.ID())
	}

	db := sqlConn.DB()
	if db == nil {
		return nil, fmt.Errorf("connection %q is not connected", conn.ID())
	}

	start := time.Now()

	if isQueryStatement(query) {
		return e.runQuery(ctx, db, query, start, args...)
	}
	return e.runExec(ctx, db, query, start, args...)
}

func (e *Executor) runQuery(ctx context.Context, db *sql.DB, query string, start time.Time, args ...any) (*entities.QueryResult, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	columns := make([]entities.ColumnInfo, len(colTypes))
	for i, ct := range colTypes {
		nullable, hasNullable := ct.Nullable()
		columns[i] = entities.ColumnInfo{
			Name:         ct.Name(),
			DatabaseType: ct.DatabaseTypeName(),
			Nullable:     nullable && hasNullable,
		}
	}

	var results [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(colTypes))
		ptrs := make([]interface{}, len(colTypes))

		for i := range values {
			ptrs[i] = &values[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		results = append(results, normalizeRow(values, columns...))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &entities.QueryResult{
		Columns:  columns,
		Rows:     results,
		Duration: time.Since(start),
		IsQuery:  true,
	}, nil
}

func (e *Executor) runExec(ctx context.Context, db *sql.DB, query string, start time.Time, args ...any) (*entities.QueryResult, error) {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := res.RowsAffected()
	lastInsertId, _ := res.LastInsertId()

	return &entities.QueryResult{
		RowsAffected: rowsAffected,
		LastInsertId: lastInsertId,
		Duration:     time.Since(start),
		IsQuery:      false,
	}, nil
}

// normalizeRow converts []byte (how most drivers return TEXT/VARCHAR/DECIMAL)
// into string so the result is JSON/JS-friendly instead of base64. Binary
// columns use a hex representation to preserve invalid UTF-8 and zero bytes.
func normalizeRow(values []interface{}, columns ...entities.ColumnInfo) []interface{} {
	out := make([]interface{}, len(values))
	for i, v := range values {
		if b, ok := v.([]byte); ok {
			if i < len(columns) && binaryColumn(columns[i].DatabaseType) {
				out[i] = "0x" + hex.EncodeToString(b)
			} else {
				out[i] = string(b)
			}
		} else if integer, ok := v.(int64); ok && (integer > 9007199254740991 || integer < -9007199254740991) {
			out[i] = fmt.Sprintf("%d", integer)
		} else {
			out[i] = v
		}
	}
	return out
}

func binaryColumn(name string) bool {
	name = strings.ToUpper(strings.TrimSpace(name))
	if index := strings.IndexByte(name, '('); index >= 0 {
		name = strings.TrimSpace(name[:index])
	}
	switch name {
	case "BYTEA", "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BINARY", "VARBINARY", "RAW", "LONG RAW", "IMAGE", "BFILE":
		return true
	}
	return false
}

func isQueryStatement(query string) bool {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return false
	}

	for strings.HasPrefix(trimmed, "--") {
		lines := strings.SplitN(trimmed, "\n", 2)
		if len(lines) == 1 {
			return false
		}
		trimmed = strings.TrimSpace(lines[1])
	}

	firstWord := strings.ToUpper(trimmed)

	// Remove SQL comments
	for strings.HasPrefix(trimmed, "--") {
		lines := strings.SplitN(trimmed, "\n", 2)
		if len(lines) == 1 {
			return false
		}
		trimmed = strings.TrimSpace(lines[1])
	}

	for _, prefix := range []string{
		"SELECT",
		"SHOW",
		"EXPLAIN",
		"WITH",
		"DESCRIBE",
		"DESC",
		"PRAGMA", "SELECT",
		"SHOW",
		"DESCRIBE",
		"DESC",
		"EXPLAIN",
		"ANALYZE",
		"WITH",
		"VALUES",
		"TABLE",
		"CALL",
	} {
		if strings.HasPrefix(firstWord, prefix) {
			return true
		}
	}
	return false
}
