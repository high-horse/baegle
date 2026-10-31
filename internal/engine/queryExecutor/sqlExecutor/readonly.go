package sqlExecutor

import (
	"fmt"
	"strings"

	"db-lens/internal/engine/sqlscript"
)

// CheckReadOnlySQL is a conservative application guard, separate from the
// detector (returning rows does not imply a statement is read-only). Drivers
// also enable native read-only sessions where supported. Database permissions
// remain the authority for user-defined functions and vendor extensions.
func CheckReadOnlySQL(query, dialect string) error {
	deny := func() error { return fmt.Errorf("this connection is read-only; only read queries are allowed") }
	parts, err := sqlscript.Split(query, dialect)
	if err != nil || len(parts) != 1 || parts[0].CopyData != nil {
		return deny()
	}
	tokens, ok := readOnlyTokens(query, dialect)
	if !ok || len(tokens) == 0 {
		return deny()
	}
	switch tokens[0] {
	case "SELECT", "WITH", "VALUES", "TABLE", "SHOW", "DESCRIBE", "DESC", "EXPLAIN":
	case "PRAGMA":
		// Only metadata pragmas; many other pragmas change files or session state.
		i := 1
		if len(tokens) > 3 && tokens[2] == "." {
			i = 3
		}
		if len(tokens) <= i {
			return deny()
		}
		switch strings.TrimPrefix(tokens[i], "\"") {
		case "TABLE_INFO", "TABLE_XINFO", "TABLE_LIST", "INDEX_LIST", "INDEX_INFO", "INDEX_XINFO", "FOREIGN_KEY_LIST", "DATABASE_LIST", "COMPILE_OPTIONS":
		default:
			return deny()
		}
		for _, token := range tokens {
			if token == "=" {
				return deny()
			}
		}
	default:
		return deny()
	}
	for i, token := range tokens {
		if i == 1 && tokens[0] == "SHOW" && token == "CREATE" {
			continue
		}
		if i+1 < len(tokens) && tokens[i+1] == "(" && (token == "REPLACE" || token == "INSERT" || token == "TRUNCATE") {
			// These mutation keywords are also ordinary scalar functions.
			continue
		}
		if token == "FOR" && i+1 < len(tokens) && tokens[i+1] == "SHARE" || token == "NEXT" && i+1 < len(tokens) && tokens[i+1] == "VALUE" {
			return deny()
		}
		switch token {
		case "INSERT", "UPDATE", "DELETE", "MERGE", "REPLACE", "UPSERT", "INTO", "OUTFILE", "DUMPFILE",
			"CREATE", "ALTER", "DROP", "TRUNCATE", "GRANT", "REVOKE", "COPY", "CALL", "EXEC", "EXECUTE",
			"SET", "RESET", "BEGIN", "START", "COMMIT", "ROLLBACK", "SAVEPOINT", "RELEASE",
			"ATTACH", "DETACH", "VACUUM", "ANALYZE", "REINDEX", "REFRESH", "LOCK", "UNLOCK",
			"UPDLOCK", "XLOCK", "HOLDLOCK", "TABLOCKX", "NEXTVAL", "SETVAL":
			return deny()
		}
		// Quoted function identifiers are checked as well as unquoted ones.
		if i+1 < len(tokens) && tokens[i+1] == "(" {
			switch strings.TrimPrefix(token, "\"") {
			case "SET_CONFIG", "NEXTVAL", "SETVAL", "LO_IMPORT", "LO_EXPORT", "LO_CREATE", "LO_UNLINK", "LOWRITE",
				"PG_LOGICAL_EMIT_MESSAGE", "PG_WRITE_FILE", "PG_FILE_WRITE", "PG_FILE_RENAME", "PG_FILE_UNLINK",
				"DBLINK_EXEC", "LOAD_EXTENSION", "WRITEFILE", "GET_LOCK", "RELEASE_LOCK":
				return deny()
			}
		}
	}
	return nil
}

// Literals and comments cannot supply keywords. Reject ambiguous escaping,
// executable MySQL/MariaDB comments, and unterminated tokens rather than guessing.
func readOnlyTokens(query, dialect string) ([]string, bool) {
	var tokens []string
	for i := 0; i < len(query); {
		ch := query[i]
		if ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t' || ch == ';' {
			i++
			continue
		}
		lineComment := strings.HasPrefix(query[i:], "--") && (dialect != "mysql" || i+2 == len(query) || query[i+2] <= ' ')
		if lineComment || dialect == "mysql" && ch == '#' {
			n := strings.IndexByte(query[i:], '\n')
			if n < 0 {
				break
			}
			i += n + 1
			continue
		}
		if strings.HasPrefix(query[i:], "/*") {
			if strings.HasPrefix(query[i:], "/*!") || strings.HasPrefix(query[i:], "/*M!") {
				return nil, false
			}
			i += 2
			depth := 1
			for i < len(query) && depth > 0 {
				if strings.HasPrefix(query[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(query[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return nil, false
			}
			continue
		}
		if ch == '$' {
			if dialect != "pgx" && dialect != "postgres" {
				return nil, false
			}
			end := i + 1
			for end < len(query) && (query[end] == '_' || query[end] >= 'a' && query[end] <= 'z' || query[end] >= 'A' && query[end] <= 'Z' || query[end] >= '0' && query[end] <= '9') {
				end++
			}
			if end >= len(query) || query[end] != '$' {
				return nil, false
			}
			tag := query[i : end+1]
			n := strings.Index(query[end+1:], tag)
			if n < 0 {
				return nil, false
			}
			i = end + 1 + n + len(tag)
			tokens = append(tokens, "'literal")
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' || ch == '[' {
			close := ch
			if ch == '[' {
				close = ']'
			}
			i++
			var value strings.Builder
			closed := false
			for i < len(query) {
				if query[i] == '\\' {
					return nil, false
				}
				if query[i] == close {
					if i+1 < len(query) && query[i+1] == close {
						value.WriteByte(close)
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				value.WriteByte(query[i])
				i++
			}
			if !closed {
				return nil, false
			}
			if ch == '\'' {
				tokens = append(tokens, "'literal")
			} else {
				tokens = append(tokens, "\""+strings.ToUpper(value.String()))
			}
			continue
		}
		if ch == '@' || ch == '\\' {
			return nil, false
		}
		if ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
			start := i
			for i < len(query) && (query[i] == '_' || query[i] >= 'a' && query[i] <= 'z' || query[i] >= 'A' && query[i] <= 'Z' || query[i] >= '0' && query[i] <= '9') {
				i++
			}
			tokens = append(tokens, strings.ToUpper(query[start:i]))
		} else {
			tokens = append(tokens, string(ch))
			i++
		}
	}
	return tokens, true
}
