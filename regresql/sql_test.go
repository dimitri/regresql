package regresql

import (
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// mustParseQueryString calls parseQueryString and fails the test on error.
func mustParseQueryString(t *testing.T, path, sql string) *Query {
	t.Helper()
	q, err := parseQueryString(path, sql)
	if err != nil {
		t.Fatalf("parseQueryString error: %v", err)
	}
	return q
}

// ── Existing named-mode tests (updated for new error return) ─────────────────

func TestParseQueryString(t *testing.T) {
	queryString := `select * from foo where id = :user_id`
	q := mustParseQueryString(t, "no/path", queryString)

	if len(q.Vars) != 1 || q.Vars[0] != "user_id" {
		t.Error("Expected [\"user_id\"], got ", q.Vars)
	}
}

func TestParseQueryStringWithTypeCast(t *testing.T) {
	queryString := `select name::text from foo where id = :user_id`
	q := mustParseQueryString(t, "no/path", queryString)

	if len(q.Vars) != 1 || q.Vars[0] != "user_id" {
		t.Error("Expected only [\"user_id\"], got ", q.Vars)
	}
}

func TestPrepareOneParam(t *testing.T) {
	queryString := `select * from foo where id = :id`
	q := mustParseQueryString(t, "no/path", queryString)
	b := map[string]string{"id": "1"}

	sql, params, err := q.Prepare(b)

	if err != nil {
		t.Fatal("Unexpected error from Prepare:", err)
	}
	if sql != "select * from foo where id = $1" {
		t.Error("Query string not as expected ", sql)
	}
	if !(len(params) == 1 && params[0] == "1") {
		t.Error("Bindings not properly applied, got ", params)
	}
}

func TestPrepareTwoParams(t *testing.T) {
	queryString := `select * from foo where a = :a and b between :a and :b`
	q := mustParseQueryString(t, "no/path", queryString)
	b := map[string]string{"a": "a", "b": "b"}

	sql, params, err := q.Prepare(b)

	if err != nil {
		t.Fatal("Unexpected error from Prepare:", err)
	}
	if sql != "select * from foo where a = $1 and b between $1 and $2" {
		t.Error("Query string not as expected ", sql)
	}
	if !(len(params) == 3 && params[0] == "a" && params[1] == "a" && params[2] == "b") {
		t.Error("Bindings not properly applied, got ", params)
	}
}

// ── \set tests (unchanged semantics) ─────────────────────────────────────────

func TestSetUnquoted(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\set n 10\nSELECT :n::int;\n")

	if v, ok := q.Defaults["n"]; !ok || v != "10" {
		t.Errorf("Expected Defaults[\"n\"]==\"10\", got %q (ok=%v)", v, ok)
	}
	if strings.Contains(q.Query, `\set`) {
		t.Error("\\set line should be stripped from q.Query")
	}
}

func TestSetSingleQuotedEscapes(t *testing.T) {
	q := mustParseQueryString(t, "no/path", `\set s 'hello\nworld'`+"\nSELECT 1;\n")
	want := "hello\nworld"
	if v := q.Defaults["s"]; v != want {
		t.Errorf("Expected Defaults[\"s\"]==%q, got %q", want, v)
	}
}

func TestSetSingleQuotedDoubleApostrophe(t *testing.T) {
	q := mustParseQueryString(t, "no/path", `\set s 'it''s'`+"\nSELECT 1;\n")
	if v := q.Defaults["s"]; v != "it's" {
		t.Errorf("Expected \"it's\", got %q", v)
	}
}

func TestSetDoubleQuoted(t *testing.T) {
	q := mustParseQueryString(t, "no/path", `\set s "hello"`+"\nSELECT 1;\n")
	if v := q.Defaults["s"]; v != `"hello"` {
		t.Errorf("Expected `\"hello\"`, got %q", v)
	}
}

func TestSetMultiTokenConcatenation(t *testing.T) {
	q := mustParseQueryString(t, "no/path", `\set x a 'b' c`+"\nSELECT 1;\n")
	if v := q.Defaults["x"]; v != "abc" {
		t.Errorf("Expected \"abc\", got %q", v)
	}
}

func TestSetNoValue(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\set EMPTY\nSELECT 1;\n")
	if v, ok := q.Defaults["EMPTY"]; !ok || v != "" {
		t.Errorf("Expected Defaults[\"EMPTY\"]==\"\", got %q (ok=%v)", v, ok)
	}
}

func TestSetUnusedVariable(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\set unused foo\nSELECT 1;\n")
	if v, ok := q.Defaults["unused"]; !ok || v != "foo" {
		t.Errorf("Expected Defaults[\"unused\"]==\"foo\", got %q (ok=%v)", v, ok)
	}
	for _, v := range q.Vars {
		if v == "unused" {
			t.Error("\"unused\" should not appear in q.Vars")
		}
	}
}

func TestPrepareDefaultFallback(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\set n 10\nSELECT :n::int;\n")
	_, params, err := q.Prepare(map[string]string{})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if len(params) != 1 || params[0] != "10" {
		t.Errorf("Expected params==[\"10\"], got %v", params)
	}
}

func TestPrepareMissingVar(t *testing.T) {
	q := mustParseQueryString(t, "no/path", `SELECT :n::int;`)
	_, _, err := q.Prepare(map[string]string{})
	if err == nil {
		t.Error("Expected error for missing variable, got nil")
	}
}

func TestPrepareBindingOverridesDefault(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\set n 10\nSELECT :n::int;\n")
	_, params, err := q.Prepare(map[string]string{"n": "99"})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if len(params) != 1 || params[0] != "99" {
		t.Errorf("Expected params==[\"99\"] (binding wins), got %v", params)
	}
}

// ── Positional $N scanning tests ─────────────────────────────────────────────

func TestPositionalParams(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "SELECT $1, $2 FROM foo;\n")

	if !q.Positional {
		t.Fatal("Expected Positional=true")
	}
	if len(q.Vars) != 2 || q.Vars[0] != "p1" || q.Vars[1] != "p2" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\"], got %v", q.Vars)
	}
	// Query must be unchanged (no $N substitution)
	if !strings.Contains(q.Query, "$1") || !strings.Contains(q.Query, "$2") {
		t.Errorf("Expected $1/$2 to remain in q.Query, got %q", q.Query)
	}
}

