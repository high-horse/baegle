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
	if consoleID == "" || requestID == "" {
		return nil, fmt.Errorf("console and request IDs are required")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	operation := &consoleQuery{requestID: requestID, cancel: cancel}
	s.consoleMu.Lock()
	if s.consoleQueries == nil {
		s.consoleQueries = make(map[string]*consoleQuery)
	}
	if s.consoleQueries[consoleID] != nil {
		s.consoleMu.Unlock()
		return nil, fmt.Errorf("a query is already running in this console")
	}
	s.consoleQueries[consoleID] = operation
	s.consoleMu.Unlock()
	defer func() {
		s.consoleMu.Lock()
		delete(s.consoleQueries, consoleID)
		s.consoleMu.Unlock()
	}()

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
