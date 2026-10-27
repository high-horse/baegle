// Package enterprise supplies the database/sql adapters for Oracle, SQL Server and Db2 LUW.
package enterprise

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/metadata"
	catalog "db-lens/internal/engine/metadata/enterprise"
	parser "db-lens/internal/engine/parser"
	queryexecutor "db-lens/internal/engine/queryExecutor"
	"db-lens/internal/engine/queryExecutor/sqlExecutor"
	"db-lens/internal/engine/sqlscript"
	"db-lens/internal/engine/transports"
	_ "github.com/microsoft/go-mssqldb"
	oracle "github.com/sijms/go-ora/v2"
)

type Driver struct{ dialect string }

func New(dialect string) *Driver                   { return &Driver{dialect: dialect} }
func (d *Driver) Name() string                     { return d.dialect }
func (d *Driver) Parser() parser.Parser            { return sqlParser{d.dialect} }
func (d *Driver) Executor() queryexecutor.Executor { return sqlExecutor.New() }
func (d *Driver) Inspector() metadata.Inspector    { return catalog.New(d.dialect) }
func (d *Driver) Create(_ context.Context, config entities.ConnectionConfig, transport transports.Transport) (manager.Connection, error) {
	if d.dialect == "db2" && !db2Enabled {
		return nil, fmt.Errorf("this build does not include IBM Db2: use a build compiled with -tags db2 and install IBM's CLI driver (see docs/database-drivers.md)")
	}
	return &Connection{config: config, transport: transport}, nil
}

type sqlParser struct{ dialect string }

func (p sqlParser) Parse(query string) (*entities.SQLQueryEntity, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("enter a SQL statement")
	}
	statements, err := sqlscript.Split(query, p.dialect)
	if err != nil {
		return nil, err
	}
	if len(statements) != 1 {
		return nil, fmt.Errorf("use Run script to execute multiple statements or batches")
	}
	query = statements[0].SQL
	return &entities.SQLQueryEntity{RawSQL: query, Dialect: entities.SQLDialect(p.dialect), StatementType: parser.ExtractStatementType(query)}, nil
}

func Head(query string) string {
	for {
		query = strings.TrimSpace(query)
		if strings.HasPrefix(query, "--") {
			i := strings.IndexByte(query, '\n')
			if i < 0 {
				return ""
			}
			query = query[i+1:]
			continue
		}
		if strings.HasPrefix(query, "/*") {
			i := strings.Index(query[2:], "*/")
			if i < 0 {
				return ""
			}
			query = query[i+4:]
			continue
		}
		break
	}
	end := 0
	for end < len(query) && (query[end] >= 'a' && query[end] <= 'z' || query[end] >= 'A' && query[end] <= 'Z') {
		end++
	}
	return strings.ToUpper(query[:end])
}

func IsEnterprise(dialect string) bool {
	return dialect == "oracle" || dialect == "sqlserver" || dialect == "db2"
}

type Connection struct {
	config    entities.ConnectionConfig
	transport transports.Transport
	db        *sql.DB
}

func (c *Connection) ID() string                        { return c.config.ID }
func (c *Connection) Name() string                      { return c.config.Name }
func (c *Connection) Type() string                      { return c.config.Type }
func (c *Connection) DatabaseName() string              { return c.config.Database }
func (c *Connection) Config() entities.ConnectionConfig { return c.config }
func (c *Connection) DB() *sql.DB                       { return c.db }
func (c *Connection) IsConnected() bool                 { return c.db != nil }
func (c *Connection) Ping(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("not connected")
	}
	return c.db.PingContext(ctx)
}
func (c *Connection) Disconnect() error {
	var err error
	if c.db != nil {
		err = c.db.Close()
		c.db = nil
	}
	return errors.Join(err, c.transport.Close())
}
func (c *Connection) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}
	name, dsn, err := ConnectionString(c.config, c.transport.Address())
	if err != nil {
		_ = c.transport.Close()
		return err
	}
	db, err := sql.Open(name, dsn)
	if err == nil {
		pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
	}
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		_ = c.transport.Close()
		return err
	}
	db.SetConnMaxIdleTime(5 * time.Minute)
	c.db = db
	return nil
}

// ConnectionString always uses the transport address, including SSH tunnels.
func ConnectionString(config entities.ConnectionConfig, address string) (string, string, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", "", fmt.Errorf("invalid database address: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", "", fmt.Errorf("invalid database port")
	}
	switch config.Type {
	case "oracle":
		if config.Database == "" {
			return "", "", fmt.Errorf("Oracle service name is required")
		}
		options := map[string]string{"CONNECTION TIMEOUT": "15"}
		if config.SSL {
			options["SSL"] = "enable"
			options["SSL VERIFY"] = "true"
		}
		return "oracle", oracle.BuildUrl(host, port, config.Database, config.User, config.Password, options), nil
	case "sqlserver":
		values := url.Values{"database": {config.Database}, "connection timeout": {"15"}, "app name": {"Baegle"}}
		if config.SSL {
			values.Set("encrypt", "true")
		} else {
			values.Set("encrypt", "false")
		}
		u := url.URL{Scheme: "sqlserver", User: url.UserPassword(config.User, config.Password), Host: address, RawQuery: values.Encode()}
		return "sqlserver", u.String(), nil
	case "db2":
		// CLI connection-string fields cannot contain delimiter/control characters.
		// Reject them rather than silently turning part of a credential into options.
		for _, value := range []string{host, config.Database, config.User, config.Password} {
			if strings.ContainsAny(value, ";\x00\r\n") {
				return "", "", fmt.Errorf("Db2 connection fields cannot contain semicolons or control characters")
			}
		}
		dsn := fmt.Sprintf("HOSTNAME=%s;PORT=%d;DATABASE=%s;UID=%s;PWD=%s;PROTOCOL=TCPIP;CONNECTTIMEOUT=15;", host, port, config.Database, config.User, config.Password)
		if config.SSL {
			dsn += "SECURITY=SSL;"
		}
		return "go_ibm_db", dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported SQL dialect %q", config.Type)
	}
}