func TestPositionalParamsDollarQuoteEmpty(t *testing.T) {
	// $1 inside $$ .. $$ must be ignored; $2 outside must be detected.
	q := mustParseQueryString(t, "no/path", "SELECT $2, $$contains $1 here$$ FROM foo;\n")
	if len(q.Vars) != 2 || q.Vars[0] != "p1" || q.Vars[1] != "p2" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\"] (maxN=2), got %v", q.Vars)
	}
}

func TestPositionalParamsTaggedDollarQuote(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "SELECT $2, $body$has $1$body$ FROM foo;\n")
	if len(q.Vars) != 2 || q.Vars[0] != "p1" || q.Vars[1] != "p2" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\"], got %v", q.Vars)
	}
}

func TestPositionalParamsStringLiteral(t *testing.T) {
	// $99 is inside a string literal and must not inflate maxN.
	// Only $2 is a real parameter, so maxN=2 and Vars=["p1","p2"].
	q := mustParseQueryString(t, "no/path", "SELECT '$99', $2 FROM foo;\n")
	if len(q.Vars) != 2 || q.Vars[0] != "p1" || q.Vars[1] != "p2" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\"] (string-literal $99 ignored), got %v", q.Vars)
	}
}

func TestPositionalParamsLineComment(t *testing.T) {
	// $99 is in a line comment and must not inflate maxN.
	q := mustParseQueryString(t, "no/path", "-- skip $99\nSELECT $2 FROM foo;\n")
	if len(q.Vars) != 2 || q.Vars[0] != "p1" || q.Vars[1] != "p2" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\"] (comment $99 ignored), got %v", q.Vars)
	}
}

func TestPositionalParamsBlockComment(t *testing.T) {
	// $99 is in a block comment and must not inflate maxN.
	q := mustParseQueryString(t, "no/path", "SELECT /* skip $99 */ $2 FROM foo;\n")
	if len(q.Vars) != 2 || q.Vars[0] != "p1" || q.Vars[1] != "p2" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\"] (comment $99 ignored), got %v", q.Vars)
	}
}

