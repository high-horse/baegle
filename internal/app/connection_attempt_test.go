package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/drivers"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/transports"
)

type waitingConnection struct {
	manager.Connection
	config  entities.ConnectionConfig
	started chan struct{}
	closed  atomic.Int32
}

func (c *waitingConnection) ID() string { return c.config.ID }
func (c *waitingConnection) Connect(ctx context.Context) error {
	close(c.started)
	<-ctx.Done()
	// Simulate a driver that returns a late success despite cancellation.
	return nil
}
func (c *waitingConnection) Disconnect() error { c.closed.Add(1); return nil }

type waitingDriver struct {
	drivers.Driver
	conn *waitingConnection
}

func (d waitingDriver) Name() string { return "waiting" }
func (d waitingDriver) Create(_ context.Context, config entities.ConnectionConfig, _ transports.Transport) (manager.Connection, error) {
	d.conn.config = config
	return d.conn, nil
}

func TestCancelConnectionAttemptBeforeTimeout(t *testing.T) {
	s, _, previous := editService(t)
	conn := &waitingConnection{started: make(chan struct{})}
	s.factory.Register(waitingDriver{conn: conn})
	done := make(chan error, 1)
	go func() {
		_, err := s.ConnectWithRequest(context.Background(), "slow", entities.ConnectionConfig{ID: "slow", Type: "waiting"})
		done <- err
	}()
	select {
	case <-conn.started:
	case <-time.After(2 * time.Second):
		t.Fatal("connection never started")
	}
	if _, err := s.ConnectWithRequest(context.Background(), "duplicate", entities.ConnectionConfig{Type: "waiting"}); err == nil {
		t.Fatal("concurrent connection accepted")
	}
	if s.CancelConnectionAttempt("unrelated") {
		t.Fatal("unrelated cancellation matched")
	}
	if !s.CancelConnectionAttempt("slow") {
		t.Fatal("cancel did not find attempt")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel result: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel waited for connection timeout")
	}
	if conn.closed.Load() == 0 {
		t.Fatal("cancelled connection was not cleaned up")
	}
	if active, ok := s.manager.Active(); !ok || active != previous {
		t.Fatal("cancelled attempt replaced the previous session")
	}
}

func TestEarlyAndLateConnectionCancellation(t *testing.T) {
	s, _, conn := editService(t)
	config := conn.Config()
	s.CancelConnectionAttempt("early")
	if _, err := s.ConnectWithRequest(context.Background(), "early", config); !errors.Is(err, context.Canceled) {
		t.Fatalf("early cancellation lost: %v", err)
	}
	if ok, err := s.ConnectWithRequest(context.Background(), "completed", config); !ok || err != nil {
		t.Fatalf("connect: %v %v", ok, err)
	}
	if !s.CancelConnectionAttempt("completed") {
		t.Fatal("late cancellation lost")
	}
	if _, ok := s.manager.Active(); ok {
		t.Fatal("late cancelled connection remained active")
	}
	if ok, err := s.ConnectWithRequest(context.Background(), "newer", config); !ok || err != nil {
		t.Fatalf("reconnect: %v %v", ok, err)
	}
	s.CancelConnectionAttempt("completed")
	if _, ok := s.manager.Active(); !ok {
		t.Fatal("stale cancellation disconnected a newer session")
	}
}
