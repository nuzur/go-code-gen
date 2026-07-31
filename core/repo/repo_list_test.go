package repo

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	projecttypes "github.com/nuzur/go-code-gen/project"
)

// The dynamic list builder assembles SQL with fmt.Sprintf at runtime, so a
// dialect mistake in it cannot be caught by anything that only compiles the
// generated project — it type-checks fine and fails on the first request. That is
// exactly how `LIMIT <offset>, <count>` shipped: MySQL-only syntax, emitted for
// both engines, breaking every Postgres list endpoint with
// `pq: LIMIT #,# syntax is not supported`.
//
// So assert on the SQL text the template emits.

func renderRepoList(t *testing.T, dbType projecttypes.DatabaseType) string {
	t.Helper()
	tmplBytes, err := files.GetTemplateBytes(templates, "repo_list")
	if err != nil {
		t.Fatalf("GetTemplateBytes: %v", err)
	}
	proj := &projecttypes.Project{
		Identifier: "testapp",
		CoreConfig: projecttypes.CoreConfig{
			RepoConfig: projecttypes.RepoConfig{DatabaseType: dbType},
		},
	}
	out, err := files.GenerateFile(context.Background(), filetools.FileRequest{
		OutputPath:    filepath.Join(t.TempDir(), "list.go"),
		TemplateBytes: tmplBytes,
		Data:          proj,
	})
	if err != nil {
		t.Fatalf("GenerateFile (%s): %v", dbType, err)
	}
	return string(out)
}

// TestBuildLimit_PortableSyntax pins the pagination clause for both engines.
func TestBuildLimit_PortableSyntax(t *testing.T) {
	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := renderRepoList(t, dbType)

		// The MySQL-only comma form must not appear for EITHER engine: Postgres
		// rejects it outright, and MySQL accepts the portable form too, so there is
		// no reason to emit it anywhere.
		if strings.Contains(out, `"LIMIT %d, %d `) {
			t.Errorf("%s: buildLimit emits the MySQL-only comma form `LIMIT <offset>, <count>`; "+
				"Postgres rejects it (pq: LIMIT #,# syntax is not supported)", dbType)
		}

		if !strings.Contains(out, `"LIMIT %d OFFSET %d `) {
			t.Errorf("%s: buildLimit does not emit the portable `LIMIT <count> OFFSET <offset>` form\n%s",
				dbType, extractBuildLimit(out))
		}

		// The comma form takes (offset, count) and the OFFSET form takes
		// (count, offset). Getting the operands backwards silently paginates wrong
		// rather than erroring, so pin the argument order too.
		if !strings.Contains(out, "request.GetPageSize(), request.GetOffset()") {
			t.Errorf("%s: buildLimit passes its operands in the wrong order — `LIMIT <count> OFFSET "+
				"<offset>` takes page size first, then offset\n%s", dbType, extractBuildLimit(out))
		}
	}
}

func extractBuildLimit(out string) string {
	i := strings.Index(out, "func buildLimit")
	if i < 0 {
		return "(buildLimit not found in rendered output)"
	}
	rest := out[i:]
	if j := strings.Index(rest, "\n}"); j >= 0 {
		return rest[:j+2]
	}
	return rest
}

// TestListQuery_IdentifiersAreQuoted pins the fix for an entity whose identifier
// is a reserved word.
//
// The runtime builder interpolated the entity identifier raw, so an entity named
// `user` — which `auth: jwt` REQUIRES — produced `SELECT user.id ... FROM user`.
// Postgres reads `user` as the CURRENT_USER keyword and fails on the following
// `.`, so every list request against that entity returned a 500, while the same
// model worked on MySQL. `order`, `group` and `session` fail identically, as do
// columns with those names.
//
// Every table/column reference must therefore go through quoteIdent /
// qualifiedColumn (defined per engine in list_fields.go), exactly as create.sql
// and the sqlc queries already do.
func TestListQuery_IdentifiersAreQuoted(t *testing.T) {
	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		out := renderRepoList(t, dbType)

		// The raw interpolations, each of which produced invalid SQL.
		for _, banned := range []string{
			`fmt.Sprintf("%s.*", entity.EntityIdentifier())`,
			`fmt.Sprintf("%s.%s", entity.EntityIdentifier()`,
			`fmt.Sprintf("%s %s %s", entity.EntityIdentifier()`,
			`fmt.Sprintf("%s %s", f.fieldIdentifier, order)`,
		} {
			if strings.Contains(out, banned) {
				t.Errorf("%s: list.go still interpolates an UNQUOTED identifier: %s", dbType, banned)
			}
		}

		// ... and the quoted forms that replaced them: select list, FROM clause,
		// primary keys (GROUP BY / count) and ORDER BY.
		for _, want := range []string{
			`fmt.Sprintf("%s.*", quoteIdent(entity.EntityIdentifier()))`,
			`quoteIdent(entity.EntityIdentifier()), joinStatement, jsonTablesFinal`,
			`qualifiedColumn(entity.EntityIdentifier(), c)`,
			`qualifiedColumn(entity.EntityIdentifier(), f.fieldIdentifier)`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: list.go does not quote an identifier where it must: expected %s", dbType, want)
			}
		}
	}
}
