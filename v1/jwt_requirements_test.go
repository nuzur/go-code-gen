package gocodegen

import (
	"context"
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
