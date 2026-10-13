package datatransfer

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func quote(name, dialect string) string {
	q := `"`
	if dialect == "mysql" {
		q = "`"
	}
	return q + strings.ReplaceAll(name, q, q+q) + q
}
func literal(v any, dialect string) (string, error) {
	if v == nil {
		return "NULL", nil
	}
	if b, ok := v.([]byte); ok {
		if dialect == "pgx" {
			return "E'\\\\x" + hex.EncodeToString(b) + "'", nil
		}
		return "X'" + hex.EncodeToString(b) + "'", nil
	}
	if b, ok := v.(bool); ok {
		if b {
			return "TRUE", nil
		}
		return "FALSE", nil
	}
	s, ok := v.(string)
	if !ok {
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		s = string(raw)
		if numberPattern.MatchString(s) {
			return s, nil
		}
	}
	prefix := ""
	if dialect == "pgx" {
		prefix = "E"
	}
	if dialect == "pgx" || dialect == "mysql" {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, "\x00", `\0`)
	}
	return prefix + "'" + strings.ReplaceAll(s, "'", "''") + "'", nil
}

var numberPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

type token struct{ kind, text string }
type lexer struct {
	s, dialect string
	pos        int
}

func (l *lexer) next() (token, error) {
	for l.pos < len(l.s) {
		if unicode.IsSpace(rune(l.s[l.pos])) {
			l.pos++
			continue
		}
		if strings.HasPrefix(l.s[l.pos:], "--") {
			for l.pos < len(l.s) && l.s[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		if strings.HasPrefix(l.s[l.pos:], "/*") {
			end := strings.Index(l.s[l.pos+2:], "*/")
			if end < 0 {
				return token{}, fmt.Errorf("unterminated SQL comment")
			}
			l.pos += end + 4
			continue
		}
		break
	}
	if l.pos >= len(l.s) {
		return token{kind: "eof"}, nil
	}
	c := l.s[l.pos]
	escaped := l.dialect == "mysql"
	kind := "word"
	if (c == 'E' || c == 'e' || c == 'X' || c == 'x') && l.pos+1 < len(l.s) && l.s[l.pos+1] == '\'' {
		if c == 'X' || c == 'x' {
			kind = "hex"
		} else {
			escaped = true
		}
		l.pos++
		c = '\''
	}
	if c == '\'' || c == '"' || c == '`' || c == '[' {
		end := c
		if c == '[' {
			end = ']'
		}
		if c == '\'' {
			if kind != "hex" {
				kind = "string"
			}
		} else {
			kind = "identifier"
			escaped = false
		}
		l.pos++
		var b strings.Builder
		for l.pos < len(l.s) {
			c = l.s[l.pos]
			l.pos++
			if c == end {
				if l.pos < len(l.s) && l.s[l.pos] == end {
					b.WriteByte(c)
					l.pos++
					continue
				}
				return token{kind, b.String()}, nil
			}
			if c == '\\' && escaped {
				if l.pos >= len(l.s) {
					break
				}
				c = l.s[l.pos]
				l.pos++
				switch c {
				case 'n':
					c = '\n'
				case 'r':
					c = '\r'
				case 't':
					c = '\t'
				case '0':
					c = 0
				case 'b':
					c = '\b'
				case 'f':
					c = '\f'
				case 'Z':
					c = 26
				case '\\', '\'', '"':
				default:
					return token{}, fmt.Errorf("unsupported SQL escape \\%c; use literal text or CSV/JSON instead", c)
				}
			}
			b.WriteByte(c)
		}
		return token{}, fmt.Errorf("unterminated SQL quoted value")
	}
	if strings.ContainsRune("(),.;", rune(c)) {
		l.pos++
		return token{string(c), string(c)}, nil
	}
	start := l.pos
	for l.pos < len(l.s) && !unicode.IsSpace(rune(l.s[l.pos])) && !strings.ContainsRune("(),;", rune(l.s[l.pos])) {
		if l.s[l.pos] == '.' && !((l.s[start] >= '0' && l.s[start] <= '9') || l.s[start] == '-' || l.s[start] == '+') {
			break
		}
		l.pos++
	}
	if start == l.pos {
		return token{}, fmt.Errorf("unexpected SQL character at %d", l.pos)
	}
	return token{"word", l.s[start:l.pos]}, nil
}

// Only literal INSERTs are accepted. Scripts cannot execute arbitrary SQL through import.
func readSQL(input, dialect string) (*Data, error) {
	l := lexer{s: input, dialect: dialect}
	d := &Data{Columns: []string{}, Rows: [][]json.RawMessage{}, Sheets: []string{}}
	current, err := l.next()
	advance := func() {
		if err == nil {
			current, err = l.next()
		}
	}
	expect := func(w string) error {
		if err != nil {
			return err
		}
		if !strings.EqualFold(current.text, w) {
			return fmt.Errorf("SQL data import expected %s, got %q; only INSERT INTO ... VALUES with literal values is supported", w, current.text)
		}
		advance()
		return err
	}
	var source string
	for err == nil && current.kind != "eof" {
		if current.kind == ";" {
			advance()
			continue
		}
		if e := expect("INSERT"); e != nil {
			return nil, e
		}
		if e := expect("INTO"); e != nil {
			return nil, e
		}
		table := ""
		for {
			if current.kind != "word" && current.kind != "identifier" {
				return nil, fmt.Errorf("expected a table identifier")
			}
			table += current.text
			advance()
			if current.kind != "." {
				break
			}
			table += "."
			advance()
		}
		if source != "" && source != table {
			return nil, fmt.Errorf("import one source table per SQL file")
		}
		source = table
		if e := expect("("); e != nil {
			return nil, e
		}
		columns := []string{}
		for {
			if current.kind != "word" && current.kind != "identifier" {
				return nil, fmt.Errorf("expected a column name")
			}
			columns = append(columns, current.text)
			advance()
			if current.kind != "," {
				break
			}
			advance()
		}
		if e := expect(")"); e != nil {
			return nil, e
		}
		if e := expect("VALUES"); e != nil {
			return nil, e
		}
		if len(d.Columns) == 0 {
			d.Columns = columns
		} else {
			if len(d.Columns) != len(columns) {
				return nil, fmt.Errorf("all INSERT statements must use the same columns")
			}
			for i, c := range columns {
				if c != d.Columns[i] {
					return nil, fmt.Errorf("all INSERT statements must use the same column order")
				}
			}
		}
		for {
			if e := expect("("); e != nil {
				return nil, e
			}
			row := []json.RawMessage{}
			for {
				var value json.RawMessage
				switch current.kind {
				case "string":
					value, _ = json.Marshal(current.text)
				case "hex":
					b, e := hex.DecodeString(current.text)
					if e != nil {
						return nil, e
					}
					value, _ = json.Marshal(Normalize(b))
				case "word":
					switch strings.ToUpper(current.text) {
					case "NULL":
						value = json.RawMessage("null")
					case "TRUE":
						value = json.RawMessage("true")
					case "FALSE":
						value = json.RawMessage("false")
					default:
						n := strings.TrimPrefix(current.text, "+")
						if !numberPattern.MatchString(n) {
							return nil, fmt.Errorf("SQL import accepts literal values only, got %q", current.text)
						}
						value = json.RawMessage(n)
					}
				default:
					return nil, fmt.Errorf("expected a literal SQL value")
				}
				row = append(row, value)
				advance()
				if current.kind != "," {
					break
				}
				advance()
			}
			if e := expect(")"); e != nil {
				return nil, e
			}
			if e := d.add(row); e != nil {
				return nil, e
			}
			if current.kind != "," {
				break
			}
			advance()
		}
		if current.kind != "eof" {
			if e := expect(";"); e != nil {
				return nil, e
			}
		}
	}
	return d, err
}
