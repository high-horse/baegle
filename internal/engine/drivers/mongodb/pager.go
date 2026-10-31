package mongodb

import (
	"context"
	"crypto/rand"
	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const cursorLifetime = 10 * time.Minute

type Pager struct {
	mu      sync.Mutex
	cursors map[string]*documentCursor
}
type documentCursor struct {
	mu         sync.Mutex
	connection string
	cursor     *mongo.Cursor
	size       int
	pending    bson.Raw
	read       int64
	page       int
	last       *entities.QueryResult
	closed     bool
	timer      *time.Timer
	totalRows  *int64
}

func NewPager() *Pager { return &Pager{cursors: make(map[string]*documentCursor)} }

func (p *Pager) Open(ctx context.Context, conn manager.Connection, query string, size int) (*entities.QueryResult, error) {
	return p.OpenInput(ctx, conn, entities.QueryInput{Query: query, PageSize: size})
}

func (p *Pager) OpenInput(ctx context.Context, conn manager.Connection, input entities.QueryInput) (*entities.QueryResult, error) {
	size := input.PageSize
	command, err := parseCommand(input.Query)
	if err != nil {
		return nil, err
	}
	if conn.Config().ReadOnly && !readCommand(command) {
		return nil, fmt.Errorf("this MongoDB connection is read-only")
	}
	c, err := mongoConnection(conn)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		size = 100
	}
	if size < 1 {
		return nil, fmt.Errorf("page size must be a positive whole number")
	}
	operation, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	var total *int64
	var offset int64
	if command[0].Key == "find" {
		countCommand, err := findCountCommand(command)
		if err != nil {
			return nil, err
		}
		var response struct {
			N int64 `bson:"n"`
		}
		if err := c.DB().RunCommand(operation, countCommand).Decode(&response); err != nil {
			return nil, fmt.Errorf("count MongoDB results: %w", err)
		}
		total = &response.N
		command, offset, err = findPageCommand(command, response.N, size, input.Page)
		if err != nil {
			return nil, err
		}
	} else if input.Type == entities.QueryExecutionNavigate {
		return nil, fmt.Errorf("direct page navigation is supported for MongoDB find commands")
	}
	switch command[0].Key {
	case "find", "aggregate", "listCollections", "listIndexes":
		if command[0].Key != "find" {
			hasCursor := false
			for _, field := range command {
				if field.Key == "cursor" {
					hasCursor = true
				}
			}
			if !hasCursor {
				command = append(command, bson.E{Key: "cursor", Value: bson.D{{Key: "batchSize", Value: size}}})
			}
		} else {
			hasBatch := false
			for _, field := range command {
				if field.Key == "batchSize" {
					hasBatch = true
				}
			}
			if !hasBatch {
				command = append(command, bson.E{Key: "batchSize", Value: size})
			}
		}
		// Reserve a slot before sending a command, including write-producing aggregations.
		var key [16]byte
		if _, err := rand.Read(key[:]); err != nil {
			return nil, err
		}
		id := hex.EncodeToString(key[:])
		stream := &documentCursor{connection: conn.ID(), size: size, read: offset, page: int(offset / int64(size)), totalRows: total}
		stream.mu.Lock()
		p.mu.Lock()
		if len(p.cursors) >= 16 {
			p.mu.Unlock()
			stream.mu.Unlock()
			return nil, fmt.Errorf("too many open results; close a result tab before running another query")
		}
		p.cursors[id] = stream
		p.mu.Unlock()
		stream.timer = time.AfterFunc(cursorLifetime, func() { p.Close(id, conn.ID()) })
		cursor, err := c.DB().RunCommandCursor(operation, command)
		stream.cursor = cursor
		var result *entities.QueryResult
		if err == nil {
			result, err = stream.fetch(operation, id, start)
		}
		stream.mu.Unlock()
		if err != nil || !result.HasMore {
			p.Close(id, conn.ID())
		}
		return result, err
	default:
		var document bson.Raw
		if err := c.DB().RunCommand(operation, command).Decode(&document); err != nil {
			return nil, err
		}
		if err := commandWriteError(document); err != nil {
			return nil, err
		}
		columns, rows, err := documentTable([]bson.Raw{document})
		if err != nil {
			return nil, err
		}
		return &entities.QueryResult{Columns: columns, Rows: rows, IsQuery: true, StartRow: 1, PageSize: size, Duration: time.Since(start)}, nil
	}
}

func (p *Pager) Fetch(ctx context.Context, id, connection string, page int) (*entities.QueryResult, error) {
	p.mu.Lock()
	stream := p.cursors[id]
	p.mu.Unlock()
	if stream == nil || stream.connection != connection {
		return nil, fmt.Errorf("result cursor expired or closed; run the query again")
	}
	stream.mu.Lock()
	if stream.closed {
		stream.mu.Unlock()
		return nil, fmt.Errorf("result cursor closed; run the query again")
	}
	if page == stream.page && stream.last != nil {
		result := stream.last
		stream.mu.Unlock()
		return result, nil
	}
	if page != stream.page+1 {
		stream.mu.Unlock()
		return nil, fmt.Errorf("invalid cursor page")
	}
	operation, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := stream.fetch(operation, id, time.Now())
	stream.mu.Unlock()
	if err != nil || !result.HasMore {
		p.Close(id, connection)
	}
	return result, err
}

