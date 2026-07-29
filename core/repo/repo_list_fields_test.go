package repo

import (
	"context"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	projecttypes "github.com/nuzur/go-code-gen/project"
)

// The dynamic filter builder assembles SQL with fmt.Sprintf at runtime, so every
// dialect construct in it is invisible to the compiler. This file pins the
// per-engine SQL surface.
//
// The Postgres forms asserted here were each executed against a real PostgreSQL
// 16 instance over representative JSON data before being encoded in the
// template — they parse AND return the right rows, not just "looks plausible".

func renderRepoListFields(t *testing.T, dbType projecttypes.DatabaseType) string {
	t.Helper()
	tmplBytes, err := files.GetTemplateBytes(templates, "repo_list_fields")
	if err != nil {
		t.Fatalf("GetTemplateBytes: %v", err)
	}
	proj := &projecttypes.Project{
		Identifier: "testapp",
		Module:     "example.com/testapp",
		CoreConfig: projecttypes.CoreConfig{
			RepoConfig: projecttypes.RepoConfig{DatabaseType: dbType},
		},
	}
	out, err := files.GenerateFile(context.Background(), filetools.FileRequest{
		OutputPath:      filepath.Join(t.TempDir(), "list_fields.go"),
		TemplateBytes:   tmplBytes,
		Data:            proj,
		DisableGoFormat: true,
	})
	if err != nil {
		t.Fatalf("GenerateFile (%s): %v", dbType, err)
	}
	return string(out)
}

// mysqlOnlyConstructs are the dialect constructs that exist ONLY in MySQL.
// Every one of them reached Postgres unbranched, which is the bug this guards.
var mysqlOnlyConstructs = []string{
	"JSON_TABLE",    // no Postgres equivalent; use json_to_recordset
	"JSON_EXTRACT",  // Postgres uses -> / ->>
	"JSON_CONTAINS", // Postgres uses @> or a quantifier
	"convert(",      // Postgres spells casts CAST(x AS t); convert() is byte-encoding
	`"INT"`,         // MySQL type names
	`"DOUBLE"`,
	`"DATETIME"`,
}

// stripComments re-prints the file from an AST parsed without comments. The
// dialect branches document themselves by naming the other engine's constructs
// ("Postgres has no JSON_CONTAINS"), so scanning raw text would flag prose as if
// it were emitted SQL.
func stripComments(t *testing.T, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "list_fields.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf strings.Builder
	if err := printer.Fprint(&buf, fset, f); err != nil {
		t.Fatalf("print: %v", err)
	}
	return buf.String()
}

func TestListFields_PostgresHasNoMySQLConstructs(t *testing.T) {
	out := stripComments(t, renderRepoListFields(t, projecttypes.POSTGRESQL))
	for _, c := range mysqlOnlyConstructs {
		if strings.Contains(out, c) {
			t.Errorf("postgres list_fields.go emits MySQL-only construct %q — it will fail at "+
				"runtime, since these clauses are built with fmt.Sprintf and never compiled", c)
		}
	}
}

// TestListFields_PostgresConstructs pins the Postgres replacements. Each was
// verified against a live PostgreSQL 16 instance.
func TestListFields_PostgresConstructs(t *testing.T) {
	out := renderRepoListFields(t, projecttypes.POSTGRESQL)
	for _, want := range []string{
		"json_to_recordset",     // JSON_TABLE replacement
		"ON TRUE",               // ... joined LATERAL
		"json_array_elements",   // JSON_CONTAINS-over-array replacement
		"EXISTS (SELECT 1 FROM", // ... as a real quantifier
		"::jsonb @> ",           // array-of-scalars containment
		" ->> ",                 // scalar extraction
		"CAST(%s AS %s)",        // cast
		`"integer"`,             // Postgres type names
		`"double precision"`,
		`"timestamp"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("postgres list_fields.go is missing expected construct %q", want)
		}
	}
}

// TestListFields_MySQLUnchanged is the no-regression guard on the engine that
// already worked: the Postgres port must not have altered MySQL's dialect.
//
// Note this covers the DIALECT only. Two MySQL SEMANTIC bugs were fixed
// deliberately and are pinned separately by TestListFields_MultiValueSemantics:
// `!=` on a multi-valued field behaved as `=`, and a multi-value string filter
// compared an array to a scalar (false for every row).
func TestListFields_MySQLUnchanged(t *testing.T) {
	out := stripComments(t, renderRepoListFields(t, projecttypes.MYSQL))
	for _, want := range []string{
		`INNER JOIN JSON_TABLE(`,
		`JSON_EXTRACT(%s.%s, '$[*].%s')`,
		`JSON_EXTRACT(%s.%s, '$.%s')`,
		`JSON_CONTAINS(%s,'%s','$')`,   // array containment
		`JSON_CONTAINS(%s, '%s', '$')`, // member equality
		`convert(%s, %s)`,
		// Values only: printer.Fprint normalizes const-block alignment.
		`sqlTypeInt`, `"INT"`,
		`sqlTypeFloat`, `"DOUBLE"`,
		`sqlTypeTimestamp`, `"DATETIME"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mysql list_fields.go no longer emits %q — the Postgres port must leave "+
				"MySQL's SQL byte-identical", want)
		}
	}
	// Postgres-only helpers must not leak into the MySQL build.
	for _, unwanted := range []string{"json_to_recordset", "json_array_elements", "::jsonb", "jsonMemberPredicate"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("mysql list_fields.go contains Postgres-only construct %q", unwanted)
		}
	}
}

