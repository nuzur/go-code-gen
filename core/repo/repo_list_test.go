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
