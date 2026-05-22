package regresql

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

// Regular Expression to find query parameters in SQL query files, as per
// the psql support for variables:
// https://www.postgresql.org/docs/9.6/static/app-psql.html#APP-PSQL-VARIABLES
const (
	psqlVarRE = `[^:]:['"]?([A-Za-z][A-Za-z0-9_]*)['"]?`
)

// setLineRE matches a psql \set metacommand line, capturing the variable name
// (group 1) and the rest-of-line value tokens (group 2).
var setLineRE = regexp.MustCompile(`(?m)^[ \t]*\\set[ \t]+([A-Za-z_][A-Za-z0-9_]*)[ \t]*([^\r\n]*)\r?\n?`)

// bindLineRE matches a psql \bind metacommand line, capturing the rest-of-line
// value tokens (group 1).
var bindLineRE = regexp.MustCompile(`(?m)^[ \t]*\\bind[ \t]*([^\r\n]*)\r?\n?`)

/*

A Query represents an SQL query read from Path. Text holds the original source,
Query holds the normalised SQL sent to PostgreSQL, and Vars / Params describe
the query parameters.

Named mode (:varname):

    SELECT * FROM foo WHERE a = :a and b between :a and :b
    -> Vars   = ["a", "b"]
    -> Params = ["a", "a", "b"]   (ordered occurrence list)
    -> Query  = "SELECT * FROM foo WHERE a = $1 and b between $1 and $2"

Positional mode ($N):

    SELECT $1::int + $2::int
    -> Vars   = ["p1", "p2"]
    -> Params = ["p1", "p2"]
    -> Query  = "SELECT $1::int + $2::int"   (unchanged)
    -> Positional = true
*/
type Query struct {
	Path         string
	Text         string            // original query text (including \set / \bind lines)
	Query        string            // normalised SQL for lib/pq
	Vars         []string          // unique variable names
	Params       []string          // ordered parameter list
	Defaults     map[string]string // defaults from \set (named mode)
	BindDefaults []string          // defaults from \bind (positional mode, 0-indexed: [0]=val for $1)
	Positional   bool              // true when using $N style
}

// ── \set support (unchanged) ─────────────────────────────────────────────────

// extractSetCommands scans text for \set metacommand lines, removes them from
// the SQL text, and returns both the cleaned text and a map of variable
// defaults parsed from those lines.
func extractSetCommands(text string) (string, map[string]string) {
	defaults := make(map[string]string)

	for _, m := range setLineRE.FindAllStringSubmatch(text, -1) {
		name := m[1]
		rawValue := strings.TrimRight(m[2], " \t")
		defaults[name] = parseSetTokens(rawValue)
	}

	cleaned := setLineRE.ReplaceAllString(text, "")
	return cleaned, defaults
}

// parseSetTokens parses the value portion of a \set line according to psql
// tokenisation rules and returns the concatenated result string.
func parseSetTokens(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		switch {
		case s[i] == ' ' || s[i] == '\t':
			i++

		case s[i] == '\'':
			i++
			for i < len(s) {
				if s[i] == '\'' {
					i++
					if i < len(s) && s[i] == '\'' {
						b.WriteByte('\'')
						i++
						continue
					}
					break
				}
				if s[i] == '\\' && i+1 < len(s) {
					i++
					switch {
					case s[i] == 'n':
						b.WriteByte('\n')
						i++
					case s[i] == 't':
						b.WriteByte('\t')
						i++
					case s[i] == 'b':
						b.WriteByte('\b')
						i++
					case s[i] == 'r':
						b.WriteByte('\r')
						i++
					case s[i] == 'f':
						b.WriteByte('\f')
						i++
					case s[i] >= '0' && s[i] <= '7':
						val := 0
						for j := 0; j < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7'; j++ {
							val = val*8 + int(s[i]-'0')
							i++
						}
						b.WriteByte(byte(val))
					case s[i] == 'x' || s[i] == 'X':
						i++
						val := 0
						for j := 0; j < 2 && i < len(s); j++ {
							c := s[i]
							if c >= '0' && c <= '9' {
								val = val*16 + int(c-'0')
								i++
							} else if c >= 'a' && c <= 'f' {
								val = val*16 + int(c-'a'+10)
								i++
							} else if c >= 'A' && c <= 'F' {
								val = val*16 + int(c-'A'+10)
								i++
							} else {
								break
							}
						}
						b.WriteByte(byte(val))
					default:
						b.WriteByte(s[i])
						i++
					}
				} else {
					b.WriteByte(s[i])
					i++
				}
			}

		case s[i] == '"':
			b.WriteByte('"')
			i++
			for i < len(s) {
				b.WriteByte(s[i])
				if s[i] == '"' {
					i++
					break
				}
				i++
			}

		default:
			for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\'' && s[i] != '"' {
				b.WriteByte(s[i])
				i++
			}
		}
	}
	return b.String()
}

