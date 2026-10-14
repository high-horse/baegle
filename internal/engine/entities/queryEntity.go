package entities

import "time"

type ColumnInfo struct {
	Name         string `json:"name"`
	DatabaseType string `json:"databaseType"`
	Nullable     bool   `json:"nullable"`
	DefaultValue string `json:"defaultValue"`
}

type QueryExecutionType int

const (
	QueryExecutionExecute QueryExecutionType = iota
	QueryExecutionFetchPaged
	QueryExecuteRefresh
	QueryExecutionClose
	QueryExecutionNavigate
	QueryExecutionCount
)

type QueryInput struct {
	Query         string             `json:"query"`
	Cursor        string             `json:"cursor"`
	Type          QueryExecutionType `json:"type"`
	PageSize      int                `json:"pageSize"`
	Page          int                `json:"page"`
	SortColumn    int                `json:"sortColumn,omitempty"`
	SortDirection string             `json:"sortDirection,omitempty"`
}

type QueryResult struct {
	Columns      []ColumnInfo    `json:"columns"`
	Documents    []string        `json:"documents,omitempty"`
	Rows         [][]interface{} `json:"rows"`
	RowsAffected int64           `json:"rowsAffected"`
	LastInsertId int64           `json:"lastInsertId"`
	Duration     time.Duration   `json:"duration"`
	IsQuery      bool            `json:"isQuery"`
	Cursor       string          `json:"cursor"`
	HasMore      bool            `json:"hasMore"`
	StartRow     int64           `json:"startRow"`
	PageSize     int             `json:"pageSize"`
	TotalRows    *int64          `json:"totalRows,omitempty"`
	CanNavigate  bool            `json:"canNavigate"`
}

type TableInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // TABLE, VIEW
	Database string `json:"database"`
}

type Column struct {
	Name         string
	Type         string
	DefaultValue string
}

type SQLQueryEntity struct {
	RawSQL        string
	StatementType StatementType
	Dialect       SQLDialect
}

type StatementType string

const (
	StatementSelect  StatementType = "select"
	StatementInsert  StatementType = "insert"
	StatementUpdate  StatementType = "update"
	StatementDelete  StatementType = "delete"
	StatementCreate  StatementType = "create"
	StatementAlter   StatementType = "alter"
	StatementDrop    StatementType = "drop"
	StatementUnknown StatementType = "unknown"
)

type SQLDialect string

const (
	DialectPostgreSQL SQLDialect = "pgx"
	DialectMySQL      SQLDialect = "mysql"
	DialectSQLite     SQLDialect = "sqlite"
)
