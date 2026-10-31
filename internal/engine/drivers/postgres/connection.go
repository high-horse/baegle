package postgres

import (
	"context"
	"database/sql"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
	"errors"
	"fmt"
	"net"
	"net/url"

	// _ "github.com/lib/pq"
	_ "github.com/jackc/pgx/v5/stdlib"
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
	return string(entities.DialectPostgreSQL)
}

func (c *Connection) DB() *sql.DB {
	return c.db
}

func (c *Connection) dsn() string {
	host, port, _ := net.SplitHostPort(c.transport.Address())
	u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(host, port), Path: "/" + c.config.Database, User: url.UserPassword(c.config.User, c.config.Password)}
	q := u.Query()
	q.Set("sslmode", "disable")
	if c.config.SSL {
		q.Set("sslmode", "require")
	}
	q.Set("connect_timeout", "15")
	if c.config.ReadOnly {
		q.Set("default_transaction_read_only", "on")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Connection) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	db, err := sql.Open("pgx", c.dsn())
	if err != nil {
		_ = c.transport.Close()
		return err
	}

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
	if c.db == nil {
		return fmt.Errorf("postgres connection not initialized")
	}

	return c.db.PingContext(ctx)
}

func (c *Connection) IsConnected() bool {
	return c.connected
}

func (c *Connection) Config() entities.ConnectionConfig {
	return c.config
}
