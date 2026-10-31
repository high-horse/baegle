// Package nosql provides native, finite query execution for Cassandra, CouchDB and Redis.
package nosql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/metadata"
	parser "db-lens/internal/engine/parser"
	executor "db-lens/internal/engine/queryExecutor"
	"db-lens/internal/engine/transports"
)

type backend interface {
	connect(context.Context) error
	close() error
	ping(context.Context) error
	query(context.Context, string, int, []byte) (*entities.QueryResult, []byte, error)
	tables(context.Context) ([]entities.InspectTableInfo, error)
	columns(context.Context, entities.InspectTableInfo) ([]entities.InspectColumnInfo, error)
	ddl(context.Context, entities.TableRef) (string, error)
}

type Driver struct{ kind string }

func New(kind string) *Driver                   { return &Driver{kind: kind} }
func (d *Driver) Name() string                  { return d.kind }
func (d *Driver) Parser() parser.Parser         { return d }
func (d *Driver) Executor() executor.Executor   { return d }
func (d *Driver) Inspector() metadata.Inspector { return d }
func (d *Driver) Parse(q string) (*entities.SQLQueryEntity, error) {
	return &entities.SQLQueryEntity{RawSQL: q}, nil
}
func (d *Driver) Create(_ context.Context, cfg entities.ConnectionConfig, t transports.Transport) (manager.Connection, error) {
	c := &Connection{cfg: cfg, transport: t, cursors: map[string]*cursor{}}
	switch d.kind {
	case "redis":
		c.backend = &redisBackend{cfg: cfg, transport: t}
	case "couchdb":
		c.backend = &couchBackend{cfg: cfg, transport: t}
	case "cassandra":
		c.backend = &cassandraBackend{cfg: cfg, transport: t}
	default:
		return nil, fmt.Errorf("unknown NoSQL driver %q", d.kind)
	}
	return c, nil
}
func (d *Driver) Execute(ctx context.Context, c manager.Connection, q string, _ ...any) (*entities.QueryResult, error) {
	return c.(*Connection).ExecuteInput(ctx, entities.QueryInput{Query: q, PageSize: 100})
}
func (d *Driver) ListDatabases(_ context.Context, c manager.Connection) ([]entities.DatabaseInfo, error) {
	return []entities.DatabaseInfo{{Name: c.DatabaseName()}}, nil
}
func (d *Driver) ListTables(ctx context.Context, c manager.Connection) ([]entities.InspectTableInfo, error) {
	return c.(*Connection).backend.tables(ctx)
}
func (d *Driver) ListColumns(ctx context.Context, c manager.Connection, t entities.InspectTableInfo) ([]entities.InspectColumnInfo, error) {
	return c.(*Connection).backend.columns(ctx, t)
}
func (d *Driver) GetTableDDL(ctx context.Context, c manager.Connection, t entities.TableRef) (string, error) {
	return c.(*Connection).backend.ddl(ctx, t)
}

// Cursors are connection-owned and opaque. Backend continuation tokens never cross
// the API, and fetching a page cannot execute a different query or replay a write.
type cursor struct {
	query      string
	size, page int
	state      []byte
	rows       int64
	last       *entities.QueryResult
	used       time.Time
}
type Connection struct {
	cfg       entities.ConnectionConfig
	transport transports.Transport
	backend   backend
	mu        sync.Mutex
	connected bool
	cursors   map[string]*cursor
}

