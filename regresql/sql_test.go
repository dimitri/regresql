package regresql

import (
	"strings"
	"testing"
)

func TestParseQueryString(t *testing.T) {
	queryString := `select * from foo where id = :user_id`
	q := parseQueryString("no/path", queryString)

	if len(q.Vars) != 1 || q.Vars[0] != "user_id" {
		t.Error("Expected [\"user_id\"], got ", q.Vars)
	}
}

func TestParseQueryStringWithTypeCast(t *testing.T) {
	queryString := `select name::text from foo where id = :user_id`
	q := parseQueryString("no/path", queryString)

	if len(q.Vars) != 1 || q.Vars[0] != "user_id" {
		t.Error("Expected only [\"user_id\"], got ", q.Vars)
	}
}

func TestPrepareOneParam(t *testing.T) {
	queryString := `select * from foo where id = :id`
	q := parseQueryString("no/path", queryString)
	b := make(map[string]string)
	b["id"] = "1"

	sql, params, err := q.Prepare(b)

	if err != nil {
		t.Fatal("Unexpected error from Prepare:", err)
	}
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

	sql, params, err := q.Prepare(b)

	if err != nil {
		t.Fatal("Unexpected error from Prepare:", err)
	}
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

// ── \set parsing tests ──────────────────────────────────────────────────────

// TestSetUnquoted checks that a bare \set line is parsed and removed from q.Query.
func TestSetUnquoted(t *testing.T) {
	queryString := "\\set n 10\nSELECT :n::int;\n"
	q := parseQueryString("no/path", queryString)

	if v, ok := q.Defaults["n"]; !ok || v != "10" {
		t.Errorf("Expected Defaults[\"n\"]==\"10\", got %q (ok=%v)", v, ok)
	}
	// \set line must be absent from the normalized query sent to PostgreSQL
	if strings.Contains(q.Query, `\set`) {
		t.Error("\\set line should be stripped from q.Query, but is still present")
	}
}

// TestSetSingleQuotedEscapes checks that single-quoted \set values have their
// outer quotes stripped and psql escape sequences expanded.
func TestSetSingleQuotedEscapes(t *testing.T) {
	queryString := `\set s 'hello\nworld'` + "\nSELECT 1;\n"
	q := parseQueryString("no/path", queryString)

	want := "hello\nworld"
	if v := q.Defaults["s"]; v != want {
		t.Errorf("Expected Defaults[\"s\"]==%q, got %q", want, v)
	}
}

// TestSetSingleQuotedDoubleApostrophe checks '' → ' inside single-quoted values.
func TestSetSingleQuotedDoubleApostrophe(t *testing.T) {
	queryString := `\set s 'it''s'` + "\nSELECT 1;\n"
	q := parseQueryString("no/path", queryString)

	want := "it's"
	if v := q.Defaults["s"]; v != want {
		t.Errorf("Expected Defaults[\"s\"]==%q, got %q", want, v)
	}
}

// TestSetDoubleQuoted checks that double-quoted tokens are kept verbatim
// including their surrounding double-quotes.
func TestSetDoubleQuoted(t *testing.T) {
	queryString := `\set s "hello"` + "\nSELECT 1;\n"
	q := parseQueryString("no/path", queryString)

	want := `"hello"`
	if v := q.Defaults["s"]; v != want {
		t.Errorf("Expected Defaults[\"s\"]==%q, got %q", want, v)
	}
}

// TestSetMultiTokenConcatenation checks that multiple tokens are joined without
// any separator (psql concatenation semantics).
func TestSetMultiTokenConcatenation(t *testing.T) {
	queryString := `\set x a 'b' c` + "\nSELECT 1;\n"
	q := parseQueryString("no/path", queryString)

	want := "abc"
	if v := q.Defaults["x"]; v != want {
		t.Errorf("Expected Defaults[\"x\"]==%q, got %q", want, v)
	}
}

// TestSetNoValue checks that \set NAME with no value stores an empty string.
func TestSetNoValue(t *testing.T) {
	queryString := "\\set EMPTY\nSELECT 1;\n"
	q := parseQueryString("no/path", queryString)

	if v, ok := q.Defaults["EMPTY"]; !ok || v != "" {
		t.Errorf("Expected Defaults[\"EMPTY\"]==\"\", got %q (ok=%v)", v, ok)
	}
}

// TestSetUnusedVariable checks that a \set variable not referenced by any :var
// is still stored in q.Defaults and is NOT added to q.Vars.
func TestSetUnusedVariable(t *testing.T) {
	queryString := "\\set unused foo\nSELECT 1;\n"
	q := parseQueryString("no/path", queryString)

	if v, ok := q.Defaults["unused"]; !ok || v != "foo" {
		t.Errorf("Expected Defaults[\"unused\"]==\"foo\", got %q (ok=%v)", v, ok)
	}
	for _, v := range q.Vars {
		if v == "unused" {
			t.Error("\"unused\" should not appear in q.Vars")
		}
	}
}

// TestPrepareDefaultFallback checks that Prepare uses q.Defaults when the
// variable is absent from the binding map.
func TestPrepareDefaultFallback(t *testing.T) {
	queryString := "\\set n 10\nSELECT :n::int;\n"
	q := parseQueryString("no/path", queryString)

	_, params, err := q.Prepare(map[string]string{})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if len(params) != 1 || params[0] != "10" {
		t.Errorf("Expected params==[\"10\"], got %v", params)
	}
}

// TestPrepareMissingVar checks that Prepare returns an error when a variable
// is absent from both the binding map and q.Defaults.
func TestPrepareMissingVar(t *testing.T) {
	queryString := `SELECT :n::int;`
	q := parseQueryString("no/path", queryString)

	_, _, err := q.Prepare(map[string]string{})
	if err == nil {
		t.Error("Expected an error for missing variable, got nil")
	}
}

// TestPrepareBindingOverridesDefault checks that an explicit binding map entry
// takes precedence over a \set default.
func TestPrepareBindingOverridesDefault(t *testing.T) {
	queryString := "\\set n 10\nSELECT :n::int;\n"
	q := parseQueryString("no/path", queryString)

	_, params, err := q.Prepare(map[string]string{"n": "99"})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if len(params) != 1 || params[0] != "99" {
		t.Errorf("Expected params==[\"99\"] (binding wins), got %v", params)
	}
}

