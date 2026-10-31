package mysql

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

type testConnector struct {
	sessions []*testSession
	fail     bool
}

func (c *testConnector) Driver() driver.Driver { return nil }
func (c *testConnector) Connect(context.Context) (driver.Conn, error) {
	s := &testSession{fail: c.fail}
	c.sessions = append(c.sessions, s)
	return s, nil
}

type testSession struct {
	query  string
	closed bool
	fail   bool
}

func (c *testSession) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unused") }
func (c *testSession) Begin() (driver.Tx, error)           { return nil, errors.New("unused") }
func (c *testSession) Close() error                        { c.closed = true; return nil }
func (c *testSession) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.query = q
	if c.fail {
		return nil, errors.New("unsupported")
	}
	return driver.RowsAffected(0), nil
}

func TestEveryPooledSessionStartsReadOnly(t *testing.T) {
	base := &testConnector{}
	connector := readOnlyConnector{base}
	for range 3 {
		conn, err := connector.Connect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}
	for _, session := range base.sessions {
		if session.query != "SET SESSION TRANSACTION READ ONLY" {
			t.Fatal("session not initialized")
		}
	}
	base.fail = true
	if conn, err := connector.Connect(context.Background()); err == nil || conn != nil || !base.sessions[3].closed {
		t.Fatal("initialization failure must close the connection")
	}
}
