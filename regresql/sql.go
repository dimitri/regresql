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

/*

A query instances represents an SQL query, read from Path filename and
stored raw as the Text slot. The query text is "parsed" into the Query slot,
and parameters are extracted into both the Vars slot and the Params slot.

    SELECT * FROM foo WHERE a = :a and b between :a and :b;

In the previous query, we would have Vars = [a b] and Params = [a a b].
*/
type Query struct {
	Path     string
	Text     string            // original query text
	Query    string            // "normalized" SQL query for lib/pq
	Vars     []string          // unique variable names used in the query
	Params   []string          // ordered list of params used in the query
	Defaults map[string]string // variable defaults from \set metacommands
}

// extractSetCommands scans text for \set metacommand lines, removes them from
// the SQL text, and returns both the cleaned text and a map of variable
// defaults parsed from those lines.
//
// Parsing follows psql token semantics: tokens are concatenated without any
// separator. Single-quoted tokens have their outer quotes stripped and psql
// escape sequences expanded. Double-quoted tokens are kept verbatim
// (including their surrounding double-quotes). Unquoted tokens are taken
// as-is.
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

// parseSetTokens parses the value portion of a \set line (everything after
// the variable name) according to psql tokenisation rules and returns the
// concatenated result string.
//
// Supported token forms (concatenated without separator, matching psql):
//   - 'text'  -- outer quotes stripped; escape sequences expanded:
//               ''->',  \n->LF, \t->TAB, \b->BS, \r->CR, \f->FF,
//               \NNN->octal byte, \xHH->hex byte, \.->literal char
//   - "text"  -- kept verbatim including the surrounding double-quotes
//   - word    -- any run of non-whitespace, non-quote characters
func parseSetTokens(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		switch {
		case s[i] == ' ' || s[i] == '\t':
			i++ // skip inter-token whitespace

		case s[i] == '\'':
			// single-quoted token: strip outer quotes, expand escapes
			i++ // skip opening quote
			for i < len(s) {
				if s[i] == '\'' {
					i++ // consume the quote
					if i < len(s) && s[i] == '\'' {
						// '' inside single-quotes -> literal single quote
						b.WriteByte('\'')
						i++
						continue
					}
					break // closing quote
				}
				if s[i] == '\\' && i+1 < len(s) {
					i++ // skip backslash
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
						// \NNN octal escape (up to 3 digits)
						val := 0
						for j := 0; j < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7'; j++ {
							val = val*8 + int(s[i]-'0')
							i++
						}
						b.WriteByte(byte(val))
					case s[i] == 'x' || s[i] == 'X':
						// \xHH hex escape (up to 2 hex digits)
						i++ // skip 'x'
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
						// \. -> literal char (e.g. \' -> ')
						b.WriteByte(s[i])
						i++
					}
				} else {
					b.WriteByte(s[i])
					i++
				}
			}

		case s[i] == '"':
			// double-quoted token: kept verbatim including the outer quotes
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
			// unquoted token: read until whitespace or a quote character
			for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\'' && s[i] != '"' {
				b.WriteByte(s[i])
				i++
			}
		}
	}
	return b.String()
}

// Parse a SQL file and returns a Query instance, with variables used in the
// query separated in the Query.Vars map.
func parseQueryFile(queryPath string) (*Query, error) {
	sqlbytes, err := ioutil.ReadFile(queryPath)
	if err != nil {
		var q *Query
		e := fmt.Errorf(
			"Failed to parse query file '%s': %s\n",
			queryPath,
			err)
		return q, e
	}
	queryString := string(sqlbytes)

	return parseQueryString(queryPath, queryString), nil
}

// let's consider as an example the following SQL query:
//
//    select * from foo where a = :a and b between :a and :b
//
// which gets rewritten
//
//    select * from foo where a = $1 and b between $1 and $2
//
// then we have:  mapv = {a: $1, b: $2}
// and we want: params = [a a b]
//
// the idea is that then we can replace the param names by their values
// thanks to the plan test bindings given by the user (see p.Execute)
func parseQueryString(queryPath string, queryString string) *Query {
	// Strip \set metacommand lines from the SQL text and collect their
	// variable defaults.  The original queryString is kept as Text so the
	// caller can inspect it; only the cleaned form is used for parsing and
	// execution.
	cleaned, defaults := extractSetCommands(queryString)

	// find all uses of variables in the cleaned SQL query text, and put
	// them in a map so that we get each of them only once, even when used
	// several times in the same query
	params := make([]string, 0)
	vars := make([]string, 0)

	r, _ := regexp.Compile(psqlVarRE)
	uses := r.FindAllStringSubmatch(cleaned, -1)

	// now compute the map of variable names (mapv)
	mapv := make(map[string]int)
	i := 1

	for _, match := range uses {
		varname := match[1]
		params = append(params, varname)
		if _, found := mapv[varname]; !found {
			mapv[varname] = i
			i++
		}
	}

	// now compute the normalized SQL query, with ordinal markers ($1,
	// $2, ...) as expected by the lib/pq driver.
	sql := cleaned

	for name, ord := range mapv {
		vars = append(vars, name)
		r, _ := regexp.Compile(fmt.Sprintf(`:["']?%s["']?`, name))
		sql = r.ReplaceAllLiteralString(sql, fmt.Sprintf("$%d", ord))
	}

	// now build and return our Query
	return &Query{queryPath, queryString, sql, vars, params, defaults}
}

// Prepare resolves query parameters from bindings and returns the normalized
// SQL string together with the ordered argument slice ready for database/sql.
//
// For each parameter the value is looked up first in bindings (the YAML plan
// for the current test case), then in q.Defaults (populated from \set
// metacommands).  An error is returned if any parameter cannot be resolved
// through either source.
func (q *Query) Prepare(bindings map[string]string) (string, []interface{}, error) {
	params := make([]interface{}, len(q.Params))

	for i, varname := range q.Params {
		if val, ok := bindings[varname]; ok {
			params[i] = val
		} else if val, ok := q.Defaults[varname]; ok {
			params[i] = val
		} else {
			return "", nil, fmt.Errorf(
				"missing value for parameter %q (not in plan bindings or \\set defaults)",
				varname)
		}
	}
	return q.Query, params, nil
}
