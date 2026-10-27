package tableedit

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"db-lens/internal/engine/entities"
)

func Quote(name, dialect string) string {
	if dialect == "sqlserver" {
		return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
	}
	quote := `"`
	if dialect == "mysql" {
		quote = "`"
	}
	return quote + strings.ReplaceAll(name, quote, quote+quote) + quote
}
func TableName(table entities.TableRef, dialect string) string {
	schema := table.Schema
	if dialect == "mysql" {
		schema = table.Database
	}
	if schema == "" {
		return Quote(table.Name, dialect)
	}
	return Quote(schema, dialect) + "." + Quote(table.Name, dialect)
}
func decodeValue(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	switch value := value.(type) {
	case json.Number:
		return value.String(), nil // Driver/server converts without JS floating-point loss.
	case map[string]any, []any:
		return string(raw), nil
	default:
		return value, nil
	}
}

func BuildSQL(info entities.TableEditInfo, change entities.RowChange) (string, []any, error) {
	allowed := map[string]bool{}
	for _, column := range info.Columns {
		identity := column.PrimaryKey || strings.EqualFold(column.Name, "id")
		for _, key := range info.Keys {
			if key == column.Name {
				identity = true
			}
		}
		allowed[column.Name] = !column.Generated && !(change.Operation == "update" && identity) && !(change.Operation == "insert" && column.AutoIncrement)
	}
	columns := make([]string, 0, len(change.Values))
	for name := range change.Values {
		if writable, exists := allowed[name]; !exists || !writable {
			return "", nil, fmt.Errorf("column %q is not writable", name)
		}
		columns = append(columns, name)
	}
	sort.Strings(columns)
	args := []any{}
	bind := func(raw json.RawMessage) (string, error) {
		value, err := decodeValue(raw)
		if err != nil {
			return "", err
		}
		args = append(args, value)
		if info.Driver == "pgx" {
			return fmt.Sprintf("$%d", len(args)), nil
		}
		if info.Driver == "oracle" {
			return fmt.Sprintf(":%d", len(args)), nil
		}
		if info.Driver == "sqlserver" {
			return fmt.Sprintf("@p%d", len(args)), nil
		}
		return "?", nil
	}
	table := TableName(info.Table, info.Driver)
	var statement string
	switch change.Operation {
	case "insert":
		if !info.CanInsert {
			return "", nil, fmt.Errorf("this table does not allow inserts")
		}
		if len(columns) == 0 {
			if info.Driver == "oracle" || info.Driver == "db2" {
				if len(info.Columns) == 0 {
					return "", nil, fmt.Errorf("table has no columns")
				}
				for _, column := range info.Columns {
					if !column.Generated {
						return "INSERT INTO " + table + " (" + Quote(column.Name, info.Driver) + ") VALUES (DEFAULT)", args, nil
					}
				}
				return "", nil, fmt.Errorf("table has no defaultable columns")
			}
			if info.Driver == "mysql" {
				return "INSERT INTO " + table + " () VALUES ()", args, nil
			}
			return "INSERT INTO " + table + " DEFAULT VALUES", args, nil
		}
		names, parameters := []string{}, []string{}
		for _, name := range columns {
			parameter, err := bind(change.Values[name])
			if err != nil {
				return "", nil, err
			}
			names = append(names, Quote(name, info.Driver))
			parameters = append(parameters, parameter)
		}
		return "INSERT INTO " + table + " (" + strings.Join(names, ",") + ") VALUES (" + strings.Join(parameters, ",") + ")", args, nil
	case "update":
		if len(columns) == 0 {
			return "", nil, fmt.Errorf("no changed values")
		}
		assignments := []string{}
		for _, name := range columns {
			parameter, err := bind(change.Values[name])
			if err != nil {
				return "", nil, err
			}
			assignments = append(assignments, Quote(name, info.Driver)+" = "+parameter)
		}
		statement = "UPDATE " + table + " SET " + strings.Join(assignments, ",")
	case "delete":
		statement = "DELETE FROM " + table
	default:
		return "", nil, fmt.Errorf("unknown row operation")
	}
	if !info.CanModify || len(info.Keys) == 0 {
		return "", nil, fmt.Errorf("a primary key is required to edit or delete rows")
	}
	if len(change.Keys) != len(info.Keys) {
		return "", nil, fmt.Errorf("all primary key values are required")
	}
	conditions := []string{}
	for _, key := range info.Keys {
		raw, ok := change.Keys[key]
		if !ok {
			return "", nil, fmt.Errorf("missing primary key %s", key)
		}
		value, err := decodeValue(raw)
		if err != nil {
			return "", nil, err
		}
		if value == nil {
			return "", nil, fmt.Errorf("primary key %s cannot be null", key)
		}
		parameter, err := bind(raw)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, Quote(key, info.Driver)+" = "+parameter)
	}
	return statement + " WHERE " + strings.Join(conditions, " AND "), args, nil
}

// Commit the whole SQL batch or leave every row unchanged.
func SaveSQL(ctx context.Context, database *sql.DB, info entities.TableEditInfo, changes []entities.RowChange) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, change := range changes {
		query, args, err := BuildSQL(info, change)
		if err != nil {
			return fmt.Errorf("change %d: %w", i+1, err)
		}
		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("change %d: %w", i+1, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("change %d affected %d rows; reload the table before retrying", i+1, affected)
		}
	}
	return tx.Commit()
}