func TestPositionalParamsNestedBlockComment(t *testing.T) {
	// $99 / $98 inside nested block comments must not inflate maxN.
	// PostgreSQL supports nested block comments.
	q := mustParseQueryString(t, "no/path", "SELECT /* /* $99 */ $98 */ $3 FROM foo;\n")
	if len(q.Vars) != 3 || q.Vars[0] != "p1" || q.Vars[1] != "p2" || q.Vars[2] != "p3" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\",\"p3\"] (nested-comment $99/$98 ignored), got %v", q.Vars)
	}
}

func TestPositionalParamsNonContiguous(t *testing.T) {
	// $2 is absent but maxN=3, so Vars should be ["p1","p2","p3"].
	q := mustParseQueryString(t, "no/path", "SELECT $3, $1 FROM foo;\n")
	if len(q.Vars) != 3 || q.Vars[0] != "p1" || q.Vars[1] != "p2" || q.Vars[2] != "p3" {
		t.Errorf("Expected Vars==[\"p1\",\"p2\",\"p3\"], got %v", q.Vars)
	}
}

// ── Mixed-mode error test ─────────────────────────────────────────────────────

func TestMixedParamStyleError(t *testing.T) {
	_, err := parseQueryString("src/sql/query.sql", "SELECT :name FROM foo WHERE id = $1;\n")
	if err == nil {
		t.Fatal("Expected mixed-mode error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, ":name") {
		t.Errorf("Error should mention :name, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "$1") {
		t.Errorf("Error should mention $1, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "mixed parameter styles") {
		t.Errorf("Error should say 'mixed parameter styles', got: %s", errMsg)
	}
}

// ── \bind extraction tests ────────────────────────────────────────────────────

func TestBindExtract(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\bind 'hello world' foo\nSELECT $1, $2;\n")

	if len(q.BindDefaults) != 2 {
		t.Fatalf("Expected 2 BindDefaults, got %d: %v", len(q.BindDefaults), q.BindDefaults)
	}
	if q.BindDefaults[0] != "hello world" {
		t.Errorf("Expected BindDefaults[0]==\"hello world\", got %q", q.BindDefaults[0])
	}
	if q.BindDefaults[1] != "foo" {
		t.Errorf("Expected BindDefaults[1]==\"foo\", got %q", q.BindDefaults[1])
	}
	if strings.Contains(q.Query, `\bind`) {
		t.Error("\\bind line should be stripped from q.Query")
	}
}

func TestBindLastWins(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\bind first\n\\bind second\nSELECT $1;\n")

	if len(q.BindDefaults) != 1 || q.BindDefaults[0] != "second" {
		t.Errorf("Expected BindDefaults==[\"second\"] (last wins), got %v", q.BindDefaults)
	}
}

func TestBindNoValues(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\bind\nSELECT $1;\n")
	if len(q.BindDefaults) != 0 {
		t.Errorf("Expected empty BindDefaults, got %v", q.BindDefaults)
	}
}

// ── Positional Prepare tests ──────────────────────────────────────────────────

func TestPositionalPrepareFromBinding(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "SELECT $1::int + $2::int AS sum;\n")
	_, params, err := q.Prepare(map[string]string{"p1": "3", "p2": "4"})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if len(params) != 2 || params[0] != "3" || params[1] != "4" {
		t.Errorf("Expected params==[\"3\",\"4\"], got %v", params)
	}
}

func TestPositionalPrepareFromBindDefault(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\bind 3 4\nSELECT $1::int + $2::int AS sum;\n")
	_, params, err := q.Prepare(map[string]string{})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if len(params) != 2 || params[0] != "3" || params[1] != "4" {
		t.Errorf("Expected params==[\"3\",\"4\"], got %v", params)
	}
}

func TestPositionalPrepareMissing(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "SELECT $1::int;\n")
	_, _, err := q.Prepare(map[string]string{})
	if err == nil {
		t.Error("Expected error for missing positional parameter, got nil")
	}
}

func TestPositionalPrepareBindingOverridesDefault(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "\\bind 3\nSELECT $1::int;\n")
	_, params, err := q.Prepare(map[string]string{"p1": "99"})
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}
	if len(params) != 1 || params[0] != "99" {
		t.Errorf("Expected params==[\"99\"] (binding wins over \\bind), got %v", params)
	}
}

