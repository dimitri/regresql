package regresql

import (
	"testing"
)

func TestParseQueryString(t *testing.T) {
	queryString := `select * from foo where id = :id`
	q := parseQueryString("no/path", queryString)

	if len(q.Vars) != 1 || q.Vars[0] != "id" {
		t.Error("Expected [\"id\"], got ", q.Vars)
	}
}

func TestPrepareOneParam(t *testing.T) {
	queryString := `select * from foo where id = :id`
	q := parseQueryString("no/path", queryString)
	b := make(map[string]string)
	b["id"] = "1"

	sql, params := q.Prepare(b)

	if sql != "select * from foo where id = $1" {
		t.Error("Query string not as expected ", sql)
	}

	if !(len(params) == 1 &&
		params[0] == "1") {
		t.Error("Bindings not properly applied, got ", params)
	}
}

func TestPrepareTwoParams(t *testing.T) {
	queryString := `select * from foo where a = :a and b between :a and :b`
	q := parseQueryString("no/path", queryString)
	b := make(map[string]string)
	b["a"] = "a"
	b["b"] = "b"

	sql, params := q.Prepare(b)

	if sql != "select * from foo where a = $1 and b between $1 and $2" {
		t.Error("Query string not as expected ", sql)
	}

	if !(len(params) == 3 &&
		params[0] == "a" &&
		params[1] == "a" &&
		params[2] == "b") {
		t.Error("Bindings not properly applied, got ", params)
	}
}