func (c *Connection) ID() string                        { return c.cfg.ID }
func (c *Connection) Name() string                      { return c.cfg.Name }
func (c *Connection) Type() string                      { return c.cfg.Type }
func (c *Connection) DatabaseName() string              { return c.cfg.Database }
func (c *Connection) Config() entities.ConnectionConfig { return c.cfg }
func (c *Connection) IsConnected() bool                 { c.mu.Lock(); defer c.mu.Unlock(); return c.connected }
func (c *Connection) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connected {
		return nil
	}
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}
	if err := c.backend.connect(ctx); err != nil {
		_ = c.backend.close()
		_ = c.transport.Close()
		return err
	}
	c.connected = true
	return nil
}
func (c *Connection) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cursors = map[string]*cursor{}
	c.connected = false
	err := c.backend.close()
	tunnelErr := c.transport.Close()
	if err != nil {
		return err
	}
	return tunnelErr
}
func (c *Connection) Ping(ctx context.Context) error { return c.backend.ping(ctx) }
func (c *Connection) ExecuteInput(ctx context.Context, in entities.QueryInput) (*entities.QueryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if in.Type == entities.QueryExecutionClose {
		delete(c.cursors, in.Cursor)
		return nil, nil
	}
	if !c.connected {
		return nil, fmt.Errorf("connection is closed")
	}
	now := time.Now()
	for id, slot := range c.cursors {
		if now.Sub(slot.used) > 10*time.Minute {
			delete(c.cursors, id)
		}
	}
	if in.Type == entities.QueryExecutionFetchPaged {
		slot := c.cursors[in.Cursor]
		if slot == nil {
			return nil, fmt.Errorf("result cursor expired; run the query again")
		}
		slot.used = now
		if in.Page == slot.page {
			return slot.last, nil
		}
		if in.Page != slot.page+1 && in.Page != 0 {
			return nil, fmt.Errorf("only the next result page is available")
		}
		if len(slot.state) == 0 {
			return nil, fmt.Errorf("no more result pages")
		}
		result, next, err := c.backend.query(ctx, slot.query, slot.size, slot.state)
		if err != nil {
			return nil, err
		}
		result.Duration = time.Since(now)
		result.PageSize = slot.size
		result.StartRow = slot.rows + 1
		result.HasMore = len(next) > 0
		if result.HasMore {
			result.Cursor = in.Cursor
		}
		slot.rows += int64(len(result.Rows))
		slot.page++
		slot.state = next
		slot.last = result
		return result, nil
	}
	if in.Type != entities.QueryExecutionExecute && in.Type != entities.QueryExecuteRefresh {
		return nil, fmt.Errorf("this database supports sequential paging; use its native query syntax for filtering and sorting")
	}
	if in.SortColumn != 0 {
		return nil, fmt.Errorf("specify sorting in the native query")
	}
	delete(c.cursors, in.Cursor)
	size := in.PageSize
	if size <= 0 {
		size = 100
	}
	result, next, err := c.backend.query(ctx, in.Query, size, nil)
	if err != nil {
		return nil, err
	}
	result.Duration = time.Since(now)
	result.PageSize = size
	result.StartRow = 1
	result.HasMore = len(next) > 0
	if len(next) > 0 {
		if len(c.cursors) >= 16 {
			var oldest string
			var at time.Time
			for id, s := range c.cursors {
				if oldest == "" || s.used.Before(at) {
					oldest = id
					at = s.used
				}
			}
			delete(c.cursors, oldest)
		}
		var key [16]byte
		if _, err := rand.Read(key[:]); err != nil {
			return nil, err
		}
		id := hex.EncodeToString(key[:])
		result.Cursor = id
		c.cursors[id] = &cursor{query: in.Query, size: size, page: 1, state: next, rows: int64(len(result.Rows)), last: result, used: now}
	}
	return result, nil
}

func collection(cfg entities.ConnectionConfig, name string) []entities.InspectTableInfo {
	return []entities.InspectTableInfo{{Name: name, Type: "COLLECTION", Database: cfg.Database, Schema: cfg.Database, Engine: cfg.Type}}
}
func valueResult(value any) *entities.QueryResult {
	return &entities.QueryResult{IsQuery: true, Columns: []entities.ColumnInfo{{Name: "value", DatabaseType: "JSON", Nullable: true}}, Rows: [][]any{{cqlValue(value)}}}
}
