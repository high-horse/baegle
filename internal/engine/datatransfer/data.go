// Package datatransfer implements data-only interchange (not database backups).
package datatransfer

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const MaxImportBytes = 32 << 20
const MaxImportRows = 100000

type Options struct {
	Format    string `json:"format"`
	Delimiter string `json:"delimiter"`
	NullToken string `json:"nullToken"`
	Sheet     string `json:"sheet"`
}
type Data struct {
	Columns []string
	Rows    [][]json.RawMessage
	Sheets  []string
}

func delimiter(value string) (rune, error) {
	if value == "" {
		return ',', nil
	}
	r, n := utf8.DecodeRuneInString(value)
	if n != len(value) || r == utf8.RuneError || r == 0 || r == '"' || r == '\r' || r == '\n' {
		return 0, fmt.Errorf("choose a single valid delimiter")
	}
	return r, nil
}
func validateColumns(columns []string) error {
	if len(columns) == 0 {
		return fmt.Errorf("the file has no columns")
	}
	seen := map[string]bool{}
	for _, name := range columns {
		if strings.TrimSpace(name) == "" || seen[name] {
			return fmt.Errorf("column names must be nonempty and unique: %q", name)
		}
		seen[name] = true
	}
	return nil
}
func (d *Data) add(row []json.RawMessage) error {
	if len(d.Rows) >= MaxImportRows {
		return fmt.Errorf("import supports at most %d rows per file", MaxImportRows)
	}
	if len(row) != len(d.Columns) {
		return fmt.Errorf("row %d has %d fields; expected %d", len(d.Rows)+2, len(row), len(d.Columns))
	}
	d.Rows = append(d.Rows, row)
	return nil
}
func textValue(value, nullToken string) json.RawMessage {
	if value == nullToken {
		return json.RawMessage("null")
	}
	if strings.HasPrefix(value, `\\`) || value == `\`+nullToken {
		value = value[1:]
	}
	raw, _ := json.Marshal(value)
	return raw
}

func Read(content []byte, options Options, dialect string) (*Data, error) {
	if len(content) > MaxImportBytes {
		return nil, fmt.Errorf("import files must be 32 MiB or smaller")
	}
	content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
	d := &Data{Columns: []string{}, Rows: [][]json.RawMessage{}, Sheets: []string{}}
	switch options.Format {
	case "csv":
		comma, err := delimiter(options.Delimiter)
		if err != nil {
			return nil, err
		}
		reader := csv.NewReader(bytes.NewReader(content))
		reader.Comma = comma
		d.Columns, err = reader.Read()
		if err != nil {
			return nil, fmt.Errorf("CSV header: %w", err)
		}
		for {
			fields, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("CSV row %d: %w", len(d.Rows)+2, err)
			}
			row := make([]json.RawMessage, len(fields))
			for i, v := range fields {
				row[i] = textValue(v, options.NullToken)
			}
			if err = d.add(row); err != nil {
				return nil, err
			}
		}
	case "json":
		var documents []map[string]json.RawMessage
		if err := json.Unmarshal(content, &documents); err != nil {
			return nil, fmt.Errorf("expected a JSON array of row objects: %w", err)
		}
		if len(documents) > MaxImportRows {
			return nil, fmt.Errorf("import supports at most %d rows", MaxImportRows)
		}
		names := map[string]bool{}
		for i, doc := range documents {
			if doc == nil {
				return nil, fmt.Errorf("row %d must be an object", i+1)
			}
			for k := range doc {
				names[k] = true
			}
		}
		for name := range names {
			d.Columns = append(d.Columns, name)
		}
		sort.Strings(d.Columns)
		for _, doc := range documents {
			row := make([]json.RawMessage, len(d.Columns))
			for i, k := range d.Columns {
				row[i] = doc[k]
			}
			d.Rows = append(d.Rows, row)
		}
	case "xlsx":
		book, err := excelize.OpenReader(bytes.NewReader(content), excelize.Options{UnzipSizeLimit: 128 << 20, UnzipXMLSizeLimit: 16 << 20})
		if err != nil {
			return nil, err
		}
		defer book.Close()
		d.Sheets = book.GetSheetList()
		sheet := options.Sheet
		if sheet == "" && len(d.Sheets) > 0 {
			sheet = d.Sheets[0]
		}
		rows, err := book.Rows(sheet)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		first := true
		for rows.Next() {
			fields, err := rows.Columns(excelize.Options{RawCellValue: true})
			if err != nil {
				return nil, err
			}
			if first {
				d.Columns = fields
				first = false
				continue
			}
			if len(fields) > len(d.Columns) {
				return nil, fmt.Errorf("worksheet row %d has more cells than headers", len(d.Rows)+2)
			}
			row := make([]json.RawMessage, len(d.Columns))
			for i := range row {
				v := ""
				if i < len(fields) {
					v = fields[i]
				}
				row[i] = textValue(v, options.NullToken)
			}
			if err = d.add(row); err != nil {
				return nil, err
			}
		}
		if err = rows.Error(); err != nil {
			return nil, err
		}
	case "sql":
		var err error
		d, err = readSQL(string(content), dialect)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported format %q", options.Format)
	}
	if err := validateColumns(d.Columns); err != nil {
		return nil, err
	}
	return d, nil
}

// Bytes use MongoDB Extended JSON's binary representation in JSON files.
func Normalize(value any) any {
	if b, ok := value.([]byte); ok {
		return map[string]any{"$binary": map[string]string{"base64": base64.StdEncoding.EncodeToString(b), "subType": "00"}}
	}
	return value
}
func Text(value any, nullToken string) (string, error) {
	if value == nil {
		return nullToken, nil
	}
	if s, ok := value.(string); ok {
		if s == nullToken || strings.HasPrefix(s, `\`) {
			s = `\` + s
		}
		return s, nil
	}
	if b, ok := value.([]byte); ok {
		return base64.StdEncoding.EncodeToString(b), nil
	}
	raw, err := json.Marshal(value)
	return string(raw), err
}

// Write consumes rows one at a time, without collecting an entire table in memory.
func Write(w io.Writer, columns []string, next func() ([]any, error), options Options, table, dialect string) (int, error) {
	if err := validateColumns(columns); err != nil {
		return 0, err
	}
	var csvWriter *csv.Writer
	var book *excelize.File
	var sheet *excelize.StreamWriter
	var err error
	switch options.Format {
	case "csv":
		csvWriter = csv.NewWriter(w)
		csvWriter.Comma, err = delimiter(options.Delimiter)
		if err != nil {
			return 0, err
		}
		if err = csvWriter.Write(columns); err != nil {
			return 0, err
		}
	case "json":
		if _, err = io.WriteString(w, "[\n"); err != nil {
			return 0, err
		}
	case "sql":
		if table == "" {
			return 0, fmt.Errorf("SQL export needs a target table name")
		}
	case "xlsx":
		if len(columns) > 16384 {
			return 0, fmt.Errorf("Excel supports at most 16384 columns")
		}
		book = excelize.NewFile()
		defer book.Close()
		sheet, err = book.NewStreamWriter("Sheet1")
		if err != nil {
			return 0, err
		}
		header := make([]any, len(columns))
		for i, c := range columns {
			header[i] = c
		}
		if err = sheet.SetRow("A1", header); err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("unsupported format %q", options.Format)
	}
	count := 0
	for {
		row, err := next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		if len(row) != len(columns) {
			return count, fmt.Errorf("result row has incorrect column count")
		}
		switch options.Format {
		case "csv", "xlsx":
			fields := make([]string, len(row))
			for i, v := range row {
				fields[i], err = Text(v, options.NullToken)
				if err != nil {
					return count, err
				}
			}
			if csvWriter != nil {
				err = csvWriter.Write(fields)
			} else {
				if count >= 1048575 {
					return count, fmt.Errorf("Excel row limit exceeded; use CSV or JSON")
				}
				cells := make([]any, len(fields))
				for i, v := range fields {
					if utf8.RuneCountInString(v) > 32767 {
						return count, fmt.Errorf("row %d column %s exceeds Excel's cell length limit", count+1, columns[i])
					}
					cells[i] = v
				}
				// All cells are text: preserve large IDs and never turn data into formulas.
				err = sheet.SetRow(fmt.Sprintf("A%d", count+2), cells)
			}
		case "json":
			obj := map[string]any{}
			for i, c := range columns {
				obj[c] = Normalize(row[i])
			}
			var raw []byte
			raw, err = json.Marshal(obj)
			if err == nil {
				if count > 0 {
					_, err = io.WriteString(w, ",\n")
				}
				if err == nil {
					_, err = w.Write(raw)
				}
			}
		case "sql":
			names, values := make([]string, len(columns)), make([]string, len(columns))
			for i, c := range columns {
				names[i] = quote(c, dialect)
				values[i], err = literal(row[i], dialect)
				if err != nil {
					return count, err
				}
			}
			_, err = fmt.Fprintf(w, "INSERT INTO %s (%s) VALUES (%s);\n", table, strings.Join(names, ", "), strings.Join(values, ", "))
		}
		if err != nil {
			return count, err
		}
		count++
	}
	if csvWriter != nil {
		csvWriter.Flush()
		err = csvWriter.Error()
	}
	if options.Format == "json" {
		_, err = io.WriteString(w, "\n]\n")
	}
	if sheet != nil {
		err = sheet.Flush()
		if err == nil {
			err = book.Write(w)
		}
	}
	return count, err
}
