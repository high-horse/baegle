package pgxInspector

import (
	"context"
	"database/sql"
	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"fmt"
)

type PostgresInspector struct{}

func NewInspector() *PostgresInspector {
	return &PostgresInspector{}
}

func (i *PostgresInspector) ListDatabases(ctx context.Context, conn manager.Connection) ([]entities.DatabaseInfo, error) {
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("connection is not sql")
	}

	q := `
		SELECT datname
		FROM pg_database
		where datistemplate = false
		order by datname
	`
	rows, err := sqlConn.DB().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []entities.DatabaseInfo

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, entities.DatabaseInfo{Name: name})
	}
	return databases, rows.Err()
}

func (i *PostgresInspector) ListTables(ctx context.Context, conn manager.Connection) ([]entities.InspectTableInfo, error) {
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("connection is not sql")
	}
	// Catalog metadata remains visible even when data access has not been granted.
	// Do not use information_schema here: it filters objects by current privileges.
	query := `
  SELECT c.relname,
   CASE c.relkind
    WHEN 'v' THEN 'VIEW'
    WHEN 'f' THEN 'FOREIGN'
    ELSE 'TABLE'
   END,
   current_database(), n.nspname,
   GREATEST(c.reltuples::bigint, 0),
   pg_catalog.obj_description(c.oid, 'pg_class'), NULL, NULL
  FROM pg_catalog.pg_class c
  JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
  WHERE c.relkind IN ('r', 'p', 'v', 'f')
   AND n.nspname <> 'information_schema'
   AND n.nspname !~ '^pg_'
  ORDER BY n.nspname, c.relname
 `
	rows, err := sqlConn.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []entities.InspectTableInfo

	for rows.Next() {

		var (
			table entities.InspectTableInfo

			rowsCnt sql.NullInt64
			comment sql.NullString
			created sql.NullTime
			updated sql.NullTime
		)

		err := rows.Scan(
			&table.Name,
			&table.Type,
			&table.Database,
			&table.Schema,
			&rowsCnt,
			&comment,
			&created,
			&updated,
		)
		if err != nil {
			return nil, err
		}

		if rowsCnt.Valid {
			table.Rows = rowsCnt.Int64
		}

		if comment.Valid {
			table.Comment = comment.String
		}

		if created.Valid {
			table.CreatedAt = &created.Time
		}

		if updated.Valid {
			table.UpdatedAt = &updated.Time
		}

		// PostgreSQL doesn't have a storage engine.
		table.Engine = ""

		tables = append(tables, table)
	}

	return tables, rows.Err()
}

func (i *PostgresInspector) ListColumns(
	ctx context.Context,
	conn manager.Connection,
	table entities.InspectTableInfo,
) ([]entities.InspectColumnInfo, error) {

	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("connection is not SQL")
	}

	query := `
  SELECT a.attname,
   pg_catalog.format_type(a.atttypid, a.atttypmod),
   CASE WHEN a.attnotnull OR t.typnotnull THEN 'NO' ELSE 'YES' END,
   pg_catalog.pg_get_expr(d.adbin, d.adrelid),
   CASE WHEN a.attidentity <> '' THEN 'YES' ELSE 'NO' END,
   CASE WHEN a.attgenerated <> '' THEN 'ALWAYS' ELSE 'NEVER' END,
   CASE WHEN EXISTS (
    SELECT 1 FROM pg_catalog.pg_constraint k
    WHERE k.conrelid = c.oid AND k.contype = 'p'
     AND a.attnum = ANY(k.conkey)
   ) THEN 'PRIMARY KEY' ELSE '' END
  FROM pg_catalog.pg_attribute a
  JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
  JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
  JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
  LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
  WHERE current_database() = $1 AND n.nspname = $2 AND c.relname = $3
   AND a.attnum > 0 AND NOT a.attisdropped
  ORDER BY a.attnum
 `
	rows, err := sqlConn.DB().QueryContext(
		ctx,
		query,
		table.Database,
		table.Schema,
		table.Name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []entities.InspectColumnInfo

	for rows.Next() {

		var (
			col entities.InspectColumnInfo

			isNullable          string
			columnKey           string
			identity, generated string

			defaultValue sql.NullString
		)

		err := rows.Scan(
			&col.Name,
			&col.DatabaseType,
			&isNullable,
			&defaultValue,
			&identity,
			&generated,
			&columnKey,
		)
		if err != nil {
			return nil, err
		}

		col.Nullable = isNullable == "YES"

		if defaultValue.Valid {
			col.DefaultValue = defaultValue.String
		}

		col.PrimaryKey = columnKey == "PRIMARY KEY"
		col.Generated = generated != "NEVER"
		col.AutoIncrement = identity == "YES"

		if defaultValue.Valid {
			// PostgreSQL serial/bigserial/identity columns
			col.AutoIncrement = col.AutoIncrement ||
				len(defaultValue.String) > 7 &&
					defaultValue.String[:7] == "nextval"
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}
