package dbmigrate

import (
	"fmt"
	"regexp"
	"strings"
)

var dollarQuote = regexp.MustCompile(`^\$(?:[A-Za-z_][A-Za-z0-9_]*)?\$`)
var transactionControl = regexp.MustCompile(`(?i)^(BEGIN|START|COMMIT|END|ROLLBACK|ABORT|PREPARE|SAVEPOINT|RELEASE|DISCARD)\b`)

// This lexer only protects the runner's transaction boundary; PostgreSQL parses
// the actual SQL. Migrations are trusted, reviewed source, not a SQL sandbox.
// Mask quoted bodies/comments so function BEGIN/END and literal semicolons do
// not look like top-level transaction commands. Standard strings use PostgreSQL's
// standard_conforming_strings=on; E'...' strings additionally escape backslashes.
func validateSQL(sql string) error {
	var top strings.Builder
	for i := 0; i < len(sql); {
		switch {
		case strings.HasPrefix(sql[i:], "--"):
			end := strings.IndexByte(sql[i:], '\n')
			if end < 0 {
				i = len(sql)
			} else {
				i += end + 1
			}
			top.WriteByte(' ')
		case strings.HasPrefix(sql[i:], "/*"):
			depth := 1
			i += 2
			for i < len(sql) && depth > 0 {
				switch {
				case strings.HasPrefix(sql[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(sql[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth != 0 {
				return fmt.Errorf("unterminated SQL comment")
			}
			top.WriteByte(' ')
		case sql[i] == '\'' || sql[i] == '"':
			quote := sql[i]
			escaped := quote == '\'' && i > 0 && (sql[i-1] == 'e' || sql[i-1] == 'E') && (i == 1 || !identifierByte(sql[i-2]))
			i++
			closed := false
			for i < len(sql) {
				if escaped && sql[i] == '\\' {
					i += 2
				} else if sql[i] == quote {
					i++
					if i < len(sql) && sql[i] == quote {
						i++
						continue
					}
					closed = true
					break
				} else {
					i++
				}
			}
			if !closed {
				return fmt.Errorf("unterminated SQL quote")
			}
			top.WriteString(" literal ")
		case sql[i] == '$' && (i == 0 || !identifierByte(sql[i-1])):
			tag := dollarQuote.FindString(sql[i:])
			if tag == "" {
				top.WriteByte(sql[i])
				i++
				continue
			}
			i += len(tag)
			end := strings.Index(sql[i:], tag)
			if end < 0 {
				return fmt.Errorf("unterminated dollar-quoted SQL")
			}
			i += end + len(tag)
			top.WriteString(" literal ")
		case sql[i] == '\\' || sql[i] == 0:
			return fmt.Errorf("psql commands and NUL bytes are unsupported")
		default:
			top.WriteByte(sql[i])
			i++
		}
	}
	statements := 0
	for _, statement := range strings.Split(top.String(), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		statements++
		if transactionControl.MatchString(statement) {
			return fmt.Errorf("explicit transaction control is unsupported")
		}
	}
	if statements == 0 {
		return fmt.Errorf("Up section contains no SQL")
	}
	return nil
}

func identifierByte(b byte) bool {
	return b == '_' || b == '$' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b >= 128
}