// ── \bind support ─────────────────────────────────────────────────────────────

// extractBindCommand scans text for \bind metacommand lines, removes them,
// and returns the cleaned text together with the parameter values from the
// last \bind line found (last one wins).
func extractBindCommand(text string) (string, []string) {
	var lastVals []string

	for _, m := range bindLineRE.FindAllStringSubmatch(text, -1) {
		rawValue := strings.TrimRight(m[1], " \t")
		lastVals = parseBindTokens(rawValue)
	}

	cleaned := bindLineRE.ReplaceAllString(text, "")
	return cleaned, lastVals
}

// parseBindTokens parses the value portion of a \bind line according to psql
// tokenisation rules and returns each token as a separate element of the
// returned slice (one element per positional parameter).
//
// Token forms follow the same rules as \set:
//   - 'text'  -- outer quotes stripped; psql escape sequences expanded
//   - "text"  -- kept verbatim including the surrounding double-quotes
//   - word    -- any run of non-whitespace, non-quote characters
func parseBindTokens(s string) []string {
	var result []string
	i := 0
	for i < len(s) {
		switch {
		case s[i] == ' ' || s[i] == '\t':
			i++

		case s[i] == '\'':
			var b strings.Builder
			i++
			for i < len(s) {
				if s[i] == '\'' {
					i++
					if i < len(s) && s[i] == '\'' {
						b.WriteByte('\'')
						i++
						continue
					}
					break
				}
				if s[i] == '\\' && i+1 < len(s) {
					i++
					switch {
					case s[i] == 'n':
						b.WriteByte('\n')
						i++
					case s[i] == 't':
						b.WriteByte('\t')
						i++
					case s[i] == 'b':
						b.WriteByte('\b')
						i++
					case s[i] == 'r':
						b.WriteByte('\r')
						i++
					case s[i] == 'f':
						b.WriteByte('\f')
						i++
					case s[i] >= '0' && s[i] <= '7':
						val := 0
						for j := 0; j < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7'; j++ {
							val = val*8 + int(s[i]-'0')
							i++
						}
						b.WriteByte(byte(val))
					case s[i] == 'x' || s[i] == 'X':
						i++
						val := 0
						for j := 0; j < 2 && i < len(s); j++ {
							c := s[i]
							if c >= '0' && c <= '9' {
								val = val*16 + int(c-'0')
								i++
							} else if c >= 'a' && c <= 'f' {
								val = val*16 + int(c-'a'+10)
								i++
							} else if c >= 'A' && c <= 'F' {
								val = val*16 + int(c-'A'+10)
								i++
							} else {
								break
							}
						}
						b.WriteByte(byte(val))
					default:
						b.WriteByte(s[i])
						i++
					}
				} else {
					b.WriteByte(s[i])
					i++
				}
			}
			result = append(result, b.String())

		case s[i] == '"':
			var b strings.Builder
			b.WriteByte('"')
			i++
			for i < len(s) {
				b.WriteByte(s[i])
				if s[i] == '"' {
					i++
					break
				}
				i++
			}
			result = append(result, b.String())

		default:
			var b strings.Builder
			for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\'' && s[i] != '"' {
				b.WriteByte(s[i])
				i++
			}
			if b.Len() > 0 {
				result = append(result, b.String())
			}
		}
	}
	return result
}

// ── Positional $N scanner ─────────────────────────────────────────────────────

