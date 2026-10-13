package datatransfer

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestTextFormatsRoundTrip(t *testing.T) {
	columns := []string{"id", "text", "empty", "null", "literalNull", "slash", "formula", "unicode"}
	values := []any{"9007199254740993", "quotes \" and comma, newline\nnext", "", nil, `\N`, `\path\file`, "=1+1", "नेपाली 😀"}
	for _, format := range []string{"csv", "xlsx"} {
		t.Run(format, func(t *testing.T) {
			options := Options{Format: format, NullToken: `\N`, Delimiter: ","}
			var b bytes.Buffer
			sent := false
			count, err := Write(&b, columns, func() ([]any, error) {
				if sent {
					return nil, io.EOF
				}
				sent = true
				return values, nil
			}, options, "", "sqlite")
			if err != nil || count != 1 {
				t.Fatalf("write: %d %v", count, err)
			}
			data, err := Read(b.Bytes(), options, "sqlite")
			if err != nil {
				t.Fatal(err)
			}
			if len(data.Rows) != 1 {
				t.Fatal(data.Rows)
			}
			for i, v := range values {
				raw, _ := json.Marshal(v)
				if string(data.Rows[0][i]) != string(raw) {
					t.Fatalf("column %s: %s != %s", columns[i], data.Rows[0][i], raw)
				}
			}
			if format == "xlsx" {
				book, err := excelize.OpenReader(bytes.NewReader(b.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				defer book.Close()
				formula, err := book.GetCellFormula("Sheet1", "G2")
				if err != nil || formula != "" {
					t.Fatalf("data became a formula: %q %v", formula, err)
				}
			}
		})
	}
}

func TestSQLLiteralRoundTrip(t *testing.T) {
	values := []any{"O'Reilly; -- not SQL\nC:\\test", json.Number("9007199254740993"), nil, true, "नेपाली", []byte{0, 1, 255}}
	for _, dialect := range []string{"pgx", "mysql", "sqlite"} {
		t.Run(dialect, func(t *testing.T) {
			var b bytes.Buffer
			sent := false
			options := Options{Format: "sql"}
			_, err := Write(&b, []string{"text", "number", "null", "bool", "unicode", "blob"}, func() ([]any, error) {
				if sent {
					return nil, io.EOF
				}
				sent = true
				return values, nil
			}, options, quote("odd table", dialect), dialect)
			if err != nil {
				t.Fatal(err)
			}
			data, err := Read(b.Bytes(), options, dialect)
			if err != nil {
				t.Fatalf("%s: %v", b.String(), err)
			}
			for i := 0; i < 5; i++ {
				raw, _ := json.Marshal(values[i])
				if string(data.Rows[0][i]) != string(raw) {
					t.Fatalf("value %d: %s != %s", i, data.Rows[0][i], raw)
				}
			}
		})
	}
}
func TestSQLRejectsCommandsAndExpressions(t *testing.T) {
	if _, err := Read([]byte(`INSERT INTO items(name) VALUES(E'\u2603');`), Options{Format: "sql"}, "pgx"); err == nil {
		t.Fatal("unsupported escape was silently changed")
	}
	for _, text := range []string{`DROP TABLE items;`, `INSERT INTO items (id) VALUES (1); DELETE FROM items;`, `INSERT INTO items (id) VALUES (random());`, `INSERT INTO items (id) VALUES (1); INSERT INTO other(id) VALUES(2);`, `INSERT INTO items(id) VALUES('unfinished);`} {
		if _, err := Read([]byte(text), Options{Format: "sql"}, "sqlite"); err == nil {
			t.Fatalf("accepted %s", text)
		}
	}
	good := `-- header
 INSERT INTO "main"."items" (id, name) VALUES (1, 'one'), (2, 'two'); /*end*/`
	data, err := Read([]byte(good), Options{Format: "sql"}, "sqlite")
	if err != nil || len(data.Rows) != 2 {
		t.Fatalf("multirow: %+v %v", data, err)
	}
}
func TestJSONPreservesNumbersMissingAndNestedValues(t *testing.T) {
	data, err := Read([]byte(`[{"id":9007199254740993,"nested":{"x":true}},{"id":2,"nested":null,"optional":""}]`), Options{Format: "json"}, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if string(data.Rows[0][0]) != "9007199254740993" || data.Rows[0][2] != nil || string(data.Rows[1][1]) != "null" {
		t.Fatal(data.Rows)
	}
}
func TestMalformedFilesAndExportLimits(t *testing.T) {
	for _, csv := range []string{"a,a\n1,2", ",b\n1,2", "a,b\n1"} {
		if _, err := Read([]byte(csv), Options{Format: "csv"}, "sqlite"); err == nil {
			t.Fatalf("accepted %q", csv)
		}
	}
	for _, jsonText := range []string{`[null]`, `{"id":1}`, `[{"id":1}] garbage`} {
		if _, err := Read([]byte(jsonText), Options{Format: "json"}, "sqlite"); err == nil {
			t.Fatalf("accepted %q", jsonText)
		}
	}
	count := 0
	_, err := Write(io.Discard, []string{"text"}, func() ([]any, error) {
		count++
		if count > 1 {
			return nil, io.EOF
		}
		return []any{strings.Repeat("x", 32768)}, nil
	}, Options{Format: "xlsx"}, "", "sqlite")
	if err == nil {
		t.Fatal("silently truncated an Excel cell")
	}
}
