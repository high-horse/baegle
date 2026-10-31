package app

import (
	"context"
	"fmt"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
)

type connectionAttempt struct {
	id         string
	cancel     context.CancelFunc
	cancelled  bool
	connection manager.Connection
}

// ConnectWithRequest performs a single handshake and keeps cancellation tied to
// this request, so a delayed Cancel cannot disconnect a newer session.
func (s *DbService) ConnectWithRequest(ctx context.Context, requestID string, config entities.ConnectionConfig) (bool, error) {
	if requestID == "" {
		return false, fmt.Errorf("a connection request ID is required")
	}
	if !s.connectMu.TryLock() {
		return false, fmt.Errorf("a connection attempt is already running")
	}
	defer s.connectMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	attempt := &connectionAttempt{id: requestID, cancel: cancel}
	s.connectionAttemptMu.Lock()
	if s.cancelledConnectionAttempt == requestID {
		attempt.cancelled = true
		cancel()
	}
	s.connectionAttempt = attempt
	s.connectionAttemptMu.Unlock()
	connected, err := s.connect(ctx, config)
	s.connectionAttemptMu.Lock()
	defer s.connectionAttemptMu.Unlock()
	if connected {
		attempt.connection, _ = s.manager.Active()
	}
	if attempt.cancelled || ctx.Err() != nil {
		if attempt.connection != nil {
			_ = s.manager.Remove(attempt.connection.ID())
			attempt.connection = nil
		}
		connected = false
		err = ctx.Err()
		if err == nil {
			err = context.Canceled
		}
	}
	return connected, err
}

// CancelConnectionAttempt also handles cancellation arriving immediately before
// registration or just after the handshake completed but before the UI received it.
func (s *DbService) CancelConnectionAttempt(requestID string) bool {
	if requestID == "" {
		return false
	}
	s.connectionAttemptMu.Lock()
	s.cancelledConnectionAttempt = requestID
	attempt := s.connectionAttempt
	if attempt == nil || attempt.id != requestID {
		s.connectionAttemptMu.Unlock()
		return false
	}
	attempt.cancelled = true
	attempt.cancel()
	completedConnection := attempt.connection
	s.connectionAttemptMu.Unlock()
	if completedConnection != nil {
		s.connectMu.Lock()
		defer s.connectMu.Unlock()
		if current, ok := s.manager.Get(completedConnection.ID()); ok && current == completedConnection {
			_ = s.manager.Remove(current.ID())
		}
	}
	return true
}