// ── Named-param context-awareness tests ──────────────────────────────────────

func TestNamedParamSkipsSingleQuote(t *testing.T) {
	// :val inside a string literal must not be treated as a named parameter.
	q := mustParseQueryString(t, "no/path", "SELECT ':val' AS x WHERE id = :id;\n")
	if len(q.Vars) != 1 || q.Vars[0] != "id" {
		t.Errorf("Expected Vars==[\"id\"], got %v", q.Vars)
	}
}

func TestNamedParamSkipsLineComment(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "-- :skip\nSELECT :id FROM t;\n")
	if len(q.Vars) != 1 || q.Vars[0] != "id" {
		t.Errorf("Expected Vars==[\"id\"] (comment :skip ignored), got %v", q.Vars)
	}
}

func TestNamedParamSkipsBlockComment(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "SELECT /* :skip */ :id FROM t;\n")
	if len(q.Vars) != 1 || q.Vars[0] != "id" {
		t.Errorf("Expected Vars==[\"id\"] (block-comment :skip ignored), got %v", q.Vars)
	}
}

func TestNamedParamSkipsDollarQuote(t *testing.T) {
	q := mustParseQueryString(t, "no/path", "SELECT $$:skip$$ AS x, :id AS y;\n")
	if len(q.Vars) != 1 || q.Vars[0] != "id" {
		t.Errorf("Expected Vars==[\"id\"] (dollar-quoted :skip ignored), got %v", q.Vars)
	}
}

func TestNamedParamSkipsJsonStringLiteral(t *testing.T) {
	// Regression: :varname inside a JSON string literal was falsely detected.
	sql := "SELECT * FROM t WHERE data @> '{\"type\":\"admin\"}' AND id = :id;\n"
	q := mustParseQueryString(t, "no/path", sql)
	if len(q.Vars) != 1 || q.Vars[0] != "id" {
		t.Errorf("Expected Vars==[\"id\"], got %v", q.Vars)
	}
	// The JSON literal must survive unchanged in the normalised query.
	if !strings.Contains(q.Query, `'{"type":"admin"}'`) {
		t.Errorf("JSON string literal should be unchanged in q.Query, got: %s", q.Query)
	}
}

func TestNamedParamTypeCastNotMatched(t *testing.T) {
	// :n::int -- :n is the variable; ::int is a cast and must NOT produce "int".
	q := mustParseQueryString(t, "no/path", "SELECT :n::int;\n")
	if len(q.Vars) != 1 || q.Vars[0] != "n" {
		t.Errorf("Expected Vars==[\"n\"], got %v", q.Vars)
	}
}

func TestNamedParamAfterStringLiteral(t *testing.T) {
	// :id that follows a closing quote must still be detected.
	q := mustParseQueryString(t, "no/path", "SELECT 'literal' AS x WHERE id = :id;\n")
	if len(q.Vars) != 1 || q.Vars[0] != "id" {
		t.Errorf("Expected Vars==[\"id\"] after string literal, got %v", q.Vars)
	}
}

func TestNamedParamStringLiteralPreserved(t *testing.T) {
	// Verify that the substitution step does NOT alter string-literal content.
	// Even if the variable name appears inside a string, the literal must reach
	// PostgreSQL unchanged (the bug was that the regex replace also ran inside
	// string literals).
	q := mustParseQueryString(t, "no/path", "SELECT ':user_id text' AS lbl WHERE id = :user_id;\n")
	if !strings.Contains(q.Query, "':user_id text'") {
		t.Errorf("String literal ':user_id text' should be unchanged in q.Query, got: %s", q.Query)
	}
	if !strings.Contains(q.Query, "$1") {
		t.Errorf("Real :user_id should be replaced by $1 in q.Query, got: %s", q.Query)
	}
}
