// Package sqlscript splits editor scripts without treating quoted semicolons as statement boundaries.
package sqlscript

import (
	"fmt"
	"strings"
)

type Statement struct {
	SQL      string
	Line     int
	CopyData *string
}

func word(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

func Split(input, dialect string) ([]Statement, error) {
	result := []Statement{}
	start, line, statementLine := 0, 1, 1
	delimiter := ";"
	hasCode := false
	firstWord, previousWord := "", ""
	copyFromStdin := false
	trigger, bodyDepth := false, 0
	oracleBlock := false
	createObject := ""
	reset := func(next int) {
		start = next
		hasCode = false
		firstWord = ""
		previousWord = ""
		copyFromStdin = false
		trigger = false
		bodyDepth = 0
		oracleBlock = false
		createObject = ""
	}
	for i := 0; i < len(input); {
		c := input[i]
		if c == '\n' {
			line++
			i++
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' {
			i++
			continue
		}
		atLineStart := (c == 'G' || c == 'g' || c == '/' || c == '-') && strings.TrimSpace(input[strings.LastIndex(input[:i], "\n")+1:i]) == ""
		if atLineStart && (dialect == "sqlserver" || dialect == "oracle" || dialect == "db2") {
			end := strings.IndexByte(input[i:], '\n')
			if end < 0 {
				end = len(input) - i
			}
			lineText := strings.TrimSpace(input[i : i+end])
			if dialect == "db2" && strings.HasPrefix(strings.ToUpper(lineText), "--#SET TERMINATOR ") {
				if hasCode {
					return nil, fmt.Errorf("line %d: terminate the current statement before changing the terminator", line)
				}
				value := strings.TrimSpace(lineText[len("--#SET TERMINATOR "):])
				if len(value) != 1 {
					return nil, fmt.Errorf("line %d: use a single-character Db2 terminator", line)
				}
				delimiter = value
				i += end
				reset(i)
				continue
			}
			goLine := strings.TrimSpace(strings.SplitN(lineText, "--", 2)[0])
			if dialect == "sqlserver" && (strings.EqualFold(goLine, "GO") || strings.HasPrefix(strings.ToUpper(goLine), "GO ")) || dialect == "oracle" && lineText == "/" {
				if dialect == "sqlserver" && !strings.EqualFold(goLine, "GO") {
					return nil, fmt.Errorf("line %d: GO repetition counts are not supported", line)
				}
				if hasCode {
					result = append(result, Statement{SQL: strings.TrimSpace(input[start:i]), Line: statementLine})
				}
				i += end
				reset(i)
				continue
			}
		}
		if !hasCode && dialect == "mysql" && (i == 0 || strings.TrimSpace(input[strings.LastIndex(input[:i], "\n")+1:i]) == "") && len(input)-i >= 9 && strings.EqualFold(input[i:i+9], "DELIMITER") && (len(input) == i+9 || input[i+9] == ' ' || input[i+9] == '\t') {
			end := strings.IndexByte(input[i:], '\n')
			if end < 0 {
				end = len(input) - i
			}
			value := strings.TrimSpace(input[i+9 : i+end])
			if value == "" || strings.ContainsAny(value, " \t\r\n") {
				return nil, fmt.Errorf("line %d: invalid DELIMITER directive", line)
			}
			delimiter = value
			i += end
			reset(i)
			continue
		}
		if strings.HasPrefix(input[i:], "--") || dialect == "mysql" && c == '#' {
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}
		if strings.HasPrefix(input[i:], "/*") {
			if dialect == "mysql" && strings.HasPrefix(input[i:], "/*!") && !hasCode {
				hasCode = true
				statementLine = line
			}
			depth := 1
			i += 2
			for i < len(input) && depth > 0 {
				if strings.HasPrefix(input[i:], "/*") && (dialect == "pgx" || dialect == "sqlserver") {
					depth++
					i += 2
				} else if strings.HasPrefix(input[i:], "*/") {
					depth--
					i += 2
				} else {
					if input[i] == '\n' {
						line++
					}
					i++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("line %d: unterminated block comment", line)
			}
			continue
		}
		if strings.HasPrefix(input[i:], delimiter) && bodyDepth == 0 && !oracleBlock && dialect != "sqlserver" {
			if hasCode {
				statement := Statement{SQL: strings.TrimSpace(input[start:i]), Line: statementLine}
				i += len(delimiter)
				if dialect == "pgx" && copyFromStdin {
					for i < len(input) && (input[i] == ' ' || input[i] == '\t' || input[i] == '\r') {
						i++
					}
					if i >= len(input) || input[i] != '\n' {
						return nil, fmt.Errorf("line %d: COPY FROM STDIN requires data on the next line", statementLine)
					}
					i++
					line++
					dataStart := i
					found := false
					for i < len(input) {
						end := strings.IndexByte(input[i:], '\n')
						if end < 0 {
							end = len(input) - i
						}
						text := strings.TrimSuffix(input[i:i+end], "\r")
						if text == `\.` {
							data := input[dataStart:i]
							statement.CopyData = &data
							i += end
							if i < len(input) {
								i++
								line++
							}
							found = true
							break
						}
						i += end
						if i < len(input) {
							i++
							line++
						}
					}
					if !found {
						return nil, fmt.Errorf("line %d: COPY data is missing its \\. terminator", statementLine)
					}
				}
				result = append(result, statement)
			} else {
				i += len(delimiter)
			}
			reset(i)
			continue
		}
		if !hasCode {
			hasCode = true
			statementLine = line
			if c == '\\' || c == '.' && dialect == "sqlite" {
				return nil, fmt.Errorf("line %d: command-line client directives are not SQL; remove this directive before running", line)
			}
		}
		if dialect == "oracle" && (c == 'q' || c == 'Q') && i+2 < len(input) && input[i+1] == '\'' {
			end := input[i+2]
			switch end {
			case '[':
				end = ']'
			case '{':
				end = '}'
			case '(':
				end = ')'
			case '<':
				end = '>'
			}
			close := strings.Index(input[i+3:], string([]byte{end, '\''}))
			if close < 0 {
				return nil, fmt.Errorf("line %d: unterminated Oracle quoted literal", line)
			}
			next := i + 3 + close + 2
			line += strings.Count(input[i:next], "\n")
			i = next
			continue
		}
		if c == '\'' || c == '"' || c == '`' || c == '[' && (dialect == "sqlite" || dialect == "sqlserver") {
			end := c
			if c == '[' {
				end = ']'
			}
			escaped := dialect == "mysql" || dialect == "pgx" && c == '\'' && i > 0 && (input[i-1] == 'E' || input[i-1] == 'e') && (i < 2 || !word(input[i-2]))
			i++
			closed := false
			for i < len(input) {
				if input[i] == '\n' {
					line++
				}
				if input[i] == '\\' && escaped {
					if i+1 < len(input) && input[i+1] == '\n' {
						line++
					}
					i += 2
					continue
				}
				if input[i] == end {
					i++
					if i < len(input) && input[i] == end {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("line %d: unterminated quoted text", statementLine)
			}
			continue
		}
		if c == '$' && dialect == "pgx" && (i == 0 || !word(input[i-1])) {
			j := i + 1
			for j < len(input) && word(input[j]) {
				j++
			}
			if j < len(input) && input[j] == '$' {
				tag := input[i : j+1]
				end := strings.Index(input[j+1:], tag)
				if end < 0 {
					return nil, fmt.Errorf("line %d: unterminated dollar-quoted body", statementLine)
				}
				next := j + 1 + end + len(tag)
				line += strings.Count(input[i:next], "\n")
				i = next
				continue
			}
		}
		if word(c) {
			j := i + 1
			for j < len(input) && word(input[j]) {
				j++
			}
			value := strings.ToUpper(input[i:j])
			if firstWord == "" {
				firstWord = value
			}
			if firstWord == "CREATE" && createObject == "" && value != "CREATE" && value != "OR" && value != "REPLACE" && value != "EDITIONABLE" && value != "NONEDITIONABLE" {
				createObject = value
			}
			if dialect == "oracle" && (firstWord == "BEGIN" || firstWord == "DECLARE" || firstWord == "CREATE" && (createObject == "PROCEDURE" || createObject == "FUNCTION" || createObject == "PACKAGE" || createObject == "TRIGGER" || createObject == "TYPE")) {
				oracleBlock = true
			}
			if firstWord == "COPY" && previousWord == "FROM" && value == "STDIN" {
				copyFromStdin = true
			}
			previousWord = value
			if dialect == "sqlite" {
				if value == "TRIGGER" && firstWord == "CREATE" {
					trigger = true
				}
				if trigger {
					if value == "BEGIN" || value == "CASE" {
						bodyDepth++
					} else if value == "END" && bodyDepth > 0 {
						bodyDepth--
					}
				}
			}
			i = j
			continue
		}
		i++
	}
	if bodyDepth != 0 {
		return nil, fmt.Errorf("line %d: unterminated trigger body", statementLine)
	}
	if hasCode {
		if dialect == "pgx" && copyFromStdin {
			return nil, fmt.Errorf("line %d: COPY FROM STDIN requires a semicolon and data block", statementLine)
		}
		result = append(result, Statement{SQL: strings.TrimSpace(input[start:]), Line: statementLine})
	}
	return result, nil
}
