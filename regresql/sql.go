package regresql

import (
	"fmt"
	"io/ioutil"
	"regexp"
)

// Regular Expression to find query parameters in SQL query files, as per
// the psql support for variables:
// https://www.postgresql.org/docs/9.6/static/app-psql.html#APP-PSQL-VARIABLES
const (
	psqlVarRE = `[^:]:['"]?([A-Za-z][A-Za-z0-9_]*)['"]?`
)

/*

A query instances represents an SQL query, read from Path filename and
stored raw as the Text slot. The query text is “parsed” into the Query slot,
and parameters are extracted into both the Vars slot and the Params slot.

    SELECT * FROM foo WHERE a = :a and b between :a and :b;

In the previous query, we would have Vars = [a b] and Params = [a a b].
*/
type Query struct {
	Path   string
	Text   string   // original query text
	Query  string   // "normalized" SQL query for lib/pq
	Vars   []string // variable names used in the query text
	Params []string // ordered list of params used in the query
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
	// find a uses of variables in the SQL query text, and put then in a
	// map so that we get each of them only once, even when used several
	// times in the same query
	params := make([]string, 0)
	vars := make([]string, 0)

	r, _ := regexp.Compile(psqlVarRE)
	uses := r.FindAllStringSubmatch(queryString, -1)

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
	sql := string(queryString)

	for name, ord := range mapv {
		vars = append(vars, name)
		r, _ := regexp.Compile(fmt.Sprintf(`:["']?%s["']?`, name))
		sql = r.ReplaceAllLiteralString(sql, fmt.Sprintf("$%d", ord))
	}

	// now build and return our Query
	return &Query{queryPath, queryString, sql, vars, params}
}

// Prepare an args... interface{} for Query from given bindings
func (q *Query) Prepare(bindings map[string]string) (string, []interface{}) {
	params := make([]interface{}, len(q.Params))

	for i, varname := range q.Params {
		params[i] = bindings[varname]
	}
	return q.Query, params
}