// Both branches must be syntactically valid Go. gofmt is disabled while
// rendering above so a syntax error surfaces here as a parse failure rather than
// as an opaque formatting error.
func TestListFields_BothEnginesParse(t *testing.T) {
	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := renderRepoListFields(t, dbType)
		if _, err := parser.ParseFile(token.NewFileSet(), "list_fields.go", out, parser.AllErrors); err != nil {
			t.Errorf("%s: rendered list_fields.go is not valid Go: %v", dbType, err)
		}
	}
}

// TestListFields_MultiValueSemantics pins the two semantics fixes, on BOTH
// engines. Both were verified against live MySQL 8 and PostgreSQL 16.
//
//  1. `!=` on a multi-valued field (JSON array, or a field inside an array of
//     objects) reused the positive containment clause, so it returned exactly the
//     rows `=` returned. It must negate.
//  2. A string filter over an array of objects compared the extracted ARRAY to a
//     scalar. On MySQL that is false for every row — the filter silently matched
//     nothing even when an element matched. It must test membership.
func TestListFields_MultiValueSemantics(t *testing.T) {
	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := stripComments(t, renderRepoListFields(t, dbType))

		// (1) negation exists and is driven by the filter function
		if !strings.Contains(out, `"NOT (%s)"`) {
			t.Errorf("%s: no NOT(...) wrapper — `!=` on a multi-valued field will behave as `=`", dbType)
		}
		if !strings.Contains(out, "filtering.FunctionNotEquals") {
			t.Errorf("%s: negation is not keyed on FunctionNotEquals", dbType)
		}
		// every containment call site must route through maybeNegate
		if n := strings.Count(out, "maybeNegate("); n < 6 {
			t.Errorf("%s: only %d maybeNegate call sites; every containment clause "+
				"(enum-multi, string-multi, and the 4 array types) must negate for `!=`", dbType, n)
		}

		// (2) the multi-value string filter must not compare an array to a scalar
		if !strings.Contains(out, "jsonMemberStringEquals(req, value)") {
			t.Errorf("%s: multi-value string equality does not use a membership test", dbType)
		}
		if !strings.Contains(out, "jsonMemberStringLike(req, value)") {
			t.Errorf("%s: multi-value string `has` does not use a membership test", dbType)
		}
	}

	// Engine-specific spellings of the membership test.
	mysql := stripComments(t, renderRepoListFields(t, projecttypes.MYSQL))
	// Exact spelling (incl. the LOWER/CAST wrapping) is pinned by
	// TestListFields_HasIsCaseInsensitive; here we only require the mechanism.
	if !strings.Contains(mysql, "JSON_SEARCH(") || !strings.Contains(mysql, "IS NOT NULL") {
		t.Error("mysql: substring match over a JSON array must use JSON_SEARCH ... IS NOT NULL")
	}
	if !strings.Contains(mysql, `JSON_CONTAINS(%s, '\"%s\"', '$')`) {
		t.Error("mysql: string membership must search for a quoted JSON literal")
	}

	pg := stripComments(t, renderRepoListFields(t, projecttypes.POSTGRESQL))
	if !strings.Contains(pg, "jsonMemberPredicate(req, sqlLike") {
		t.Error("postgres: substring match over a JSON array must use a like quantifier")
	}
}

// TestListFields_HasIsCaseInsensitive guards a difference that is invisible in
// the SQL's shape and only shows up as wrong search results.
//
// `has` (substring search) must mean the same thing everywhere. Three of the
// four paths did not agree, verified on live engines:
//   - MySQL `like` on a column   : case-INsensitive (default collation)
//   - MySQL JSON_SEARCH          : case-SENSITIVE  (JSON strings collate utf8mb4_bin)
//   - Postgres `like`            : case-SENSITIVE
//   - Postgres `ilike`           : case-insensitive
//
// So Postgres uses ilike, and the MySQL JSON path lowers both sides.
func TestListFields_HasIsCaseInsensitive(t *testing.T) {
	mysql := stripComments(t, renderRepoListFields(t, projecttypes.MYSQL))
	if !strings.Contains(mysql, `sqlLike = "like"`) {
		t.Error("mysql: expected `like`, which is case-insensitive under the default collation")
	}
	// JSON_SEARCH is case-sensitive, so both sides must be lowered.
	if !strings.Contains(mysql, `JSON_SEARCH(LOWER(CAST(%s AS CHAR)), 'one', LOWER('%%%s%%')) IS NOT NULL`) {
		t.Error("mysql: JSON_SEARCH is case-sensitive; both sides must be lowered so that `has` " +
			"behaves the same for a field in a JSON array as for a plain column")
	}

	pg := stripComments(t, renderRepoListFields(t, projecttypes.POSTGRESQL))
	if !strings.Contains(pg, `sqlLike = "ilike"`) {
		t.Error("postgres: `like` is case-sensitive there; `has` must use ilike to match MySQL")
	}
	if strings.Contains(pg, `"like"`) {
		t.Error("postgres: a bare case-sensitive `like` is still emitted somewhere")
	}

	// Neither engine may hardcode the operator at a call site, or it will drift
	// from the constant again.
	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := stripComments(t, renderRepoListFields(t, dbType))
		if strings.Contains(out, `" like '"`) || strings.Contains(out, `%s like '`) {
			t.Errorf("%s: `like` is hardcoded at a call site instead of using sqlLike", dbType)
		}
	}
}