// documentTable uses the union of top-level keys, in first-seen order.
// Canonical Extended JSON preserves BSON values (including large integers).
func documentTable(documents []bson.Raw) ([]entities.ColumnInfo, [][]any, error) {
	columns := []entities.ColumnInfo{}
	indexes := map[string]int{}
	values := make([]map[string]any, 0, len(documents))
	for _, document := range documents {
		elements, err := document.Elements()
		if err != nil {
			return nil, nil, err
		}
		encoded, err := bson.MarshalExtJSON(document, true, false)
		if err != nil {
			return nil, nil, err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			return nil, nil, err
		}
		row := map[string]any{}
		for _, element := range elements {
			key, value := element.Key(), element.Value()
			kind := value.Type.String()
			if value.Type == bson.TypeEmbeddedDocument || value.Type == bson.TypeArray {
				kind = "Extended JSON"
			}
			if index, exists := indexes[key]; !exists {
				indexes[key] = len(columns)
				columns = append(columns, entities.ColumnInfo{Name: key, DatabaseType: kind})
			} else if columns[index].DatabaseType != kind && value.Type != bson.TypeNull {
				columns[index].DatabaseType = "Extended JSON"
			}
			switch value.Type {
			case bson.TypeString:
				row[key] = value.StringValue()
			case bson.TypeBoolean:
				row[key] = value.Boolean()
			case bson.TypeNull:
				row[key] = nil
			case bson.TypeInt32:
				row[key] = value.Int32()
			case bson.TypeDouble:
				row[key] = value.Double()
			default:
				row[key] = string(fields[key])
			}
		}
		values = append(values, row)
	}
	rows := make([][]any, len(values))
	for i, values := range values {
		rows[i] = make([]any, len(columns))
		for key, value := range values {
			rows[i][indexes[key]] = value
		}
	}
	return columns, rows, nil
}
func documentRow(document bson.Raw) ([]any, error) {
	encoded, err := bson.MarshalExtJSON(document, true, false)
	if err != nil {
		return nil, fmt.Errorf("encode MongoDB document: %w", err)
	}
	return []any{string(encoded)}, nil
}
func (c *documentCursor) fetch(ctx context.Context, id string, start time.Time) (*entities.QueryResult, error) {
	result := &entities.QueryResult{IsQuery: true, PageSize: c.size, StartRow: c.read + 1, TotalRows: c.totalRows, CanNavigate: c.totalRows != nil}
	documents := make([]bson.Raw, 0, min(c.size, 100))
	if c.pending != nil {
		documents = append(documents, c.pending)
		c.pending = nil
	}
	for len(documents) < c.size && c.cursor.Next(ctx) {
		documents = append(documents, append(bson.Raw(nil), c.cursor.Current...))
	}
	if len(documents) == c.size && c.cursor.Next(ctx) {
		c.pending = append(bson.Raw(nil), c.cursor.Current...)
		result.HasMore, result.Cursor = true, id
	}
	var err error
	result.Columns, result.Rows, err = documentTable(documents)
	for _, document := range documents {
		encoded, encodeErr := bson.MarshalExtJSON(document, true, false)
		if encodeErr != nil {
			return nil, encodeErr
		}
		result.Documents = append(result.Documents, string(encoded))
	}
	if err != nil {
		return nil, err
	}
	if err := c.cursor.Err(); err != nil {
		return nil, err
	}
	c.read += int64(len(result.Rows))
	c.page++
	result.Duration = time.Since(start)
	c.last = result
	return result, nil
}
func (p *Pager) Close(id, connection string) {
	p.mu.Lock()
	stream := p.cursors[id]
	if stream == nil || stream.connection != connection {
		p.mu.Unlock()
		return
	}
	delete(p.cursors, id)
	p.mu.Unlock()
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.closed {
		return
	}
	stream.closed = true
	if stream.timer != nil {
		stream.timer.Stop()
	}
	if stream.cursor != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = stream.cursor.Close(ctx)
		cancel()
	}
}
func (p *Pager) CloseConnection(connection string) {
	p.mu.Lock()
	ids := []string{}
	for id, stream := range p.cursors {
		if stream.connection == connection {
			ids = append(ids, id)
		}
	}
	p.mu.Unlock()
	for _, id := range ids {
		p.Close(id, connection)
	}
}

// RunCommand reports command errors, but write commands may report per-item errors with ok: 1.
func commandWriteError(document bson.Raw) error {
	type writeError struct {
		Code    int    `bson:"code"`
		Message string `bson:"errmsg"`
	}
	var response struct {
		Errors  []writeError `bson:"writeErrors"`
		Concern *writeError  `bson:"writeConcernError"`
	}
	if err := bson.Unmarshal(document, &response); err != nil {
		return err
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("MongoDB write error %d: %s", response.Errors[0].Code, response.Errors[0].Message)
	}
	if response.Concern != nil {
		return fmt.Errorf("MongoDB write concern error %d: %s", response.Concern.Code, response.Concern.Message)
	}
	return nil
}
