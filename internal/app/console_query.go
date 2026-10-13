package app

import (
	"context"
	"db-lens/internal/engine/entities"
	"fmt"
)

type consoleQuery struct {
	requestID string
	cancel    context.CancelFunc
}

// ExecuteConsoleQuery keeps one operation per console until the driver returns,
// including while cancellation is in progress. Different consoles are independent.
func (s *DbService) ExecuteConsoleQuery(ctx context.Context, consoleID, requestID string, input entities.QueryInput) (*entities.QueryResult, error) {
	ctx, finish, err := s.beginConsoleQuery(ctx, consoleID, requestID)
	if err != nil {
		return nil, err
	}
	defer finish()
	result, err := s.ExecuteQuery(ctx, input)
	if ctx.Err() != nil {
		if result != nil && result.Cursor != "" {
			if conn, ok := s.manager.Active(); ok {
				s.pager.Close(result.Cursor, conn.ID())
				s.mongoPager.Close(result.Cursor, conn.ID())
			}
		}
		return nil, ctx.Err()
	}
	return result, err
}

func (s *DbService) beginConsoleQuery(ctx context.Context, consoleID, requestID string) (context.Context, func(), error) {
	if consoleID == "" || requestID == "" {
		return nil, nil, fmt.Errorf("console and request IDs are required")
	}
	ctx, cancel := context.WithCancel(ctx)
	operation := &consoleQuery{requestID: requestID, cancel: cancel}
	s.consoleMu.Lock()
	if s.consoleQueries == nil {
		s.consoleQueries = make(map[string]*consoleQuery)
	}
	if s.consoleQueries[consoleID] != nil {
		s.consoleMu.Unlock()
		cancel()
		return nil, nil, fmt.Errorf("a query is already running in this console")
	}
	s.consoleQueries[consoleID] = operation
	s.consoleMu.Unlock()
	return ctx, func() {
		cancel()
		s.consoleMu.Lock()
		delete(s.consoleQueries, consoleID)
		s.consoleMu.Unlock()
	}, nil
}

// CancelConsoleQuery targets only this execution, so a delayed Stop cannot
// cancel a newer query in the same console. The execution call acknowledges completion.
func (s *DbService) CancelConsoleQuery(consoleID, requestID string) bool {
	s.consoleMu.Lock()
	defer s.consoleMu.Unlock()
	query := s.consoleQueries[consoleID]
	if query == nil || query.requestID != requestID {
		return false
	}
	query.cancel()
	return true
}
