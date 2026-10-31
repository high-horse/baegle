package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"db-lens/internal/db"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	manager "db-lens/internal/engine/connectionManager"
	"db-lens/internal/engine/datatransfer"
	"db-lens/internal/engine/entities"
	"db-lens/internal/engine/queryExecutor/sqlExecutor"
	"db-lens/internal/engine/tableedit"
	"github.com/wailsapp/wails/v3/pkg/application"
	"go.mongodb.org/mongo-driver/bson"
)

type ImportPreview struct {
	SHA     string     `json:"sha"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	Total   int        `json:"total"`
	Sheets  []string   `json:"sheets"`
}
type ImportRequest struct {
	Path    string               `json:"path"`
	SHA     string               `json:"sha"`
	Options datatransfer.Options `json:"options"`
	Table   entities.TableRef    `json:"table"`
	Mapping map[string]string    `json:"mapping"`
}
type ExportRequest struct {
	ExcludedColumns []string             `json:"excludedColumns"`
	ConnectionID    string               `json:"connectionId"`
	Scope           string               `json:"scope"`
	Options         datatransfer.Options `json:"options"`
	Table           entities.TableRef    `json:"table"`
	Query           string               `json:"query"`
	Columns         []string             `json:"columns"`
	Rows            [][]json.RawMessage  `json:"rows"`
	Documents       []string             `json:"documents"`
}
type ExportResult struct {
	Path string `json:"path"`
	Rows int    `json:"rows"`
}

func (s *DbService) ChooseImportFile() (string, error) {
	return application.Get().Dialog.OpenFile().SetTitle("Import table data").AddFilter("Data files", "*.csv;*.json;*.sql;*.xlsx").PromptForSingleSelection()
}
func readTransferFile(ctx context.Context, path string) ([]byte, error) {
	var limitMiB int64
	if db.Conn != nil {
		settings, err := db.GetResultSettings(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not load import settings: %w", err)
		}
		limitMiB = settings.ImportLimitMiB
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("select a file, not a directory or special device")
	}
	limit := limitMiB * (1 << 20)
	tooLarge := func() error {
		return fmt.Errorf("this file exceeds your %d MiB import limit; increase it in Settings or set it to 0 for unlimited", limitMiB)
	}
	if limit > 0 && stat.Size() > limit {
		return nil, tooLarge()
	}
	var reader io.Reader = file
	if limit > 0 {
		reader = io.LimitReader(file, limit+1)
	}
	var content bytes.Buffer
	buffer := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := reader.Read(buffer)
		content.Write(buffer[:n])
		if limit > 0 && int64(content.Len()) > limit {
			return nil, tooLarge()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return content.Bytes(), nil
}

type SQLScriptFile struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

// ReadSQLScript only loads text. Opening a file never executes its SQL.
func (s *DbService) ReadSQLScript(ctx context.Context, path string) (*SQLScriptFile, error) {
	content, err := readTransferFile(ctx, path)
	if err != nil {
		return nil, err
	}
	content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return nil, fmt.Errorf("SQL scripts must be UTF-8 text; convert this file to UTF-8 before opening it")
	}
	return &SQLScriptFile{Name: filepath.Base(path), SQL: string(content)}, nil
}

func readImport(ctx context.Context, path string, options datatransfer.Options, dialect string) (*datatransfer.Data, string, error) {
	content, err := readTransferFile(ctx, path)
	if err != nil {
		return nil, "", err
	}
	data, err := datatransfer.Read(content, options, dialect)
	return data, fmt.Sprintf("%x", sha256.Sum256(content)), err
}
func (s *DbService) PreviewImport(ctx context.Context, input ImportRequest) (*ImportPreview, error) {
	conn, ok := s.manager.Active()
	if !ok || conn.ID() != input.Table.ConnectionID {
		return nil, fmt.Errorf("active connection changed")
	}
	data, sha, err := readImport(ctx, input.Path, input.Options, conn.Type())
	if err != nil {
		return nil, err
	}
	p := &ImportPreview{SHA: sha, Columns: data.Columns, Total: len(data.Rows), Sheets: data.Sheets, Rows: [][]string{}}
	for i, row := range data.Rows {
		if i == 5 {
			break
		}
		cells := make([]string, len(row))
		for j, v := range row {
			if v == nil {
				cells[j] = "(missing)"
			} else {
				cells[j] = string(v)
			}
		}
		p.Rows = append(p.Rows, cells)
	}
	return p, nil
}

func (s *DbService) ImportData(ctx context.Context, input ImportRequest) (*entities.TableSaveResult, error) {
	s.transferMu.Lock()
	if s.importing {
		s.transferMu.Unlock()
		return nil, fmt.Errorf("an import is already running")
	}
	s.importing = true
	s.transferMu.Unlock()
	defer func() { s.transferMu.Lock(); s.importing = false; s.transferMu.Unlock() }()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	info, err := s.DescribeTableEdit(ctx, input.Table)
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
	data, sha, err := readImport(ctx, input.Path, input.Options, conn.Type())
	if err != nil {
		return nil, err
	}
	if sha != input.SHA {
		return nil, fmt.Errorf("the file changed; preview it again before importing")
	}
	if len(data.Rows) == 0 {
		return nil, fmt.Errorf("the file contains no rows")
	}
	seen := map[string]bool{}
	source := map[string]bool{}
	for _, c := range data.Columns {
		source[c] = true
	}
	for from, to := range input.Mapping {
		if !source[from] {
			return nil, fmt.Errorf("unknown source column %q", from)
		}
		if to == "" {
			continue
		}
		if seen[to] {
			return nil, fmt.Errorf("destination column %q is mapped more than once", to)
		}
		seen[to] = true
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("map at least one destination column")
	}
	changes := make([]entities.RowChange, len(data.Rows))
	for i, row := range data.Rows {
		values := map[string]json.RawMessage{}
		for j, c := range data.Columns {
			if to := input.Mapping[c]; to != "" && row[j] != nil {
				values[to] = row[j]
			}
		}
		changes[i] = entities.RowChange{Operation: "insert", Values: values}
	}
	if mongoConn, ok := conn.(manager.NoSQLConnection); ok {
		if input.Options.Format == "sql" {
			return nil, fmt.Errorf("use JSON, CSV or XLSX for MongoDB")
		}
		// Validate every document before inserting the first one.
		for i, c := range changes {
			if _, err = tableedit.MongoDocument(c.Values); err != nil {
				return nil, fmt.Errorf("row %d: %w", i+1, err)
			}
		}
		result := tableedit.SaveMongo(ctx, mongoConn.DB().Collection(input.Table.Name), changes)
		return &result, nil
	}
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return nil, fmt.Errorf("unsupported database")
	}
	if conn.Type() == "mysql" {
		var engine string
		err := sqlConn.DB().QueryRowContext(ctx, "SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", input.Table.Database, input.Table.Name).Scan(&engine)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(engine, "InnoDB") {
			return nil, fmt.Errorf("transactional imports require an InnoDB destination table (found %s)", engine)
		}
	}
	// Imports may explicitly preserve identity values; generated expressions stay read-only.
	for i := range info.Columns {
		info.Columns[i].AutoIncrement = false
	}
	tx, err := sqlConn.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for i, change := range changes {
		query, args, err := tableedit.BuildSQL(*info, change)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		names := make([]string, 0, len(change.Values))
		for k := range change.Values {
			names = append(names, k)
		}
		sort.Strings(names)
		for j, name := range names {
			for _, column := range info.Columns {
				if column.Name == name {
					args[j], err = importValue(change.Values[name], column.DatabaseType)
					break
				}
			}
			if err != nil {
				return nil, fmt.Errorf("row %d, column %s: %w", i+1, name, err)
			}
		}
		if _, err = tx.ExecContext(ctx, query, args...); err != nil {
			return nil, fmt.Errorf("row %d: %w (SQL import rolled back)", i+1, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &entities.TableSaveResult{Applied: len(changes)}, nil
}
func binaryType(kind string) bool {
	k := strings.ToUpper(kind)
	return strings.Contains(k, "BLOB") || strings.Contains(k, "BINARY") || k == "BYTEA"
}
func importValue(raw json.RawMessage, kind string) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var v any
	if err := decoder.Decode(&v); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	if binaryType(kind) {
		var value struct {
			Binary *struct {
				Base64 *string `json:"base64"`
			} `json:"$binary"`
		}
		if s, ok := v.(string); ok {
			if strings.HasPrefix(s, `\x`) {
				return hex.DecodeString(s[2:])
			}
			return base64.StdEncoding.DecodeString(s)
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		object, ok := v.(map[string]any)
		if !ok || object["$binary"] == nil || value.Binary == nil || value.Binary.Base64 == nil {
			return nil, fmt.Errorf("binary values must be base64 text or Extended JSON $binary")
		}
		return base64.StdEncoding.DecodeString(*value.Binary.Base64)
	}
	switch x := v.(type) {
	case json.Number:
		return x.String(), nil
	case map[string]any, []any:
		return string(raw), nil
	case string:
		if strings.EqualFold(kind, "boolean") || strings.EqualFold(kind, "bool") {
			switch strings.ToLower(x) {
			case "true", "1":
				return true, nil
			case "false", "0":
				return false, nil
			default:
				return nil, fmt.Errorf("expected true, false, 1 or 0")
			}
		}
	}
	return v, nil
}

func (s *DbService) ExportData(ctx context.Context, input ExportRequest) (*ExportResult, error) {
	if input.Options.Format != "csv" && input.Options.Format != "json" && input.Options.Format != "xlsx" && input.Options.Format != "sql" {
		return nil, fmt.Errorf("unsupported format")
	}
	name := input.Table.Name
	if name == "" {
		name = "query-result"
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "_"))
	path, err := application.Get().Dialog.SaveFile().SetMessage("Export data").SetFilename(name+"."+input.Options.Format).AddFilter(strings.ToUpper(input.Options.Format), "*."+input.Options.Format).PromptForSingleSelection()
	if err != nil || path == "" {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	// Finish a temporary file before replacing the chosen destination.
	file, err := os.CreateTemp(filepath.Dir(path), ".dbviewer-export-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	count, err := s.writeExport(ctx, file, input)
	if err != nil {
		return nil, err
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return nil, err
	}
	return &ExportResult{Path: path, Rows: count}, nil
}

func (s *DbService) writeExport(ctx context.Context, w io.Writer, input ExportRequest) (int, error) {
	conn, ok := s.manager.Active()
	if !ok || conn.ID() != input.ConnectionID {
		return 0, fmt.Errorf("active connection changed")
	}
	target := ""
	if input.Table.Name != "" {
		target = tableedit.TableName(input.Table, conn.Type())
	}
	if (conn.Type() == "mongodb" || conn.Type() == "cassandra" || conn.Type() == "couchdb" || conn.Type() == "redis") && input.Options.Format == "sql" {
		return 0, fmt.Errorf("SQL export is not available for this NoSQL database")
	}
	if input.Scope == "page" {
		if conn.Type() == "mongodb" && input.Options.Format == "json" {
			if len(input.Documents) != len(input.Rows) {
				return 0, fmt.Errorf("original MongoDB documents are unavailable; export the entire collection instead")
			}
			documents := make([]json.RawMessage, len(input.Documents))
			for i, document := range input.Documents {
				var err error
				documents[i], err = excludeDocumentFields(json.RawMessage(document), input.ExcludedColumns)
				if err != nil {
					return 0, err
				}
			}
			err := json.NewEncoder(w).Encode(documents)
			return len(documents), err
		}
		index := 0
		return writeExportColumns(w, input.Columns, func() ([]any, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if index >= len(input.Rows) {
				return nil, io.EOF
			}
			row := input.Rows[index]
			index++
			values := make([]any, len(row))
			for i, raw := range row {
				d := json.NewDecoder(bytes.NewReader(raw))
				d.UseNumber()
				if err := d.Decode(&values[i]); err != nil {
					return nil, err
				}
			}
			return values, nil
		}, input.Options, target, conn.Type(), input.ExcludedColumns)
	}
	if input.Scope != "table" && input.Scope != "query" {
		return 0, fmt.Errorf("unknown export scope")
	}
	if input.Scope == "table" {
		if input.Table.ConnectionID != conn.ID() {
			return 0, fmt.Errorf("table belongs to another connection")
		}
		driver, err := s.factory.Driver(conn.Type())
		if err != nil {
			return 0, err
		}
		tables, err := driver.Inspector().ListTables(ctx, conn)
		if err != nil {
			return 0, err
		}
		found := false
		for _, t := range tables {
			if t.Name == input.Table.Name && t.Schema == input.Table.Schema && t.Database == input.Table.Database {
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("table not found")
		}
	}
	if mongoConn, ok := conn.(manager.NoSQLConnection); ok {
		if input.Scope != "table" {
			return 0, fmt.Errorf("MongoDB export supports entire collections or the current result page")
		}
		cursor, err := mongoConn.DB().Collection(input.Table.Name).Find(ctx, bson.D{})
		if err != nil {
			return 0, err
		}
		defer cursor.Close(context.Background())
		// JSON documents can have different fields; retain the complete original documents.
		if input.Options.Format != "json" {
			return 0, fmt.Errorf("use JSON to export an entire MongoDB collection; CSV/XLSX can export the current page")
		}
		if _, err = io.WriteString(w, "[\n"); err != nil {
			return 0, err
		}
		count := 0
		for cursor.Next(ctx) {
			raw, err := bson.MarshalExtJSON(cursor.Current, true, false)
			if err == nil {
				raw, err = excludeDocumentFields(raw, input.ExcludedColumns)
			}
			if err != nil {
				return count, err
			}
			if count > 0 {
				if _, err = io.WriteString(w, ",\n"); err != nil {
					return count, err
				}
			}
			if _, err = w.Write(raw); err != nil {
				return count, err
			}
			count++
		}
		if err = cursor.Err(); err != nil {
			return count, err
		}
		_, err = io.WriteString(w, "\n]\n")
		return count, err
	}
	sqlConn, ok := conn.(manager.SQLConnection)
	if !ok {
		return 0, fmt.Errorf("unsupported database")
	}
	query := input.Query
	if input.Scope == "table" {
		query = "SELECT * FROM " + target
	} else {
		var pageable bool
		query, pageable = sqlExecutor.PageableSQL(query)
		if !pageable {
			return 0, fmt.Errorf("export all query rows requires a single SELECT query")
		}
	}
	tx, err := sqlConn.DB().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return 0, err
	}
	return writeExportColumns(w, columns, func() ([]any, error) {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return nil, err
			}
			return nil, io.EOF
		}
		row := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range row {
			pointers[i] = &row[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		for i, v := range row {
			switch x := v.(type) {
			case []byte:
				if !binaryType(types[i].DatabaseTypeName()) {
					row[i] = string(x)
				}
			case time.Time:
				row[i] = x.Format(time.RFC3339Nano)
			}
		}
		return row, nil
	}, input.Options, target, conn.Type(), input.ExcludedColumns)
}
