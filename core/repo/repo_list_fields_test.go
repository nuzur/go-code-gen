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
		`"$%d"`,                 // numbered bind placeholder
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
//
// "Byte-identical" no longer holds for the two JSON_CONTAINS forms: the JSON
// operand used to be a literal spliced into the statement text (`'%s'`) and is
// now a bound argument cast back to JSON (`CAST(%s AS JSON)`) — that is the
// injection fix, and the operand is a bound STRING because the driver renders a
// []byte as _binary'...', which MySQL rejects with Error 3144. Everything else
// about the dialect is unchanged and still pinned here.
func TestListFields_MySQLUnchanged(t *testing.T) {
	out := stripComments(t, renderRepoListFields(t, projecttypes.MYSQL))
	for _, want := range []string{
		`INNER JOIN JSON_TABLE(`,
		// The column operand is now one already-quoted `table`.`column` built by
		// qualifiedColumn, instead of two raw identifiers — see
		// TestListFields_IdentifiersAreQuoted. The JSON path arguments are
		// unchanged.
		`JSON_EXTRACT(%s, '$[*].%s')`,
		`JSON_EXTRACT(%s, '$.%s')`,
		`JSON_CONTAINS(%s, CAST(%s AS JSON), '$')`, // array containment AND member equality
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

// TestListFields_IdentifiersAreQuoted pins the per-engine identifier quoting the
// clause builder relies on (see TestListQuery_IdentifiersAreQuoted for why an
// entity named `user` used to 500 every list request on Postgres).
func TestListFields_IdentifiersAreQuoted(t *testing.T) {
	mysqlQuote := extractFunc(stripComments(t, renderRepoListFields(t, projecttypes.MYSQL)), "func quoteIdent")
	if !strings.Contains(mysqlQuote, "``") {
		t.Errorf("mysql quoteIdent does not quote with backticks (nor escape an embedded one):\n%s", mysqlQuote)
	}

	postgresQuote := extractFunc(stripComments(t, renderRepoListFields(t, projecttypes.POSTGRESQL)), "func quoteIdent")
	if !strings.Contains(postgresQuote, `""`) {
		t.Errorf("postgres quoteIdent does not quote with double quotes (nor escape an embedded one):\n%s", postgresQuote)
	}

	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := stripComments(t, renderRepoListFields(t, dbType))
		if !strings.Contains(out, "func qualifiedColumn(table, column string) string") {
			t.Errorf("%s: list_fields.go is missing qualifiedColumn", dbType)
		}
		// The pre-fix interpolations: a table.column pair rendered raw.
		for _, banned := range []string{
			`fmt.Sprintf("%s.%s", req.entity.EntityIdentifier(), req.fieldIdentifier)`,
			`"JSON_EXTRACT(%s.%s,`,
			`json_array_elements(%s.%s)`,
			`json_to_recordset(%s.%s)`,
		} {
			if strings.Contains(out, banned) {
				t.Errorf("%s: list_fields.go still interpolates an unquoted table.column pair: %s", dbType, banned)
			}
		}
	}
}

// TestListFields_EnumValueResolution pins the fix for enum filtering.
//
// REST declared enum fields as plain strings while the clause builder resolved
// them ONLY through a protobuf enum declaration, so every REST enum filter
// failed with "enum declaration not found" — no syntax worked, on any field.
// The resolver now reads the value table the transports supply (REST generates
// no protobuf enums at all), accepts BOTH the quoted string and the bare ident
// spelling, and reports an unknown value instead of dereferencing nil.
func TestListFields_EnumValueResolution(t *testing.T) {
	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := stripComments(t, renderRepoListFields(t, dbType))

		for _, want := range []string{
			"func resolveEnumValue(req SingleClauseRequest, name string) (int64, error)",
			"req.cex.Args[1].GetIdentExpr().GetName()",        // `status = STATUS_ACTIVE`
			"req.cex.Args[1].GetConstExpr().GetStringValue()", // `status = "active"`
			"req.enumValues[name]",                            // transport-supplied table
			"strings.EqualFold(spelling, raw)",                // case-insensitive
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: enum resolution is missing %q", dbType, want)
			}
		}

		// ByName returns nil for an unknown value; calling .Number() on it
		// panicked the request goroutine on a typo'd filter.
		if strings.Contains(out, ".ByName(protoreflect.Name(value)).Number()") {
			t.Errorf("%s: enum resolution still dereferences the result of ByName unguarded", dbType)
		}
	}
}

// TestListFields_NoValueIsInterpolated is the guard that keeps the SQL injection
// class from coming back.
//
// The clause builders used to splice the caller's filter VALUE straight into the
// statement text — `fmt.Sprintf("%s %s '%s'", left, op, value)` — and the query
// then ran with zero arguments. A value containing a single quote closed the
// literal and continued the statement. Every value is now appended to the query's
// binder and referenced by a placeholder, so the only way to regress is to write
// one of these quoted formats again.
//
// The scan is per clause-builder body rather than over the whole file because
// `'%s'` also spells a JSON PATH — `->> '%s'`, `'$.%s'` — whose operand is a
// dependant field identifier the builder has already checked against the entity's
// schema, never caller-supplied text.
func TestListFields_NoValueIsInterpolated(t *testing.T) {
	clauseBuilders := []string{
		"func buildStringClause",
		"func buildEnumClause",
		"func buildIntClause",
		"func buildFloatClause",
		"func buildBooleanClause",
		"func buildTimestampClause",
		"func buildArrayClause",
	}
	// A clause builder either binds the value itself, or hands it to one of the
	// dialect helpers that binds (they take the whole request precisely so they
	// can reach the binder).
	bindingMarkers := []string{
		"req.args.bind(",
		"jsonArrayContains(req,",
		"jsonMemberEquals(req,",
		"jsonMemberStringEquals(req,",
		"jsonMemberStringLike(req,",
	}

	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := stripComments(t, renderRepoListFields(t, dbType))

		// These two are never a path or an identifier — they only ever wrapped a
		// value — so they must not appear anywhere in the file.
		for _, banned := range []string{`'%%%s%%'`, `\"%s\"`} {
			if strings.Contains(out, banned) {
				t.Errorf("%s: list_fields.go interpolates a value into the SQL text (%s); "+
					"filter values must be bound, not spliced", dbType, banned)
			}
		}

		for _, header := range clauseBuilders {
			body := extractFunc(out, header)
			if strings.HasPrefix(body, "(") {
				t.Errorf("%s: %s not found in rendered output", dbType, header)
				continue
			}
			if strings.Contains(body, `'%s'`) {
				t.Errorf("%s: %s interpolates a quoted value into the SQL text; it must bind it\n%s",
					dbType, header, body)
			}
			bound := false
			for _, marker := range bindingMarkers {
				if strings.Contains(body, marker) {
					bound = true
					break
				}
			}
			if !bound {
				t.Errorf("%s: %s never reaches the query binder — its value ends up in the "+
					"statement text\n%s", dbType, header, body)
			}
		}
	}
}

func extractFunc(out, header string) string {
	i := strings.Index(out, header)
	if i < 0 {
		return "(" + header + " not found in rendered output)"
	}
	rest := out[i:]
	if j := strings.Index(rest, "\n}"); j >= 0 {
		return rest[:j+2]
	}
	return rest
}
