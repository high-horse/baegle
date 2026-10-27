package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/tableedit"
)

func (s *DbService) DescribeTableEdit(ctx context.Context, table entities.TableRef) (*entities.TableEditInfo, error) {
	conn, ok := s.manager.Active()
	if !ok || conn.ID() != table.ConnectionID {
		return nil, fmt.Errorf("the table belongs to a different or disconnected connection")
	}
	driver, err := s.factory.Driver(conn.Type())
	if err != nil {
		return nil, err
	}
	tables, err := driver.Inspector().ListTables(ctx, conn)
	if err != nil {
		return nil, err
	}
	var found *entities.InspectTableInfo
	for _, candidate := range tables {
		if candidate.Name == table.Name && candidate.Schema == table.Schema && candidate.Database == table.Database {
			copy := candidate
			found = &copy
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("table not found")
	}
	info := &entities.TableEditInfo{Table: table, Driver: conn.Type(), Columns: []entities.InspectColumnInfo{}, Keys: []string{}}
	if conn.Type() == "cassandra" || conn.Type() == "couchdb" || conn.Type() == "redis" {
		info.Reason = "Use the native query console to modify data in this database"
		return info, nil
	}
	if conn.Config().ReadOnly {
		info.Reason = "This connection is read-only"
		return info, nil
	}
	if strings.EqualFold(found.Type, "VIEW") {
		info.Reason = "Views are read-only in the data editor"
		return info, nil
	}
	if conn.Type() == "mongodb" {
		info.Keys = []string{"_id"}
		info.CanInsert = true
		info.CanModify = true
		return info, nil
	}
	info.Columns, err = driver.Inspector().ListColumns(ctx, conn, *found)
	if err != nil {
		return nil, err
	}
	for _, column := range info.Columns {
		if column.PrimaryKey {
			info.Keys = append(info.Keys, column.Name)
		}
	}
	info.CanInsert = true
	info.CanModify = len(info.Keys) > 0
	if !info.CanModify {
		info.Reason = "No primary key: existing rows cannot be safely edited or deleted"
	}
	return info, nil
}

func (s *DbService) SaveTableChanges(ctx context.Context, input entities.TableChanges) (*entities.TableSaveResult, error) {
	if len(input.Changes) == 0 || len(input.Changes) > 500 {
		return nil, fmt.Errorf("save between 1 and 500 row changes at a time")
	}
	operation, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	info, err := s.DescribeTableEdit(operation, input.Table)
	if err != nil {
		return nil, err
	}
	if !info.CanInsert {
		return nil, fmt.Errorf("%s", info.Reason)
	}
	conn, ok := s.manager.Active()
	if !ok || conn.ID() != input.Table.ConnectionID {
		return nil, fmt.Errorf("active connection changed")
	}
	s.pager.Close(input.Cursor, conn.ID())
	s.mongoPager.Close(input.Cursor, conn.ID())
	if mongoConn, ok := conn.(manager.NoSQLConnection); ok {
		result := tableedit.SaveMongo(operation, mongoConn.DB().Collection(input.Table.Name), input.Changes)
		return &result, nil
	}
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("unsupported database")
	}
	if err := tableedit.SaveSQL(operation, sqlConn.DB(), *info, input.Changes); err != nil {
		return nil, err
	}
	return &entities.TableSaveResult{Applied: len(input.Changes)}, nil
}
