package app

import (
	"context"
	"database/sql"
	"db-lens/internal/db"
	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/drivers/mongodb"
	"db-lens/internal/engine/drivers/mysql"
	"db-lens/internal/engine/drivers/postgres"
	"db-lens/internal/engine/drivers/sqlite"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/factory"
	"db-lens/internal/engine/queryExecutor/sqlExecutor"
	"db-lens/internal/engine/transports"
	"db-lens/internal/types"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DbService struct {
	factory        *factory.Factory
	manager        *manager.ConnectionManager
	pager          *sqlExecutor.Pager
	mongoPager     *mongodb.Pager
	historyRepo    *db.HistoryRepository
	consoleMu      sync.Mutex
	consoleQueries map[string]*consoleQuery
	transferMu     sync.Mutex
	importing      bool
}

func NewDbService(historyRepo *db.HistoryRepository) *DbService {
	f := factory.New()
	mongoPager := mongodb.NewPager()
	f.Register(mongodb.NewDriver(mongoPager))
	f.Register(mysql.NewDriver())
	f.Register(postgres.NewDriver())
	f.Register(sqlite.NewDriver())
	return &DbService{
		factory:     f,
		pager:       sqlExecutor.NewPager(),
		mongoPager:  mongoPager,
		manager:     manager.NewConnectionManager(),
		historyRepo: historyRepo,
	}
}

func (s *DbService) Connect(ctx context.Context, config entities.ConnectionConfig) (bool, error) {
	if config.ID == "" {
		config.ID = strconv.FormatInt(-time.Now().UnixNano(), 10)
	}

	transport, err := transports.ForConfig(config)
	if err != nil {
		return false, err
	}

	conn, err := s.factory.Create(
		ctx,
		config,
		transport,
	)
	if err != nil {
		return false, fmt.Errorf("failed to create connection: %w", err)
	}

	// First test the new connection.
	if err := conn.Connect(ctx); err != nil {
		return false, fmt.Errorf("failed to connect: %w", err)
	}

	if old, ok := s.manager.Active(); ok {
		s.pager.CloseConnection(old.ID())
		s.mongoPager.CloseConnection(old.ID())
		_ = s.manager.Remove(old.ID())
	}

	if err := s.manager.Add(conn); err != nil {
		_ = conn.Disconnect()
		return false, fmt.Errorf("failed to register connection: %w", err)
	}

	if err := s.manager.SetActive(conn.ID()); err != nil {
		_ = s.manager.Remove(conn.ID())
		return false, fmt.Errorf("failed to set active connection: %w", err)
	}

	return true, nil
}

func (s *DbService) GetActiveConnectionObject() (types.Connection, bool) {
	conn, ok := s.manager.GetActiveConnection()
	if !ok {
		return types.Connection{}, false
	}

	id, err := strconv.Atoi(conn.ID())
	if err != nil {
		return types.Connection{}, false
	}

	config := conn.Config()
	host := config.Host
	if config.Type == "mongodb" && (strings.HasPrefix(host, "mongodb://") || strings.HasPrefix(host, "mongodb+srv://")) {
		if uri, err := url.Parse(host); err == nil {
			host = uri.Host
		}
	}
	return types.Connection{
		Id:   id,
		Host: host,
		Port: sql.NullInt64{
			Int64: int64(config.Port),
			Valid: host == config.Host,
		},
		Name:     conn.Name(),
		DBName:   conn.DatabaseName(),
		Driver:   conn.Type(),
		ReadOnly: config.ReadOnly,
	}, true
}

func (s *DbService) Disconnect(ctx context.Context, connID string) error {
	s.pager.CloseConnection(connID)
	s.mongoPager.CloseConnection(connID)
	return s.manager.Remove(connID)
}

func (s *DbService) ServiceShutdown() error {
	s.consoleMu.Lock()
	for _, query := range s.consoleQueries {
		query.cancel()
	}
	s.consoleMu.Unlock()
	for _, conn := range s.manager.List() {
		s.pager.CloseConnection(conn.ID())
		s.mongoPager.CloseConnection(conn.ID())
	}
	return s.manager.CloseAll()
}

func (s *DbService) InspectDatabase(ctx context.Context) ([]entities.InspectTableInfo, error) {
	conn, ok := s.manager.Active()
	if !ok {
		return nil, fmt.Errorf("active connection not found")
	}

	driver, err := s.factory.Driver(conn.Type())
	if err != nil {
		return nil, err
	}

	return driver.Inspector().ListTables(ctx, conn)
}

// InspectTableColumns is read-only metadata, including for views and read-only connections.
func (s *DbService) InspectTableColumns(ctx context.Context, table entities.TableRef) ([]entities.InspectColumnInfo, error) {
	conn, ok := s.manager.Active()
	if !ok || conn.ID() != table.ConnectionID {
		return nil, fmt.Errorf("the table belongs to a different or disconnected connection")
	}
	if conn.Type() == "mongodb" {
		return nil, fmt.Errorf("SQL column metadata is not available for MongoDB")
	}
	driver, err := s.factory.Driver(conn.Type())
	if err != nil {
		return nil, err
	}
	operation, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return driver.Inspector().ListColumns(operation, conn, entities.InspectTableInfo{
		Name: table.Name, Schema: table.Schema, Database: table.Database,
	})
}

func (s *DbService) ExecuteQuery(ctx context.Context, queryInput entities.QueryInput) (responseResult *entities.QueryResult, responseError error) {
	if queryInput.Type != entities.QueryExecutionClose {
		operation, cancel, err := queryTimeoutContext(ctx)
		if err != nil {
			return nil, err
		}
		ctx = operation
		defer cancel()
		defer func() {
			if ctx.Err() != nil {
				responseResult = nil
				responseError = context.Cause(ctx)
			}
		}()
	}
	conn, ok := s.manager.Active()
	if !ok {
		return nil, fmt.Errorf("active connection not found")
	}

	if conn.Type() == "mongodb" {
		return s.executeMongo(ctx, conn, queryInput)
	}

	if queryInput.Type == entities.QueryExecutionClose {
		s.pager.Close(queryInput.Cursor, conn.ID())
		return nil, nil
	}
	if queryInput.Type == entities.QueryExecutionFetchPaged {
		result, err := s.pager.FetchContext(ctx, queryInput.Cursor, conn.ID(), queryInput.Page)
		if err != nil {
			return nil, err
		}
		response := *result
		response.Duration = time.Duration(result.Duration.Milliseconds())
		return &response, nil
	}
	if queryInput.Type != entities.QueryExecutionExecute && queryInput.Type != entities.QueryExecuteRefresh && queryInput.Type != entities.QueryExecutionNavigate && queryInput.Type != entities.QueryExecutionCount {
		return nil, fmt.Errorf("unknown query execution type")
	}
	if queryInput.Cursor != "" && queryInput.Type != entities.QueryExecutionCount {
		s.pager.Close(queryInput.Cursor, conn.ID())
	}
	driver, err := s.factory.Driver(conn.Type())
	if err != nil {
		return nil, err
	}
	parsed, err := driver.Parser().Parse(queryInput.Query)
	if err != nil {
		return nil, fmt.Errorf("query parsing error: %w", err)
	}
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok || sqlConn.DB() == nil {
		return nil, fmt.Errorf("active SQL connection not available")
	}
	start := time.Now()
	if queryInput.Type == entities.QueryExecutionCount {
		total, err := sqlExecutor.CountRead(ctx, sqlConn.DB(), parsed.RawSQL)
		if err != nil {
			return nil, fmt.Errorf("count result rows: %w", err)
		}
		return &entities.QueryResult{TotalRows: &total, Duration: time.Duration(time.Since(start).Milliseconds())}, nil
	}
	var result *entities.QueryResult
	if _, pageable := sqlExecutor.PageableSQL(parsed.RawSQL); pageable {
		queryInput.Query = parsed.RawSQL
		result, err = s.pager.OpenRead(ctx, sqlConn.DB(), conn.ID(), conn.Type(), queryInput)
	} else if queryInput.Type == entities.QueryExecutionNavigate || queryInput.SortColumn != 0 {
		return nil, fmt.Errorf("this statement does not support counted page navigation or sorting")
	} else {
		result, err = s.pager.Open(ctx, sqlConn.DB(), conn.ID(), parsed.RawSQL, queryInput.PageSize)
	}
	if err == nil {
		// Do not mutate the page cached by the cursor.
		response := *result
		response.Duration = time.Since(start)
		result = &response
	}
	historyEntry := db.QueryHistoryEntity{
		ConnectionId: conn.ID(), DatabaseName: conn.DatabaseName(),
		QueryText: queryInput.Query, Status: "SUCCESS",
	}
	if err != nil {
		historyEntry.Status = "ERROR"
	} else {
		historyEntry.Duration = int(result.Duration.Milliseconds())
	}
	if s.historyRepo != nil && queryInput.Type != entities.QueryExecutionNavigate {
		go func() {
			logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.historyRepo.Log(logCtx, historyEntry)
		}()
	}
	if err != nil {
		return nil, fmt.Errorf("SQL execution error: %w", err)
	}
	response := *result
	response.Duration = time.Duration(result.Duration.Milliseconds())
	return &response, nil
}

func (s *DbService) GetDDL(ctx context.Context, table entities.TableRef) (string, error) {
	conn, ok := s.manager.Get(table.ConnectionID)
	if !ok {
		return "", fmt.Errorf("table connection not found")
	}

	driver, err := s.factory.Driver(conn.Type())
	if err != nil {
		return "", err
	}

	return driver.Inspector().GetTableDDL(ctx, conn, table)

}

func (s *DbService) PingConnection(ctx context.Context, connID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, ok := s.manager.Get(connID)
	if !ok {
		return false, fmt.Errorf("connection not found")
	}

	if err := conn.Ping(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *DbService) TestSSHConnection(ctx context.Context, config entities.SSHConfig) (bool, error) {
	if err := transports.TestSSH(ctx, config); err != nil {
		return false, err
	}
	return true, nil
}

func (s *DbService) PingConfig(ctx context.Context, config entities.ConnectionConfig) (bool, error) {

	transport, err := transports.ForConfig(config)
	if err != nil {
		return false, err
	}

	conn, err := s.factory.Create(
		ctx,
		config,
		transport,
	)

	if err != nil {
		return false, err
	}

	if err := conn.Connect(ctx); err != nil {
		return false, err
	}
	defer conn.Disconnect()

	return true, conn.Ping(ctx)
}

func (s *DbService) GetQueryHistory(ctx context.Context, limit int, since time.Time) ([]db.QueryHistoryEntity, error) {
	return s.historyRepo.GetHistory(ctx, limit, since)
}

func (s *DbService) SaveAndConnect(ctx context.Context, config entities.ConnectionConfig) (bool, error) {
	newID, err := db.StoreConnection(config)
	if err != nil {
		return false, fmt.Errorf("failed to store connection: %w", err)
	}
	config.ID = strconv.FormatInt(newID, 10)
	return s.Connect(ctx, config)
}

func (s *DbService) GetActiveConnection() (string, error) {
	conn, ok := s.manager.Active()
	if !ok {
		return "", fmt.Errorf("no active connection")
	}
	return conn.ID(), nil
}

func (s *DbService) executeMongo(ctx context.Context, conn manager.Connection, input entities.QueryInput) (*entities.QueryResult, error) {
	if input.Type == entities.QueryExecutionClose {
		s.mongoPager.Close(input.Cursor, conn.ID())
		return nil, nil
	}
	var result *entities.QueryResult
	var err error
	if input.Type == entities.QueryExecutionFetchPaged {
		result, err = s.mongoPager.Fetch(ctx, input.Cursor, conn.ID(), input.Page)
	} else {
		if input.Type != entities.QueryExecutionExecute && input.Type != entities.QueryExecuteRefresh && input.Type != entities.QueryExecutionNavigate {
			return nil, fmt.Errorf("unknown MongoDB query execution type")
		}
		if input.SortColumn != 0 {
			return nil, fmt.Errorf("specify MongoDB sorting in the JSON command")
		}
		s.mongoPager.Close(input.Cursor, conn.ID())
		result, err = s.mongoPager.OpenInput(ctx, conn, input)
		if s.historyRepo != nil && input.Type != entities.QueryExecutionNavigate {
			entry := db.QueryHistoryEntity{ConnectionId: conn.ID(), DatabaseName: conn.DatabaseName(), QueryText: input.Query, Status: "SUCCESS"}
			if err != nil {
				entry.Status = "ERROR"
			} else {
				entry.Duration = int(result.Duration.Milliseconds())
			}
			go func() {
				logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = s.historyRepo.Log(logCtx, entry)
			}()
		}
	}
	if err != nil {
		return nil, fmt.Errorf("MongoDB execution error: %w", err)
	}
	response := *result
	response.Duration = time.Duration(result.Duration.Milliseconds())
	return &response, nil
}
