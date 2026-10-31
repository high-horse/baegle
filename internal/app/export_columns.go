package app

import (
	"encoding/json"
	"fmt"
	"io"

	"db-lens/internal/engine/datatransfer"
)

// Filter headers and row values together, once per streamed row. Nil exclusions
// preserve the existing export behavior and do not allocate a projected row.
func writeExportColumns(w io.Writer, columns []string, next func() ([]any, error), options datatransfer.Options, target, dialect string, excluded []string) (int, error) {
	if len(excluded) == 0 {
		return datatransfer.Write(w, columns, next, options, target, dialect)
	}
	omitted := make(map[string]bool, len(excluded))
	for _, name := range excluded {
		omitted[name] = true
	}
	available := make(map[string]bool, len(columns))
	selected := make([]string, 0, len(columns))
	indexes := make([]int, 0, len(columns))
	for i, name := range columns {
		if !omitted[name] {
			selected = append(selected, name)
			indexes = append(indexes, i)
		}
		available[name] = true
	}
	for name := range omitted {
		if !available[name] {
			return 0, fmt.Errorf("an excluded column is no longer present; reopen export settings")
		}
	}
	if len(selected) == 0 {
		return 0, fmt.Errorf("select at least one column to export")
	}
	return datatransfer.Write(w, selected, func() ([]any, error) {
		row, err := next()
		if err != nil {
			return nil, err
		}
		if len(row) != len(columns) {
			return nil, fmt.Errorf("export row has %d values; expected %d", len(row), len(columns))
		}
		values := make([]any, len(indexes))
		for i, index := range indexes {
			values[i] = row[index]
		}
		return values, nil
	}, options, target, dialect)
}

// Keep canonical MongoDB Extended JSON values intact while omitting top-level fields.
func excludeDocumentFields(document json.RawMessage, excluded []string) (json.RawMessage, error) {
	if len(excluded) == 0 {
		return document, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(document, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected a document object")
	}
	for _, name := range excluded {
		delete(fields, name)
	}
	return json.Marshal(fields)
}
