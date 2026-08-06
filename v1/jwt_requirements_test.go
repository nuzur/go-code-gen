package gocodegen

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// TestGenerateRejectsJWTAuthWithoutUserEntity locks in the pre-flight check:
// enabling JWT auth against a schema the jwtserver templates cannot satisfy
// must fail before any code is emitted, rather than producing a workspace that
// only fails later at go build.
func TestGenerateRejectsJWTAuthWithoutUserEntity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gocodegen-jwt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	params := &project.ProjectParams{
		Project: &nemgen.Project{Name: "TestProject"},
		ProjectVersion: &nemgen.ProjectVersion{
			Entities: []*nemgen.Entity{
				{
					Uuid:       "c1682705-1a89-4cc1-9b1f-e9a888c00000",
					Identifier: "article",
					Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
					Fields: []*nemgen.Field{
						{
							Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00001",
							Identifier: "id",
							Type:       nemgen.FieldType_FIELD_TYPE_UUID,
							Key:        true,
							Required:   true,
							Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
							TypeConfig: &nemgen.FieldTypeConfig{},
						},
					},
					TypeConfig: &nemgen.EntityTypeConfig{
						Standalone: &nemgen.EntityTypeStandaloneConfig{
							Indexes: []*nemgen.Index{
								{
									Uuid:       "idx-pk",
									Identifier: "primary",
									Type:       nemgen.IndexType_INDEX_TYPE_PRIMARY,
									Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
									Fields:     []*nemgen.IndexField{{FieldUuid: "d1682705-1a89-4cc1-9b1f-e9a888c00001"}},
								},
							},
						},
					},
				},
			},
		},
		RootPath:       tmpDir,
		Identifier:     "testproject",
		Module:         "github.com/mklfarha/testproject",
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity"},
		CoreConfig: project.CoreConfig{
			Enabled: true,
			CoreDir: "core",
			RepoConfig: project.RepoConfig{
				DatabaseType: project.MYSQL,
			},
		},
		AuthConfig: project.AuthConfig{
			Enabled: true,
			Type:    project.JWT_SERVER_AUTH_TYPE,
		},
	}

	err = Generate(context.Background(), params)
	if err == nil {
		t.Fatal("expected Generate to reject jwt auth without a user entity")
	}
	if !strings.Contains(err.Error(), `"user"`) || !strings.Contains(err.Error(), `"usuario"`) {
		t.Fatalf("expected the error to name the missing user entity, got: %v", err)
	}

	// Nothing beyond the go.mod written by project.New should exist.
	if _, statErr := os.Stat(filepath.Join(tmpDir, "testproject", "entity")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no generated entities, stat returned: %v", statErr)
	}
}

// TestGenerateJWTSchemaWithUniqueEmailOnly proves field-level unique:true is a
// complete substitute for a hand-drawn index, end to end: the schema below
// carries NO index on email, only the flag. Generation must clear
// ValidateJWTAuthRequirements, the select resolver must emit FetchUserByEmail
// from the index normalization synthesized, and the whole generated module must
// build — the flag alone used to produce a workspace that failed at go build
// because the signin template called a method the repo never generated.
func TestGenerateJWTSchemaWithUniqueEmailOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated-code build in -short mode")
	}

	version := configurationsSchema()
	for _, e := range version.Entities {
		if e.Identifier != "user" {
			continue
		}
		for _, f := range e.Fields {
			if f.Identifier == "email" {
				f.Unique = true
			}
		}
		// Drop everything but the primary key: the unique flag is now the only
		// thing that can produce the fetch-by-email select.
		e.TypeConfig.Standalone.Indexes = e.TypeConfig.Standalone.Indexes[:1]
	}

	id := "cfg_rest_jwt_unique_email"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: version,
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig: project.CoreConfig{
			Enabled:    true,
			CoreDir:    "core",
			RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL},
		},
		RESTConfig:     project.RESTConfig{Enabled: true, OpenAPI: true},
		AuthConfig:     project.AuthConfig{Enabled: true, Type: project.JWT_SERVER_AUTH_TYPE},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate failed for unique-email schema: %v", err)
	}

	dir := filepath.Join(root, id)

	// The select is what the whole feature exists for, so assert it in the repo
	// layer as well as in the caller. It is emitted from the synthesized index by
	// the same resolver that handles hand-drawn ones.
	selects, err := os.ReadFile(indexedSelectsPath(t, dir))
	if err != nil {
		t.Fatalf("reading generated indexed selects: %v", err)
	}
	if !strings.Contains(string(selects), "UserByEmail") {
		t.Errorf("generated indexed selects have no UserByEmail statement:\n%s", string(selects))
	}

	signin, err := os.ReadFile(filepath.Join(dir, "auth", "jwtserver", "signin.go"))
	if err != nil {
		t.Fatalf("reading generated signin.go: %v", err)
	}
	if !strings.Contains(string(signin), "FetchUserByEmail") {
		t.Errorf("generated signin.go missing FetchUserByEmail")
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... failed for unique-email schema:\n%s", string(out))
	}
}

// indexedSelectsPath locates the generated single-index select file, which lives
// under the configurable repo directory. It is found by walking rather than by
// composing the path so the test does not encode the repo layout twice.
func indexedSelectsPath(t *testing.T, dir string) string {
	t.Helper()
	found := ""
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "select_indexed_simple.sql" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking generated project: %v", err)
	}
	if found == "" {
		t.Fatalf("no select_indexed_simple.sql generated under %s", dir)
	}
	return found
}

// TestGenerateSpanishJWTSchema proves the JWT server generated for a
// Spanish-authored schema compiles: the module path, core accessor and
// fetch-by-email select are all rendered from the schema's own identifiers
// (usuario / correo), not from hardcoded English ones.
func TestGenerateSpanishJWTSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated-code build in -short mode")
	}

	version := configurationsSchema()
	for _, e := range version.Entities {
		if e.Identifier != "user" {
			continue
		}
		e.Identifier = "usuario"
		for _, f := range e.Fields {
			switch f.Identifier {
			case "email":
				f.Identifier = "correo"
			case "password":
				f.Identifier = "contrasena"
			}
		}
	}

	id := "cfg_rest_jwt_spanish"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: version,
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig: project.CoreConfig{
			Enabled:    true,
			CoreDir:    "core",
			RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL},
		},
		RESTConfig:     project.RESTConfig{Enabled: true, OpenAPI: true},
		AuthConfig:     project.AuthConfig{Enabled: true, Type: project.JWT_SERVER_AUTH_TYPE},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate failed for spanish schema: %v", err)
	}

	dir := filepath.Join(root, id)
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... failed for spanish schema:\n%s", string(out))
	}

	signin, err := os.ReadFile(filepath.Join(dir, "auth", "jwtserver", "signin.go"))
	if err != nil {
		t.Fatalf("reading generated signin.go: %v", err)
	}
	for _, want := range []string{"core/module/usuario/types", "i.core.Usuario()", "FetchUsuarioByCorreo"} {
		if !strings.Contains(string(signin), want) {
			t.Errorf("generated signin.go missing %q", want)
		}
	}
}
