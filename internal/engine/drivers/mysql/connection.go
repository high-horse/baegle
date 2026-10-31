package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

type Connection struct {
	config entities.ConnectionConfig

	transport transports.Transport

	db        *sql.DB
	connected bool
}

func New(config entities.ConnectionConfig, transport transports.Transport) *Connection {
	return &Connection{
		config:    config,
		transport: transport,
	}
}

func (c *Connection) ID() string {
	return c.config.ID
}

func (c *Connection) Name() string {
	return c.config.Name
}

func (c *Connection) DatabaseName() string {
	return c.config.Database
}

func (c *Connection) Type() string {
	return string(entities.DialectMySQL)
}

func (c *Connection) DB() *sql.DB {
	return c.db
}

func (c *Connection) dsn() string {
	cfg := mysqldriver.NewConfig()
	cfg.User, cfg.Passwd, cfg.DBName = c.config.User, c.config.Password, c.config.Database
	cfg.Net, cfg.Addr = "tcp", c.transport.Address()
	cfg.ParseTime = true
	cfg.Timeout = 15 * time.Second
	if c.config.SSL {
		cfg.TLSConfig = "true"
	}
	return cfg.FormatDSN()
}

func (c *Connection) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	cfg, err := mysqldriver.ParseDSN(c.dsn())
	if err != nil {
		_ = c.transport.Close()
		return err
	}
	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		_ = c.transport.Close()
		return err
	}
	if c.config.ReadOnly {
		connector = readOnlyConnector{connector}
	}
	db := sql.OpenDB(connector)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		_ = c.transport.Close()
		return err
	}

	c.db = db
	c.connected = true

	return nil
}

func (c *Connection) Disconnect() error {
	var err error
	if c.db != nil {
		err = c.db.Close()
		c.db = nil
	}
	c.connected = false
	return errors.Join(err, c.transport.Close())
}

func (c *Connection) Ping(ctx context.Context) error {

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	if c.db == nil {
		return fmt.Errorf("mysql connection not initialized")
	}

	return c.db.PingContext(ctx)
}

func (c *Connection) IsConnected() bool {
	return c.connected
}

func (c *Connection) Config() entities.ConnectionConfig {
	return c.config
}

// Initialize every pooled session, including replacements after reconnects.
// This syntax is shared by MySQL and MariaDB, unlike their variable names.
type readOnlyConnector struct{ driver.Connector }

func (c readOnlyConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	exec, ok := conn.(driver.ExecerContext)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("driver cannot initialize a read-only session")
	}
	if _, err := exec.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY", nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("enable read-only session: %w", err)
	}
	return conn, nil
}