// scanPositionalParams scans SQL text for $N parameter references (N >= 1)
// that appear outside of string literals, comments, and dollar-quoted blocks.
// It returns the highest parameter index found (maxN), or 0 if none.
//
// The scanner implements a minimal SQL state machine with states:
//   - normal
//   - single-quoted string  (handles '' escape)
//   - line comment          (-- to newline)
//   - block comment         (/* ... */ with nesting support)
//   - dollar-quoted string  ($tag$...$tag$ or $$...$$)
func scanPositionalParams(text string) int {
	const (
		stNormal = iota
		stSingleQuote
		stLineComment
		stBlockComment
		stDollarQuote
	)

	maxN := 0
	state := stNormal
	blockDepth := 0
	dollarTag := ""

	n := len(text)
	i := 0
	for i < n {
		switch state {
		case stNormal:
			switch {
			case text[i] == '\'':
				state = stSingleQuote
				i++

			case i+1 < n && text[i] == '-' && text[i+1] == '-':
				state = stLineComment
				i += 2

			case i+1 < n && text[i] == '/' && text[i+1] == '*':
				state = stBlockComment
				blockDepth = 1
				i += 2

			case text[i] == '$':
				if i+1 < n {
					next := text[i+1]
					switch {
					case next >= '1' && next <= '9':
						// positional parameter $N
						i++ // skip '$'
						val := 0
						for i < n && text[i] >= '0' && text[i] <= '9' {
							val = val*10 + int(text[i]-'0')
							i++
						}
						if val > maxN {
							maxN = val
						}

					case next == '$':
						// $$ — empty-tag dollar-quote
						dollarTag = ""
						state = stDollarQuote
						i += 2

					case isSQLIdentStart(next):
						// $tag$ — read identifier tag
						i++ // skip '$'
						tagStart := i
						for i < n && isSQLIdentCont(text[i]) {
							i++
						}
						if i < n && text[i] == '$' {
							// valid dollar-quote opening
							dollarTag = text[tagStart:i]
							state = stDollarQuote
							i++ // skip closing '$' of opening tag
						}
						// else: $ident not followed by $ — not a dollar-quote, ignore

					default:
						i++ // bare $, $0, etc. — ignore
					}
				} else {
					i++ // lone $ at end of text
				}

			default:
				i++
			}

		case stSingleQuote:
			if text[i] == '\'' {
				if i+1 < n && text[i+1] == '\'' {
					i += 2 // '' escape — stay in string
				} else {
					state = stNormal
					i++
				}
			} else {
				i++
			}

		case stLineComment:
			if text[i] == '\n' {
				state = stNormal
			}
			i++

		case stBlockComment:
			switch {
			case i+1 < n && text[i] == '/' && text[i+1] == '*':
				blockDepth++
				i += 2
			case i+1 < n && text[i] == '*' && text[i+1] == '/':
				blockDepth--
				i += 2
				if blockDepth == 0 {
					state = stNormal
				}
			default:
				i++
			}

		case stDollarQuote:
			closing := "$" + dollarTag + "$"
			if i+len(closing) <= n && text[i:i+len(closing)] == closing {
				state = stNormal
				i += len(closing)
			} else {
				i++
			}
		}
	}
	return maxN
}

func isSQLIdentStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func isSQLIdentCont(c byte) bool {
	return isSQLIdentStart(c) || (c >= '0' && c <= '9')
}

// ── Query parsing ─────────────────────────────────────────────────────────────

// parseQueryFile reads a SQL file and returns a Query instance.
func parseQueryFile(queryPath string) (*Query, error) {
	sqlbytes, err := ioutil.ReadFile(queryPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse query file '%s': %s\n", queryPath, err)
	}
	return parseQueryString(queryPath, string(sqlbytes))
}

