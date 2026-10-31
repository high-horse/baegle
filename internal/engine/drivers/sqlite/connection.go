package sqlite

import (
	"context"
	"database/sql"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
	"fmt"
	"log"
	"net/url"

	_ "modernc.org/sqlite"
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
		connected: false,
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
	return string(entities.DialectSQLite)
}

func (c *Connection) DB() *sql.DB {
	return c.db
}

func (c *Connection) dsn() (string, error) {
	u, err := url.Parse("file:" + c.config.Database)
	if err != nil {
		return "", fmt.Errorf("invalid SQLite database path: %w", err)
	}
	q := u.Query()
	if c.config.ReadOnly {
		q.Set("mode", "ro")
		q.Add("_pragma", "query_only(1)")
	}
	q.Set("_journal_mode", "WAL")
	q.Set("_foreign_keys", "on")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Connection) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}
	log.Println("connected to transport")
	log.Println("opening database connection")
	dsn, err := c.dsn()
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return err
	}

	c.db = db
	c.connected = true

	return nil
}

func (c *Connection) Disconnect() error {
	if !c.connected {
		return nil
	}

	if c.db == nil {
		return nil
	}

	err := c.db.Close()
	c.connected = false
	return err
}

func (c *Connection) Ping(ctx context.Context) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}
	if c.db == nil {
		return fmt.Errorf("db not initialized")
	}
	return c.db.PingContext(ctx)
}

func (c *Connection) IsConnected() bool {
	return c.connected
}

func (c *Connection) Config() entities.ConnectionConfig {
	return c.config
}
