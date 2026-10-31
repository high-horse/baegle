package sqlExecutor

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"db-lens/internal/engine/entities"
)

const CursorLifetime = 10 * time.Minute
const maxCursors = 16

// Pager retains driver row streams, never rewrites SQL or materializes a full result.
// Each cursor pins one database connection until exhausted, closed or expired.
type Pager struct {
	mu      sync.Mutex
	cursors map[string]*rowCursor
}

type rowCursor struct {
	mu         sync.Mutex
	connection string
	rows       *sql.Rows
	cancel     context.CancelFunc
	columns    []entities.ColumnInfo
	pending    []interface{}
	size       int
	read       int64
	page       int
	last       *entities.QueryResult
	closed     bool
	totalRows  *int64
}

func NewPager() *Pager { return &Pager{cursors: make(map[string]*rowCursor)} }

func (p *Pager) Open(ctx context.Context, db *sql.DB, connection, query string, size int) (*entities.QueryResult, error) {
	return p.open(ctx, db, connection, query, size, 0, nil)
}

func (p *Pager) OpenPage(ctx context.Context, db *sql.DB, connection, query string, size int, offset int64, total int64) (*entities.QueryResult, error) {
	return p.open(ctx, db, connection, query, size, offset, &total)
}

func (p *Pager) open(ctx context.Context, db *sql.DB, connection, query string, size int, offset int64, total *int64) (*entities.QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if size == 0 {
		size = 100
	}
	if size < 1 {
		return nil, fmt.Errorf("page size must be a positive whole number")
	}
	var key [16]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(key[:])
	// A Wails request ends after the first page; the stream has its own bounded lifetime.
	streamCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), CursorLifetime)
	c := &rowCursor{connection: connection, cancel: cancel, size: size, read: offset, page: int(offset / int64(size)), totalRows: total}
	c.mu.Lock()
	p.mu.Lock()
	if len(p.cursors) >= maxCursors {
		p.mu.Unlock()
		c.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("too many open results; close a result tab before running another query")
	}
	p.cursors[id] = c
	p.mu.Unlock()
	context.AfterFunc(streamCtx, func() { p.Close(id, connection) })
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	start := time.Now()
	rows, err := db.QueryContext(streamCtx, query)
	if err == nil {
		c.rows = rows
		var types []*sql.ColumnType
		types, err = rows.ColumnTypes()
		// SQL Server batches can emit update-count results before their SELECT.
		for err == nil && len(types) == 0 && rows.NextResultSet() {
			types, err = rows.ColumnTypes()
		}
		if err == nil {
			err = rows.Err()
		}
		for _, ct := range types {
			nullable, known := ct.Nullable()
			c.columns = append(c.columns, entities.ColumnInfo{Name: ct.Name(), DatabaseType: ct.DatabaseTypeName(), Nullable: nullable && known})
		}
	}
	var result *entities.QueryResult
	if err == nil {
		result, err = c.fetch(id, start)
	}
	c.mu.Unlock()
	if err != nil || !result.HasMore {
		p.Close(id, connection)
	}
	return result, err
}

func (p *Pager) Fetch(id, connection string, page int) (*entities.QueryResult, error) {
	p.mu.Lock()
	c := p.cursors[id]
	p.mu.Unlock()
	if c == nil || c.connection != connection {
		return nil, fmt.Errorf("result cursor expired or closed; run the query again")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("result cursor closed; run the query again")
	}
	if page == c.page && c.last != nil {
		result := c.last
		c.mu.Unlock()
		return result, nil
	}
	if page != c.page+1 {
		c.mu.Unlock()
		return nil, fmt.Errorf("invalid cursor page")
	}
	result, err := c.fetch(id, time.Now())
	c.mu.Unlock()
	if err != nil || !result.HasMore {
		p.Close(id, connection)
	}
	return result, err
}

func (p *Pager) FetchContext(ctx context.Context, id, connection string, page int) (*entities.QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { p.Close(id, connection) })
	defer stop()
	result, err := p.Fetch(id, connection, page)
	if ctx.Err() != nil {
		return nil, context.Cause(ctx)
	}
	return result, err
}

func (c *rowCursor) fetch(id string, start time.Time) (*entities.QueryResult, error) {
	result := &entities.QueryResult{Columns: c.columns, Rows: make([][]interface{}, 0, min(c.size, 100)), IsQuery: len(c.columns) > 0, PageSize: c.size, StartRow: c.read + 1, TotalRows: c.totalRows, CanNavigate: c.totalRows != nil}
	if result.IsQuery {
		if c.pending != nil {
			result.Rows = append(result.Rows, c.pending)
			c.pending = nil
		}
		for len(result.Rows) < c.size && c.rows.Next() {
			row, err := scanCursorRow(c.rows, len(c.columns))
			if err != nil {
				return nil, err
			}
			result.Rows = append(result.Rows, row)
		}
		// One lookahead distinguishes an exact full final page from an unfinished result.
		if len(result.Rows) == c.size && c.rows.Next() {
			row, err := scanCursorRow(c.rows, len(c.columns))
			if err != nil {
				return nil, err
			}
			c.pending = row
			result.HasMore = true
			result.Cursor = id
		}
		if err := c.rows.Err(); err != nil {
			return nil, err
		}
	}
	c.read += int64(len(result.Rows))
	c.page++
	result.Duration = time.Since(start)
	c.last = result
	return result, nil
}

func scanCursorRow(rows *sql.Rows, count int) ([]interface{}, error) {
	values := make([]interface{}, count)
	ptrs := make([]interface{}, count)
	for i := range values {
		ptrs[i] = &values[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	return normalizeRow(values), nil
}

func (p *Pager) Close(id, connection string) {
	p.mu.Lock()
	c := p.cursors[id]
	if c == nil || c.connection != connection {
		p.mu.Unlock()
		return
	}
	delete(p.cursors, id)
	p.mu.Unlock()
	// Cancel before waiting for a fetch so a blocked driver read is interrupted.
	c.cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.rows != nil {
		_ = c.rows.Close()
	}
}

func (p *Pager) CloseConnection(connection string) {
	p.mu.Lock()
	var ids []string
	for id, c := range p.cursors {
		if c.connection == connection {
			ids = append(ids, id)
		}
	}
	p.mu.Unlock()
	for _, id := range ids {
		p.Close(id, connection)
	}
}
