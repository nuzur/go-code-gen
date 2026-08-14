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
	// Module and EntitiesConfig.Dir are not decoration: the json override's
	// go_type import is built from them, so a Project without them renders an
	// import of "//mapper" that still parses as YAML.
	proj := &projecttypes.Project{
		Identifier: "testapp",
		Module:     "github.com/example/testapp",
		EntitiesConfig: projecttypes.EntitiesConfig{
			Dir: "entity",
		},
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
		`db_type: "pg_catalog.json"`,     // json → mapper.JSON
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

// TestRepoYAML_JSONColumnsUseMapperJSON pins the type JSON columns map to.
//
// It must not be []byte. go-sql-driver/mysql renders a []byte parameter as a
// _binary'...' literal when the DSN sets interpolateParams=true, and MySQL refuses
// to cast a binary-charset string to JSON ("Error 3144 (22032): Cannot create a
// JSON value from a string with CHARACTER SET 'binary'"), so every insert/update
// of an entity with a JSON column failed on such a DSN. mapper.JSON is a
// driver.Valuer that hands the driver a string instead.
//
// The STATIC type of the params struct field is the only lever: a json.RawMessage
// assigned into a []byte field is erased back to []byte before the driver sees it.
// So this is asserted on the override, not on any call site.
func TestRepoYAML_JSONColumnsUseMapperJSON(t *testing.T) {
	for _, dbType := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		t.Run(string(dbType), func(t *testing.T) {
			out := renderRepoYAML(t, dbType)

			jsonKey := `db_type: "json"`
			if dbType == projecttypes.POSTGRESQL {
				jsonKey = `db_type: "pg_catalog.json"`
			}
			// Both nullabilities are overridden, so the key appears twice, and each
			// occurrence must be followed by the mapper.JSON go_type.
			if got := strings.Count(out, jsonKey); got != 2 {
				t.Fatalf("expected the json override twice (nullable and not), got %d\n%s", got, out)
			}
			for _, want := range []string{
				`import: "github.com/example/testapp/entity/mapper"`,
				`package: "mapper"`,
				`type: "JSON"`,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("json override missing %s\n%s", want, out)
				}
			}
			for _, block := range strings.Split(out, jsonKey)[1:] {
				if !strings.Contains(block[:min(len(block), 200)], `type: "JSON"`) {
					t.Errorf("a json override does not map to mapper.JSON\n%s", out)
				}
			}
		})
	}
}
