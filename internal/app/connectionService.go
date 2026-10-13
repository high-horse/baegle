package app

import (
	"db-lens/internal/db"
	"db-lens/internal/engine/entities"
	"db-lens/internal/types"
)

type DatabaseService struct{}

func NewDatabaseService() *DatabaseService {
	return &DatabaseService{}
}

func (s *DatabaseService) GetConnections() ([]types.Connection, error) {
	return db.GetConnectionList()
}

// func (s *DatabaseService) CreateConnection(connection types.Connection) error {
// 	return db.CreateConnection(connection)
// }

func (s *DatabaseService) DeleteConnection(id int) error {
	return db.DeleteConnection(id)
}

func (s *DatabaseService) UpdateConnection(config entities.ConnectionConfig) error {
	return db.UpdateConnection(config)
}

func (s *DatabaseService) SetConnectionGroup(id int, group string) error {
	return db.SetConnectionGroup(id, group)
}
