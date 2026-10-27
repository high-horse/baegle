package sqlExecutor

import (
	"context"
	"database/sql"
	"errors"

	"fmt"
	"github.com/go-sql-driver/mysql"
	mssql "github.com/microsoft/go-mssqldb"
	"strings"
	"time"
	"unicode"

	"db-lens/internal/engine/entities"
)

// PageableSQL accepts a single SELECT or read-only CTE shape. Commands, locking
// reads, SELECT INTO, and data-modifying CTEs must keep execute-once semantics.
// This is a conservative capability check, not SQL validation or a security boundary.
func PageableSQL(query string) (string, bool) {
	query = strings.TrimSpace(query)
	var words []string
	end := len(query)
	terminated := false
	for i := 0; i < len(query); {
		if unicode.IsSpace(rune(query[i])) {
			i++
			continue
		}
		if strings.HasPrefix(query[i:], "--") {
			if n := strings.IndexByte(query[i:], '\n'); n >= 0 {
				i += n + 1
				continue
			}
			break
		}
		if strings.HasPrefix(query[i:], "/*") {
			// Nested/versioned comments and dollar quoting are deliberately unsupported.
			n := strings.Index(query[i+2:], "*/")
			if n < 0 || strings.Contains(query[i+2:i+2+n], "/*") || strings.HasPrefix(query[i:], "/*!") {
				return "", false
			}
			i += n + 4
			continue
		}
		if terminated {
			return "", false
		}
		ch := query[i]
		if ch == ';' {
			end = i
			terminated = true
			i++
			continue
		}
		if ch == '$' || ch == '#' || ch == '@' {
			return "", false
		}
		if ch == '\'' || ch == '"' || ch == '`' || ch == '[' {
			quote := ch
			if ch == '[' {
				quote = ']'
			}
			i++
			closed := false
			for i < len(query) {
				if query[i] == '\\' {
					return "", false
				}
				if query[i] == quote {
					if i+1 < len(query) && query[i+1] == quote {
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return "", false
			}
			continue
		}
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_' {
			start := i
			for i < len(query) && (query[i] >= 'a' && query[i] <= 'z' || query[i] >= 'A' && query[i] <= 'Z' || query[i] >= '0' && query[i] <= '9' || query[i] == '_') {
				i++
			}
			words = append(words, strings.ToUpper(query[start:i]))
			continue
		}
		i++
	}
	if len(words) == 0 || words[0] != "SELECT" && words[0] != "WITH" {
		return "", false
	}
	for _, word := range words {
		switch word {
		case "INSERT", "UPDATE", "DELETE", "MERGE", "REPLACE", "INTO", "FOR", "LOCK", "CALL", "EXEC", "EXECUTE", "CREATE", "DROP", "ALTER", "PRAGMA", "ATTACH", "DETACH":
			return "", false
		}
	}
	return strings.TrimSpace(query[:end]), true
}

// CountRead runs only when the user requests an exact total. Original filters,
// grouping and explicit limits remain part of the counted result.
func CountRead(ctx context.Context, db *sql.DB, query string) (int64, error) {
	return CountReadDialect(ctx, db, query, "")
}

// SQL Server cannot embed a CTE or an unrestricted ORDER BY in a derived table.
// Keep these queries on the execute-once streaming path instead of rewriting them.
func PageableDialectSQL(query, dialect string) (string, bool) {
	base, ok := PageableSQL(query)
	if ok && dialect == "sqlserver" {
		upper := strings.ToUpper(base)
		if strings.Contains(upper, "WITH") || strings.Contains(upper, "ORDER") || strings.Contains(upper, "OPTION") {
			return "", false
		}
	}
	return base, ok
}

func CountReadDialect(ctx context.Context, db *sql.DB, query, dialect string) (int64, error) {
	base, ok := PageableDialectSQL(query, dialect)
	if !ok {
		return 0, fmt.Errorf("this statement does not support row counting")
	}
	var total int64
	alias := " AS dbv_count"
	if dialect == "oracle" {
		alias = " dbv_count"
	}
	count := "COUNT(*)"
	if dialect == "sqlserver" {
		count = "COUNT_BIG(*)"
	}
	err := db.QueryRowContext(ctx, "SELECT "+count+" FROM (\n"+base+"\n)"+alias).Scan(&total)
	if isDuplicateColumnError(err) {
		// A top-level SELECT allows repeated labels; a derived table does not.
		rows, readErr := db.QueryContext(ctx, base)
		if readErr != nil {
			return 0, readErr
		}
		defer rows.Close()
		for rows.Next() {
			total++
		}
		return total, rows.Err()
	}
	return total, err
}

// OpenRead fetches a bounded page and one lookahead row, without counting the
// full result or retaining a database connection between navigation requests.
func (p *Pager) OpenRead(ctx context.Context, db *sql.DB, connection, dialect string, input entities.QueryInput) (*entities.QueryResult, error) {
	base, ok := PageableDialectSQL(input.Query, dialect)
	if !ok {
		return nil, fmt.Errorf("this statement does not support page navigation or sorting")
	}
	size := input.PageSize
	if size == 0 {
		size = 100
	}
	if size < 1 || size > 500 {
		return nil, fmt.Errorf("page size must be between 1 and 500")
	}
	if input.SortColumn < 0 {
		return nil, fmt.Errorf("invalid sort column")
	}
	direction := strings.ToUpper(input.SortDirection)
	if input.SortColumn > 0 && direction != "ASC" && direction != "DESC" {
		return nil, fmt.Errorf("invalid sort direction")
	}
	wrapped := "(\n" + base + "\n) AS _dbv_result"
	if dialect == "db2" {
		wrapped = "(\n" + base + "\n) AS dbv_result"
	}
	if dialect == "oracle" {
		wrapped = "(\n" + base + "\n) dbv_result"
	}
	page := int64(input.Page)
	if page < 1 {
		page = 1
	}
	if page-1 > (1<<63-1)/int64(size) {
		return nil, fmt.Errorf("page offset is too large")
	}
	offset := (page - 1) * int64(size)
	query := "SELECT * FROM " + wrapped
	if input.SortColumn > 0 {
		// Ordinals support duplicate output labels and cannot inject identifiers.
		query += fmt.Sprintf(" ORDER BY %d %s", input.SortColumn, direction)
	}
	switch dialect {
	case "sqlserver":
		if input.SortColumn == 0 {
			query += " ORDER BY (SELECT NULL)"
		}
		query += fmt.Sprintf("\nOFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, size+1)
	case "oracle", "db2":
		query += fmt.Sprintf("\nOFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, size+1)
	default:
		query += fmt.Sprintf("\nLIMIT %d OFFSET %d", size+1, offset)
	}
	start := time.Now()
	rows, err := db.QueryContext(ctx, query)
	if isDuplicateColumnError(err) {
		if input.SortColumn > 0 || page > 1 {
			return nil, fmt.Errorf("this result has duplicate column names; use Next to fetch more rows, or give columns unique aliases to enable sorting and page jumps")
		}
		return p.Open(ctx, db, connection, base, size)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	result := &entities.QueryResult{Rows: make([][]interface{}, 0, size), IsQuery: true, PageSize: size, StartRow: offset + 1, CanNavigate: true}
	for _, ct := range types {
		nullable, known := ct.Nullable()
		result.Columns = append(result.Columns, entities.ColumnInfo{Name: ct.Name(), DatabaseType: ct.DatabaseTypeName(), Nullable: nullable && known})
	}
	for rows.Next() {
		if len(result.Rows) == size {
			result.HasMore = true
			break
		}
		row, err := scanCursorRow(rows, len(types))
		if err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if offset > 0 && len(result.Rows) == 0 {
		return nil, fmt.Errorf("this page no longer has rows; refresh the result")
	}
	if !result.HasMore {
		total := offset + int64(len(result.Rows))
		result.TotalRows = &total
	}
	result.Duration = time.Since(start)
	return result, nil
}

// MySQL rejects repeated output labels only when the SELECT is a derived table.
func isDuplicateColumnError(err error) bool {
	var mysqlErr *mysql.MySQLError
	var sqlServerErr mssql.Error
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1060 || errors.As(err, &sqlServerErr) && sqlServerErr.Number == 8156 || err != nil && strings.Contains(err.Error(), "ORA-00918")
}
