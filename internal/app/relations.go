package app

import (
	"context"
	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/metadata"
	"fmt"
	"time"
)

func (s *DbService) InspectTableRelations(ctx context.Context, table entities.TableRef) ([]metadata.TableRelation, error) {
	conn, ok := s.manager.Active()
	if !ok || conn.ID() != table.ConnectionID {
		return nil, fmt.Errorf("the table belongs to a different or disconnected connection")
	}
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("foreign-key navigation requires a SQL database")
	}
	operation, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return metadata.InspectRelations(operation, sqlConn.DB(), conn.Type(), table)
}
