package enterprise

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/tableedit"
)

type Inspector struct{ dialect string }

func New(dialect string) *Inspector { return &Inspector{dialect: dialect} }
func database(conn manager.Connection) (*sql.DB, error) {
	c, ok := conn.(manager.SQLConnection)
	if !ok || c.DB() == nil {
		return nil, fmt.Errorf("SQL connection unavailable")
	}
	return c.DB(), nil
}
func (i *Inspector) ListDatabases(_ context.Context, conn manager.Connection) ([]entities.DatabaseInfo, error) {
	return []entities.DatabaseInfo{{Name: conn.DatabaseName()}}, nil
}

func (i *Inspector) ListTables(ctx context.Context, conn manager.Connection) ([]entities.InspectTableInfo, error) {
	db, err := database(conn)
	if err != nil {
		return nil, err
	}
	query := ""
	switch i.dialect {
	case "oracle":
		query = `SELECT owner,table_name,'TABLE' FROM all_tables WHERE owner NOT IN ('SYS','SYSTEM','XDB','MDSYS','CTXSYS','ORDSYS','WMSYS','DBSNMP','OUTLN') UNION ALL SELECT owner,view_name,'VIEW' FROM all_views WHERE owner NOT IN ('SYS','SYSTEM','XDB','MDSYS','CTXSYS','ORDSYS','WMSYS','DBSNMP','OUTLN') ORDER BY 1,2`
	case "sqlserver":
		query = `SELECT s.name,o.name,CASE o.type WHEN 'V' THEN 'VIEW' ELSE 'TABLE' END FROM sys.objects o JOIN sys.schemas s ON s.schema_id=o.schema_id WHERE o.type IN ('U','V') AND o.is_ms_shipped=0 ORDER BY s.name,o.name`
	case "db2":
		query = `SELECT RTRIM(TABSCHEMA),TABNAME,CASE TYPE WHEN 'V' THEN 'VIEW' ELSE 'TABLE' END FROM SYSCAT.TABLES WHERE TYPE IN ('T','V') AND TABSCHEMA NOT LIKE 'SYS%' ORDER BY TABSCHEMA,TABNAME`
	default:
		return nil, fmt.Errorf("unknown database dialect")
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := []entities.InspectTableInfo{}
	for rows.Next() {
		var t entities.InspectTableInfo
		t.Database = conn.DatabaseName()
		if err = rows.Scan(&t.Schema, &t.Name, &t.Type); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (i *Inspector) ListColumns(ctx context.Context, conn manager.Connection, table entities.InspectTableInfo) ([]entities.InspectColumnInfo, error) {
	db, err := database(conn)
	if err != nil {
		return nil, err
	}
	query := ""
	switch i.dialect {
	case "oracle":
		query = `SELECT c.column_name,c.data_type,CASE c.nullable WHEN 'Y' THEN 1 ELSE 0 END,c.data_default,
 CASE WHEN EXISTS (SELECT 1 FROM all_constraints k JOIN all_cons_columns kc ON kc.owner=k.owner AND kc.constraint_name=k.constraint_name WHERE k.constraint_type='P' AND k.owner=c.owner AND k.table_name=c.table_name AND kc.column_name=c.column_name) THEN 1 ELSE 0 END,
 CASE c.virtual_column WHEN 'YES' THEN 1 ELSE 0 END,CASE c.identity_column WHEN 'YES' THEN 1 ELSE 0 END,c.data_length
 FROM all_tab_cols c WHERE c.owner=:1 AND c.table_name=:2 AND c.hidden_column='NO' ORDER BY c.column_id`
	case "sqlserver":
		query = `SELECT c.name,t.name,CAST(c.is_nullable AS int),dc.definition,
 CASE WHEN EXISTS(SELECT 1 FROM sys.indexes ix JOIN sys.index_columns ic ON ic.object_id=ix.object_id AND ic.index_id=ix.index_id WHERE ix.is_primary_key=1 AND ic.object_id=c.object_id AND ic.column_id=c.column_id) THEN 1 ELSE 0 END,
 CASE WHEN c.is_computed=1 OR c.generated_always_type<>0 OR t.name IN ('timestamp','rowversion') THEN 1 ELSE 0 END,CAST(c.is_identity AS int),CAST(c.max_length AS bigint)
 FROM sys.columns c JOIN sys.types t ON t.user_type_id=c.user_type_id JOIN sys.objects o ON o.object_id=c.object_id JOIN sys.schemas s ON s.schema_id=o.schema_id LEFT JOIN sys.default_constraints dc ON dc.object_id=c.default_object_id
 WHERE s.name=@p1 AND o.name=@p2 ORDER BY c.column_id`
	case "db2":
		query = `SELECT c.COLNAME,c.TYPENAME,CASE c.NULLS WHEN 'Y' THEN 1 ELSE 0 END,c.DEFAULT,
 CASE WHEN EXISTS(SELECT 1 FROM SYSCAT.TABCONST t JOIN SYSCAT.KEYCOLUSE k ON k.TABSCHEMA=t.TABSCHEMA AND k.TABNAME=t.TABNAME AND k.CONSTNAME=t.CONSTNAME WHERE t.TYPE='P' AND t.TABSCHEMA=c.TABSCHEMA AND t.TABNAME=c.TABNAME AND k.COLNAME=c.COLNAME) THEN 1 ELSE 0 END,
 CASE WHEN c.GENERATED='A' AND c.IDENTITY<>'Y' THEN 1 ELSE 0 END,CASE c.IDENTITY WHEN 'Y' THEN 1 ELSE 0 END,BIGINT(c.LENGTH)
 FROM SYSCAT.COLUMNS c WHERE c.TABSCHEMA=? AND c.TABNAME=? ORDER BY c.COLNO`
	}
	rows, err := db.QueryContext(ctx, query, table.Schema, table.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := []entities.InspectColumnInfo{}
	for rows.Next() {
		var c entities.InspectColumnInfo
		var nullable, pk, generated, identity int
		var def sql.NullString
		if err = rows.Scan(&c.Name, &c.DatabaseType, &nullable, &def, &pk, &generated, &identity, &c.Length); err != nil {
			return nil, err
		}
		c.Nullable = nullable != 0
		c.PrimaryKey = pk != 0
		c.Generated = generated != 0
		c.AutoIncrement = identity != 0
		if def.Valid {
			c.DefaultValue = def.String
		}
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

func (i *Inspector) GetTableDDL(ctx context.Context, conn manager.Connection, table entities.TableRef) (string, error) {
	db, err := database(conn)
	if err != nil {
		return "", err
	}
	if i.dialect == "oracle" {
		var kind string
		if err = db.QueryRowContext(ctx, `SELECT object_type FROM all_objects WHERE owner=:1 AND object_name=:2 AND object_type IN ('TABLE','VIEW')`, table.Schema, table.Name).Scan(&kind); err != nil {
			return "", err
		}
		var ddl string
		err = db.QueryRowContext(ctx, `SELECT DBMS_METADATA.GET_DDL(:1,:2,:3) FROM dual`, kind, table.Name, table.Schema).Scan(&ddl)
		return ddl, err
	}
	if i.dialect == "sqlserver" {
		var kind string
		var definition sql.NullString
		err = db.QueryRowContext(ctx, `SELECT o.type,OBJECT_DEFINITION(o.object_id) FROM sys.objects o JOIN sys.schemas s ON s.schema_id=o.schema_id WHERE s.name=@p1 AND o.name=@p2 AND o.type IN ('U','V')`, table.Schema, table.Name).Scan(&kind, &definition)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(kind) == "V" {
			if !definition.Valid {
				return "", fmt.Errorf("view definition unavailable; VIEW DEFINITION permission may be required")
			}
			return definition.String, nil
		}
	} else {
		var kind string
		if err = db.QueryRowContext(ctx, `SELECT TYPE FROM SYSCAT.TABLES WHERE TABSCHEMA=? AND TABNAME=?`, table.Schema, table.Name).Scan(&kind); err != nil {
			return "", err
		}
		if kind == "V" {
			var ddl string
			err = db.QueryRowContext(ctx, `SELECT TEXT FROM SYSCAT.VIEWS WHERE VIEWSCHEMA=? AND VIEWNAME=?`, table.Schema, table.Name).Scan(&ddl)
			return ddl, err
		}
	}
	// SQL Server and Db2 have no equivalent of SHOW CREATE TABLE. Describe the
	// column definitions explicitly; mark the limits instead of implying a full backup.
	var query string
	if i.dialect == "sqlserver" {
		query = `SELECT QUOTENAME(c.name)+' '+CASE WHEN c.is_computed=1 THEN 'AS '+cc.definition+CASE WHEN cc.is_persisted=1 THEN ' PERSISTED' ELSE '' END ELSE
  QUOTENAME(t.name)+CASE WHEN t.name IN ('varchar','char','varbinary','binary') THEN '('+CASE WHEN c.max_length=-1 THEN 'MAX' ELSE CONVERT(varchar(10),c.max_length) END+')' WHEN t.name IN ('nvarchar','nchar') THEN '('+CASE WHEN c.max_length=-1 THEN 'MAX' ELSE CONVERT(varchar(10),c.max_length/2) END+')' WHEN t.name IN ('decimal','numeric') THEN '('+CONVERT(varchar(10),c.precision)+','+CONVERT(varchar(10),c.scale)+')' WHEN t.name IN ('datetime2','datetimeoffset','time') THEN '('+CONVERT(varchar(10),c.scale)+')' ELSE '' END
  +CASE WHEN c.is_identity=1 THEN ' IDENTITY('+CONVERT(varchar(40),ic.seed_value)+','+CONVERT(varchar(40),ic.increment_value)+')' ELSE '' END
  +CASE c.is_nullable WHEN 1 THEN ' NULL' ELSE ' NOT NULL' END+COALESCE(' DEFAULT '+dc.definition,'') END
  FROM sys.columns c JOIN sys.types t ON t.user_type_id=c.user_type_id JOIN sys.objects o ON o.object_id=c.object_id JOIN sys.schemas s ON s.schema_id=o.schema_id LEFT JOIN sys.default_constraints dc ON dc.object_id=c.default_object_id LEFT JOIN sys.computed_columns cc ON cc.object_id=c.object_id AND cc.column_id=c.column_id LEFT JOIN sys.identity_columns ic ON ic.object_id=c.object_id AND ic.column_id=c.column_id
  WHERE s.name=@p1 AND o.name=@p2 ORDER BY c.column_id`
	} else {
		query = `SELECT '"'||REPLACE(COLNAME,'"','""')||'" '||TYPENAME||CASE WHEN TYPENAME IN ('CHARACTER','VARCHAR','BINARY','VARBINARY') THEN '('||RTRIM(CHAR(LENGTH))||')' WHEN TYPENAME IN ('DECIMAL') THEN '('||RTRIM(CHAR(LENGTH))||','||RTRIM(CHAR(SCALE))||')' ELSE '' END||CASE WHEN IDENTITY='Y' THEN ' GENERATED ALWAYS AS IDENTITY' WHEN GENERATED='A' THEN ' GENERATED ALWAYS AS ('||COALESCE(TEXT,'')||')' ELSE COALESCE(' DEFAULT '||DEFAULT,'') END||CASE NULLS WHEN 'N' THEN ' NOT NULL' ELSE '' END FROM SYSCAT.COLUMNS WHERE TABSCHEMA=? AND TABNAME=? ORDER BY COLNO`
	}
	rows, err := db.QueryContext(ctx, query, table.Schema, table.Name)
	if err != nil {
		return "", err
	}
	lines := []string{}
	for rows.Next() {
		var line string
		if err = rows.Scan(&line); err != nil {
			rows.Close()
			return "", err
		}
		lines = append(lines, "    "+line)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("table has no visible columns")
	}
	return "-- Column structure only: indexes, keys, checks, triggers and storage options are not included.\n-- Review generated/identity columns and database-specific options before recreating.\nCREATE TABLE " + tableedit.TableName(table, i.dialect) + " (\n" + strings.Join(lines, ",\n") + "\n);", nil
}
