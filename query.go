package main

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/godror/godror"
)

func connectOracle(loginStr string) (*sql.DB, error) {
	dsn, err := buildDSN(loginStr)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("godror", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("login failed: %w", err)
	}
	return db, nil
}

// buildDSN converts "user/password[@connectString]" to godror DSN format.
func buildDSN(loginStr string) (string, error) {
	slash := strings.Index(loginStr, "/")
	if slash < 0 {
		return "", fmt.Errorf("login must be user/password[@connectString]")
	}
	user := loginStr[:slash]
	rest := loginStr[slash+1:]

	var password, connectString string
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		password = rest[:at]
		connectString = rest[at+1:]
	} else {
		password = rest
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		return strings.ReplaceAll(s, `"`, `\"`)
	}
	dsn := fmt.Sprintf(`user="%s" password="%s"`, escape(user), escape(password))
	if connectString != "" {
		dsn += fmt.Sprintf(` connectString="%s"`, escape(connectString))
	}
	return dsn, nil
}

// bindVarRe matches Oracle-style bind variables: named (:name) and
// positional/numeric (:1, :2, ...).
var bindVarRe = regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*|[0-9]+)`)

// isNumericName reports whether a bind name consists only of digits (:1, :2).
func isNumericName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// extractBindVars returns unique bind variable names (without leading ':')
// in order of first appearance, skipping occurrences inside string literals,
// quoted identifiers, and comments.
func extractBindVars(sqlStr string) []string {
	stripped := stripNonCode(sqlStr)
	matches := bindVarRe.FindAllString(stripped, -1)
	seen := make(map[string]bool)
	var names []string
	for _, m := range matches {
		upper := strings.ToUpper(m[1:])
		if !seen[upper] {
			seen[upper] = true
			names = append(names, m[1:])
		}
	}
	return names
}

// stripNonCode blanks out regions that must not be scanned for bind variables:
// single-quoted string literals (e.g. the ':MI'/':SS' in a 'HH24:MI:SS' date
// mask), double-quoted identifiers, line comments (-- ...), and block comments
// (/* ... */). Blanked bytes are replaced with spaces so byte offsets and the
// surrounding SQL structure are preserved.
func stripNonCode(s string) string {
	out := []byte(s)
	n := len(s)
	blank := func(from, to int) {
		for j := from; j < to && j < n; j++ {
			if out[j] != '\n' {
				out[j] = ' '
			}
		}
	}
	i := 0
	for i < n {
		switch {
		case s[i] == '\'': // single-quoted string literal ('' escapes a quote)
			j := i + 1
			for j < n {
				if s[j] == '\'' {
					if j+1 < n && s[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			blank(i, j)
			i = j
		case s[i] == '"': // double-quoted identifier
			j := i + 1
			for j < n && s[j] != '"' {
				j++
			}
			if j < n {
				j++ // include closing quote
			}
			blank(i, j)
			i = j
		case i+1 < n && s[i] == '-' && s[i+1] == '-': // line comment
			j := i + 2
			for j < n && s[j] != '\n' {
				j++
			}
			blank(i, j)
			i = j
		case i+1 < n && s[i] == '/' && s[i+1] == '*': // block comment
			j := i + 2
			for j+1 < n && !(s[j] == '*' && s[j+1] == '/') {
				j++
			}
			if j+1 < n {
				j += 2 // include closing */
			} else {
				j = n
			}
			blank(i, j)
			i = j
		default:
			i++
		}
	}
	return string(out)
}

// execQuery executes sqlStr against db, writes CSV rows to w, and returns
// (noData, error). noData is true when the query returned zero rows.
func execQuery(ctx context.Context, db *sql.DB, cfg *Config, sqlStr string, w *csvWriter) (bool, error) {
	bindNames := extractBindVars(sqlStr)

	args := make([]interface{}, 0, len(bindNames)+1)
	for i, name := range bindNames {
		val := ""
		if i < len(cfg.BindValues) {
			val = cfg.BindValues[i]
		}
		// Numeric/positional binds (:1, :2) cannot be bound by name (godror
		// requires names to start with a letter), so pass them positionally.
		if isNumericName(name) {
			args = append(args, val)
		} else {
			args = append(args, sql.Named(name, val))
		}
	}
	// godror.FetchRowCount sets the Oracle array fetch size, equivalent to
	// "EXEC SQL FOR :FETCH_ARRAY FETCH" in the original Pro*C code.
	args = append(args, godror.FetchRowCount(cfg.FetchArray))

	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return false, fmt.Errorf("execute: %w", err)
	}
	defer rows.Close()

	cols, err := rows.ColumnTypes()
	if err != nil {
		return false, err
	}

	isNum := make([]bool, len(cols))
	for i, c := range cols {
		switch c.DatabaseTypeName() {
		case "NUMBER", "FLOAT", "BINARY_FLOAT", "BINARY_DOUBLE", "INTEGER":
			isNum[i] = true
		}
	}

	if cfg.Header {
		names := make([]string, len(cols))
		for i, c := range cols {
			names[i] = c.Name()
		}
		w.writeHeader(names, isNum)
	}

	vals := make([]sql.NullString, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	strs := make([]string, len(cols))

	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return false, fmt.Errorf("scan: %w", err)
		}
		for i, v := range vals {
			if v.Valid {
				strs[i] = v.String
			} else {
				strs[i] = ""
			}
		}
		w.writeRow(strs, isNum)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	return rowCount == 0, nil
}
