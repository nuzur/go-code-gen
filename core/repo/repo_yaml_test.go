package repo

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	projecttypes "github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	"gopkg.in/yaml.v3"
)

// renderRepoYAML renders the sqlc.yaml template for the given engine, using the
// same data shape generateRepositorySQLCode passes.
func renderRepoYAML(t *testing.T, dbType projecttypes.DatabaseType) string {
	t.Helper()
	tmplBytes, err := files.GetTemplateBytes(templates, "repo_yaml")
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
		OutputPath:    filepath.Join(t.TempDir(), "sqlc.yaml"),
		TemplateBytes: tmplBytes,
		Data: struct {
			Project  *projecttypes.Project
			Fields   map[string]string
			Entities map[string]string
		}{Project: proj, Fields: map[string]string{}, Entities: map[string]string{}},
		DisableGoFormat: true,
		Funcs: template.FuncMap{
			"StringContains": gcgstrings.StringContains,
			"ToCamelCase":    gcgstrings.ToCamelCase,
		},
	})
	if err != nil {
		t.Fatalf("GenerateFile: %v", err)
	}
	// Every rendering must be valid YAML.
	var parsed interface{}
	if err := yaml.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("rendered sqlc.yaml (%s) is not valid YAML: %v\n%s", dbType, err, out)
	}
	return string(out)
}

func TestRepoYAML_Overrides_Postgres(t *testing.T) {
	out := renderRepoYAML(t, projecttypes.POSTGRESQL)
	// db_type keys must be the Postgres catalog names sqlc reports (calibrated
	// against sqlc so the overrides actually match; the previous bare MySQL names
	// silently didn't match and PG fell back to native uuid.UUID / sql.NullString /
	// pqtype.NullRawMessage — which the generated mappers don't compile against).
	for _, want := range []string{
		`db_type: "uuid"`,                // native uuid → string (below)
		`db_type: "pg_catalog.int4"`,     // int
		`db_type: "pg_catalog.json"`,     // json → []byte
		`db_type: "pg_catalog.timestamp"`,
		`db_type: "pg_catalog.time"`,
		`db_type: "pg_catalog.varchar"`,
		`db_type: "pg_catalog.bpchar"`, // char(n)
		`db_type: "pg_catalog.float8"`,
		`db_type: "pg_catalog.bool"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("postgres sqlc.yaml missing override %s\n%s", want, out)
		}
	}
	// The MySQL-only bare names must NOT appear for Postgres.
	for _, unwanted := range []string{
		`db_type: "datetime"`, `db_type: "varchar"`, `db_type: "char"`, `db_type: "tinyint"`, `db_type: "double"`, `db_type: "int"`, `db_type: "json"`,
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("postgres sqlc.yaml should not contain MySQL override %q\n%s", unwanted, out)
		}
	}
}

func TestRepoYAML_Overrides_MySQL(t *testing.T) {
	out := renderRepoYAML(t, projecttypes.MYSQL)
	for _, want := range []string{
		`db_type: "int"`,
		`db_type: "json"`,
		`db_type: "datetime"`,
		`db_type: "varchar"`,
		`db_type: "char"`,
		`db_type: "tinyint"`,
		`db_type: "double"`,
		`db_type: "float"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("mysql sqlc.yaml missing override %s\n%s", want, out)
		}
	}
	// No Postgres catalog names and no uuid override for MySQL (UUIDs are char(36)).
	for _, unwanted := range []string{`db_type: "pg_catalog`, `db_type: "uuid"`} {
		if strings.Contains(out, unwanted) {
			t.Errorf("mysql sqlc.yaml should not contain %q\n%s", unwanted, out)
		}
	}
}
