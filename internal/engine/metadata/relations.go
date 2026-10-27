package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"db-lens/internal/engine/entities"
)

// TableRelation is oriented from the inspected table to its related table.
// Columns and TargetColumns have matching positions, including composite keys.
type TableRelation struct {
	Name          string            `json:"name"`
	Direction     string            `json:"direction"`
	Target        entities.TableRef `json:"target"`
	Columns       []string          `json:"columns"`
	TargetColumns []string          `json:"targetColumns"`
}

type foreignKey struct {
	name                   string
	source, target         entities.TableRef
	columns, targetColumns []string
}

func InspectRelations(ctx context.Context, db *sql.DB, dialect string, table entities.TableRef) ([]TableRelation, error) {
	var keys []foreignKey
	var err error
	switch dialect {
	case "sqlite":
		keys, err = sqliteForeignKeys(ctx, db, table)
	case "pgx", "postgres":
		keys, err = catalogForeignKeys(ctx, db, table, `SELECT c.oid::text, ns.nspname, src.relname, nt.nspname, dst.relname, sa.attname, ta.attname
FROM pg_catalog.pg_constraint c
JOIN pg_catalog.pg_class src ON src.oid=c.conrelid JOIN pg_catalog.pg_namespace ns ON ns.oid=src.relnamespace
JOIN pg_catalog.pg_class dst ON dst.oid=c.confrelid JOIN pg_catalog.pg_namespace nt ON nt.oid=dst.relnamespace
CROSS JOIN LATERAL unnest(c.conkey, c.confkey) WITH ORDINALITY AS k(s,t,n)
JOIN pg_catalog.pg_attribute sa ON sa.attrelid=src.oid AND sa.attnum=k.s
JOIN pg_catalog.pg_attribute ta ON ta.attrelid=dst.oid AND ta.attnum=k.t
WHERE c.contype='f' AND ((ns.nspname=$1 AND src.relname=$2) OR (nt.nspname=$1 AND dst.relname=$2))
ORDER BY c.oid,k.n`, table.Schema, table.Name)
	case "mysql":
		keys, err = catalogForeignKeys(ctx, db, table, `SELECT CONSTRAINT_NAME,TABLE_SCHEMA,TABLE_NAME,REFERENCED_TABLE_SCHEMA,REFERENCED_TABLE_NAME,COLUMN_NAME,REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL
AND ((TABLE_SCHEMA=? AND TABLE_NAME=?) OR (REFERENCED_TABLE_SCHEMA=? AND REFERENCED_TABLE_NAME=?))
ORDER BY TABLE_SCHEMA,TABLE_NAME,CONSTRAINT_NAME,ORDINAL_POSITION`, table.Database, table.Name, table.Database, table.Name)
	default:
		return nil, fmt.Errorf("foreign keys are unavailable for %s", dialect)
	}
	if err != nil {
		return nil, err
	}
	same := func(a, b entities.TableRef) bool {
		if dialect == "mysql" {
			return a.Name == b.Name && a.Database == b.Database
		}
		return a.Name == b.Name && a.Schema == b.Schema
	}
	result := []TableRelation{}
	for _, key := range keys {
		if same(key.source, table) {
			result = append(result, TableRelation{key.name, "Referenced row", key.target, key.columns, key.targetColumns})
		}
		if same(key.target, table) {
			result = append(result, TableRelation{key.name, "Referencing rows", key.source, key.targetColumns, key.columns})
		}
	}
	return result, nil
}

func catalogForeignKeys(ctx context.Context, db *sql.DB, table entities.TableRef, query string, args ...any) ([]foreignKey, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []foreignKey{}
	for rows.Next() {
		var name, ss, sn, ts, tn, sc, tc string
		if err := rows.Scan(&name, &ss, &sn, &ts, &tn, &sc, &tc); err != nil {
			return nil, err
		}
		// PostgreSQL schemas and MySQL databases both retain their namespace here.
		source := entities.TableRef{ConnectionID: table.ConnectionID, Schema: ss, Database: table.Database, Name: sn}
		target := entities.TableRef{ConnectionID: table.ConnectionID, Schema: ts, Database: table.Database, Name: tn}
		if strings.Contains(query, "information_schema.KEY_COLUMN_USAGE") {
			source.Database = ss
			source.Schema = ""
			target.Database = ts
			target.Schema = ""
		}
		if len(keys) == 0 || keys[len(keys)-1].name != name || keys[len(keys)-1].source != source {
			keys = append(keys, foreignKey{name: name, source: source, target: target})
		}
		key := &keys[len(keys)-1]
		key.columns = append(key.columns, sc)
		key.targetColumns = append(key.targetColumns, tc)
	}
	return keys, rows.Err()
}

func sqliteForeignKeys(ctx context.Context, db *sql.DB, table entities.TableRef) ([]foreignKey, error) {
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	schema := table.Schema
	if schema == "" {
		schema = "main"
	}
	rows, err := db.QueryContext(ctx, `SELECT name FROM `+quote(schema)+`.sqlite_schema WHERE type='table' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	keys := []foreignKey{}
	for _, name := range names {
		rows, err := db.QueryContext(ctx, `PRAGMA `+quote(schema)+`.foreign_key_list(`+quote(name)+`)`)
		if err != nil {
			return nil, err
		}
		grouped := map[int]*foreignKey{}
		for rows.Next() {
			var id, seq int
			var target, column, update, delete, match string
			var targetColumn sql.NullString
			if err = rows.Scan(&id, &seq, &target, &column, &targetColumn, &update, &delete, &match); err != nil {
				rows.Close()
				return nil, err
			}
			key := grouped[id]
			if key == nil {
				source := table
				source.Name = name
				dest := table
				dest.Name = target
				key = &foreignKey{name: fmt.Sprintf("%s_fk_%d", name, id), source: source, target: dest}
				grouped[id] = key
			}
			for len(key.columns) <= seq {
				key.columns = append(key.columns, "")
				key.targetColumns = append(key.targetColumns, "")
			}
			key.columns[seq] = column
			key.targetColumns[seq] = targetColumn.String
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		ids := []int{}
		for id := range grouped {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			key := grouped[id]
			// REFERENCES parent without a column list targets the ordered primary key.
			if len(key.targetColumns) > 0 && key.targetColumns[0] == "" {
				rows, err := db.QueryContext(ctx, `PRAGMA `+quote(schema)+`.table_info(`+quote(key.target.Name)+`)`)
				if err != nil {
					return nil, err
				}
				for rows.Next() {
					var cid, notnull, pk int
					var n, t string
					var def any
					if err = rows.Scan(&cid, &n, &t, &notnull, &def, &pk); err != nil {
						rows.Close()
						return nil, err
					}
					if pk > 0 && pk <= len(key.targetColumns) {
						key.targetColumns[pk-1] = n
					}
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					return nil, err
				}
			}
			keys = append(keys, *key)
		}
	}
	return keys, nil
}