// parseQueryString parses a SQL string (previously read from queryPath) and
// returns a Query instance, or an error if the file mixes :varname and $N
// parameter styles.
//
// Named mode (:varname):
//
//	select * from foo where a = :a and b between :a and :b
//	->  Query = "select * from foo where a = $1 and b between $1 and $2"
//	    Vars  = ["a","b"], Params = ["a","a","b"]
//
// Positional mode ($N):
//
//	SELECT $1::int + $2::int
//	->  Query = "SELECT $1::int + $2::int"  (unchanged)
//	    Vars  = ["p1","p2"], Params = ["p1","p2"], Positional = true
func parseQueryString(queryPath string, queryString string) (*Query, error) {
	// Strip \set metacommands; collect named-param defaults.
	afterSet, setDefaults := extractSetCommands(queryString)

	// Strip \bind metacommands; collect positional defaults.
	cleanedSQL, bindDefaults := extractBindCommand(afterSet)

	// ── Detect parameter style ───────────────────────────────────────────────

	// Named params scan
	r, _ := regexp.Compile(psqlVarRE)
	namedUses := r.FindAllStringSubmatch(cleanedSQL, -1)
	seenNames := make(map[string]bool)
	var namedVars []string
	for _, match := range namedUses {
		vname := match[1]
		if !seenNames[vname] {
			seenNames[vname] = true
			namedVars = append(namedVars, vname)
		}
	}

	// Positional params scan
	maxN := scanPositionalParams(cleanedSQL)

	// Mixed-mode error
	if maxN > 0 && len(namedVars) > 0 {
		positionalLabels := make([]string, maxN)
		for idx := 1; idx <= maxN; idx++ {
			positionalLabels[idx-1] = fmt.Sprintf("$%d", idx)
		}
		namedLabels := make([]string, len(namedVars))
		for idx, v := range namedVars {
			namedLabels[idx] = ":" + v
		}
		return nil, fmt.Errorf(
			"mixed parameter styles in %q:\n  named variables: %s\n  positional parameters: %s\nUse either :varname or $N, not both.",
			queryPath,
			strings.Join(namedLabels, ", "),
			strings.Join(positionalLabels, ", "))
	}

	// ── Positional mode ──────────────────────────────────────────────────────
	if maxN > 0 {
		vars := make([]string, maxN)
		params := make([]string, maxN)
		for idx := 0; idx < maxN; idx++ {
			vars[idx] = fmt.Sprintf("p%d", idx+1)
			params[idx] = vars[idx]
		}
		return &Query{
			Path:         queryPath,
			Text:         queryString,
			Query:        cleanedSQL,
			Vars:         vars,
			Params:       params,
			Defaults:     setDefaults,
			BindDefaults: bindDefaults,
			Positional:   true,
		}, nil
	}

	// ── Named mode (existing logic) ──────────────────────────────────────────
	params := make([]string, 0)
	vars := make([]string, 0)

	mapv := make(map[string]int)
	ordinal := 1

	for _, match := range namedUses {
		vname := match[1]
		params = append(params, vname)
		if _, found := mapv[vname]; !found {
			mapv[vname] = ordinal
			ordinal++
		}
	}

	// Build normalised SQL with $N placeholders.
	sql := cleanedSQL
	for name, ord := range mapv {
		vars = append(vars, name)
		re, _ := regexp.Compile(fmt.Sprintf(`:["']?%s["']?`, name))
		sql = re.ReplaceAllLiteralString(sql, fmt.Sprintf("$%d", ord))
	}

	return &Query{
		Path:     queryPath,
		Text:     queryString,
		Query:    sql,
		Vars:     vars,
		Params:   params,
		Defaults: setDefaults,
	}, nil
}

// ── Prepare ───────────────────────────────────────────────────────────────────

// Prepare resolves query parameters from bindings and returns the normalised
// SQL string together with the ordered argument slice ready for database/sql.
//
// Resolution order for each parameter:
//  1. bindings[varname]   — YAML plan value (always wins)
//  2. q.BindDefaults[i]   — \bind default  (positional mode, index i)
//     q.Defaults[varname] — \set default   (named mode)
//  3. error               — parameter unresolvable
func (q *Query) Prepare(bindings map[string]string) (string, []interface{}, error) {
	params := make([]interface{}, len(q.Params))

	for i, varname := range q.Params {
		if val, ok := bindings[varname]; ok {
			params[i] = val
		} else if q.Positional && i < len(q.BindDefaults) {
			params[i] = q.BindDefaults[i]
		} else if val, ok := q.Defaults[varname]; ok {
			params[i] = val
		} else {
			return "", nil, fmt.Errorf(
				"missing value for parameter %q (not in plan bindings or \\bind/\\set defaults)",
				varname)
		}
	}
	return q.Query, params, nil
}
